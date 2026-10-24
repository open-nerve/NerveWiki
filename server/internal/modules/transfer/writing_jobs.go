package transfer

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres"
)

// WritingJobs tells whether a notebook has a job that writes its tree, an
// import, a trash restore or a revert, queued or running, anyone's (M10
// design 4.5): the notebook module's downgrade waits for them. It reads in
// the caller's transaction when ctx carries one. Built from the pool alone,
// as NewNotebookDeletion.
type WritingJobs struct{ store *postgresadapter.Store }

// NewWritingJobs returns the reader.
func NewWritingJobs(pool *pgxpool.Pool) *WritingJobs {
	return &WritingJobs{store: postgresadapter.New(pool)}
}

// Writing reports whether notebookID has a job that writes queued or
// running: the one the writing key admits.
func (w *WritingJobs) Writing(ctx context.Context, notebookID uuid.UUID) (bool, error) {
	return w.store.Writing(ctx, notebookID)
}
