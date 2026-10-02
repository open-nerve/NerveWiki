package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxContentBytes is how large a page's content is at most: 5 MB (v0.1
// design 3.6). page_contents and page_revisions hold the same bound.
const MaxContentBytes = 5 << 20

// CheckContent checks a page's content as field, as it is stored: byte for
// byte, valid UTF-8 without NUL, which PostgreSQL's text cannot hold, of
// at most MaxContentBytes. A content that breaks a rule is 422 on field.
// The HTTP boundary answers invalid UTF-8 with 400 before: the check here
// holds for the other callers.
func CheckContent(field, content string) error {
	switch {
	case len(content) > MaxContentBytes:
		return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldTooLong,
			Message: "must be at most 5 MB"})
	case strings.IndexByte(content, 0) >= 0 || !utf8.ValidString(content):
		return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldInvalidFormat,
			Message: "must be UTF-8 text without NUL characters"})
	}
	return nil
}
