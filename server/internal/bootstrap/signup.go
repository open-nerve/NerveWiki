package bootstrap

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// signupPolicy is identity's SignupPolicy (M2/P3 design 3.6): open while
// auth.signup_enabled is on; while it is off, only for an invitation of
// the workspace module to the address.
type signupPolicy struct {
	open        bool
	invitations workspace.InvitationCheck
}

func (p signupPolicy) AllowSignup(ctx context.Context, a identity.SignupAttempt) (bool, error) {
	if p.open {
		return true, nil
	}
	if a.Invitation == nil {
		return false, nil
	}
	return p.invitations.Admits(ctx, a.Invitation.ID, a.Invitation.Token, a.Email)
}
