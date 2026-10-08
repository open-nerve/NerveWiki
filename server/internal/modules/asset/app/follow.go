package app

import (
	"context"
	"time"
	"uuid"
)

// Follow keeps the attachments' rows with their nodes and notebooks (M7/P2
// design 3.8): a node deleted, or a notebook, deletes its attachments'
// rows at its time, in its transaction, for the purge to take later with
// their files.
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

// NotebooksDeleted deletes at at the rows of the notebooks ids.
func (f Follow) NotebooksDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return f.rows.DeleteBlobsOfNotebooks(ctx, ids, at)
}
