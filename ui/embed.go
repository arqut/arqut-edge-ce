package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist/spa
var assets embed.FS

// FS contains the web ui assets.
var FS, _ = fs.Sub(assets, "dist/spa")
