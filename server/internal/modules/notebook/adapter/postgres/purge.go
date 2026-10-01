package postgresadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
)

// The purge's statements (v0.1 design 13.1, item 6): each deletes up to
// batch rows deleted before before, skipping those another transaction
// holds, and returns how many it deleted.

// PurgeMembers purges the member rows, those of the deleted notebooks.
func (s *Store) PurgeMembers(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeMembers(ctx, gen.PurgeMembersParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge notebook members: %w", err)
	}
	return int(n), nil
}

// PurgeAuditEvents purges the audit events, those of the deleted
// workspaces.
func (s *Store) PurgeAuditEvents(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeAuditEvents(ctx, gen.PurgeAuditEventsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge notebook audit events: %w", err)
	}
	return int(n), nil
}

// PurgeNotebooks purges the notebooks whose member rows are gone.
func (s *Store) PurgeNotebooks(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeNotebooks(ctx, gen.PurgeNotebooksParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge notebooks: %w", err)
	}
	return int(n), nil
}
