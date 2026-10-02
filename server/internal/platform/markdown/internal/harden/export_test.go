package harden

import "github.com/yuin/goldmark"

// Hardened is hardened, for the external tests.
func Hardened() goldmark.Markdown { return hardened() }
