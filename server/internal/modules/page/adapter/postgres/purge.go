package postgresadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
)

// The purge's statements (v0.1 design 13.1, item 6; M4/P1 design 3.10):
// each deletes up to batch rows deleted before before, skipping those
// another transaction holds, and returns how many it deleted. Leaf to
// root: what follows a node, the nodes, the changesets.

// PurgeChangesetItems purges the changeset items.
func (s *Store) PurgeChangesetItems(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeChangesetItems(ctx, gen.PurgeChangesetItemsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge changeset items: %w", err)
	}
	return int(n), nil
}

// PurgePageRevisions purges the page versions.
func (s *Store) PurgePageRevisions(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgePageRevisions(ctx, gen.PurgePageRevisionsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge page revisions: %w", err)
	}
	return int(n), nil
}

// PurgePageContents purges the page contents.
func (s *Store) PurgePageContents(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgePageContents(ctx, gen.PurgePageContentsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge page contents: %w", err)
	}
	return int(n), nil
}

// PurgeNodes purges the nodes, leaves first: one statement deletes the
// leaves it sees when it begins, which takes one level of a tree, so it
// runs again, each statement seeing the last one's deletions, until the
// batch is full or a statement deletes nothing. A deleted notebook's tree
// is gone in one run, before the notebook's purger needs it gone.
func (s *Store) PurgeNodes(ctx context.Context, before time.Time, batch int) (int, error) {
	total := 0
	for total < batch {
		n, err := s.queries(ctx).PurgeNodeLeaves(ctx, gen.PurgeNodeLeavesParams{Before: before, Batch: int32(batch - total)})
		if err != nil {
			return total, fmt.Errorf("purge nodes: %w", err)
		}
		if n == 0 {
			break
		}
		total += int(n)
	}
	return total, nil
}

// PurgeChangesets purges the changesets whose items and versions are gone.
func (s *Store) PurgeChangesets(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeChangesets(ctx, gen.PurgeChangesetsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge changesets: %w", err)
	}
	return int(n), nil
}
