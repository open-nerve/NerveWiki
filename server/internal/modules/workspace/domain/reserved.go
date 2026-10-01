package domain

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"
)

// reservedFile is the one list of reserved slugs (M2/P1 design 3.6): the
// web app's tests and the server's read it too.
//
//go:embed reserved_slugs.txt
var reservedFile string

// ReservedSlugs are the names no workspace may take, by why.
type ReservedSlugs struct {
	App      []string // the web app's top-level route segments, and public/'s top-level directories
	Server   []string // the top-level paths the server answers itself, beside the web app's pages
	Reserved []string // names held for top-level paths to come
}

// All is every reserved name.
func (r ReservedSlugs) All() []string {
	return slices.Concat(r.App, r.Server, r.Reserved)
}

// Reserved returns the list. It parses the embedded file on each call: a
// few dozen lines, and no package state to keep.
func Reserved() ReservedSlugs {
	r, err := parseReserved(reservedFile)
	if err != nil {
		panic("workspace: reserved_slugs.txt: " + err.Error()) // the domain's tests parse it
	}
	return r
}

// parseReserved reads the list: sections [app], [server] and [reserved],
// one slug per line, blank lines and # comments skipped. A name spelled
// unlike a slug, outside a section, or listed twice is an error.
func parseReserved(text string) (ReservedSlugs, error) {
	var r ReservedSlugs
	var section *[]string
	seen := map[string]bool{}
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "[app]":
			section = &r.App
		case line == "[server]":
			section = &r.Server
		case line == "[reserved]":
			section = &r.Reserved
		case strings.HasPrefix(line, "["):
			return ReservedSlugs{}, fmt.Errorf("line %d: %q is not [app], [server] or [reserved]", i+1, line)
		case section == nil:
			return ReservedSlugs{}, fmt.Errorf("line %d: %q is in no section", i+1, line)
		case !slugPattern(line):
			return ReservedSlugs{}, fmt.Errorf("line %d: %q is not spelled as a slug", i+1, line)
		case seen[line]:
			return ReservedSlugs{}, fmt.Errorf("line %d: %q is listed twice", i+1, line)
		default:
			seen[line] = true
			*section = append(*section, line)
		}
	}
	return r, nil
}
