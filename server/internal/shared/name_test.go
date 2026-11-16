package shared_test

import (
	"strings"
	"testing"
	"unicode"

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
	// A name that shows nothing is required (M12 design 4.6.4, 30.2): one of
	// white space and default ignorables alone, before it is too long or
	// holds a bidirectional control.
	for _, in := range []string{
		"\U0000200B", "\U00002060", "\U00003164", "\U0000200B\U00002060\U00003164", // zero width space, word joiner, Hangul filler
		"  \U0000200B\U00003164 \n", "\U0000200B \U00002060", // white space around them and between
		"\U000E0041\U000E0042",                     // tag characters
		"\U0000202E",                               // a bidirectional control alone: invalid_format before M12
		strings.Repeat("\U0000200B", 300),          // required, not too_long
		"\U0000FEFF\U0000180E\U0000034F\U0000FE0F", // a byte order mark, a Mongolian separator, a grapheme joiner, a variation selector
	} {
		if _, f := shared.CheckName("display_name", in, 100); f == nil || f.Code != shared.FieldRequired || f.Field != "display_name" {
			t.Errorf("CheckName(%q) = %+v, want required", in, f)
		}
	}
	// What shows passes as it is: an emoji's variation selector, its
	// joiners, Persian's non-joiner (above), a subdivision's flag of tag
	// characters; U+2800, the braille blank, a symbol (So) that shows as
	// nothing; and U+06DD and U+0600 alone, format characters Unicode keeps
	// visible (Prepended_Concatenation_Mark), each a name of its own.
	for _, in := range []string{
		"I \U00002764\U0000FE0F Go",
		"\U0001F3F4\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F",
		"\U00002800", "\U000006DD", "\U00000600", "\U0000200B\U00000600",
	} {
		if got, f := shared.CheckName("display_name", in, 100); f != nil || got != in {
			t.Errorf("CheckName(%q) = %q, %+v; want it as it is", in, got, f)
		}
	}
}

// Blank is white space and Unicode's Default_Ignorable_Code_Point, which
// DerivedCoreProperties.txt derives as Other_Default_Ignorable_Code_Point,
// Cf and Variation_Selector, but White_Space, U+FFF9–FFFB, U+13430–13440
// and Prepended_Concatenation_Mark: each code point alone against that
// derivation, from Go's tables, written here and not calling Blank (M12
// design 4.6.4; P3 design review B-M1). Two other judges keep the two
// from being wrong alike: the totals of DerivedCoreProperties.txt and of
// PropList.txt for unicode.Version, and code points written out.
func TestBlankIsUnicodesDerivation(t *testing.T) {
	derived := func(r rune) (whiteSpace, ignorable bool) {
		if unicode.Is(unicode.White_Space, r) {
			return true, false
		}
		in := unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) || unicode.Is(unicode.Cf, r) ||
			unicode.Is(unicode.Variation_Selector, r)
		out := (0xFFF9 <= r && r <= 0xFFFB) || (0x13430 <= r && r <= 0x13440) || unicode.Is(unicode.Prepended_Concatenation_Mark, r)
		return false, in && !out
	}
	whiteSpaces, ignorables := 0, 0
	for r := range unicode.MaxRune + 1 {
		if 0xD800 <= r && r <= 0xDFFF {
			continue // a surrogate is no UTF-8: string(r) would be U+FFFD
		}
		w, i := derived(r)
		if w {
			whiteSpaces++
		}
		if i {
			ignorables++
		}
		if got := shared.Blank(string(r)); got != (w || i) {
			t.Errorf("Blank(%U) = %v, want %v (White_Space %v, Default_Ignorable_Code_Point %v)", r, got, w || i, w, i)
		}
	}
	// DerivedCoreProperties.txt and PropList.txt of Unicode 17.0.0: "Total
	// code points: 4174" and 25 for White_Space. A Go of a later Unicode
	// may change them: check the files of its version, then the counts.
	if unicode.Version != "17.0.0" || ignorables != 4174 || whiteSpaces != 25 {
		t.Errorf("Unicode %s: %d default ignorables and %d white spaces derived, want 4174 and 25 of 17.0.0", unicode.Version,
			ignorables, whiteSpaces)
	}
	for r, want := range map[rune]bool{
		0x200B: true, 0x3164: true, 0xFE0F: true, 0xE0041: true, 0x00AD: true, 0x3000: true, 0x2060: true, 0x1D173: true,
		0x06DD: false, 0x0600: false, 0x110BD: false, 0xFFF9: false, 0xFFFB: false, 0x13430: false, 0x13440: false,
		0x2800: false, 0x0041: false, 0x00A0: true, 0xFFFC: false,
	} {
		if got := shared.Blank(string(r)); got != want {
			t.Errorf("Blank(%U) = %v, want %v", r, got, want)
		}
	}
	if !shared.Blank("") || shared.Blank("\U0000200Bx\U0000200B") {
		t.Error(`Blank("") is false, or a visible letter among blanks is blank`)
	}
}
