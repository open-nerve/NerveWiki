package app

import "context"

// MembershipEnder ends memberships through the extension point: the one
// step of removing, leaving and (P4) deactivating. Its caller has locked
// the workspaces' rows.
type MembershipEnder struct {
	Members     MemberUpdater
	Vetoers     []MembershipEndVetoer
	Subscribers []MembershipEndSubscriber
}

// End asks the vetoers, ends the memberships, then tells the subscribers.
// The first error stops it, for the caller's transaction to roll back.
func (m MembershipEnder) End(ctx context.Context, e MembershipEnd) error {
	for _, v := range m.Vetoers {
		if err := v.VetoMembershipEnd(ctx, e); err != nil {
			return err
		}
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
