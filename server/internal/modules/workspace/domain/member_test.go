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

// Rule two refuses only the sole admin of a workspace with others in it.
func TestStandingBlocksDeactivation(t *testing.T) {
	for _, tt := range []struct {
		name            string
		role            shared.WorkspaceRole
		admins, members int
		want            bool
	}{
		{"the only admin, with others", shared.WorkspaceAdmin, 1, 3, true},
		{"the only admin, alone", shared.WorkspaceAdmin, 1, 1, false},
		{"one of two admins", shared.WorkspaceAdmin, 2, 2, false},
		{"a member beside one admin", shared.WorkspaceMember, 1, 2, false},
		{"a guest beside one admin", shared.WorkspaceGuest, 1, 2, false},
	} {
		s := Standing{Role: tt.role, Admins: tt.admins, Members: tt.members}
		if got := s.BlocksDeactivation(); got != tt.want {
			t.Errorf("%s: BlocksDeactivation() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// The refusal is workspace.sole_admin, naming the workspaces.
func TestErrSoleAdminOf(t *testing.T) {
	err := ErrSoleAdminOf([]string{"acme", "beta"})
	if !errors.Is(err, ErrSoleAdmin) || err.ProblemStatus() != 409 ||
		err.Detail != "The account is the only admin of workspaces that have other members (acme, beta): "+
			"make another member an admin of each first." {
		t.Errorf("ErrSoleAdminOf() = %d %s %q", err.ProblemStatus(), err.Code, err.Detail)
	}
}
