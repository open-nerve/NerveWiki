package domain

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Invitation is a pending invitation of an e-mail address to a workspace:
// whoever holds its token and signs in with the address joins with the
// role.
type Invitation struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Email       string
	Role        shared.WorkspaceRole
	CreatedAt   time.Time
}

// InvitationDraft is a new invitation's checked values.
type InvitationDraft struct {
	Email string
	Role  shared.WorkspaceRole
}

// CheckInvitation checks a new invitation's address and role, every
// problem at once (422): the address normalized and checked by the rules of
// an account's, so that it compares with one; the role one of the three.
func CheckInvitation(email, role string) (InvitationDraft, error) {
	var problems []shared.FieldError
	email = shared.NormalizeEmail(email)
	if f := shared.CheckEmail("email", email); f != nil {
		problems = append(problems, *f)
	}
	if f := checkRole(role); f != nil {
		problems = append(problems, *f)
	}
	if problems != nil {
		return InvitationDraft{}, shared.Invalid(problems...)
	}
	return InvitationDraft{Email: email, Role: shared.WorkspaceRole(role)}, nil
}
