package nbpoffline

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestExtractDateFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		want     int
		wantErr  bool
	}{
		{"archiwum_tab_a_2024.csv", 2024, false},
		{"archiwum_tab_a_2025.csv", 2025, false}, // Assuming some flexibility or strictness based on user input
		{"invalid_filename.csv", 0, true},
		{"2023.csv", 0, true},
	}

	for _, tt := range tests {
		got, err := extractYearFromFilename(tt.filename)
		if (err != nil) != tt.wantErr {
			t.Errorf("extractYearFromFilename(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("extractYearFromFilename(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestLoadData(t *testing.T) {
	// Create a mock FS
	mockFS := fstest.MapFS{
		"archiwum_tab_a_2024.csv": &fstest.MapFile{
			Data: []byte(`data;1THB;1USD;1AUD
20240102;0,1154;3,9455;2,6934
20240103;0,1147;3,9850;2,6845
`),
		},
	}

	repo := NewRepository(mockFS)
	if err := repo.LoadData(); err != nil {
		t.Fatalf("LoadData() failed: %v", err)
	}

	// Verify data loaded
	rate, err := repo.GetRate("USD", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Errorf("GetRate(USD, 2024-01-02) failed: %v", err)
	}
	if rate != 3.9455 {
		t.Errorf("GetRate(USD, 2024-01-02) = %f, want 3.9455", rate)
	}

	// Test case insensitivity for currency
	rate, err = repo.GetRate("usd", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Errorf("GetRate(usd, 2024-01-02) failed: %v", err)
	}
	if rate != 3.9455 {
		t.Errorf("GetRate(usd, 2024-01-02) = %f, want 3.9455", rate)
	}
}

func TestGetRate_Fallback(t *testing.T) {
	// Setup data: 2024-01-02 (Tuesday), 2024-01-03 (Wednesday)
	mockFS := fstest.MapFS{
		"archiwum_tab_a_2024.csv": &fstest.MapFile{
			Data: []byte(`data;1USD
20240102;3,9455
20240105;3,9700
`),
		},
	}

	repo := NewRepository(mockFS)
	if err := repo.LoadData(); err != nil {
		t.Fatalf("LoadData() failed: %v", err)
	}

	// Query for 2024-01-03 (Wed). Should get 2024-01-02.
	targetDate := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	rate, err := repo.GetRate("USD", targetDate)
	if err != nil {
		t.Errorf("GetRate(USD, %v) failed: %v", targetDate, err)
	}
	if rate != 3.9455 {
		t.Errorf("GetRate(USD, %v) = %f, want 3.9455 (from 2024-01-02)", targetDate, rate)
	}
}

func TestGetRate_NotFound(t *testing.T) {
	mockFS := fstest.MapFS{}
	repo := NewRepository(mockFS)
	// No data loaded

	_, err := repo.GetRate("USD", time.Now())
	if err == nil {
		t.Error("GetRate should fail when no data is loaded")
	}
}

func TestScaleParsing(t *testing.T) {
	mockFS := fstest.MapFS{
		"archiwum_tab_a_2024.csv": &fstest.MapFile{
			Data: []byte(`data;1THB;100JPY
20240102;0,1154;2,7563
`),
		},
	}

	repo := NewRepository(mockFS)
	if err := repo.LoadData(); err != nil {
		t.Fatalf("LoadData() failed: %v", err)
	}

	// 100JPY = 2.7563 -> 1JPY = 0.027563
	rate, err := repo.GetRate("JPY", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Errorf("GetRate(JPY) failed: %v", err)
	}
	expected := 2.7563 / 100.0
	// Floating point comparison
	if mismatch(rate, expected) {
		t.Errorf("GetRate(JPY) = %f, want %f", rate, expected)
	}
}

func mismatch(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.000001
}
