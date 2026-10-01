package app

import (
	"context"
	"fmt"
	"uuid"
)

// MembershipEnder ends memberships through the extension point: the step
// of removing, leaving and deactivating. Its caller has locked the
// workspaces' rows. Removing and leaving call End; a deactivation calls
// Veto and Write from the two phases of identity's (M2/P4 design 3.1).
type MembershipEnder struct {
	Members     MemberUpdater
	Invitations InvitationUpdater
	Profiles    MemberProfiles
	Vetoers     []MembershipEndVetoer
	Subscribers []MembershipEndSubscriber
}

// End asks the vetoers, then writes the end with the account's address.
// The address is read without a lock: the caller holds the workspaces'
// rows, not the account's, and an invitation to an address the account no
// longer has can only bring back that address's new holder.
func (m MembershipEnder) End(ctx context.Context, e MembershipEnd) error {
	if err := m.Veto(ctx, e); err != nil {
		return err
	}
	profiles, err := m.Profiles.MemberProfiles(ctx, []uuid.UUID{e.UserID})
	if err != nil {
		return err
	}
	p, ok := profiles[e.UserID]
	if !ok {
		return fmt.Errorf("no profile of account %s, whose memberships end", e.UserID)
	}
	return m.Write(ctx, e, p.Email)
}

// Veto asks the vetoers, before any write: the first refusal stops it, for
// the caller's transaction to roll back.
func (m MembershipEnder) Veto(ctx context.Context, e MembershipEnd) error {
	for _, v := range m.Vetoers {
		if err := v.VetoMembershipEnd(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// Write deletes the workspaces' pending invitations to email, the
// account's address, so that no invitation sent before brings it back
// (M2/P3 design 3.4); ends the memberships; then tells the subscribers.
// The first error stops it, for the caller's transaction to roll back.
func (m MembershipEnder) Write(ctx context.Context, e MembershipEnd, email string) error {
	if err := m.Invitations.DeleteInvitationsTo(ctx, e.WorkspaceIDs, email, e.By, e.At); err != nil {
		return err
	}
	if err := m.Members.EndMemberships(ctx, e.UserID, e.WorkspaceIDs, e.By, e.At); err != nil {
		return err
	}
	for _, s := range m.Subscribers {
		if err := s.MembershipEnded(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
