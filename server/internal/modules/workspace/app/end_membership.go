package app

import (
	"context"
	"fmt"
	"uuid"
)

// MembershipEnder ends memberships through the extension point: the one
// step of removing, leaving and (P4) deactivating. Its caller has locked
// the workspaces' rows.
type MembershipEnder struct {
	Members     MemberUpdater
	Invitations InvitationUpdater
	Profiles    MemberProfiles
	Vetoers     []MembershipEndVetoer
	Subscribers []MembershipEndSubscriber
}

// End asks the vetoers; deletes the workspaces' pending invitations to the
// account's address, so that no invitation sent before brings it back
// (M2/P3 design 3.4); ends the memberships; then tells the subscribers. The
// address is read without a lock: the caller holds the workspaces' rows,
// not the account's, and an invitation to an address the account no
// longer has can only bring back that address's new holder. The first
// error stops it, for the caller's transaction to roll back.
func (m MembershipEnder) End(ctx context.Context, e MembershipEnd) error {
	for _, v := range m.Vetoers {
		if err := v.VetoMembershipEnd(ctx, e); err != nil {
			return err
		}
	}
	profiles, err := m.Profiles.MemberProfiles(ctx, []uuid.UUID{e.UserID})
	if err != nil {
		return err
	}
	p, ok := profiles[e.UserID]
	if !ok {
		return fmt.Errorf("no profile of account %s, whose memberships end", e.UserID)
	}
	if err := m.Invitations.DeleteInvitationsTo(ctx, e.WorkspaceIDs, p.Email, e.By, e.At); err != nil {
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
