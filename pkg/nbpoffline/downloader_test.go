package nbpoffline

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateData(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect URL: /dane/kursy/archiwum/archiwum_tab_a_<YEAR>.csv
		// Check year
		if r.URL.Path == "/dane/kursy/archiwum/archiwum_tab_a_2024.csv" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("mock data 2024"))
			return
		}
		if r.URL.Path == "/dane/kursy/archiwum/archiwum_tab_a_2025.csv" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// Inject the mock URL base into the downloader (we need to make it configurable)
	// or we pass it as argument. Since it's a specific requirement to download from static.nbp.pl,
	// maybe we can verify the URL construction logic or use a variable for base URL.
	// Let's use a variable for base URL in the package, which we can override in tests (but it needs to be exported or loop-hole).
	// Better: pass baseURL to UpdateData or have a struct "Downloader".

	// Let's assume we implement a Downloader struct.
	d := NewDownloader(tmpDir)
	d.BaseURL = server.URL + "/dane/kursy/archiwum/"

	// Test case 1: Success
	err := d.UpdateData(2024, 2024)
	if err != nil {
		t.Errorf("UpdateData(2024, 2024) failed: %v", err)
	}

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "archiwum_tab_a_2024.csv")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Errorf("File not created: %v", err)
	}
	if string(content) != "mock data 2024" {
		t.Errorf("File content mismatch: got %s, want mock data 2024", string(content))
	}

	// Test case 2: Failure (404)
	// We want to verify it returns error but continues for other years?
	// Or returns error? The prompt says "pobranie najnowszych danych".
	// If one fails, maybe warn?
	err = d.UpdateData(2025, 2025)
	if err == nil {
		t.Errorf("UpdateData(2025, 2025) should fail for 404")
	}
}

func TestDownloader_URLs(t *testing.T) {
	// Verify URL construction logic without network
	d := NewDownloader("tmp")
	url := d.constructURL(2024)
	expected := "https://static.nbp.pl/dane/kursy/archiwum/archiwum_tab_a_2024.csv"
	if url != expected {
		t.Errorf("constructURL(2024) = %s, want %s", url, expected)
	}
}
