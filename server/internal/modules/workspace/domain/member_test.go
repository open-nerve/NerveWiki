package domain

import (
	"errors"
	"slices"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestCheckRole(t *testing.T) {
	for _, role := range shared.WorkspaceRoles() {
		if got, err := CheckRole(string(role)); err != nil || got != role {
			t.Errorf("CheckRole(%q) = %q, %v", role, got, err)
		}
	}
	want := []shared.FieldError{{Field: "role", Code: shared.FieldInvalidFormat, Message: "must be admin, member or guest"}}
	for _, role := range []string{"", "Admin", "owner", " admin"} {
		_, err := CheckRole(role)
		var se *shared.Error
		if !errors.As(err, &se) || se.Code != shared.CodeValidationFailed || !slices.Equal(se.Fields, want) {
			t.Errorf("CheckRole(%q) = %v, want validation_failed on role", role, err)
		}
	}
}
