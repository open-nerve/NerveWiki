package app

import (
	"context"
	"uuid"
)

// NotebooksDeleted is notebooks being deleted, as the notebook module's
// deletion tells it: bootstrap converts it.
type NotebooksDeleted struct {
	NotebookIDs []uuid.UUID
}

// NotebookDeletion is the module's registrant of the notebook module's
// deletion (M6/P3 design 3.5): it drops the notebooks' index in the
// deletion's transaction, which holds their rows FOR NO KEY UPDATE, so
// that no page write of theirs, and no reindex, runs beside it.
type NotebookDeletion struct {
	Store Store
}

// NotebookDeleted follows the deletion d.
func (n NotebookDeletion) NotebookDeleted(ctx context.Context, d NotebooksDeleted) error {
	return n.Store.DeleteNotebooks(ctx, d.NotebookIDs)
}
