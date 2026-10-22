package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/gites/nbpoffline/pkg/nbpoffline"
)

type mockRepo struct {
	rate       float64
	found      time.Time
	err        error
	loadErr    error
	gotCur     string
	gotDate    time.Time
	getCalls   int
	gotPrevDay bool
}

func (m *mockRepo) LoadData() error { return m.loadErr }

func (m *mockRepo) GetRate(currency string, date time.Time) (float64, time.Time, error) {
	m.getCalls++
	m.gotCur = currency
	m.gotDate = date
	return m.rate, m.found, m.err
}

func (m *mockRepo) GetRateBefore(currency string, date time.Time) (float64, time.Time, error) {
	m.gotPrevDay = true
	return m.GetRate(currency, date)
}

type yearRange struct{ start, end int }

type mockDownloader struct {
	errs  map[int]error // keyed by start year
	calls []yearRange
}

func (m *mockDownloader) UpdateData(start, end int) error {
	m.calls = append(m.calls, yearRange{start, end})
	return m.errs[start]
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		give     []string
		wantCmd  string
		wantCur  string
		wantDate string
		wantPrev bool
		wantErr  error
		wantAny  bool
	}{
		{name: "currency", give: []string{"nbp", "USD"}, wantCmd: "query", wantCur: "USD"},
		{name: "currency and date", give: []string{"nbp", "USD", "2024-01-01"}, wantCmd: "query", wantCur: "USD", wantDate: "2024-01-01"},
		{name: "update", give: []string{"nbp", "update"}, wantCmd: "update"},
		{name: "flag before currency", give: []string{"nbp", "-a", "EUR"}, wantCmd: "query", wantCur: "EUR"},
		{name: "flag before update", give: []string{"nbp", "-a", "update"}, wantCmd: "update"},
		{name: "flag after update", give: []string{"nbp", "update", "-a"}, wantCmd: "update"},
		{name: "flag after currency", give: []string{"nbp", "USD", "-a"}, wantCmd: "query", wantCur: "USD"},
		{name: "flag after date", give: []string{"nbp", "USD", "2024-01-01", "-a"}, wantCmd: "query", wantCur: "USD", wantDate: "2024-01-01"},
		{name: "prev day", give: []string{"nbp", "-p", "USD", "2024-01-08"}, wantCmd: "query", wantCur: "USD", wantDate: "2024-01-08", wantPrev: true},
		{name: "prev day after args", give: []string{"nbp", "USD", "-p"}, wantCmd: "query", wantCur: "USD", wantPrev: true},
		{name: "short help", give: []string{"nbp", "-h"}, wantErr: flag.ErrHelp},
		{name: "long help", give: []string{"nbp", "--help"}, wantErr: flag.ErrHelp},
		{name: "help after args", give: []string{"nbp", "USD", "-h"}, wantErr: flag.ErrHelp},
		{name: "no args", give: []string{"nbp"}, wantAny: true},
		{name: "only flag", give: []string{"nbp", "-a"}, wantAny: true},
		{name: "unknown flag", give: []string{"nbp", "-x", "USD"}, wantAny: true},
		{name: "too many", give: []string{"nbp", "USD", "2024-01-01", "x"}, wantAny: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.give)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			case tt.wantAny:
				if err == nil {
					t.Fatal("expected error")
				}
				return
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
			want := cliArgs{cmd: tt.wantCmd, cur: tt.wantCur, date: tt.wantDate, prevDay: tt.wantPrev}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    time.Time
		wantErr bool
	}{
		{name: "dashed", give: "2024-01-05", want: day(2024, 1, 5)},
		{name: "compact", give: "20240105", want: day(2024, 1, 5)},
		{name: "garbage", give: "yesterday", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDate(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if got := err.Error(); !bytes.Contains([]byte(got), []byte(tt.give)) {
					t.Errorf("error %q does not mention input %q", got, tt.give)
				}
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunQuery(t *testing.T) {
	fixedNow := time.Date(2024, 3, 10, 23, 30, 0, 0, time.FixedZone("X", 5*3600))

	tests := []struct {
		name        string
		giveArgs    []string
		giveRepo    *mockRepo
		wantOut     string
		wantCur     string
		wantDate    time.Time
		wantPrev    bool
		wantErr     bool
		wantNoCalls bool
	}{
		{
			name:     "exact date",
			giveArgs: []string{"nbp", "USD", "2024-01-05"},
			giveRepo: &mockRepo{rate: 3.965, found: day(2024, 1, 5)},
			wantOut:  "Rate for USD on 2024-01-05: 3.9650\n",
			wantCur:  "USD",
			wantDate: day(2024, 1, 5),
		},
		{
			name:     "fallback",
			giveArgs: []string{"nbp", "usd", "2024-01-07"},
			giveRepo: &mockRepo{rate: 3.965, found: day(2024, 1, 5)},
			wantOut:  "Rate for USD on 2024-01-05 (requested 2024-01-07): 3.9650\n",
			wantCur:  "usd",
			wantDate: day(2024, 1, 7),
		},
		{
			name:     "prev day",
			giveArgs: []string{"nbp", "-p", "USD", "2024-01-08"},
			giveRepo: &mockRepo{rate: 3.985, found: day(2024, 1, 5)},
			wantOut:  "Rate for USD on 2024-01-05 (D-1 for 2024-01-08): 3.9850\n",
			wantCur:  "USD",
			wantDate: day(2024, 1, 8),
			wantPrev: true,
		},
		{
			name:     "default date is today",
			giveArgs: []string{"nbp", "USD"},
			giveRepo: &mockRepo{rate: 4, found: day(2024, 3, 10)},
			wantOut:  "Rate for USD on 2024-03-10: 4.0000\n",
			wantCur:  "USD",
			wantDate: day(2024, 3, 10),
		},
		{
			name:     "rate error",
			giveArgs: []string{"nbp", "XYZ"},
			giveRepo: &mockRepo{err: errors.New("not found")},
			wantErr:  true,
			wantCur:  "XYZ",
			wantDate: day(2024, 3, 10),
		},
		{
			name:        "load error",
			giveArgs:    []string{"nbp", "USD"},
			giveRepo:    &mockRepo{loadErr: errors.New("boom")},
			wantErr:     true,
			wantNoCalls: true,
		},
		{
			name:        "bad date",
			giveArgs:    []string{"nbp", "USD", "nope"},
			giveRepo:    &mockRepo{},
			wantErr:     true,
			wantNoCalls: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(tt.giveArgs, &stdout, &stderr, tt.giveRepo, &mockDownloader{}, func() time.Time { return fixedNow })
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantNoCalls {
				if tt.giveRepo.getCalls != 0 {
					t.Errorf("GetRate called %d times, want 0", tt.giveRepo.getCalls)
				}
				return
			}
			if tt.giveRepo.gotCur != tt.wantCur {
				t.Errorf("currency = %q, want %q", tt.giveRepo.gotCur, tt.wantCur)
			}
			if tt.giveRepo.gotPrevDay != tt.wantPrev {
				t.Errorf("GetRateBefore used = %v, want %v", tt.giveRepo.gotPrevDay, tt.wantPrev)
			}
			if !tt.giveRepo.gotDate.Equal(tt.wantDate) {
				t.Errorf("date = %v, want %v", tt.giveRepo.gotDate, tt.wantDate)
			}
			if stdout.String() != tt.wantOut {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantOut)
			}
		})
	}
}

func TestRunUpdate(t *testing.T) {
	notPublished := fmt.Errorf("downloading: %w", nbpoffline.ErrNotPublished)
	other := errors.New("network down")

	tests := []struct {
		name      string
		giveNow   time.Time
		giveErrs  map[int]error
		wantCalls []yearRange
		wantErr   bool
		wantNote  bool
	}{
		{
			name:      "mid year",
			giveNow:   day(2025, 6, 15),
			wantCalls: []yearRange{{2025, 2025}},
		},
		{
			name:      "january includes previous year",
			giveNow:   day(2025, 1, 2),
			wantCalls: []yearRange{{2024, 2024}, {2025, 2025}},
		},
		{
			name:      "current year not published tolerated",
			giveNow:   day(2025, 1, 2),
			giveErrs:  map[int]error{2025: notPublished},
			wantCalls: []yearRange{{2024, 2024}, {2025, 2025}},
			wantNote:  true,
		},
		{
			name:      "previous year not published fails",
			giveNow:   day(2025, 1, 2),
			giveErrs:  map[int]error{2024: notPublished},
			wantCalls: []yearRange{{2024, 2024}},
			wantErr:   true,
		},
		{
			name:      "other error on current year fails",
			giveNow:   day(2025, 6, 15),
			giveErrs:  map[int]error{2025: other},
			wantCalls: []yearRange{{2025, 2025}},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			dl := &mockDownloader{errs: tt.giveErrs}
			err := run([]string{"nbp", "update"}, &stdout, &stderr, &mockRepo{}, dl, func() time.Time { return tt.giveNow })
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if fmt.Sprint(dl.calls) != fmt.Sprint(tt.wantCalls) {
				t.Errorf("calls = %v, want %v", dl.calls, tt.wantCalls)
			}
			if got := bytes.Contains(stdout.Bytes(), []byte("not published")); got != tt.wantNote {
				t.Errorf("note printed = %v, want %v (stdout %q)", got, tt.wantNote, stdout.String())
			}
		})
	}
}

func TestRunHelpAndUsage(t *testing.T) {
	tests := []struct {
		name       string
		give       []string
		wantErr    bool
		wantStdout bool
		wantStderr bool
	}{
		{name: "help", give: []string{"nbp", "-h"}, wantStdout: true},
		{name: "bad args", give: []string{"nbp"}, wantErr: true, wantStderr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(tt.give, &stdout, &stderr, &mockRepo{}, &mockDownloader{}, time.Now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got := stdout.String() == usageText; got != tt.wantStdout {
				t.Errorf("usage on stdout = %v, want %v", got, tt.wantStdout)
			}
			if got := stderr.String() == usageText; got != tt.wantStderr {
				t.Errorf("usage on stderr = %v, want %v", got, tt.wantStderr)
			}
		})
	}
}
