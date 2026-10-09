package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"
)

// ContentSize is a page's content's size and the time of its last write.
type ContentSize struct {
	Bytes     int
	UpdatedAt time.Time
}

// ContentSizes is the size of each content not deleted of the pages ids,
// by page, read in the caller's transaction (M7/P5 design 3.8).
func (s *Store) ContentSizes(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]ContentSize, error) {
	rows, err := s.planned(ctx).ContentSizes(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("content sizes: %w", err)
	}
	out := make(map[uuid.UUID]ContentSize, len(rows))
	for _, r := range rows {
		out[r.NodeID] = ContentSize{Bytes: int(r.ByteSize), UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

// Contents is each content not deleted of the pages ids, by page, read in
// the caller's transaction.
func (s *Store) Contents(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := s.queries(ctx).Contents(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("contents: %w", err)
	}
	out := make(map[uuid.UUID]string, len(rows))
	for _, r := range rows {
		out[r.NodeID] = r.Content
	}
	return out, nil
}
