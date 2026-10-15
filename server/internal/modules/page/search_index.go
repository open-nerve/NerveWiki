package page

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// SearchRun is what a run of the search's job did.
type SearchRun = app.SearchRun

// SearchIndex is the search's index outside the server's job (M8/P5 design
// 3.4): nervewiki reindex queues a notebook's nodes whose rows are stale,
// and the tests run the job themselves.
type SearchIndex struct {
	store *postgresadapter.Store
	job   *app.IndexSearch
}

// requeueBatch is how many nodes Requeue queues a statement.
const requeueBatch = 10_000

// NewSearchIndex returns the search's index over pool alone, its
// transactions committed within commitTimeout (database.commit_timeout).
func NewSearchIndex(pool *pgxpool.Pool, commitTimeout time.Duration) SearchIndex {
	store := postgresadapter.New(pool)
	return SearchIndex{store: store,
		job: app.NewIndexSearch(postgres.NewTxManager(pool, commitTimeout), store, clock.System{}, slog.New(slog.DiscardHandler), 0)}
}

// Requeue queues in bulk the live nodes of the notebook whose row is
// missing or of another tokenizer than the current, by id, a statement of
// requeueBatch nodes at a time, each its own transaction, and returns how
// many: the server's job builds their rows. A row of the current tokenizer
// stays as it is, so that a reindex for the links alone builds nothing.
func (s SearchIndex) Requeue(ctx context.Context, notebookID uuid.UUID) (int, error) {
	queued := 0
	after := uuid.Nil()
	for {
		ids, err := s.store.StaleSearchNodes(ctx, notebookID, after, domain.SearchTokenizer, requeueBatch)
		if err != nil || len(ids) == 0 {
			return queued, err
		}
		if err := s.store.QueueSearch(ctx, notebookID, ids, true); err != nil {
			return queued, err
		}
		queued += len(ids)
		after = ids[len(ids)-1]
	}
}

// Drain runs the job once, as the server's does, but builds a row again
// however recently it built it: a test empties the queue.
func (s SearchIndex) Drain(ctx context.Context) (SearchRun, error) {
	return s.job.Drain(ctx)
}
