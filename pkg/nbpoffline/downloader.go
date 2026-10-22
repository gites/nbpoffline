package nbpoffline

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const _httpTimeout = 30 * time.Second

// _headerPrefix is the expected beginning of every NBP archive CSV file
// (compared case-insensitively).
const _headerPrefix = "data;"

var (
	// ErrNotPublished is returned when the server has no file for the
	// requested year (HTTP 404), e.g. early in a new year.
	ErrNotPublished = errors.New("data not published")

	errInvalidContent = errors.New("invalid content")
)

// Downloader fetches NBP archive CSV files into a local directory.
type Downloader struct {
	dataDir string

	// Client is the HTTP client used for downloads.
	Client *http.Client
	// BaseURL is the archive URL prefix, including the trailing slash.
	BaseURL string
}

// NewDownloader returns a Downloader that stores files in dataDir.
func NewDownloader(dataDir string) *Downloader {
	return &Downloader{
		dataDir: dataDir,
		Client:  &http.Client{Timeout: _httpTimeout},
		BaseURL: "https://static.nbp.pl/dane/kursy/archiwum/",
	}
}

func (d *Downloader) constructURL(year int) string {
	// URL format: https://static.nbp.pl/dane/kursy/archiwum/archiwum_tab_a_<YEAR>.csv
	return fmt.Sprintf("%sarchiwum_tab_a_%d.csv", d.BaseURL, year)
}

// UpdateData downloads the archive files for every year in
// [yearStart, yearEnd]. Files are replaced atomically and only after a
// complete, valid download. Errors for individual years are joined; a missing
// file yields an error matching ErrNotPublished.
func (d *Downloader) UpdateData(yearStart, yearEnd int) error {
	if err := os.MkdirAll(d.dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	var errs []error
	for year := yearStart; year <= yearEnd; year++ {
		if err := d.downloadYear(year); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (d *Downloader) downloadYear(year int) (err error) {
	resp, err := d.Client.Get(d.constructURL(year))
	if err != nil {
		return fmt.Errorf("downloading %d: %w", year, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("downloading %d: %w", year, ErrNotPublished)
	default:
		return fmt.Errorf("downloading %d: status %s", year, resp.Status)
	}

	body := bufio.NewReader(resp.Body)
	head, _ := body.Peek(len(_headerPrefix) + 3) // room for a UTF-8 BOM
	head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	if len(head) < len(_headerPrefix) || !bytes.EqualFold(head[:len(_headerPrefix)], []byte(_headerPrefix)) {
		return fmt.Errorf("downloading %d: %w: missing %q header", year, errInvalidContent, _headerPrefix)
	}

	path := filepath.Join(d.dataDir, fmt.Sprintf("archiwum_tab_a_%d.csv", year))
	tmp, err := os.CreateTemp(d.dataDir, fmt.Sprintf("archiwum_tab_a_%d.*.tmp", year))
	if err != nil {
		return fmt.Errorf("creating temp file for %d: %w", year, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	if _, err = io.Copy(tmp, body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", tmpName, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpName, err)
	}
	if err = os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", tmpName, err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpName, path, err)
	}
	return nil
}
