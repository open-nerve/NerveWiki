// Package domain holds the identity module's rules: pure functions and
// values. The rules of e-mail addresses, which other modules share, are in
// internal/shared.
package domain

import (
	"regexp"
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

// UserPatch is a partial update of the caller's account: a nil field stays
// as it is. The address is not in it: only the administrator's command
// changes it (P4).
type UserPatch struct {
	DisplayName *string
}

// CheckUserPatch checks p (M1/P3 design 3.5) and returns it as it is
// stored: the display name without its surrounding white space, 1–100
// characters, no control character. A problem is 422 validation_failed.
func CheckUserPatch(p UserPatch) (UserPatch, error) {
	if p.DisplayName == nil {
		return p, nil
	}
	name, f := shared.CheckName("display_name", *p.DisplayName, MaxDisplayNameLength)
	if f != nil {
		return UserPatch{}, shared.Invalid(*f)
	}
	return UserPatch{DisplayName: &name}, nil
}

// MaxOnboardingSteps is how many steps an account records at most:
// users_onboarding_steps_check's bound.
const MaxOnboardingSteps = 32

// onboardingStepID is the form of a step id: a lower-case letter, then
// letters, digits and underscores, 32 characters at most (M1 design 8).
var onboardingStepID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// CheckOnboardingStep checks the id of a completed onboarding step; the
// server does not know the steps, the web app's registry does (M1 design
// 8). A problem is 422 validation_failed on step.
func CheckOnboardingStep(step string) error {
	if !onboardingStepID.MatchString(step) {
		return shared.Invalid(shared.FieldError{
			Field: "step", Code: shared.FieldInvalidFormat,
			Message: "must be a lower-case letter, then lower-case letters, digits and underscores, 32 characters at most",
		})
	}
	return nil
}

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

// NewEmail checks an address that the server's administrator gives an
// account (M1/P4 design 3.6) and returns it normalized; a problem is 422
// validation_failed on field.
func NewEmail(field, email string) (string, error) {
	email = shared.NormalizeEmail(email)
	if f := checkEmail(field, email); f != nil {
		return "", shared.Invalid(*f)
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
