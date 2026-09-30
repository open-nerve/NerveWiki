// Package domain holds the identity module's rules: pure functions and
// values. The rules of e-mail addresses, which other modules share, are in
// internal/shared.
package domain

import (
	"strings"
	"unicode/utf8"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// User is an account as the API shows it.
type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	// OnboardingSteps are the ids of the onboarding steps the account has
	// completed; the web app's registry defines the steps (M1 design 8).
	OnboardingSteps []string
}

// MaxDisplayNameLength is the length of users.display_name, varchar(100),
// in characters.
const MaxDisplayNameLength = 100

// NewAccount checks the e-mail address and the password of a new account
// and returns the normalized address. Every problem is reported at once, as
// one 422 validation_failed.
func NewAccount(rules *PasswordRules, email, password string) (string, error) {
	email = shared.NormalizeEmail(email)
	var fields []shared.FieldError
	if f := checkEmail("email", email); f != nil {
		fields = append(fields, *f)
	}
	if f := rules.Check("password", password, email); f != nil {
		fields = append(fields, *f)
	}
	if len(fields) > 0 {
		return "", shared.Invalid(fields...)
	}
	return email, nil
}

// checkEmail checks a normalized address.
func checkEmail(field, email string) *shared.FieldError {
	switch {
	case email == "":
		return &shared.FieldError{Field: field, Code: shared.FieldRequired, Message: "is required"}
	case utf8.RuneCountInString(email) > shared.MaxEmailLength:
		return &shared.FieldError{Field: field, Code: shared.FieldTooLong, Message: "must be at most 255 characters"}
	case !shared.ValidEmail(email):
		return &shared.FieldError{Field: field, Code: shared.FieldInvalidFormat, Message: "is not a valid e-mail address"}
	}
	return nil
}

// DisplayNameFromEmail is the display name a new account gets (M1/P1 design
// 3.7): the address's local part, cut to 100 characters. It is never empty
// for a valid address.
func DisplayNameFromEmail(email string) string {
	name := localPart(email)
	if utf8.RuneCountInString(name) <= MaxDisplayNameLength {
		return name
	}
	return string([]rune(name)[:MaxDisplayNameLength])
}

// localPart is what comes before an address's last @: a quoted local part
// can hold an @ itself, a domain cannot.
func localPart(email string) string {
	if at := strings.LastIndexByte(email, '@'); at >= 0 {
		return email[:at]
	}
	return email
}
