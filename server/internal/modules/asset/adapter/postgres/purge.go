package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres/gen"
)

// ExpiredBlobs implements app.ExpiredRows.
func (s *Store) ExpiredBlobs(ctx context.Context, before time.Time, batch int) ([]uuid.UUID, error) {
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
