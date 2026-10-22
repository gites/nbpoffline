# 🏦 NBP Offline

![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?style=flat&logo=go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**NBP Offline** is a robust Go module and CLI tool designed to provide offline access to historical currency exchange rates from the National Bank of Poland (NBP). It parses local CSV archives to deliver fast, reliable currency data without needing a constant internet connection for every query.

## 🚀 About

This project was built to solve the need for reliable, offline-first currency conversion in financial applications. It automatically handles data synchronization with NBP archives and provides a simple interface for querying rates.

**Key Features:**
- 📅 **Offline Access**: Query rates from local CSV files.
- 🔄 **Smart Fallback**: Automatically finds the last known rate if data for a specific date is missing (e.g., weekends/holidays).
- 📦 **Easy Updates**: `nbp update` command fetches the latest data from NBP servers.
- 🛠️ **Dual Mode**: Use as a CLI tool or import as a Go library.

## 📦 Installation

```bash
git clone https://github.com/gites/nbpoffline.git
cd nbpoffline
go install ./cmd/nbp
```

## 💻 CLI Usage

The `nbp` command-line tool allows you to quickly check rates or update data.

### Query a Rate
Get the exchange rate for a currency on a specific date.

```bash
# Syntax: nbp [-a] [-p] <currency> [date]
nbp USD 2024-01-05
```

**Output:**
```
Rate for USD on 2024-01-05: 3.9850
```

If no rate was published for the requested date (weekend or holiday), the most recent earlier rate is used and its date is shown:

```
Rate for USD on 2024-01-05 (requested 2024-01-07): 3.9850
```

### D-1 Rate (tax)
Polish tax rules (PIT/CIT) require the NBP rate from the last business day **before** the transaction date. Use `-p`:

```bash
nbp -p USD 2024-01-08
```

```
Rate for USD on 2024-01-05 (D-1 for 2024-01-08): 3.9850
```

In the library use `repo.GetRateBefore(currency, date)`.

If the date is omitted, it defaults to today. Run `nbp -h` for all options.

### Update Data
Download the current year's data (and the previous year during January) into `pkg/data/a`. Run it from the repository root. A current-year file that NBP has not published yet is tolerated.

> [!IMPORTANT]
> Since data is embedded into the binary at compile time, you must **rebuild** the application after updating data for changes to take effect.

```bash
nbp update
go install ./cmd/nbp
```

## 📚 Library Usage

You can easily integrate `nbpoffline` into your own Go projects.

```go
package main

import (
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/gites/nbpoffline/pkg/data"
	"github.com/gites/nbpoffline/pkg/nbpoffline"
)

func main() {
	// Initialize repository with embedded data
	subFS, err := fs.Sub(data.FS, "a")
	if err != nil {
		log.Fatal(err)
	}
	repo := nbpoffline.NewRepository(subFS)

	// Load data
	if err := repo.LoadData(); err != nil {
		log.Fatal(err)
	}

	// Get rate; rateDate is the date of the rate actually used
	// (differs from the requested date on weekends/holidays).
	rate, rateDate, err := repo.GetRate("EUR", time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("EUR Rate: %.4f (as of %s)\n", rate, rateDate.Format("2006-01-02"))
}
```

## 🧪 Testing

The project includes unit tests.

```bash
go test -v ./...
```

## 📂 Data Format supports

The module parses NBP's `archiwum_tab_a_<YEAR>.csv` files. It handles:
- Semicolon delimiters
- Comma decimal separators
- Currency scaling (e.g., `100JPY` -> `1JPY`)

---
*Built with ❤️ in Golang*
