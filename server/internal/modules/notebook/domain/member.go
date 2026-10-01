package domain

import (
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Member is an account's membership of a notebook: one row per pair, ended
// and restored in place (M3 design 4).
type Member struct {
	ID         uuid.UUID
	NotebookID uuid.UUID
	UserID     uuid.UUID
	Role       shared.NotebookRole
	CreatedAt  time.Time  // when the account first joined
	EndedAt    *time.Time // when it was removed or left; nil while active
}

// Active reports whether the membership has not ended.
func (m Member) Active() bool { return m.EndedAt == nil }

// CheckRole checks a role sent for a member (422): one of the three. The
// contract's enum is not checked before: a body's structure check leaves
// values to the domain.
func CheckRole(role string) (shared.NotebookRole, error) {
	if f := checkRole(role); f != nil {
		return "", shared.Invalid(*f)
	}
	return shared.NotebookRole(role), nil
}

// checkRole is a role's problem, or nil.
func checkRole(role string) *shared.FieldError {
	if !slices.Contains(shared.NotebookRoles(), shared.NotebookRole(role)) {
		return &shared.FieldError{Field: "role", Code: shared.FieldInvalidFormat, Message: "must be admin, editor or reader"}
	}
	return nil
}

// CheckAddition checks a new member (422), every problem at once: the role
// one of the three; the account an active member of the notebook's
// workspace (inWorkspace), so that the notebook's members stay among the
// workspace's (M3 design 4); and not an active member of the notebook
// already (existing, its membership if it ever had one).
func CheckAddition(role string, inWorkspace bool, existing *Member) (shared.NotebookRole, error) {
	var problems []shared.FieldError
	if !inWorkspace {
		problems = append(problems, shared.FieldError{Field: "user_id", Code: shared.FieldNotAllowed,
			Message: "must be an active member of the notebook's workspace"})
	} else if existing != nil && existing.Active() {
		problems = append(problems, shared.FieldError{Field: "user_id", Code: shared.FieldDuplicate,
			Message: "is a member of the notebook already"})
	}
	if f := checkRole(role); f != nil {
		problems = append(problems, *f)
	}
	if problems != nil {
		return "", shared.Invalid(problems...)
	}
	return shared.NotebookRole(role), nil
}

// CheckLeave is rule one (M3 design 4) for m leaving its notebook, which
// has admins active admins: its only one cannot, even alone in it; another
// member becomes an admin first, or the admin deletes the notebook.
func CheckLeave(m Member, admins int) error {
	if m.Role == shared.NotebookAdmin && admins <= 1 {
		return ErrSoleAdmin
	}
	return nil
}
