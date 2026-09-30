package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestNewAccountNormalizesTheEmail(t *testing.T) {
	email, err := NewAccount(NewPasswordRules(), "  Alice@Corp.COM ", "Tr0ub4dor&3")
	if err != nil || email != "alice@corp.com" {
		t.Errorf("NewAccount() = %q, %v; want alice@corp.com", email, err)
	}
}

// Every problem is reported at once, as one 422.
func TestNewAccountReportsEveryField(t *testing.T) {
	rules := NewPasswordRules()
	tests := []struct {
		name, email, password string
		want                  []shared.FieldError
	}{
		{"both empty", "", "", []shared.FieldError{
			{Field: "email", Code: "required", Message: "is required"},
			{Field: "password", Code: "required", Message: "is required"},
		}},
		{"white space only", " \t", "", []shared.FieldError{
			{Field: "email", Code: "required", Message: "is required"},
			{Field: "password", Code: "required", Message: "is required"},
		}},
		{"bad address, short password", "not-an-address", "short", []shared.FieldError{
			{Field: "email", Code: "invalid_format", Message: "is not a valid e-mail address"},
			{Field: "password", Code: "too_short", Message: "must be at least 8 characters"},
		}},
		{"long address, long password", strings.Repeat("a", 250) + "@x.com", strings.Repeat("x", 129), []shared.FieldError{
			{Field: "email", Code: "too_long", Message: "must be at most 255 characters"},
			{Field: "password", Code: "too_long", Message: "must be at most 128 characters"},
		}},
		{"common password", "bob@corp.com", "Password1!", []shared.FieldError{
			{Field: "password", Code: "common_password", Message: "is too common or too close to the e-mail address"},
		}},
		{"the address's local part", "Bob.Stone@corp.com", "bob.stone2026", []shared.FieldError{
			{Field: "password", Code: "common_password", Message: "is too common or too close to the e-mail address"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAccount(rules, tt.email, tt.password)
			var se *shared.Error
			if !errors.As(err, &se) || se.Kind != shared.KindInvalid || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, tt.want) {
				t.Errorf("NewAccount() = %+v, want validation_failed with %+v", err, tt.want)
			}
		})
	}
}

func TestDisplayNameFromEmail(t *testing.T) {
	for in, want := range map[string]string{
		"alice@corp.com":                   "alice",
		`"a@b"@example.com`:                `"a@b"`,
		"élodie@exämple.com":               "élodie",
		"first.last+tag@corp.com":          "first.last+tag",
		strings.Repeat("a", 150) + "@x.io": strings.Repeat("a", 100),
	} {
		if got := DisplayNameFromEmail(in); got != want {
			t.Errorf("DisplayNameFromEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCheckUserPatch(t *testing.T) {
	name := "  Alice Stone\t"
	got, err := CheckUserPatch(UserPatch{DisplayName: &name})
	if err != nil || got.DisplayName == nil || *got.DisplayName != "Alice Stone" {
		t.Errorf("CheckUserPatch() = %+v, %v; want the name trimmed", got, err)
	}
	if got, err := CheckUserPatch(UserPatch{}); err != nil || got.DisplayName != nil {
		t.Errorf("CheckUserPatch(empty) = %+v, %v; want nothing to change", got, err)
	}
	for in, code := range map[string]string{" ": "required", strings.Repeat("a", 101): "too_long", "a b\x07": "invalid_format"} {
		_, err := CheckUserPatch(UserPatch{DisplayName: &in})
		var se *shared.Error
		if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || len(se.Fields) != 1 ||
			se.Fields[0].Field != "display_name" || se.Fields[0].Code != code {
			t.Errorf("CheckUserPatch(%q) = %+v, want %s on display_name", in, err, code)
		}
	}
}

func TestCheckOnboardingStep(t *testing.T) {
	for _, ok := range []string{"profile", "a", "first_notebook_2", strings.Repeat("a", 32)} {
		if err := CheckOnboardingStep(ok); err != nil {
			t.Errorf("CheckOnboardingStep(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "Profile", "2fa", "_x", "first-notebook", "a b", "profile\n", strings.Repeat("a", 33), "é"} {
		var se *shared.Error
		if err := CheckOnboardingStep(bad); !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Field != "step" || se.Fields[0].Code != "invalid_format" {
			t.Errorf("CheckOnboardingStep(%q) = %v, want invalid_format on step", bad, err)
		}
	}
}
