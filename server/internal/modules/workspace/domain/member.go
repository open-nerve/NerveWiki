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
	CreatedAt   time.Time  // when the account first joined
	EndedAt     *time.Time // when the membership ended; nil while it is active
}

// Active reports whether the membership has not ended.
func (m Member) Active() bool {
	return m.EndedAt == nil
}

// Standing is an account's active membership of a workspace as rule two
// reads it, under the workspace's lock: the account's role, and how many
// active admins and members the workspace has, the account included.
type Standing struct {
	WorkspaceID uuid.UUID
	Slug        string
	Role        shared.WorkspaceRole
	Admins      int
	Members     int
}

// BlocksDeactivation is rule two (M2 design 4): the only active admin of a
// workspace with other active members cannot be deactivated, or the others
// would be left without one. Alone in it, the account can: nobody is left.
func (s Standing) BlocksDeactivation() bool {
	return s.Role == shared.WorkspaceAdmin && s.Admins == 1 && s.Members > 1
}

// JoinLeavesNoAdmin is rule three (M2 design 4): a workspace with active
// members has an active admin. Rule two lets its only admin be deactivated
// when nobody else is left, so a workspace may have no active member; then
// an admin comes back first. A membership becoming active with role, of a
// workspace with admins active admins, would leave its members without one.
func JoinLeavesNoAdmin(role shared.WorkspaceRole, admins int) bool {
	return admins == 0 && role != shared.WorkspaceAdmin
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
