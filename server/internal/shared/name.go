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
// characters and holds no character that changes how the text around it
// reads where the name is shown; otherwise a problem on field. See
// Unshowable.
func CheckName(field, s string, maxLen int) (string, *FieldError) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
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
