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
	names := make([]string, 0, len(ancestors)+1)
	for _, a := range ancestors {
		names = append(names, a.Name)
	}
	return PathOfNames(append(names, node.Name), node.Kind)
}

// PathOfNames is the path of a node of kind whose names, from its root's
// to its own, are names: the one place a path is written (M9/P2 design
// 3.7).
func PathOfNames(names []string, kind Kind) string {
	p := strings.Join(names, "/")
	if kind == KindPage {
		p += pageSuffix
	}
	return p
}

// Quoted is path in double quotes as it is, as an answer quotes it (M9/P2
// design 3.10.4): no title holds " or \ (shared.CheckTitle), so what the
// quotes hold is the path, byte for byte, which %q would write with
// escapes for the characters a title may hold, U+3000 or U+00AD.
func Quoted(path string) string { return `"` + path + `"` }

// PagePath is 422 on a's path when p is an attachment's: tool, write or
// edit, writes pages, whose paths end with .md (M9/P2 design 3.8.1,
// 3.8.2); attachments are uploaded in the web.
func PagePath(a Args, p Path, tool string) error {
	if p.Kind != KindPage {
		return NotAllowed(a.Field("path"),
			"is an attachment's path: "+tool+" "+tool+"s pages, whose paths end with .md; attachments are uploaded in the web")
	}
	return nil
}

// PathNotFound is page.not_found for a path, with the path as written and
// a hint, which page's adapter/mcp writes out. ParentOf names the field
// whose parent Path is, when Path is no path the call gave but its
// parent's: move's "to", a batch's "ops[i].to" (M9/P2 design 3.11.5).
type PathNotFound struct {
	Path     string
	Hint     string
	ParentOf string
}

func (e *PathNotFound) Error() string { return ErrNotFound.Error() }
func (e *PathNotFound) Unwrap() error { return ErrNotFound }

// PageRule is the rule PageHint tells, which versions' refusal of an
// attachment's path tells too, without a path: the refusal comes before
// any lookup (M9/P2 S2 review B-N1).
const PageRule = "A page's path ends with .md"

// PageHint is PathNotFound's hint for an attachment's path that names a
// page, whose path is path.
func PageHint(path string) string {
	return PageRule + ": " + Quoted(path) + "."
}

// AttachmentHint is PathNotFound's hint for a page's path that names an
// attachment, whose path is path.
func AttachmentHint(path string) string {
	return "That is an attachment: its path is " + Quoted(path) + "."
}

// TitleHint is PathNotFound's hint for a page's path that names nothing
// while a page there is titled with its last segment, ".md" and all: that
// page's path, path, has ".md" twice.
func TitleHint(path string) string {
	return "A page there is titled with the .md: its path is " + Quoted(path) + "."
}

// MoveParentHint is PathNotFound's hint for the parent of move's to,
// which names nothing: a move makes no pages (M9/P2 design 3.8.3).
const MoveParentHint = "move creates no parent pages: write the parent first, or move to an existing page's path."

// ChildrenHint is the hint for list's pattern that names a page, at path,
// where its children were meant: they are listed by its path without
// ".md" and with a trailing "/".
func ChildrenHint(path string) string {
	return path + " is a page; list " + Quoted(strings.TrimSuffix(path, pageSuffix)+"/") + " for its children."
}

// Lookup is a point lookup of a notebook's tree: the node keyed key under
// the last of chain, the notebook's root for none. NodePaths looks up the
// store, a batch's plan the tree it simulates (M9/P3 design 3.4.1).
type Lookup func(chain []Node, key string) (Node, bool, error)

// FindPath is the node path names, with its ancestors from the root, a
// lookup a segment: page.not_found, a *PathNotFound, for none, for an
// attachment where the path has a page or the other way round, which its
// hint tells, and for a path through an attachment. A page's path that
// names nothing looks its last segment up once more, ".md" and all: a page
// titled so is hinted (NodePaths.Find; M9/P1 design 3.9.2).
func FindPath(path Path, child Lookup) (Node, []Node, error) {
	missing := &PathNotFound{Path: path.Written}
	var ancestors []Node
	for i, key := range path.Keys {
		n, ok, err := child(ancestors, key)
		switch {
		case err != nil:
			return Node{}, nil, err
		case !ok && i == len(path.Keys)-1 && path.Kind == KindPage:
			return Node{}, nil, titled(child, ancestors, key, missing)
		case !ok:
			return Node{}, nil, missing
		case i < len(path.Keys)-1:
			if n.Kind != KindPage {
				return Node{}, nil, missing // an attachment has no children
			}
			ancestors = append(ancestors, n)
			continue
		case n.Kind != path.Kind && n.Kind == KindPage:
			missing.Hint = PageHint(PathOf(ancestors, n))
			return Node{}, nil, missing
		case n.Kind != path.Kind:
			missing.Hint = AttachmentHint(PathOf(ancestors, n))
			return Node{}, nil, missing
		}
		return n, ancestors, nil
	}
	return Node{}, nil, missing
}

// titled is missing, hinted when the parent of ancestors' end, the root
// for none, has a page titled key with ".md", the path's last segment as
// it was written.
func titled(child Lookup, ancestors []Node, key string, missing *PathNotFound) error {
	n, ok, err := child(ancestors, key+pageSuffix)
	switch {
	case err != nil:
		return err
	case ok && n.Kind == KindPage:
		missing.Hint = TitleHint(PathOf(ancestors, n))
	}
	return missing
}

// WalkPath looks keys up from the notebook's root, one segment after
// another as FindPath does: the nodes found, from the root, up to the
// first key that names nothing (all of them when each names one). A key
// before the last that names an attachment is an *AttachmentOnPath of its
// segment: an attachment holds no nodes; the nodes found end with it then.
// No hint is looked up: a write walks under its notebook's lock, which
// keeps the chain as found (M9 design 4.8).
func WalkPath(keys []string, child Lookup) ([]Node, error) {
	var found []Node
	for i, key := range keys {
		n, ok, err := child(found, key)
		switch {
		case err != nil:
			return nil, err
		case !ok:
			return found, nil
		}
		found = append(found, n)
		if i < len(keys)-1 && n.Kind != KindPage {
			return found, &AttachmentOnPath{Segment: i + 1}
		}
	}
	return found, nil
}
