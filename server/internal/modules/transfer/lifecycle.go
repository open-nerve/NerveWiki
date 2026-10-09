package transfer

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
)

// NotebookDeletion follows the notebook module's deletions (M7 design
// 4.12): it deletes the deleted notebooks' jobs at the deletion's time, in
// its transaction; a queued job then does nothing, a running one stops at
// its next heartbeat. Built from the pool alone (v0.1 design 13.1, item
// 21).
type NotebookDeletion interface {
	NotebooksDeleted(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// NewNotebookDeletion returns the subscriber.
func NewNotebookDeletion(pool *pgxpool.Pool) NotebookDeletion {
	return app.NewFollow(postgresadapter.New(pool))
}
