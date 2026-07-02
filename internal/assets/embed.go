// Package assets embeds the built frontend into the binary.
package assets

import (
	"embed"
	"io/fs"
)

// distFS holds the built Vue frontend (produced by `make frontend`). The
// `all:` prefix includes files like .gitkeep so this compiles before a build.
//
//go:embed all:dist
var distFS embed.FS

// DistFS returns the frontend filesystem rooted at the dist directory. HasDist
// reports whether a real build is present (an index.html exists).
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// HasDist reports whether a built frontend is embedded.
func HasDist() bool {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return false
	}
	return true
}
