package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// ExpiredBlobs implements app.ExpiredRows. It locks the rows for the
// batch's transaction: on the pool the locks would end with the
// statement, and two purges could take the same rows, so it is refused
// there.
func (s *Store) ExpiredBlobs(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
	if !postgres.InTx(ctx) {
		return nil, errors.New("expired blobs: not in a transaction, the rows' locks would end with the statement")
	}
	ids, err := s.queries(ctx).ExpiredBlobs(ctx, gen.ExpiredBlobsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return nil, fmt.Errorf("expired blobs: %w", err)
	}
	return ids, nil
}

// DeleteBlobs implements app.ExpiredRows.
func (s *Store) DeleteBlobs(ctx context.Context, ids []uuid.UUID) (int, error) {
	n, err := s.queries(ctx).DeleteBlobs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("delete blobs: %w", err)
	}
	return int(n), nil
}
