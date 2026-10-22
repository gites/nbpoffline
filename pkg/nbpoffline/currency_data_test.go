package nbpoffline

import (
	"errors"
	"math"
	"testing"
	"testing/fstest"
	"time"
)

func newTestRepo(t *testing.T, data string) *Repository {
	t.Helper()

	repo := NewRepository(fstest.MapFS{
		"archiwum_tab_a_2024.csv": &fstest.MapFile{Data: []byte(data)},
	})
	if err := repo.LoadData(); err != nil {
		t.Fatalf("LoadData() error = %v", err)
	}
	return repo
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestExtractYearFromFilename(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    int
		wantErr bool
	}{
		{name: "valid 2024", give: "archiwum_tab_a_2024.csv", want: 2024},
		{name: "valid 2025", give: "archiwum_tab_a_2025.csv", want: 2025},
		{name: "wrong name", give: "invalid_filename.csv", wantErr: true},
		{name: "year only", give: "2023.csv", wantErr: true},
		{name: "non-numeric year", give: "archiwum_tab_a_abcd.csv", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractYearFromFilename(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetRate(t *testing.T) {
	const data = "data;1THB;1USD;100JPY\n" +
		"20240102;0,1154;3,9455;2,7563\n" +
		"20240105;0,1147;3,9700;2,7000\n"

	tests := []struct {
		name         string
		giveCurrency string
		giveDate     time.Time
		wantRate     float64
		wantDate     time.Time
	}{
		{"exact", "USD", date(2024, 1, 2), 3.9455, date(2024, 1, 2)},
		{"lowercase currency", "usd", date(2024, 1, 2), 3.9455, date(2024, 1, 2)},
		{"scaled", "JPY", date(2024, 1, 2), 0.027563, date(2024, 1, 2)},
		{"fallback returns found date", "USD", date(2024, 1, 4), 3.9455, date(2024, 1, 2)},
		{"time of day ignored", "USD", time.Date(2024, 1, 5, 15, 4, 5, 0, time.UTC), 3.97, date(2024, 1, 5)},
		{"fallback window edge", "USD", date(2024, 1, 15), 3.97, date(2024, 1, 5)},
	}

	repo := newTestRepo(t, data)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, got, err := repo.GetRate(tt.giveCurrency, tt.giveDate)
			if err != nil {
				t.Fatalf("GetRate() error = %v", err)
			}
			if math.Abs(rate-tt.wantRate) > 1e-9 {
				t.Errorf("rate = %v, want %v", rate, tt.wantRate)
			}
			if !got.Equal(tt.wantDate) {
				t.Errorf("date = %v, want %v", got, tt.wantDate)
			}
		})
	}
}

func TestGetRateBefore(t *testing.T) {
	const data = "data;1USD\n" +
		"20240102;3,9455\n" +
		"20240105;3,9700\n"

	tests := []struct {
		name     string
		giveDate time.Time
		wantRate float64
		wantDate time.Time
	}{
		{"skips same day", date(2024, 1, 5), 3.9455, date(2024, 1, 2)},
		{"day after rate", date(2024, 1, 6), 3.97, date(2024, 1, 5)},
		{"after weekend", date(2024, 1, 8), 3.97, date(2024, 1, 5)},
	}

	repo := newTestRepo(t, data)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, got, err := repo.GetRateBefore("USD", tt.giveDate)
			if err != nil {
				t.Fatalf("GetRateBefore() error = %v", err)
			}
			if math.Abs(rate-tt.wantRate) > 1e-9 {
				t.Errorf("rate = %v, want %v", rate, tt.wantRate)
			}
			if !got.Equal(tt.wantDate) {
				t.Errorf("date = %v, want %v", got, tt.wantDate)
			}
		})
	}
}

func TestGetRateErrors(t *testing.T) {
	loaded := newTestRepo(t, "data;1USD\n20240102;3,9455\n")

	tests := []struct {
		name         string
		giveRepo     *Repository
		giveCurrency string
		giveDate     time.Time
		wantErr      error
	}{
		{"no data", NewRepository(fstest.MapFS{}), "USD", date(2024, 1, 2), ErrNoData},
		{"unknown currency", loaded, "XXX", date(2024, 1, 2), ErrCurrencyNotFound},
		{"before data", loaded, "USD", date(2024, 1, 1), ErrRateNotFound},
		{"beyond window", loaded, "USD", date(2024, 1, 13), ErrRateNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.giveRepo.GetRate(tt.giveCurrency, tt.giveDate)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadDataHeaders(t *testing.T) {
	tests := []struct {
		name string
		give string
		want map[string]float64
	}{
		{
			name: "new format",
			give: "data;1THB;100JPY;nr tabeli;pełny numer tabeli;\r\n20240102;0,1154;2,7563;1/A/NBP/2024;x;\r\n",
			want: map[string]float64{"THB": 0.1154, "JPY": 0.027563},
		},
		{
			name: "2012 format with glued names",
			give: "Data;1THBbat (Tajlandia);1USDdolar amerykański;100JPYjen (Japonia);numer tabeli\r\n" +
				"20240102;0,1154;3,9455;2,7563;1/A/NBP/2024\r\n",
			want: map[string]float64{"THB": 0.1154, "USD": 3.9455, "JPY": 0.027563},
		},
		{
			name: "lowercase code skipped",
			give: "data;1usd;1EUR\r\n20240102;3,9;4,3\r\n",
			want: map[string]float64{"EUR": 4.3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTestRepo(t, tt.give)
			if len(repo.rates) != len(tt.want) {
				t.Errorf("loaded %d currencies, want %d", len(repo.rates), len(tt.want))
			}
			for cur, want := range tt.want {
				got, _, err := repo.GetRate(cur, date(2024, 1, 2))
				if err != nil {
					t.Errorf("GetRate(%s) error = %v", cur, err)
					continue
				}
				if math.Abs(got-want) > 1e-9 {
					t.Errorf("GetRate(%s) = %v, want %v", cur, got, want)
				}
			}
		})
	}
}

func TestLoadDataSkipsNonDateRows(t *testing.T) {
	repo := newTestRepo(t, "data;1USD;100JPY\n"+
		"20240102;3,9455;2,7563\n"+
		"liczba jednostek;1;100\n"+
		"kod ISO;USD;JPY\n"+
		"nazwa waluty;dolar;jen\n"+
		"2024010;1;1\n"+
		"abcdefgh;1;1\n")

	for cur, n := range repo.rates {
		if len(n) != 1 {
			t.Errorf("%s has %d dates, want 1", cur, len(n))
		}
	}
	if _, ok := repo.rates["USD"]["20240102"]; !ok {
		t.Error("valid row not stored")
	}
}
