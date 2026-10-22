package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gites/nbpoffline/pkg/data"
	"github.com/gites/nbpoffline/pkg/nbpoffline"
)

const (
	dateLayout        = "2006-01-02"
	compactDateLayout = "20060102"

	cmdQuery  = "query"
	cmdUpdate = "update"

	usageText = `Usage:
  nbp [-a] [-p] <currency> [date]
  nbp [-a] update

  date     YYYY-MM-DD or YYYYMMDD (default: today)
  -a       use table A (the only supported table)
  -p       use the rate from the last business day before date (D-1)
  -h       show this help

The update command downloads current data into pkg/data/a and must be run
from the repository root. Rebuild afterwards to embed the new data.
`
)

// _dataDir is relative to the repository root.
var _dataDir = filepath.Join("pkg", "data", "a")

// RateProvider looks up exchange rates.
type RateProvider interface {
	GetRate(currency string, date time.Time) (float64, time.Time, error)
	GetRateBefore(currency string, date time.Time) (float64, time.Time, error)
	LoadData() error
}

// DataUpdater downloads exchange rate data for a range of years.
type DataUpdater interface {
	UpdateData(start, end int) error
}

func main() {
	subFS, err := fs.Sub(data.FS, "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating subFS: %v\n", err)
		os.Exit(1)
	}

	repo := nbpoffline.NewRepository(subFS)
	downloader := nbpoffline.NewDownloader(_dataDir)

	if err := run(os.Args, os.Stdout, os.Stderr, repo, downloader, time.Now); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(
	args []string,
	stdout, stderr io.Writer,
	repo RateProvider,
	updater DataUpdater,
	now func() time.Time,
) error {
	opts, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(stdout, usageText)
		return nil
	}
	if err != nil {
		fmt.Fprint(stderr, usageText)
		return err
	}

	if opts.cmd == cmdUpdate {
		return runUpdate(stdout, updater, now())
	}
	return runQuery(stdout, repo, opts, now())
}

func runUpdate(stdout io.Writer, updater DataUpdater, now time.Time) error {
	fmt.Fprintln(stdout, "Updating data... (rebuild required for embedded data to take effect)")

	year := now.Year()
	if now.Month() == time.January {
		if err := updater.UpdateData(year-1, year-1); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	}

	err := updater.UpdateData(year, year)
	if errors.Is(err, nbpoffline.ErrNotPublished) {
		fmt.Fprintf(stdout, "Note: data for %d is not published yet, skipping.\n", year)
	} else if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Fprintln(stdout, "Update complete.")
	return nil
}

func runQuery(stdout io.Writer, repo RateProvider, opts cliArgs, now time.Time) error {
	if err := repo.LoadData(); err != nil {
		return fmt.Errorf("loading data: %w. Try running 'nbp update'", err)
	}

	target := now
	if opts.date != "" {
		parsed, err := parseDate(opts.date)
		if err != nil {
			return err
		}
		target = parsed
	}
	// Compare by calendar day regardless of time zone.
	target = time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)

	getRate := repo.GetRate
	if opts.prevDay {
		getRate = repo.GetRateBefore
	}
	rate, found, err := getRate(opts.cur, target)
	if err != nil {
		return fmt.Errorf("getting rate: %w", err)
	}

	label := strings.ToUpper(opts.cur)
	switch {
	case opts.prevDay:
		fmt.Fprintf(stdout, "Rate for %s on %s (D-1 for %s): %.4f\n",
			label, found.Format(dateLayout), target.Format(dateLayout), rate)
	case found.Format(dateLayout) == target.Format(dateLayout):
		fmt.Fprintf(stdout, "Rate for %s on %s: %.4f\n", label, found.Format(dateLayout), rate)
	default:
		fmt.Fprintf(stdout, "Rate for %s on %s (requested %s): %.4f\n",
			label, found.Format(dateLayout), target.Format(dateLayout), rate)
	}
	return nil
}

// cliArgs holds parsed command-line arguments.
type cliArgs struct {
	cmd     string
	cur     string
	date    string
	prevDay bool // -p: use the D-1 rate
}

// parseArgs parses os.Args-style arguments. Flags are accepted anywhere.
// It returns flag.ErrHelp when help was requested.
func parseArgs(args []string) (cliArgs, error) {
	var (
		opts       cliArgs
		positional []string
	)
	for _, arg := range args[1:] {
		switch {
		case arg == "-a":
			// Only table A is supported; the flag is accepted for compatibility.
		case arg == "-p":
			opts.prevDay = true
		case arg == "-h" || arg == "-help" || arg == "--help":
			return cliArgs{}, flag.ErrHelp
		case strings.HasPrefix(arg, "-"):
			return cliArgs{}, fmt.Errorf("unknown flag %q", arg)
		default:
			positional = append(positional, arg)
		}
	}

	switch {
	case len(positional) == 0:
		return cliArgs{}, errors.New("not enough arguments")
	case positional[0] == cmdUpdate && len(positional) == 1:
		return cliArgs{cmd: cmdUpdate}, nil
	case len(positional) > 2:
		return cliArgs{}, fmt.Errorf("too many arguments: %q", positional)
	}

	opts.cmd = cmdQuery
	opts.cur = positional[0]
	if len(positional) > 1 {
		opts.date = positional[1]
	}
	return opts, nil
}

// parseDate accepts YYYY-MM-DD or YYYYMMDD.
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{dateLayout, compactDateLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD or YYYYMMDD", s)
}
