package data

import (
	"embed"
)

//go:embed a/*.csv
var FS embed.FS
