package domain

import (
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Member is an account's membership of a workspace.
type Member struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        shared.WorkspaceRole
	CreatedAt   time.Time // when the account joined
}

// CheckRole checks a role sent for a member (422): one of the three. The
// contract's enum is not checked before: a body's structure check leaves
// values to the domain.
func CheckRole(role string) (shared.WorkspaceRole, error) {
	if f := checkRole(role); f != nil {
		return "", shared.Invalid(*f)
	}
	return shared.WorkspaceRole(role), nil
}

// checkRole is a role's problem, or nil.
func checkRole(role string) *shared.FieldError {
	if !slices.Contains(shared.WorkspaceRoles(), shared.WorkspaceRole(role)) {
		return &shared.FieldError{Field: "role", Code: shared.FieldInvalidFormat, Message: "must be admin, member or guest"}
	}
	return nil
}
