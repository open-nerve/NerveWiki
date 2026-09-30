// Package configs embeds the built-in configuration files, so a single
// nervewiki binary runs without any file next to it.
package configs

import (
	"embed"
	"io/fs"
)

// config.local.yaml is personal and never embedded.
//
//go:embed config.yaml config.dev.yaml config.test.yaml config.prod.yaml
var files embed.FS

// FS returns the built-in configuration files.
func FS() fs.FS {
	return files
}
