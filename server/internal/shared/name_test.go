package shared_test

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckName(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Alice", "Alice"},
		{"  Alice Stone \n", "Alice Stone"},
		{strings.Repeat("名", 100), strings.Repeat("名", 100)},
		{"O'Brien – QA", "O'Brien – QA"},
		// Joiners: an emoji sequence, a Persian word.
		{"\U0001F468\u200d\U0001F469\u200d\U0001F467", "\U0001F468\u200d\U0001F469\u200d\U0001F467"},
		{"\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645", "\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645"},
	} {
		if got, f := shared.CheckName("display_name", tt.in, 100); f != nil || got != tt.want {
			t.Errorf("CheckName(%q) = %q, %+v; want %q", tt.in, got, f, tt.want)
		}
	}
	for _, in := range []string{
		"a\x00b", "a\tb", "a\u007fb", "a\u0085b", // control characters
		"a\u2028b", "a\u2029b", // line and paragraph separators
		"a\u202eb", "a\u2066b", "a\u200fb", "a\u061cb", // bidirectional controls
	} {
		if _, f := shared.CheckName("display_name", in, 100); f == nil || f.Code != shared.FieldInvalidFormat {
			t.Errorf("CheckName(%q) = %+v, want invalid_format", in, f)
		}
	}
}
