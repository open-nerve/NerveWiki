// Package webui serves the web frontend embedded in the nervewiki binary: the
// static files of the single-page app, and its index.html for every page
// path.
package webui

import (
	"embed"
	"io/fs"
)

// dist holds the frontend that make build copies from web/apps/web/dist.
// Only dist/.gitkeep is committed; "all:" embeds it, so the pattern still
// matches while the frontend is not built.
//
//go:embed all:dist
var dist embed.FS

// FS returns the embedded frontend, with index.html at its root once built.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // unreachable: "dist" is a valid path embedded above
	}
	return sub
}
