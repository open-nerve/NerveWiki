package app

import (
	"context"
	"time"
	"uuid"
)

// UpgradesOfNotebooks soft-deletes deleted notebooks' upgrades.
type UpgradesOfNotebooks interface {
	SoftDeleteOf(ctx context.Context, notebookIDs []uuid.UUID, by uuid.UUID, at time.Time) error
}

// NotebookDeletion follows the notebook module's deletions (M11/P1 design
// 3.15): the deleted notebooks' upgrades, under way or ended, go with them
// at the deletion's time, in its transaction, which holds the notebooks'
// rows FOR NO KEY UPDATE.
type NotebookDeletion struct {
	Upgrades UpgradesOfNotebooks
}

// NotebooksDeleted follows the deletion of the notebooks ids by by at at.
func (d NotebookDeletion) NotebooksDeleted(ctx context.Context, ids []uuid.UUID, by uuid.UUID, at time.Time) error {
	return d.Upgrades.SoftDeleteOf(ctx, ids, by, at)
}
