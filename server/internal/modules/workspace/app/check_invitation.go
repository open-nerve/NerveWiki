package app

import (
	"context"
	"errors"
	"uuid"
)

// InvitationCheck tells the sign-up policy whether an invitation lets an
// address register while sign-up is off (M2/P3 design 3.6). It reads
// without a lock: registering does not accept the invitation; the page
// accepts it next, and the acceptance checks again under its locks.
type InvitationCheck struct {
	Tokens      InvitationTokens
	Invitations InvitationFinder
}

// Admits reports whether token is the invitation id's, the invitation is
// pending, its workspace not deleted, and it was sent to email, a
// normalized address. The token is checked before any read.
func (c InvitationCheck) Admits(ctx context.Context, id uuid.UUID, token, email string) (bool, error) {
	if !c.Tokens.Valid(id, token) {
		return false, nil
	}
	inv, err := c.Invitations.FindPendingInvitation(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return inv.Email == email, nil
}
