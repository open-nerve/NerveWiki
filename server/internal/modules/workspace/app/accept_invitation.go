package app

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AcceptInvitationDeps are what AcceptInvitation needs.
type AcceptInvitationDeps struct {
	Tokens      InvitationTokens
	Finder      InvitationFinder
	Accounts    Accounts
	Locker      WorkspaceLocker
	Invitations InvitationUpdater
	Members     MemberFinder
	Updater     MemberUpdater
	Subscribers []MembershipRestoreSubscriber
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// AcceptInvitation joins the caller to a workspace by an invitation:
// POST /api/v0/workspace-invitations/{workspace_invitation_id}/accept (M2/P3
// design 3.3). It is not decided by the rule table: the caller is no member
// yet; the token and the caller's address are the credentials.
type AcceptInvitation struct {
	d AcceptInvitationDeps
}

// NewAcceptInvitation returns the use case.
func NewAcceptInvitation(d AcceptInvitationDeps) *AcceptInvitation {
	return &AcceptInvitation{d: d}
}

// joining is what an acceptance did to the caller's membership.
type joining string

const (
	joinAdded    joining = "added"    // a new one
	joinRestored joining = "restored" // an ended one, active again
	joinKept     joining = "kept"     // an active one, unchanged
)

// Execute accepts the invitation id for the caller. The token is checked
// before any read. In the lock order, the transaction shares the caller's
// account row, which tells its address under the lock, then locks the
// workspace's row and the invitation's: the address must be the one
// invited (403). An active membership is kept as it is; else, unless the
// workspace has no active admin and the invitation's role is not admin
// (409, rule three), an ended one is restored with the invitation's role,
// its subscribers told, or a new one is added. The invitation is then used
// up, at the same time; a refusal leaves it pending.
func (a *AcceptInvitation) Execute(ctx context.Context, id uuid.UUID, token string) (Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Membership{}, err
	}
	if !a.d.Tokens.Valid(id, token) {
		return Membership{}, domain.ErrInvitationNotFound
	}
	inv, err := a.d.Finder.FindPendingInvitation(ctx, id)
	if err != nil {
		return Membership{}, found(err, domain.ErrInvitationNotFound)
	}
	var joined Membership
	var how joining
	err = a.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		email, err := a.d.Accounts.ShareActiveAccount(ctx, actor.UserID)
		if err != nil {
			return err
		}
		w, err := a.d.Locker.LockWorkspaceByID(ctx, inv.WorkspaceID)
		if err != nil {
			return found(err, domain.ErrInvitationNotFound)
		}
		if inv, err = a.d.Invitations.LockPendingInvitation(ctx, id); err != nil {
			return found(err, domain.ErrInvitationNotFound)
		}
		if inv.Email != email {
			return domain.ErrInvitationEmailMismatch
		}
		now := a.d.Clock.Now()
		role, j, err := a.join(ctx, inv, actor.UserID, now)
		if err != nil {
			return err
		}
		joined, how = Membership{Workspace: w, Role: role}, j
		return a.d.Invitations.AcceptInvitation(ctx, id, actor.UserID, now)
	})
	if err != nil {
		return Membership{}, err
	}
	a.d.Logger.InfoContext(ctx, "workspace invitation accepted", slog.String("workspace_id", inv.WorkspaceID.String()),
		slog.String("invitation_id", id.String()), slog.String("membership", string(how)), slog.String("user_id", actor.UserID.String()))
	return joined, nil
}

// join gives userID a membership of inv's workspace with inv's role, as
// rule three admits, or keeps the active one it has, and returns its role.
func (a *AcceptInvitation) join(ctx context.Context, inv domain.Invitation, userID uuid.UUID, now time.Time) (shared.WorkspaceRole, joining, error) {
	m, err := a.d.Members.FindMembership(ctx, inv.WorkspaceID, userID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", "", err
	}
	had := err == nil // a membership, active or ended
	if had && m.Active() {
		return m.Role, joinKept, nil
	}
	if err := admit(ctx, a.d.Members, inv.WorkspaceID, inv.Role); err != nil {
		return "", "", err
	}
	if had {
		return inv.Role, joinRestored, restore(ctx, a.d.Updater, a.d.Subscribers, m, inv.Role, userID, now)
	}
	m = domain.Member{ID: uuid.NewV7(), WorkspaceID: inv.WorkspaceID, UserID: userID, Role: inv.Role, CreatedAt: now}
	return inv.Role, joinAdded, a.d.Updater.AddMember(ctx, m, userID)
}
