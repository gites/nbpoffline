# 🏦 NBP Offline

![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8?style=flat&logo=go)
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
# Syntax: nbp [-a] <currency> [date]
nbp USD 2024-01-05
```

**Output:**
```
Rate for USD on 2024-01-05: 3.9650
```

If the date is omitted, it defaults to today.

### Update Data
Download the latest historical data (from 2012 to present) to your local source directory.

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
	"time"
	"nbpoffline/pkg/data"
	"nbpoffline/pkg/nbpoffline"
    "io/fs"
)

func main() {
	// Initialize repository with embedded data
    subFS, _ := fs.Sub(data.FS, "a")
	repo := nbpoffline.NewRepository(subFS)
	
	// Load data
	if err := repo.LoadData(); err != nil {
		panic(err)
	}

	// Get rate
	rate, err := repo.GetRate("EUR", time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("EUR Rate: %.4f\n", rate)
}
```

## 🧪 Testing

The project includes comprehensive unit tests with >95% coverage.

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
ooooo
