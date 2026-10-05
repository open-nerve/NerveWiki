package domain_test

import (
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A target is cut into whether it is relative, how far up, whether it is
// from the root, and its segments' title keys, the last without ".md", in
// any case, and with it, and that last segment as written; an empty
// segment, or "." or ".." past its head, is none.
func TestATargetIsCutIntoItsSegmentsKeys(t *testing.T) {
	tests := []struct {
		target string
		want   domain.Target
		ok     bool
	}{
		{"Note", domain.Target{Keys: []string{"note"}, Name: "Note"}, true},
		{"A/B/Note", domain.Target{Keys: []string{"a", "b", "note"}, Name: "Note"}, true},
		{"./Note", domain.Target{Relative: true, Keys: []string{"note"}, Name: "Note"}, true},
		{"../../Note", domain.Target{Relative: true, Up: 2, Keys: []string{"note"}, Name: "Note"}, true},
		{"./../Note", domain.Target{Relative: true, Up: 1, Keys: []string{"note"}, Name: "Note"}, true},
		{"/A/Note", domain.Target{Rooted: true, Keys: []string{"a", "note"}, Name: "Note"}, true},
		{"A/Note.md", domain.Target{Keys: []string{"a", "note"}, AltLast: "note.md", Name: "Note"}, true},
		{"Note.md.md", domain.Target{Keys: []string{"note.md"}, AltLast: "note.md.md", Name: "Note.md"}, true},
		{"Note.MD", domain.Target{Keys: []string{"note"}, AltLast: "note.md", Name: "Note"}, true},
		{"Note.mD", domain.Target{Keys: []string{"note"}, AltLast: "note.md", Name: "Note"}, true},
		{"Notemd", domain.Target{Keys: []string{"notemd"}, Name: "Notemd"}, true},
		{".md", domain.Target{Keys: []string{".md"}, Name: ".md"}, true},
		{"Straße", domain.Target{Keys: []string{"strasse"}, Name: "Straße"}, true},
		{"", domain.Target{}, false},
		{"A//Note", domain.Target{}, false},
		{"A/", domain.Target{}, false},
		{"A/./Note", domain.Target{}, false},
		{"A/../Note", domain.Target{}, false},
		{"/../Note", domain.Target{}, false},
		{"..", domain.Target{}, false},
	}
	for _, tt := range tests {
		got, ok := domain.ParseTarget(tt.target)
		if ok != tt.ok || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseTarget(%q) = %+v, %v; want %+v, %v", tt.target, got, ok, tt.want, tt.ok)
		}
	}
}

// The keys a page must have to be the target are the last segment's, in
// each form it may be written in.
func TestATargetsLastKeysAreItsForms(t *testing.T) {
	for target, want := range map[string][]string{
		"A/Note":    {"note"},
		"A/Note.md": {"note", "note.md"},
	} {
		parsed, _ := domain.ParseTarget(target)
		if got := parsed.LastKeys(); !reflect.DeepEqual(got, want) {
			t.Errorf("LastKeys of %q = %v, want %v", target, got, want)
		}
	}
}
