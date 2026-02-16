package nbpoffline

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Downloader struct {
	dataDir string
	Client  *http.Client
	BaseURL string
}

func NewDownloader(dataDir string) *Downloader {
	return &Downloader{
		dataDir: dataDir,
		Client:  &http.Client{},
		BaseURL: "https://static.nbp.pl/dane/kursy/archiwum/",
	}
}

func (d *Downloader) constructURL(year int) string {
	// URL format: https://static.nbp.pl/dane/kursy/archiwum/archiwum_tab_a_<YEAR>.csv
	return fmt.Sprintf("%sarchiwum_tab_a_%d.csv", d.BaseURL, year)
}

func (d *Downloader) UpdateData(yearStart, yearEnd int) error {
	if err := os.MkdirAll(d.dataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	var errs []error

	for year := yearStart; year <= yearEnd; year++ {
		url := d.constructURL(year)
		resp, err := d.Client.Get(url)
		if err != nil {
			errs = append(errs, fmt.Errorf("downloading %d: %w", year, err))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errs = append(errs, fmt.Errorf("downloading %d failed: status %s", year, resp.Status))
			continue
		}

		// Save to file
		filename := fmt.Sprintf("archiwum_tab_a_%d.csv", year)
		path := filepath.Join(d.dataDir, filename)

		out, err := os.Create(path)
		if err != nil {
			resp.Body.Close()
			errs = append(errs, fmt.Errorf("creating file %s: %w", path, err))
			continue
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()
		resp.Body.Close()

		if err != nil {
			errs = append(errs, fmt.Errorf("writing file %s: %w", path, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during update: %v", len(errs), errs[0])
	}
	return nil
}
