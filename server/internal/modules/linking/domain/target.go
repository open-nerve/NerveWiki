// Package domain holds the linking module's rules (v0.1 design 4.4; M6
// design 4.4; M6/P3 design 2): how a link's target is cut, and which page
// it resolves to.
package domain

import (
	"slices"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Target is a link's target cut into what its resolution reads: whether it
// is relative (written from "./" or "../", Up the "../"), from the root (a
// leading "/"), and its segments' title keys, the last one without its
// ".md", in any case, when it was written with one; AltLast is then the
// last one with it. Name is the last segment as written, without that
// ".md": a page made for the target is titled so (Land). Key holds each
// of the fields its resolution reads.
type Target struct {
	Relative bool
	Up       int
	Rooted   bool
	Keys     []string
	AltLast  string
	Name     string
}

// TargetKey is a Target as a value to compare: targets of one key resolve
// alike from one page (P3B fix check), however each was written.
type TargetKey struct {
	relative, rooted bool
	up               int
	keys, alt        string
}

// Key is t's TargetKey. No title key holds a '/' (NFC and case folding
// make none from a segment, which holds none), so the keys joined by it
// are one string.
func (t Target) Key() TargetKey {
	return TargetKey{t.Relative, t.Rooted, t.Up, strings.Join(t.Keys, "/"), t.AltLast}
}

// ParseTarget cuts target, a link's target as extracted. A target with an
// empty segment, or a "." or ".." past its head, resolves to nothing:
// false (nerve-defined; Obsidian's way is not checked).
func ParseTarget(target string) (Target, bool) {
	var t Target
	rest := target
	switch {
	case strings.HasPrefix(rest, "/"):
		t.Rooted, rest = true, rest[1:]
	case strings.HasPrefix(rest, "./"):
		t.Relative, rest = true, rest[2:]
	}
	if !t.Rooted {
		for strings.HasPrefix(rest, "../") {
			t.Relative, t.Up, rest = true, t.Up+1, rest[3:]
		}
	}
	segments := strings.Split(rest, "/")
	for _, s := range segments {
		if s == "" || s == "." || s == ".." {
			return Target{}, false
		}
	}
	last := segments[len(segments)-1]
	if n := len(last) - len(".md"); n > 0 && strings.EqualFold(last[n:], ".md") {
		t.AltLast = shared.TitleKey(last)
		segments[len(segments)-1] = last[:n]
	}
	t.Name = segments[len(segments)-1]
	t.Keys = make([]string, len(segments))
	for i, s := range segments {
		t.Keys[i] = shared.TitleKey(s)
	}
	return t, true
}

// ByAlias tells whether t may lead to a page by an alias: a name alone,
// neither relative nor from the root (M6/P3 design 2, step 4).
func (t Target) ByAlias() bool {
	return len(t.Keys) == 1 && !t.Relative && !t.Rooted
}

// LastKeys are the keys a page must have to be the target: the last
// segment's, in the forms it may be written in.
func (t Target) LastKeys() []string {
	if t.AltLast == "" {
		return []string{t.Keys[len(t.Keys)-1]}
	}
	return []string{t.Keys[len(t.Keys)-1], t.AltLast}
}

// form is the key path the target is read as among candidates, the pages
// with one of its LastKeys (M6/P3 design 2), as Obsidian reads it: written
// with ".md", it is the page without it when the notebook has a page of
// that name anywhere, and the page with it otherwise.
func (t Target) form(candidates candidateSet) []string {
	stem := t.Keys[len(t.Keys)-1]
	if t.AltLast == "" || candidates.has(stem) {
		return t.Keys
	}
	return append(slices.Clone(t.Keys[:len(t.Keys)-1]), t.AltLast)
}
