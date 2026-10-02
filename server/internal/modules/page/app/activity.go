package app

import (
	"context"
	"time"
	"uuid"
)

// NotebookActivity is the pages' part in a notebook's activity: the bytes
// of its pages, and its changesets' last write. The module hands it to
// bootstrap, which hands it to the notebook module's ownerless list; no
// use case reads it.
type NotebookActivity struct {
	Bytes       int64
	LastWriteAt time.Time
}

// Activities reads the pages' part in notebooks' activity, unlocked: a
// notebook that never had a page is not in the answer.
type Activities interface {
	NotebookActivities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]NotebookActivity, error)
}
