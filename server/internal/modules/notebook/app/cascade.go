package app

import (
	"context"
	"maps"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module's registrants of the workspace module's membership end and
// restore (M3/P3 design 3.2). They run in the end's or the restore's
// transaction, which holds the workspaces' rows FOR NO KEY UPDATE: no
// notebook management write of them runs beside it, as each takes its
// workspace's row FOR SHARE, so what they read under the notebooks' locks
// holds until it commits.

// WorkspaceMembershipEnd is an account's workspace memberships ending, as
// the workspace module tells it: bootstrap converts
// workspace.MembershipEnd, Voluntary when the account leaves or is
// deactivated, not when an admin removes it.
type WorkspaceMembershipEnd struct {
	UserID       uuid.UUID
	WorkspaceIDs []uuid.UUID
	Voluntary    bool
	By           uuid.UUID
	At           time.Time
}

// WorkspaceMembershipRestore is an account's ended workspace membership
// active again, field by field as the workspace module's restore tells it:
// bootstrap converts workspace.MembershipRestore.
type WorkspaceMembershipRestore struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        shared.WorkspaceRole
	By          uuid.UUID // the account itself
	At          time.Time
}

// MembershipEnd is the module's registrant of the workspace module's
// membership end: rule two as its vetoer, the account's notebook
// memberships ended as its subscriber.
type MembershipEnd struct {
	Holdings    Holdings
	Workspaces  WorkspaceSlugs
	Subscribers []VisibilitySubscriber
}

// VetoMembershipEnd is rule two (M3 design 4): an account leaving its
// workspaces itself is refused while it is the only active admin of one of
// their notebooks with another active explicit member, naming the
// workspaces and how many each has; a removal is never refused. It locks
// the account's notebooks of the workspaces, which MembershipEnded then
// writes.
func (m MembershipEnd) VetoMembershipEnd(ctx context.Context, e WorkspaceMembershipEnd) error {
	if !e.Voluntary {
		return nil
	}
	holdings, err := m.Holdings.LockHoldings(ctx, e.UserID, e.WorkspaceIDs)
	if err != nil {
		return err
	}
	blocked := domain.RuleTwo(holdings)
	if len(blocked) == 0 {
		return nil
	}
	slugs, err := m.Workspaces.Slugs(ctx, slices.Collect(maps.Keys(blocked)))
	if err != nil {
		return err
	}
	bySlug := make(map[string]int, len(blocked))
	for id, n := range blocked {
		bySlug[slugs[id]] = n
	}
	return domain.ErrSoleAdminOf(bySlug)
}

// MembershipEnded ends the account's memberships of the workspaces'
// notebooks, by the end's ender at its time; leaves ownerless those it was
// the only active admin of; and tells the visibility's subscribers once
// per workspace, whose notebooks the account no longer sees, by membership
// or by workspace access.
func (m MembershipEnd) MembershipEnded(ctx context.Context, e WorkspaceMembershipEnd) error {
	holdings, err := m.Holdings.LockHoldings(ctx, e.UserID, e.WorkspaceIDs)
	if err != nil {
		return err
	}
	var held, orphaned []uuid.UUID
	for _, h := range holdings {
		held = append(held, h.NotebookID)
		if h.SoleAdmin() {
			orphaned = append(orphaned, h.NotebookID)
		}
	}
	if len(held) > 0 {
		if err := m.Holdings.EndMembershipsOf(ctx, e.UserID, held, e.By, e.At); err != nil {
			return err
		}
	}
	if len(orphaned) > 0 {
		if err := m.Holdings.SetOwnerless(ctx, orphaned, e.UserID, e.At); err != nil {
			return err
		}
	}
	for _, ws := range e.WorkspaceIDs {
		if err := publishVisibility(ctx, m.Subscribers, VisibilityChange{WorkspaceID: ws, UserIDs: []uuid.UUID{e.UserID}, At: e.At}); err != nil {
			return err
		}
	}
	return nil
}

// MembershipRestore is the module's registrant of the workspace module's
// membership restore: it returns to the account the notebooks it left
// ownerless in the workspace that no workspace admin took over or deleted
// since (M3 design 4).
type MembershipRestore struct {
	Returner    Returner
	Audit       AuditRecorder
	Subscribers []VisibilitySubscriber
}

// MembershipRestored returns the notebooks, whatever role the account
// comes back with, a guest's included (a guest may hold any notebook
// role): its membership of each active again as their admin, each owned
// again, each recorded returned by the account; and returns how many it
// returned. It tells the visibility's subscribers when the account sees a
// notebook again: one returned, or the open ones, which reach an admin or
// a member. Its other notebook memberships stay ended: those notebooks
// have admins, theirs to decide.
func (m MembershipRestore) MembershipRestored(ctx context.Context, r WorkspaceMembershipRestore) (int, error) {
	notebooks, err := m.Returner.LockOwnerlessOf(ctx, r.WorkspaceID, r.UserID)
	if err != nil {
		return 0, err
	}
	if len(notebooks) > 0 {
		ids := make([]uuid.UUID, len(notebooks))
		for i, n := range notebooks {
			ids[i] = n.ID
		}
		if err := m.Returner.ReturnNotebooks(ctx, ids, r.UserID, r.By, r.At); err != nil {
			return 0, err
		}
	}
	for _, n := range notebooks {
		e := domain.AuditEvent{
			ID: uuid.NewV7(), WorkspaceID: r.WorkspaceID, NotebookID: n.ID, NotebookName: n.Name, Action: domain.AuditReturned,
			FormerOwnerID: r.UserID, ActorID: r.By, At: r.At,
		}
		if err := m.Audit.AddAuditEvent(ctx, e); err != nil {
			return 0, err
		}
	}
	if len(notebooks) == 0 && !shared.ReachedByAccess(r.Role) {
		return 0, nil
	}
	v := VisibilityChange{WorkspaceID: r.WorkspaceID, UserIDs: []uuid.UUID{r.UserID}, At: r.At}
	return len(notebooks), publishVisibility(ctx, m.Subscribers, v)
}
