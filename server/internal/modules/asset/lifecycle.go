package asset

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// The attachments' parts in the other modules' events (M7/P2 design 3.8),
// built from the pool alone (v0.1 design 13.1, item 21): the command
// line's compositions build them, as they build every registrant, though
// no command deletes a node or a notebook. Each runs in its caller's
// transaction, which the store finds in the context.

// PageObserver follows the page module's units: bootstrap hands it the
// nodes a unit deleted, whose attachments' rows it deletes at the unit's
// time.
type PageObserver interface {
	NodesDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// NewPageObserver returns the observer.
func NewPageObserver(pool *pgxpool.Pool) PageObserver {
	return app.NewFollow(postgresadapter.New(pool))
}

// NotebookDeletion follows the notebook module's deletions: it deletes
// the deleted notebooks' attachments' rows at the deletion's time.
type NotebookDeletion interface {
	NotebooksDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// NewNotebookDeletion returns the subscriber.
func NewNotebookDeletion(pool *pgxpool.Pool) NotebookDeletion {
	return app.NewFollow(postgresadapter.New(pool))
}

// Activity is the attachments' part in a notebook's activity: the bytes
// of its attachments not deleted.
type Activity = app.Activity

// Activities reads it: bootstrap converts it into the notebook module's
// activity source.
type Activities = app.Activities

// NewNotebookActivity returns the attachments' activity. It reads in its
// caller's read, unlocked; a notebook without an attachment is not in its
// answer.
func NewNotebookActivity(pool *pgxpool.Pool) Activities {
	return postgresadapter.New(pool)
}
