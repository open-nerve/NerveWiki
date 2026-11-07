package postgresadapter

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// setResolutionBatchQuerier is used only by ordered SetResolutions batches.
// The caller selects from the actual Tx's description capacity once. Cached
// descriptions still execute an unnamed statement; no named plan is reused.
type setResolutionBatchQuerier struct {
	postgres.Querier
	mode pgx.QueryExecMode
}

func (q setResolutionBatchQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	mode := q.mode
	return q.Querier.Exec(ctx, sql, append([]any{mode}, args...)...)
}
