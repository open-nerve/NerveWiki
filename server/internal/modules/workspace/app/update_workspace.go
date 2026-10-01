package app

import (
	"context"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpdateWorkspaceDeps are what UpdateWorkspace needs.
type UpdateWorkspaceDeps struct {
	Locker     WorkspaceLocker
	Workspaces WorkspaceUpdater
	Auth       shared.Authorizer
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
}

// UpdateWorkspace renames a workspace: PATCH /api/v0/workspaces/{slug}
// (M2/P2 design 3.2).
type UpdateWorkspace struct {
	d UpdateWorkspaceDeps
}

// NewUpdateWorkspace returns the use case.
func NewUpdateWorkspace(d UpdateWorkspaceDeps) *UpdateWorkspace {
	return &UpdateWorkspace{d: d}
}

// Execute gives the workspace of slug the name, and returns it with the
// caller's role. It locks the workspace's row, then decides, then checks
// the name: a caller who cannot see the workspace gets 404, not 422.
func (u *UpdateWorkspace) Execute(ctx context.Context, slug, name string) (Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Membership{}, err
	}
	if !domain.ValidSlug(slug) {
		return Membership{}, domain.ErrNotFound
	}
	var out Membership
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		w, err := u.d.Locker.LockWorkspaceBySlug(ctx, slug)
		if err != nil {
			return found(err, domain.ErrNotFound)
		}
		grant, err := authorize(ctx, u.d.Auth, actor, domain.ActionUpdate, w.ID, domain.ErrNotFound)
		if err != nil {
			return err
		}
		name, err := domain.CheckName(name)
		if err != nil {
			return err
		}
		now := u.d.Clock.Now()
		if err := u.d.Workspaces.RenameWorkspace(ctx, w.ID, name, actor.UserID, now); err != nil {
			return err
		}
		w.Name, w.UpdatedAt = name, now
		out = Membership{Workspace: w, Role: grant.WorkspaceRole}
		return nil
	})
	if err != nil {
		return Membership{}, err
	}
	u.d.Logger.InfoContext(ctx, "workspace renamed",
		slog.String("workspace_id", out.Workspace.ID.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
