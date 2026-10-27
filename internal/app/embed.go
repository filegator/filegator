package app

import (
	"embed"
	"io/fs"
)

// frontendFiles holds the embedded frontend. Everything under this package's
// frontend/ directory (HTML, CSS and JavaScript) is compiled into the binary,
// so there is no separate asset deployment step.
//
//go:embed all:frontend
var frontendFiles embed.FS

// frontendRoot returns the filesystem rooted at the embedded frontend
// directory.
func frontendRoot() fs.FS {
	sub, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		// Cannot happen: the directory is embedded at build time.
		panic(err)
	}
	return sub
}
