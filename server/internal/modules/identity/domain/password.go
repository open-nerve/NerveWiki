package domain

import (
	_ "embed"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Password lengths, in UTF-16 code units like the web app's password.length.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

//go:embed common_passwords.txt
var commonPasswordsFile string

// PasswordRules are the server's password rules (M1/P1 design 3.4): a length
// and the common-password list, no composition rules (NIST SP 800-63B).
// The list holds the common passwords of 8 characters or more and the cores
// of every common password (tools/password-blocklist).
type PasswordRules struct {
	common []string // sorted: binary search
}

// NewPasswordRules parses the embedded common-password list: a header, a
// blank line, then one lowercased entry per line, sorted by bytes.
func NewPasswordRules() *PasswordRules {
	_, list, _ := strings.Cut(commonPasswordsFile, "\n\n")
	return &PasswordRules{common: strings.Split(strings.TrimSuffix(list, "\n"), "\n")}
}

// Check returns the field error for a new password of the account with the
// given normalized e-mail address, or nil when it is acceptable:
//
//   - too_short, too_long: not 8–128 characters;
//   - common_password: the lowercased password or its core is on the list,
//     or its core is the core of the address's local part, or it is one
//     character repeated, or white space and control characters alone.
func (p *PasswordRules) Check(field, password, email string) *shared.FieldError {
	n := len(utf16.Encode([]rune(password)))
	switch {
	case password == "":
		return &shared.FieldError{Field: field, Code: shared.FieldRequired, Message: "is required"}
	case n < MinPasswordLength:
		return &shared.FieldError{Field: field, Code: shared.FieldTooShort, Message: "must be at least 8 characters"}
	case n > MaxPasswordLength:
		return &shared.FieldError{Field: field, Code: shared.FieldTooLong, Message: "must be at most 128 characters"}
	case p.isCommon(password, email):
		return &shared.FieldError{Field: field, Code: shared.FieldCommonPassword, Message: "is too common or too close to the e-mail address"}
	}
	return nil
}

func (p *PasswordRules) isCommon(password, email string) bool {
	c := core(password)
	return trivial(password) || p.listed(strings.ToLower(password)) || p.listed(c) || (c != "" && c == core(localPart(email)))
}

// trivial reports a password of one character repeated, or of white space
// and control characters alone: the list holds a few such (aaaaaaaa,
// 11111111), not every one.
func trivial(password string) bool {
	first, _ := utf8.DecodeRuneInString(password)
	return strings.Trim(password, string(first)) == "" ||
		strings.TrimFunc(password, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) == ""
}

func (p *PasswordRules) listed(s string) bool {
	_, found := slices.BinarySearch(p.common, s)
	return found
}

// core lowercases s and trims every character that is not a letter
// (unicode.IsLetter, \p{L}) from both ends: Password1!~, ~Password1! and
// "Password1! " all have the core "password".
func core(s string) string {
	return strings.TrimFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) })
}
