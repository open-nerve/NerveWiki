package shared_test

import (
	"testing"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The literal cases state the title-key contract. The byte-pair comparison
// preserves the original Unicode pipeline at every ASCII/UTF-8 boundary,
// including malformed bytes and controls that other callers may pass.
func TestTitleKeysKeepASCIIAndUnicodeBoundaries(t *testing.T) {
	for _, tt := range []struct{ raw, want string }{
		{"", ""},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz"},
		{"[](){}<>/\\:;@#^_`~0123456789", "[](){}<>/\\:;@#^_`~0123456789"},
		{"A\x00B\x1fC\x7fZ", "a\x00b\x1fc\x7fz"},
		{"I", "i"},
		{"İ", "i\u0307"},
		{"A/STRAẞE.md", "a/strasse.md"},
		{"E\u0301/KZ", "é/kz"},
	} {
		if got := shared.TitleKey(tt.raw); got != tt.want {
			t.Fatalf("TitleKey(%q)=%q, want %q", tt.raw, got, tt.want)
		}
	}
	for a := range 256 {
		for b := range 256 {
			raw := string([]byte{'A', byte(a), byte(b), 'Z'})
			want := norm.NFC.String(cases.Fold().String(norm.NFC.String(raw)))
			if got := shared.TitleKey(raw); got != want {
				t.Fatalf("TitleKey(%x)=%x, original full Unicode pipeline=%x", raw, got, want)
			}
		}
	}
}
