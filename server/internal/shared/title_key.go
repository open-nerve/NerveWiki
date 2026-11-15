package shared

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// TitleKey is the key titles compare by (v0.1 design 3.5): NFC, full
// Unicode case folding, and NFC again, as folding need not give NFC; then
// Cherokee's small letters as its capitals (CherokeeCapital), so that a
// key is its own key. Two pages under one parent may not share it, and
// M6's links resolve by it; the search folds its lexemes and its queries
// by it too (M8/P5A review A-M1). The folding is language-independent:
// Turkish İ keys as i with a dot above, not as i; Obsidian lowercases, so
// Straße and STRASSE, apart in Obsidian, are one title here. A new Unicode
// version may change keys, which nervewiki reindex recomputes (M6).
func TitleKey(s string) string {
	// ASCII is already NFC and its full fold is its lowercase spelling.
	// Any non-ASCII byte keeps the whole original Unicode pipeline.
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			// A Caser keeps state: one per call, never shared.
			return cherokeeCapitals(norm.NFC.String(cases.Fold().String(norm.NFC.String(s))))
		}
	}
	return strings.ToLower(s)
}

// cherokeeCapitals is s with each small Cherokee letter as its capital
// (CherokeeCapital), every other byte as it is: one that is no UTF-8 too,
// which strings.Map would make U+FFFD, so that a key changes nowhere but
// at Cherokee's small letters.
func cherokeeCapitals(s string) string {
	var b []byte
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if c := CherokeeCapital(r); c != r {
			if b == nil {
				b = []byte(s)
			}
			utf8.EncodeRune(b[i:], c)
		}
		i += n
	}
	if b == nil {
		return s
	}
	return string(b)
}

// CherokeeCapital is r, a small Cherokee letter, as its capital; any other
// rune as it is: the step of a title key after its fold (M12 design 4.6.3,
// 25.1). Unicode's case folding takes Cherokee to its capitals, its small
// letters being the later ones; x/text's full fold turns its capitals
// small and its small letters capital, so that a key would not be its own
// key. The keyers that fold by themselves (linking's IndexKeyer) take the
// step as TitleKey does. A small letter and its capital are three bytes
// each in UTF-8: a keyer may swap them in place.
func CherokeeCapital(r rune) rune {
	switch {
	case 0xAB70 <= r && r <= 0xABBF: // the small letters of U+AB70-ABBF: U+13A0-13EF
		return r - 0xAB70 + 0x13A0
	case 0x13F8 <= r && r <= 0x13FD: // U+13F8-13FD: U+13F0-13F5
		return r - 0x13F8 + 0x13F0
	}
	return r
}
