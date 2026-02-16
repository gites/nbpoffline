package nbpoffline

import (
	"bufio"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

type Repository struct {
	dataFS fs.FS
	// Internal storage for rates.
	// Map: Currency Code -> Date (YYYYMMDD) -> Value
	// We use YYYYMMDD string for simple key.
	rates map[string]map[string]float64
}

func NewRepository(dataFS fs.FS) *Repository {
	return &Repository{
		dataFS: dataFS,
		rates:  make(map[string]map[string]float64),
	}
}

func extractYearFromFilename(filename string) (int, error) {
	// Format: archiwum_tab_a_2024.csv
	// Expected length: len("archiwum_tab_a_") + 4 + len(".csv") = 15 + 4 + 4 = 23
	prefix := "archiwum_tab_a_"
	suffix := ".csv"

	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
		return 0, fmt.Errorf("invalid filename format")
	}

	yearStr := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), suffix)
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return 0, fmt.Errorf("invalid year: %w", err)
	}

	return year, nil
}

func (r *Repository) LoadData() error {
	entries, err := fs.ReadDir(r.dataFS, ".")
	if err != nil {
		return fmt.Errorf("scanning data dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, err := extractYearFromFilename(entry.Name()); err == nil {
			if err := r.loadFile(entry.Name()); err != nil {
				return fmt.Errorf("loading file %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func (r *Repository) loadFile(path string) error {
	f, err := r.dataFS.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// Read header
	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}
	headerLine := scanner.Text()
	headers := strings.Split(headerLine, ";")
	if len(headers) < 2 {
		return fmt.Errorf("invalid header")
	}

	// Parse headers to map column index to (Currency, Scale)
	type colInfo struct {
		currency string
		scale    float64
	}
	cols := make(map[int]colInfo)

	for i, h := range headers {
		if i == 0 {
			continue // Date column
		}
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}

		// Parse scale and currency, e.g. "1THB", "100JPY"
		// Find first non-digit
		firstLetterIdx := -1
		for j, char := range h {
			if char < '0' || char > '9' {
				firstLetterIdx = j
				break
			}
		}

		if firstLetterIdx <= 0 {
			// No digit at start or no letters? content like "nr tabeli"
			continue
		}

		scaleStr := h[:firstLetterIdx]
		currency := h[firstLetterIdx:]
		scale, err := strconv.ParseFloat(scaleStr, 64)
		if err != nil {
			continue
		}

		cols[i] = colInfo{
			currency: currency,
			scale:    scale,
		}
	}

	// Read data
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ";")
		if len(parts) < 2 {
			continue
		}

		dateStr := strings.TrimSpace(parts[0])
		// data format YYYYMMDD

		for i, part := range parts {
			if i == 0 {
				continue
			}
			info, ok := cols[i]
			if !ok {
				continue
			}

			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			// Replace , with .
			valStr := strings.Replace(part, ",", ".", 1)
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				continue
			}

			// Normalize value
			rate := val / info.scale

			// Store
			if _, ok := r.rates[info.currency]; !ok {
				r.rates[info.currency] = make(map[string]float64)
			}
			r.rates[info.currency][dateStr] = rate
		}
	}

	return scanner.Err()
}

func (r *Repository) GetRate(currency string, date time.Time) (float64, error) {
	currency = strings.ToUpper(currency)
	if len(r.rates) == 0 {
		return 0, fmt.Errorf("no data loaded")
	}

	rateMap, ok := r.rates[currency]
	if !ok {
		return 0, fmt.Errorf("currency not found: %s", currency)
	}

	// Try exact match
	key := date.Format("20060102")
	if rate, ok := rateMap[key]; ok {
		return rate, nil
	}

	// Try previous days (limit 10 days)
	for i := 1; i <= 10; i++ {
		prevDate := date.AddDate(0, 0, -i)
		key := prevDate.Format("20060102")
		if rate, ok := rateMap[key]; ok {
			return rate, nil
		}
	}

	return 0, fmt.Errorf("rate not found for %s near %s", currency, key)
}
