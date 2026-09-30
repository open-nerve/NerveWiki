package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CheckName checks a name a person gives, a display name or a token's name
// (M1/P3 design 3.2, 3.5): it returns s without its surrounding white space
// when that is 1–maxLen characters and holds no character that changes how
// the text around it reads where the name is shown; otherwise a problem on
// field. See unshowable.
func CheckName(field, s string, maxLen int) (string, *shared.FieldError) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", &shared.FieldError{Field: field, Code: shared.FieldRequired, Message: "is required"}
	case utf8.RuneCountInString(s) > maxLen:
		return "", &shared.FieldError{Field: field, Code: shared.FieldTooLong, Message: fmt.Sprintf("must be at most %d characters", maxLen)}
	case strings.ContainsFunc(s, unshowable):
		return "", &shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "must not contain control characters"}
	}
	return s, nil
}

// unshowable reports whether a name must not hold r: a control character
// (Cc); a line or paragraph separator, which breaks the line the name is
// shown on; a bidirectional control, which can show a name backwards to pass
// for another's. The other format characters stay: joiners make up emoji
// sequences and words of scripts such as Persian.
func unshowable(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp, unicode.Bidi_Control)
}
