package app

import (
	"context"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeleteWorkspaceDeps are what DeleteWorkspace needs.
type DeleteWorkspaceDeps struct {
	Locker      WorkspaceLocker
	Workspaces  WorkspaceUpdater
	Members     MemberUpdater
	Subscribers []WorkspaceDeletionSubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// DeleteWorkspace deletes a workspace softly, with its members:
// DELETE /api/v0/workspaces/{slug} (M2/P2 design 3.2, 3.5).
type DeleteWorkspace struct {
	d DeleteWorkspaceDeps
}

// NewDeleteWorkspace returns the use case.
func NewDeleteWorkspace(d DeleteWorkspaceDeps) *DeleteWorkspace {
	return &DeleteWorkspace{d: d}
}

// Execute deletes the workspace of slug. Under its row lock and the
// decision, every membership of it, ended ones too, then the workspace are
// deleted at one time, which the deletion's subscribers take for their
// rows, in the same transaction. Its slug is free at once.
func (d *DeleteWorkspace) Execute(ctx context.Context, slug string) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	if !domain.ValidSlug(slug) {
		return domain.ErrNotFound
	}
	var deleted WorkspaceDeletion
	err = d.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		w, err := d.d.Locker.LockWorkspaceBySlug(ctx, slug)
		if err != nil {
			return found(err, domain.ErrNotFound)
		}
		if _, err := authorize(ctx, d.d.Auth, actor, domain.ActionDelete, w.ID, domain.ErrNotFound); err != nil {
			return err
		}
		deleted = WorkspaceDeletion{WorkspaceID: w.ID, By: actor.UserID, At: d.d.Clock.Now()}
		if err := d.d.Members.DeleteMembersOf(ctx, w.ID, actor.UserID, deleted.At); err != nil {
			return err
		}
		if err := d.d.Workspaces.DeleteWorkspace(ctx, w.ID, actor.UserID, deleted.At); err != nil {
			return err
		}
		for _, s := range d.d.Subscribers {
			if err := s.WorkspaceDeleted(ctx, deleted); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	d.d.Logger.InfoContext(ctx, "workspace deleted",
		slog.String("workspace_id", deleted.WorkspaceID.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
