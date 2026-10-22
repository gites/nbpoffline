package nbpoffline

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const _testPrefix = "/dane/kursy/archiwum/"

func newTestDownloader(t *testing.T, handler http.HandlerFunc) (*Downloader, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	dir := t.TempDir()
	d := NewDownloader(dir)
	d.BaseURL = server.URL + _testPrefix
	return d, dir
}

func yearHandler(responses map[string]func(http.ResponseWriter)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, _testPrefix)
		if fn, ok := responses[name]; ok {
			fn(w)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}
}

func body(s string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { _, _ = w.Write([]byte(s)) }
}

func status(code int) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { w.WriteHeader(code) }
}

func TestUpdateData_Success(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "lowercase header", content: "data;1USD\n20240102;4.0\n"},
		{name: "capitalized header", content: "Data;1USD\n20240102;4.0\n"},
		{name: "uppercase header", content: "DATA;1USD\n20240102;4.0\n"},
		{name: "bom header", content: "\xef\xbb\xbfdata;1USD\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, dir := newTestDownloader(t, yearHandler(map[string]func(http.ResponseWriter){
				"archiwum_tab_a_2024.csv": body(tt.content),
			}))

			if err := d.UpdateData(2024, 2024); err != nil {
				t.Fatalf("UpdateData() error = %v", err)
			}

			got, err := os.ReadFile(filepath.Join(dir, "archiwum_tab_a_2024.csv"))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if string(got) != tt.content {
				t.Errorf("content = %q, want %q", got, tt.content)
			}
			assertNoTempFiles(t, dir)
		})
	}
}

func TestUpdateData_Failures(t *testing.T) {
	const old = "data;old\n"

	tests := []struct {
		name      string
		handler   func(http.ResponseWriter)
		wantErr   error
		wantInMsg string
	}{
		{name: "not found", handler: status(http.StatusNotFound), wantErr: ErrNotPublished},
		{name: "server error", handler: status(http.StatusInternalServerError), wantInMsg: "500"},
		{name: "html with 200", handler: body("<html><body>Error</body></html>"), wantErr: errInvalidContent},
		{name: "empty body", handler: body(""), wantErr: errInvalidContent},
		{name: "short body", handler: body("dat"), wantErr: errInvalidContent},
		{
			name: "truncated body",
			handler: func(w http.ResponseWriter) {
				w.Header().Set("Content-Length", "1000")
				_, _ = w.Write([]byte("data;partial"))
			},
			wantInMsg: "writing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, dir := newTestDownloader(t, yearHandler(map[string]func(http.ResponseWriter){
				"archiwum_tab_a_2024.csv": tt.handler,
			}))
			path := filepath.Join(dir, "archiwum_tab_a_2024.csv")
			if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
				t.Fatal(err)
			}

			err := d.UpdateData(2024, 2024)

			if err == nil {
				t.Fatal("UpdateData() error = nil, want error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error %v does not match %v", err, tt.wantErr)
			}
			if tt.wantInMsg != "" && !strings.Contains(err.Error(), tt.wantInMsg) {
				t.Errorf("error %q does not contain %q", err, tt.wantInMsg)
			}
			if !strings.Contains(err.Error(), "2024") {
				t.Errorf("error %q does not mention the year", err)
			}

			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != old {
				t.Errorf("existing file changed: got %q, want %q", got, old)
			}
			assertNoTempFiles(t, dir)
		})
	}
}

func TestUpdateData_ErrNotPublishedWithOtherYears(t *testing.T) {
	d, dir := newTestDownloader(t, yearHandler(map[string]func(http.ResponseWriter){
		"archiwum_tab_a_2024.csv": body("data;ok\n"),
		"archiwum_tab_a_2025.csv": status(http.StatusNotFound),
	}))

	err := d.UpdateData(2024, 2025)

	if !errors.Is(err, ErrNotPublished) {
		t.Errorf("error %v does not match ErrNotPublished", err)
	}
	assertExists(t, filepath.Join(dir, "archiwum_tab_a_2024.csv"), true)
	assertExists(t, filepath.Join(dir, "archiwum_tab_a_2025.csv"), false)
}

func TestUpdateData_ReportsAllErrors(t *testing.T) {
	d, dir := newTestDownloader(t, yearHandler(map[string]func(http.ResponseWriter){
		"archiwum_tab_a_2022.csv": status(http.StatusInternalServerError),
		"archiwum_tab_a_2023.csv": body("data;ok\n"),
		"archiwum_tab_a_2024.csv": body("<html>"),
		"archiwum_tab_a_2025.csv": status(http.StatusNotFound),
	}))

	err := d.UpdateData(2022, 2025)

	if err == nil {
		t.Fatal("UpdateData() error = nil, want error")
	}
	if !errors.Is(err, ErrNotPublished) {
		t.Errorf("error does not match ErrNotPublished: %v", err)
	}
	if !errors.Is(err, errInvalidContent) {
		t.Errorf("error does not match errInvalidContent: %v", err)
	}
	for _, year := range []string{"2022", "2024", "2025"} {
		if !strings.Contains(err.Error(), "downloading "+year) {
			t.Errorf("error missing year %s: %v", year, err)
		}
	}
	if strings.Contains(err.Error(), "downloading 2023") {
		t.Errorf("error mentions successful year 2023: %v", err)
	}
	assertExists(t, filepath.Join(dir, "archiwum_tab_a_2023.csv"), true)
}

func TestDownloader_URLs(t *testing.T) {
	d := NewDownloader("tmp")
	got := d.constructURL(2024)
	want := "https://static.nbp.pl/dane/kursy/archiwum/archiwum_tab_a_2024.csv"
	if got != want {
		t.Errorf("constructURL(2024) = %s, want %s", got, want)
	}
}

func TestNewDownloader_Timeout(t *testing.T) {
	if got := NewDownloader("tmp").Client.Timeout; got != _httpTimeout {
		t.Errorf("Timeout = %v, want %v", got, _httpTimeout)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) > 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

func assertExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Stat(path)
	if got := err == nil; got != want {
		t.Errorf("exists(%s) = %v, want %v", path, got, want)
	}
}
