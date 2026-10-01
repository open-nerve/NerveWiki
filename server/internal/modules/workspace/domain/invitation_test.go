package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckInvitation(t *testing.T) {
	got, err := CheckInvitation("  Dana@Example.COM ", "guest")
	if err != nil || got != (InvitationDraft{Email: "dana@example.com", Role: shared.WorkspaceGuest}) {
		t.Errorf("CheckInvitation(a valid one) = %+v, %v, want the address normalized", got, err)
	}

	email := func(code, message string) shared.FieldError {
		return shared.FieldError{Field: "email", Code: code, Message: message}
	}
	role := shared.FieldError{Field: "role", Code: shared.FieldInvalidFormat, Message: "must be admin, member or guest"}
	for _, tt := range []struct {
		email, role string
		want        []shared.FieldError
	}{
		{"", "member", []shared.FieldError{email(shared.FieldRequired, "is required")}},
		{"   ", "member", []shared.FieldError{email(shared.FieldRequired, "is required")}},
		{"dana", "member", []shared.FieldError{email(shared.FieldInvalidFormat, "is not a valid e-mail address")}},
		{strings.Repeat("d", 250) + "@x.com", "member", []shared.FieldError{email(shared.FieldTooLong, "must be at most 255 characters")}},
		{"dana@example.com", "owner", []shared.FieldError{role}},
		{"dana@example.com", "Admin", []shared.FieldError{role}},
		{"dana", "", []shared.FieldError{email(shared.FieldInvalidFormat, "is not a valid e-mail address"), role}},
	} {
		_, err := CheckInvitation(tt.email, tt.role)
		var se *shared.Error
		if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, tt.want) {
			t.Errorf("CheckInvitation(%q, %q) = %v, want %v", tt.email, tt.role, err, tt.want)
		}
	}
}
