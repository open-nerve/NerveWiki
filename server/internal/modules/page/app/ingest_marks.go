package app

import (
	"context"
	"uuid"
)

// IngestMarkResult is the number newly appended and the persisted total.
// A count of 10000 is a lower bound; marks beyond it are not retained.
type IngestMarkResult struct{ Added, Count int }

// IngestMarks appends sorted, unique proven sources to an owned live edit
// changeset, in the caller's transaction. Source IDs need not remain live.
type IngestMarks interface {
	Add(ctx context.Context, notebookID, changesetID uuid.UUID, sourceIDs []uuid.UUID) (IngestMarkResult, error)
}
