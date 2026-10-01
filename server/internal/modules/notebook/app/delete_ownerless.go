package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeleteOwnerlessNotebookDeps are what DeleteOwnerlessNotebook needs.
type DeleteOwnerlessNotebookDeps struct {
	Workspaces  Workspaces
	Finder      NotebookFinder
	Notebooks   NotebookWriter
	Audit       AuditRecorder
	Subscribers []NotebookDeletionSubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// DeleteOwnerlessNotebook has a workspace admin delete an ownerless
// notebook: DELETE /api/v0/ownerless-notebooks/{notebook_id} (M3/P3
// design 3.3).
type DeleteOwnerlessNotebook struct {
	d DeleteOwnerlessNotebookDeps
}

// NewDeleteOwnerlessNotebook returns the use case.
func NewDeleteOwnerlessNotebook(d DeleteOwnerlessNotebookDeps) *DeleteOwnerlessNotebook {
	return &DeleteOwnerlessNotebook{d: d}
}

// Execute deletes ownerless notebook id as DeleteNotebook does, its
// members with it at one time and the deletion's subscribers told, and
// records the deletion.
func (d *DeleteOwnerlessNotebook) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	n, err := find(ctx, d.d.Finder, id)
	if err != nil {
		return err
	}
	err = d.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		n, err := lockOwnerless(ctx, d.d.Workspaces, d.d.Notebooks, d.d.Auth, actor, domain.ActionDeleteOwnerless, n)
		if err != nil {
			return err
		}
		now := d.d.Clock.Now()
		if err := d.d.Notebooks.DeleteNotebook(ctx, n.ID, actor.UserID, now); err != nil {
			return err
		}
		e := domain.AuditEvent{
			ID: uuid.NewV7(), WorkspaceID: n.WorkspaceID, NotebookID: n.ID, NotebookName: n.Name, Action: domain.AuditDeleted,
			FormerOwnerID: n.Ownerless.FormerOwner, ActorID: actor.UserID, At: now,
		}
		if err := d.d.Audit.AddAuditEvent(ctx, e); err != nil {
			return err
		}
		return publishDeletion(ctx, d.d.Subscribers, NotebookDeletion{WorkspaceID: n.WorkspaceID, NotebookIDs: []uuid.UUID{n.ID},
			By: actor.UserID, At: now})
	})
	if err != nil {
		return err
	}
	d.d.Logger.InfoContext(ctx, "ownerless notebook deleted", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
