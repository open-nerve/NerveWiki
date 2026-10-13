package app

import (
	"context"
	"time"
	"uuid"
)

// Follow keeps the attachments' rows with their nodes and notebooks (M7/P2
// design 3.8): a node deleted, or a notebook, deletes its attachments'
// rows at its time, in its transaction, for the purge to take later with
// their files; a node restored brings its row back (M8/P2 design 3.6).
type Follow struct {
	rows Deletions
}

// NewFollow returns it.
func NewFollow(rows Deletions) Follow {
	return Follow{rows: rows}
}

// NodesDeleted deletes at at the rows of the nodes ids, of whatever kind:
// a page has none. Nothing for no node.
func (f Follow) NodesDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return f.rows.DeleteBlobsOfNodes(ctx, ids, at)
}

// NodesRestored brings back the rows of the nodes ids deleted at
// deletedAt, their nodes' time, and tells how many: one an attachment
// restored, unless the purge took it. Nothing for no node.
func (f Follow) NodesRestored(ctx context.Context, ids []uuid.UUID, deletedAt time.Time) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	return f.rows.RestoreBlobsOfNodes(ctx, ids, deletedAt)
}

// NotebooksDeleted deletes at at the rows of the notebooks ids.
func (f Follow) NotebooksDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return f.rows.DeleteBlobsOfNotebooks(ctx, ids, at)
}
