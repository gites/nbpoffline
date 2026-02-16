package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"io/fs"
	"nbpoffline/pkg/data"
	"nbpoffline/pkg/nbpoffline"
)

type RateProvider interface {
	GetRate(currency string, date time.Time) (float64, error)
	LoadData() error
}

type DataUpdater interface {
	UpdateData(start, end int) error
}

func main() {
	// Setup data directory
	// ... (path logic)
	wd, _ := os.Getwd()
	dataDir := filepath.Join(wd, "pkg", "data", "a")

	// Dependencies

	subFS, err := fs.Sub(data.FS, "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating subFS: %v\n", err)
		os.Exit(1)
	}

	repo := nbpoffline.NewRepository(subFS)
	downloader := nbpoffline.NewDownloader(dataDir)

	if err := run(os.Args, os.Stdout, repo, downloader); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, repo RateProvider, updater DataUpdater) error {
	cmd, cur, dateStr, err := parseArgs(args)
	if err != nil {
		printUsage()
		return err
	}

	if cmd == "update" {
		fmt.Fprintln(stdout, "Updating data... (Rebuild required to effect embedded data)")
		// Download data from 2012 to current year
		currentYear := time.Now().Year()
		if err := updater.UpdateData(2012, currentYear); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		fmt.Fprintln(stdout, "Update complete.")
		return nil
	}

	if cmd == "query" {
		// Load data
		if err := repo.LoadData(); err != nil {
			return fmt.Errorf("loading data: %w. Try running 'nbp update'", err)
		}

		// Parse date
		targetDate := time.Now()
		if dateStr != "" {
			parsed, err := parseDate(dateStr)
			if err != nil {
				return fmt.Errorf("invalid date format: %w", err)
			}
			targetDate = parsed
		}

		rate, err := repo.GetRate(cur, targetDate)
		if err != nil {
			return fmt.Errorf("getting rate: %w", err)
		}

		fmt.Fprintf(stdout, "Rate for %s on %s: %.4f\n", strings.ToUpper(cur), targetDate.Format("2006-01-02"), rate)
		return nil
	}

	return nil
}

func parseArgs(args []string) (cmd, cur, dateStr string, err error) {
	// args[0] is program name
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	useTableA := fs.Bool("a", true, "use table A")

	// flag package expects flags before args.
	// But user requirement: nbp [-a] <valuta> <data>
	// -a is optional.
	// If inputs are mixed, flag might fail.
	// Let's simple parse manually or use flag if it works.
	// args[1:] passed to fs.Parse

	// However, if args has "update", it might not have flags.
	if len(args) < 2 {
		return "", "", "", fmt.Errorf("not enough arguments")
	}

	// Simple manual check for update
	if args[1] == "update" {
		return "update", "", "", nil
	}
	if len(args) > 2 && args[2] == "update" && args[1] == "-a" {
		return "update", "", "", nil
	}

	if err := fs.Parse(args[1:]); err != nil {
		return "", "", "", err
	}

	// Check defaults
	if !*useTableA {
		// Requirements only support table A for now.
		// Ignore or warn? Prompt says "Na razie nie ma potrzeby implementować ich obsługi".
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return "", "", "", fmt.Errorf("missing currency")
	}

	if remaining[0] == "update" {
		return "update", "", "", nil
	}

	cur = remaining[0]
	if len(remaining) > 1 {
		dateStr = remaining[1]
	}

	return "query", cur, dateStr, nil
}

func parseDate(s string) (time.Time, error) {
	// formats: YYYYMMDD or YYYY-MM-DD
	// Try YYYY-MM-DD
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	// Try YYYYMMDD
	t, err = time.Parse("20060102", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unknown format")
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  nbp [-a] <currency> [date]")
	fmt.Println("  nbp [-a] update")
}
