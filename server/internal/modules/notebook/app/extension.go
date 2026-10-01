package app

import (
	"context"
	"time"
	"uuid"
)

// The module's extension point (M3 design 8), and its registrant of the
// workspace module's deletion. The registrants are built from the pool
// alone and composed in bootstrap's registrants.go; their statements reach
// the transaction through the context.

// NotebookDeletion is notebooks being deleted: one by deleteNotebook, every
// one of a workspace by the workspace's deletion. A subscriber deletes its
// own rows at At, so the cleanup removes them with the notebooks.
type NotebookDeletion struct {
	WorkspaceID uuid.UUID
	NotebookIDs []uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// NotebookDeletionSubscriber follows a deletion, after the notebooks and
// their members were deleted and in their transaction: an error rolls
// everything back. A deletion has no vetoer: it is an admin's explicit,
// destructive act.
type NotebookDeletionSubscriber interface {
	NotebookDeleted(ctx context.Context, d NotebookDeletion) error
}

// publishDeletion calls the subscribers in order; the first error stops it.
func publishDeletion(ctx context.Context, subscribers []NotebookDeletionSubscriber, d NotebookDeletion) error {
	for _, s := range subscribers {
		if err := s.NotebookDeleted(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// WorkspaceDeleted is a workspace being deleted, field by field as the
// workspace module's deletion tells it: bootstrap converts
// workspace.WorkspaceDeletion.
type WorkspaceDeleted struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// WorkspaceDeletion is the module's registrant of the workspace module's
// deletion (M3/P1 design 3.8): it deletes the workspace's notebooks and
// their members at the deletion's time, and tells the notebook deletion's
// subscribers once, with every id. It runs in the deletion's transaction,
// which holds the workspace's row FOR NO KEY UPDATE: no notebook write of
// the workspace runs beside it, as each takes the row FOR SHARE.
type WorkspaceDeletion struct {
	Notebooks   NotebooksDeleter
	Subscribers []NotebookDeletionSubscriber
}

// WorkspaceDeleted follows the deletion of d.WorkspaceID.
func (w WorkspaceDeletion) WorkspaceDeleted(ctx context.Context, d WorkspaceDeleted) error {
	ids, err := w.Notebooks.DeleteNotebooksOf(ctx, d.WorkspaceID, d.By, d.At)
	if err != nil || len(ids) == 0 {
		return err
	}
	return publishDeletion(ctx, w.Subscribers, NotebookDeletion{WorkspaceID: d.WorkspaceID, NotebookIDs: ids, By: d.By, At: d.At})
}
