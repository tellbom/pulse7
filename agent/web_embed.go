package main

import (
	"embed"
	"io/fs"
)

//go:embed web
var embeddedWeb embed.FS

func webFS() fs.FS { return embeddedWeb }
