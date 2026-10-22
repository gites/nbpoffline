package nbpoffline

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

const (
	_dateLayout     = "20060102"
	_fallbackDays   = 10
	_filenamePrefix = "archiwum_tab_a_"
	_filenameSuffix = ".csv"
	_currencyLen    = 3
)

var (
	// ErrNoData is returned when no rates have been loaded.
	ErrNoData = errors.New("no data loaded")
	// ErrCurrencyNotFound is returned when the currency is unknown.
	ErrCurrencyNotFound = errors.New("currency not found")
	// ErrRateNotFound is returned when no rate exists within the fallback window.
	ErrRateNotFound = errors.New("rate not found")
)

// Repository holds NBP table A exchange rates loaded from CSV archives.
type Repository struct {
	dataFS fs.FS
	// rates maps currency code -> date (YYYYMMDD) -> rate for one unit.
	rates map[string]map[string]float64
}

// NewRepository returns a Repository reading archives from dataFS.
func NewRepository(dataFS fs.FS) *Repository {
	return &Repository{
		dataFS: dataFS,
		rates:  make(map[string]map[string]float64),
	}
}

func extractYearFromFilename(filename string) (int, error) {
	if !strings.HasPrefix(filename, _filenamePrefix) || !strings.HasSuffix(filename, _filenameSuffix) {
		return 0, fmt.Errorf("invalid filename format")
	}

	yearStr := strings.TrimSuffix(strings.TrimPrefix(filename, _filenamePrefix), _filenameSuffix)
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return 0, fmt.Errorf("invalid year: %w", err)
	}

	return year, nil
}

// LoadData reads all table A archives found in the repository's file system.
func (r *Repository) LoadData() error {
	entries, err := fs.ReadDir(r.dataFS, ".")
	if err != nil {
		return fmt.Errorf("scanning data dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, err := extractYearFromFilename(entry.Name()); err != nil {
			continue
		}
		if err := r.loadFile(entry.Name()); err != nil {
			return fmt.Errorf("loading file %s: %w", entry.Name(), err)
		}
	}
	return nil
}

type colInfo struct {
	currency string
	scale    float64
}

// parseHeader parses a column header such as "1THB", "100JPY" or
// "1USDdolar amerykański" into its currency code and scale.
func parseHeader(h string) (colInfo, bool) {
	h = strings.TrimSpace(h)
	i := 0
	for i < len(h) && h[i] >= '0' && h[i] <= '9' {
		i++
	}
	if i == 0 || len(h) < i+_currencyLen {
		return colInfo{}, false
	}
	code := h[i : i+_currencyLen]
	for j := 0; j < len(code); j++ {
		if code[j] < 'A' || code[j] > 'Z' {
			return colInfo{}, false
		}
	}
	scale, err := strconv.ParseFloat(h[:i], 64)
	if err != nil || scale == 0 {
		return colInfo{}, false
	}
	return colInfo{currency: code, scale: scale}, true
}

func (r *Repository) loadFile(path string) error {
	f, err := r.dataFS.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}
	headers := strings.Split(scanner.Text(), ";")
	if len(headers) < 2 {
		return fmt.Errorf("invalid header")
	}

	cols := make(map[int]colInfo)
	for i, h := range headers[1:] {
		if info, ok := parseHeader(h); ok {
			cols[i+1] = info
		}
	}

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ";")
		dateStr := strings.TrimSpace(parts[0])
		if len(dateStr) != len(_dateLayout) {
			continue
		}
		if _, err := time.Parse(_dateLayout, dateStr); err != nil {
			continue
		}

		for i, part := range parts[1:] {
			info, ok := cols[i+1]
			if !ok {
				continue
			}
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			val, err := strconv.ParseFloat(strings.Replace(part, ",", ".", 1), 64)
			if err != nil {
				continue
			}

			if r.rates[info.currency] == nil {
				r.rates[info.currency] = make(map[string]float64)
			}
			r.rates[info.currency][dateStr] = val / info.scale
		}
	}

	return scanner.Err()
}

// GetRate returns the PLN rate of one unit of currency on date, along with the
// date of the rate actually found. If date has no rate, the closest earlier
// day within the fallback window is used. The returned date is at midnight UTC.
func (r *Repository) GetRate(currency string, date time.Time) (float64, time.Time, error) {
	currency = strings.ToUpper(currency)
	if len(r.rates) == 0 {
		return 0, time.Time{}, ErrNoData
	}

	rateMap, ok := r.rates[currency]
	if !ok {
		return 0, time.Time{}, fmt.Errorf("%w: %s", ErrCurrencyNotFound, currency)
	}

	y, m, d := date.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	for i := 0; i <= _fallbackDays; i++ {
		found := day.AddDate(0, 0, -i)
		if rate, ok := rateMap[found.Format(_dateLayout)]; ok {
			return rate, found, nil
		}
	}

	return 0, time.Time{}, fmt.Errorf("%w: %s near %s", ErrRateNotFound, currency, day.Format(_dateLayout))
}

// GetRateBefore returns the rate from the last day with a published rate
// strictly before date (the "D-1" rule used e.g. for Polish income tax),
// along with the date of that rate.
func (r *Repository) GetRateBefore(currency string, date time.Time) (float64, time.Time, error) {
	return r.GetRate(currency, date.AddDate(0, 0, -1))
}
