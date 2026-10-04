// Package domain holds the linking module's rules (v0.1 design 4.4; M6
// design 4.4; M6/P3 design 2): how a link's target is cut, and which page
// it resolves to.
package domain

import (
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Target is a link's target cut into what its resolution reads: whether it
// is relative (written from "./" or "../", Up the "../"), from the root (a
// leading "/"), and its segments' title keys, the last one without its
// ".md" when it was written with one; AltLast is then the last one with it.
type Target struct {
	Relative bool
	Up       int
	Rooted   bool
	Keys     []string
	AltLast  string
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
	if stem, ok := strings.CutSuffix(last, ".md"); ok && stem != "" {
		t.AltLast = shared.TitleKey(last)
		segments[len(segments)-1] = stem
	}
	t.Keys = make([]string, len(segments))
	for i, s := range segments {
		t.Keys[i] = shared.TitleKey(s)
	}
	return t, true
}

// LastKeys are the keys a page must have to be the target: the last
// segment's, in the forms it may be written in.
func (t Target) LastKeys() []string {
	if t.AltLast == "" {
		return []string{t.Keys[len(t.Keys)-1]}
	}
	return []string{t.Keys[len(t.Keys)-1], t.AltLast}
}

// forms are the key paths the target may be written as: without the
// ".md", then with it.
func (t Target) forms() [][]string {
	out := [][]string{t.Keys}
	if t.AltLast != "" {
		alt := append(append([]string(nil), t.Keys[:len(t.Keys)-1]...), t.AltLast)
		out = append(out, alt)
	}
	return out
}
