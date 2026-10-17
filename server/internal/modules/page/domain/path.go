package domain

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxPathBytes and MaxPathSegments bound a path an MCP tool names (M9
// design 4.8): ten levels of pages and an attachment.
const (
	MaxPathBytes    = 4096
	MaxPathSegments = MaxDepth + 1
)

// pageSuffix ends a page's path, in any case: an attachment's name never
// does (CheckAssetName).
const pageSuffix = ".md"

// Path is a path from a notebook's root (M9 design 4.8).
type Path struct {
	Written string   // the path as given, which a not found's answer repeats
	Titles  []string // each segment's title, the last a page's title without ".md" or an attachment's name
	Keys    []string // shared.TitleKey of each
	Kind    Kind     // the last segment's: KindPage when it ends with ".md", in any case
	Dir     bool     // list's "A/": the children of the page A; no Titles for the root
}

// ParsePath checks s as field, a path: one leading "/" may go; "./", ".."
// and empty segments are refused; a trailing "/" names a page's children
// when dir allows it (list), and is refused otherwise; each segment, the
// last without its ".md", is a title shared.CheckTitle keeps as it is, in
// NFC. A path that breaks a rule is 422 on field, before any read.
func ParsePath(field, s string, dir bool) (Path, error) {
	if len(s) > MaxPathBytes {
		return Path{}, invalidPath(field, shared.FieldTooLong, fmt.Sprintf("must be at most %d bytes", MaxPathBytes))
	}
	p := Path{Written: s}
	s = strings.TrimPrefix(s, "/")
	switch {
	case dir && s == "":
		p.Kind, p.Dir = KindPage, true
		return p, nil
	case dir && strings.HasSuffix(s, "/"):
		s, p.Dir, p.Kind = strings.TrimSuffix(s, "/"), true, KindPage
	case strings.HasSuffix(s, "/"):
		return Path{}, invalidPath(field, shared.FieldInvalidFormat, "must not end with /: only list takes a path ending with /")
	case s == "":
		return Path{}, invalidPath(field, shared.FieldRequired, "is required")
	}
	segments := strings.Split(s, "/")
	if len(segments) > MaxPathSegments {
		return Path{}, invalidPath(field, shared.FieldTooLong, fmt.Sprintf("must hold at most %d segments", MaxPathSegments))
	}
	if !wellFormed(segments) {
		return Path{}, invalidPath(field, shared.FieldInvalidFormat, "must not hold empty, . or .. segments")
	}
	if !p.Dir {
		last := segments[len(segments)-1]
		p.Kind = KindAsset
		if n := len(last) - len(pageSuffix); n >= 0 && strings.EqualFold(last[n:], pageSuffix) {
			segments[len(segments)-1], p.Kind = last[:n], KindPage
		}
	}
	p.Titles, p.Keys = make([]string, len(segments)), make([]string, len(segments))
	for i, seg := range segments {
		title, f := shared.CheckTitle(field, seg)
		switch {
		case f != nil:
			return Path{}, invalidPath(field, f.Code, fmt.Sprintf("segment %d %s", i+1, f.Message))
		case title != norm.NFC.String(seg):
			// CheckTitle trims the white space at the ends first.
			return Path{}, invalidPath(field, shared.FieldInvalidFormat, fmt.Sprintf("segment %d must not start or end with white space", i+1))
		}
		p.Titles[i], p.Keys[i] = title, shared.TitleKey(title)
	}
	return p, nil
}

// wellFormed reports whether no segment is empty, "." or "..".
func wellFormed(segments []string) bool {
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// invalidPath is 422 on field with code and message.
func invalidPath(field, code, message string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: code, Message: message})
}

// PathOf is the path of node, whose ancestors from the root are ancestors:
// their titles, then its own, with ".md" for a page.
func PathOf(ancestors []Node, node Node) string {
	var b strings.Builder
	for _, a := range ancestors {
		b.WriteString(a.Name)
		b.WriteByte('/')
	}
	b.WriteString(node.Name)
	if node.Kind == KindPage {
		b.WriteString(pageSuffix)
	}
	return b.String()
}

// PathNotFound is page.not_found for a path, with the path as written and
// a hint, which page's adapter/mcp writes out.
type PathNotFound struct {
	Path string
	Hint string
}

func (e *PathNotFound) Error() string { return ErrNotFound.Error() }
func (e *PathNotFound) Unwrap() error { return ErrNotFound }

// PageHint is PathNotFound's hint for an attachment's path that names a
// page, whose path is path.
func PageHint(path string) string {
	return fmt.Sprintf("A page's path ends with .md: %q.", path)
}

// AttachmentHint is PathNotFound's hint for a page's path that names an
// attachment, whose path is path.
func AttachmentHint(path string) string {
	return fmt.Sprintf("That is an attachment: its path is %q.", path)
}
