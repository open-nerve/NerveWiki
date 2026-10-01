package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// DeleteNotebookDeps are what DeleteNotebook needs.
type DeleteNotebookDeps struct {
	Workspaces  Workspaces
	Finder      NotebookFinder
	Notebooks   NotebookWriter
	Subscribers []NotebookDeletionSubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// DeleteNotebook deletes a notebook softly, with its members:
// DELETE /api/v0/notebooks/{notebook_id} (M3/P1 design 3.7, 3.8).
type DeleteNotebook struct {
	d DeleteNotebookDeps
	m manager
}

// NewDeleteNotebook returns the use case.
func NewDeleteNotebook(d DeleteNotebookDeps) *DeleteNotebook {
	return &DeleteNotebook{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute deletes notebook id. Under the locks and the decision, the
// notebook and every membership of it, ended ones too, are deleted at one
// time, which the deletion's subscribers take for their rows, in the same
// transaction.
func (d *DeleteNotebook) Execute(ctx context.Context, id uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	n, err := find(ctx, d.d.Finder, id)
	if err != nil {
		return err
	}
	err = d.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		n, _, err := d.m.lock(ctx, actor, domain.ActionDelete, n, domain.ErrNotFound)
		if err != nil {
			return err
		}
		deleted := NotebookDeletion{WorkspaceID: n.WorkspaceID, NotebookIDs: []uuid.UUID{n.ID}, By: actor.UserID, At: d.d.Clock.Now()}
		if err := d.d.Notebooks.DeleteNotebook(ctx, n.ID, actor.UserID, deleted.At); err != nil {
			return err
		}
		return publishDeletion(ctx, d.d.Subscribers, deleted)
	})
	if err != nil {
		return err
	}
	d.d.Logger.InfoContext(ctx, "notebook deleted", slog.String("workspace_id", n.WorkspaceID.String()),
		slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()))
	return nil
}
