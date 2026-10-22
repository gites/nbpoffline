package data

import (
	"embed"
)

// FS holds the embedded NBP archive CSV files (archiwum_tab_a_<YEAR>.csv)
// under the "a" directory. Use fs.Sub(FS, "a") to get the directory root.
//
//go:embed a/archiwum_tab_a_*.csv
var FS embed.FS
