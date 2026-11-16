package shared

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CheckName checks a name a person gives: a display name, a token's name
// (M1/P3 design 3.2, 3.5), a workspace's name (M2/P1 design 3.6). It
// returns s without its surrounding white space when that is 1–maxLen
// characters, shows something (Blank) and holds no character that changes
// how the text around it reads where the name is shown; otherwise a
// problem on field. See Unshowable. A name that shows nothing is required
// before it is too long or holds such a character: one of a bidirectional
// control alone is empty (M12 design 4.6.4, 30.2).
func CheckName(field, s string, maxLen int) (string, *FieldError) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || Blank(s):
		return "", &FieldError{Field: field, Code: FieldRequired, Message: "is required"}
	case utf8.RuneCountInString(s) > maxLen:
		return "", &FieldError{Field: field, Code: FieldTooLong, Message: fmt.Sprintf("must be at most %d characters", maxLen)}
	case strings.ContainsFunc(s, Unshowable):
		return "", &FieldError{Field: field, Code: FieldInvalidFormat, Message: "must not contain control characters"}
	}
	return s, nil
}

// Unshowable reports whether a name must not hold r: a control character
// (Cc); a line or paragraph separator (Zl, Zp), which breaks the line the
// name is shown on; a bidirectional control (Bidi_Control), which can show
// a name backwards to pass for another's. The other format characters
// stay: joiners make up emoji sequences and words of scripts such as
// Persian. page's client checks of the changesets and edit_sessions tables
// spell out the same set (migration 00044).
func Unshowable(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp, unicode.Bidi_Control)
}

// Blank reports whether s shows nothing: every rune of it white space or a
// default ignorable code point (M12 design 4.6.4, 30.2). A name of only
// these passes for empty where it is shown, so it is no name. Go's unicode
// does not hold the derived property Default_Ignorable_Code_Point: it is
// derived here as Unicode's DerivedCoreProperties.txt derives it,
// Other_Default_Ignorable_Code_Point, Cf and Variation_Selector, but
// White_Space, U+FFF9–FFFB, U+13430–13440 and
// Prepended_Concatenation_Mark, the format characters that should be
// visible: U+06DD alone shows. U+2800, the braille blank, is a symbol
// (So), no default ignorable: it is no blank. The tag characters
// (U+E0000–E007F) are default ignorables: a name of them alone is blank,
// though with an emoji they make a flag.
func Blank(s string) bool {
	for _, r := range s {
		if !blankRune(r) {
			return false
		}
	}
	return true
}

// blankRune reports whether r is white space or a default ignorable code
// point, as Blank derives it.
func blankRune(r rune) bool {
	switch {
	case unicode.Is(unicode.White_Space, r):
		return true
	case 0xFFF9 <= r && r <= 0xFFFB, 0x13430 <= r && r <= 0x13440, unicode.Is(unicode.Prepended_Concatenation_Mark, r):
		return false
	}
	return unicode.In(r, unicode.Other_Default_Ignorable_Code_Point, unicode.Cf, unicode.Variation_Selector)
}
