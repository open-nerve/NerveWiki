package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// LeaveWorkspaceDeps are what LeaveWorkspace needs.
type LeaveWorkspaceDeps struct {
	Locker WorkspaceLocker
	Finder MemberFinder
	Ender  MembershipEnder
	Auth   shared.Authorizer
	Tx     shared.TxManager
	Clock  Clock
	Logger *slog.Logger
}

// LeaveWorkspace ends the caller's own membership:
// POST /api/v0/workspaces/{slug}/leave (M2/P2 design 3.2, 3.3).
type LeaveWorkspace struct {
	d LeaveWorkspaceDeps
}

// NewLeaveWorkspace returns the use case.
func NewLeaveWorkspace(d LeaveWorkspaceDeps) *LeaveWorkspace {
	return &LeaveWorkspace{d: d}
}

// Execute ends the caller's membership of the workspace of slug through
// the extension point. Under the workspace's row lock, the decision tells
// the caller's role; the workspace's only active admin cannot leave it
// (rule one), even alone in it, and can delete it instead. Two admins
// leaving at once count each other in turn, so one of them stays.
func (l *LeaveWorkspace) Execute(ctx context.Context, slug string) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	if !domain.ValidSlug(slug) {
		return domain.ErrNotFound
	}
	var left uuid.UUID
	err = l.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		w, err := l.d.Locker.LockWorkspaceBySlug(ctx, slug)
		if err != nil {
			return found(err, domain.ErrNotFound)
		}
		grant, err := authorize(ctx, l.d.Auth, actor, domain.ActionLeave, w.ID, domain.ErrNotFound)
		if err != nil {
			return err
		}
		if grant.WorkspaceRole == shared.WorkspaceAdmin {
			admins, err := l.d.Finder.CountActiveAdmins(ctx, w.ID)
			if err != nil {
				return err
			}
			if admins <= 1 {
				return domain.ErrSoleAdmin
			}
		}
		left = w.ID
		return l.d.Ender.End(ctx, MembershipEnd{
			UserID: actor.UserID, WorkspaceIDs: []uuid.UUID{w.ID}, Cause: EndLeft, By: actor.UserID, At: l.d.Clock.Now(),
		})
	})
	if err != nil {
		return err
	}
	l.d.Logger.InfoContext(ctx, "workspace left",
		slog.String("workspace_id", left.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
