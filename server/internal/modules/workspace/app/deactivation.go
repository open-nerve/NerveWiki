package app

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
)

// Deactivated is an account being deactivated, as identity's deactivation
// tells it (bootstrap converts identity.Deactivation): its id, its address
// read under the account row lock, and the deactivation's instant.
type Deactivated struct {
	UserID uuid.UUID
	Email  string
	At     time.Time
}

// Deactivation is the module's part in an account's deactivation (M2/P4
// design 3.1): rule two, and the end of the account's active memberships
// through the extension point. identity calls it in the deactivation's
// transaction, under the account row lock: the vetoer before any write,
// the subscriber after the account's own.
type Deactivation struct {
	Locker    WorkspacesLocker
	Standings StandingLister
	Lister    MembershipLister
	Ender     MembershipEnder
}

// VetoDeactivation locks the workspaces of the account's active
// memberships, reads the memberships again under the locks, refuses by
// rule two with every workspace that blocks, then asks the membership
// end's vetoers. An account with no active membership passes untouched.
func (d Deactivation) VetoDeactivation(ctx context.Context, x Deactivated) error {
	locked, err := d.Locker.LockWorkspacesOf(ctx, x.UserID)
	if err != nil || len(locked) == 0 {
		return err
	}
	ids := make([]uuid.UUID, len(locked))
	for i, w := range locked {
		ids[i] = w.ID
	}
	standings, err := d.Standings.ListStandings(ctx, x.UserID, ids)
	if err != nil || len(standings) == 0 {
		return err
	}
	var blocking []string
	ending := make([]uuid.UUID, len(standings))
	for i, s := range standings {
		if s.BlocksDeactivation() {
			blocking = append(blocking, s.Slug)
		}
		ending[i] = s.WorkspaceID
	}
	if len(blocking) > 0 {
		slices.Sort(blocking)
		return domain.ErrSoleAdminOf(blocking)
	}
	return d.Ender.Veto(ctx, deactivationEnd(x, ending))
}

// AccountDeactivated ends the account's active memberships. They are the
// ones the vetoer saw: the account's row, which the deactivation holds,
// keeps the paths that add a membership waiting, and the workspaces' rows,
// which the vetoer locked, those that end one.
func (d Deactivation) AccountDeactivated(ctx context.Context, x Deactivated) error {
	memberships, err := d.Lister.ListWorkspacesOf(ctx, x.UserID)
	if err != nil || len(memberships) == 0 {
		return err
	}
	ids := make([]uuid.UUID, len(memberships))
	for i, m := range memberships {
		ids[i] = m.Workspace.ID
	}
	slices.SortFunc(ids, uuid.UUID.Compare) // as the vetoer saw them
	return d.Ender.Write(ctx, deactivationEnd(x, ids), x.Email)
}

// deactivationEnd is the membership end of a deactivation: by the account
// itself, at the deactivation's instant.
func deactivationEnd(x Deactivated, workspaceIDs []uuid.UUID) MembershipEnd {
	return MembershipEnd{UserID: x.UserID, WorkspaceIDs: workspaceIDs, Cause: EndDeactivated, By: x.UserID, At: x.At}
}
