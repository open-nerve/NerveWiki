package shared_test

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckTitle(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Engineering", "Engineering"},
		{"  项目 A \n", "项目 A"},
		{"Notes 2026：计划？／草稿", "Notes 2026：计划？／草稿"}, // full-width forms are allowed
		{"v1.2 notes", "v1.2 notes"},
		{"COM10", "COM10"},
		{"Console", "Console"},
		{"cons.txt", "cons.txt"},
		// NFD in, NFC out: e and a combining acute accent is é.
		{"Cafe\u0301", "Café"},
		{strings.Repeat("a", 255), strings.Repeat("a", 255)},
		// 85 Han characters are 255 bytes.
		{strings.Repeat("名", 85), strings.Repeat("名", 85)},
		// 256 bytes as given, 255 in NFC: counted as stored.
		{strings.Repeat("a", 253) + "e\u0301", strings.Repeat("a", 253) + "é"},
		// An emoji sequence keeps its joiners.
		{"\U0001F468\u200d\U0001F469\u200d\U0001F467 family", "\U0001F468\u200d\U0001F469\u200d\U0001F467 family"},
	} {
		if got, f := shared.CheckTitle("name", tt.in); f != nil || got != tt.want {
			t.Errorf("CheckTitle(%q) = %q, %+v; want %q", tt.in, got, f, tt.want)
		}
	}
	for _, tt := range []struct {
		in, code string
	}{
		{"", shared.FieldRequired},
		{" \t\n", shared.FieldRequired},
		{strings.Repeat("a", 256), shared.FieldTooLong},
		{strings.Repeat("名", 86), shared.FieldTooLong},
		{"a/b", shared.FieldInvalidFormat}, {`a\b`, shared.FieldInvalidFormat}, {"a:b", shared.FieldInvalidFormat},
		{"a*b", shared.FieldInvalidFormat}, {"a?b", shared.FieldInvalidFormat}, {`a"b`, shared.FieldInvalidFormat},
		{"a<b", shared.FieldInvalidFormat}, {"a>b", shared.FieldInvalidFormat}, {"a|b", shared.FieldInvalidFormat},
		{"a#b", shared.FieldInvalidFormat}, {"a^b", shared.FieldInvalidFormat}, {"a[b", shared.FieldInvalidFormat},
		{"a]b", shared.FieldInvalidFormat},
		{"a\x00b", shared.FieldInvalidFormat}, {"a\tb", shared.FieldInvalidFormat}, {"a\u007fb", shared.FieldInvalidFormat},
		{"a\u2028b", shared.FieldInvalidFormat}, {"a\u2029b", shared.FieldInvalidFormat},
		{"report\u202egpj.exe", shared.FieldInvalidFormat}, {"a\u2066b", shared.FieldInvalidFormat},
		{".hidden", shared.FieldInvalidFormat}, {"trailing.", shared.FieldInvalidFormat}, {"...", shared.FieldInvalidFormat},
		{"CON", shared.FieldNotAllowed}, {"con", shared.FieldNotAllowed}, {"Aux.md", shared.FieldNotAllowed},
		{"nul.tar.gz", shared.FieldNotAllowed}, {"PRN", shared.FieldNotAllowed}, {"com1", shared.FieldNotAllowed},
		{"LPT9.txt", shared.FieldNotAllowed},
	} {
		if _, f := shared.CheckTitle("name", tt.in); f == nil || f.Code != tt.code || f.Field != "name" {
			t.Errorf("CheckTitle(%q) = %+v, want %s on name", tt.in, f, tt.code)
		}
	}
}
