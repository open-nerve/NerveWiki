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
// when that is 1–maxLen characters and holds no control character, which
// would break the lines it is shown or logged on; otherwise a problem on
// field.
func CheckName(field, s string, maxLen int) (string, *shared.FieldError) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", &shared.FieldError{Field: field, Code: shared.FieldRequired, Message: "is required"}
	case utf8.RuneCountInString(s) > maxLen:
		return "", &shared.FieldError{Field: field, Code: shared.FieldTooLong, Message: fmt.Sprintf("must be at most %d characters", maxLen)}
	case strings.ContainsFunc(s, unicode.IsControl):
		return "", &shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "must not contain control characters"}
	}
	return s, nil
}
