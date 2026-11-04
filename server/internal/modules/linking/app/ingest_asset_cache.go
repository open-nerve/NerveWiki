package app

import (
	"context"
	"slices"
	"uuid"
)

const ingestAssetCacheRows = 10_000

// One private capture's last complete input and qualified result. Both are
// owned snapshots: neither the raw producer nor a followed callback can mutate
// the cache. This bounds caching only; longer inputs are still read in full.
type ingestAssetCache struct {
	raw, qualified []uuid.UUID
	ready          bool
}

func (c *ingestAssetCache) qualify(ids []uuid.UUID, qualify func([]uuid.UUID) ([]uuid.UUID, error)) ([]uuid.UUID, error) {
	if c == nil { // The public Capture route never caches qualification work.
		return qualify(ids)
	}
	if c.ready && (ids == nil) == (c.raw == nil) && slices.Equal(ids, c.raw) {
		// The current source owns ids. Keep the original exact-capacity rule;
		// never expose either cached snapshot or a previous source's output.
		if cap(ids) != len(c.qualified) {
			ids = make([]uuid.UUID, len(c.qualified))
		} else {
			ids = ids[:len(c.qualified)]
		}
		copy(ids, c.qualified)
		return ids, nil
	}
	c.raw, c.qualified, c.ready = nil, nil, false
	cacheable := len(ids) <= ingestAssetCacheRows
	var raw []uuid.UUID
	if cacheable {
		raw = copyIngestAssetIDs(ids) // Before any in-place filter, sort or compaction.
	}
	out, err := qualify(ids)
	if err != nil {
		return nil, err
	}
	if cacheable {
		c.raw, c.qualified, c.ready = raw, copyIngestAssetIDs(out), true
	}
	return out, nil
}

func copyIngestAssetIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return nil
	}
	out := make([]uuid.UUID, len(ids))
	copy(out, ids)
	return out
}

func (r *ingestBeforeRead) assets(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	// Load every asset-flagged target, including a real page and its head.
	// On an exact cache hit all these IDs already exist in this same reader;
	// its original load would issue no SQL and has no additional ctx check.
	if err := r.load(ctx, ids); err != nil {
		return nil, err
	}
	kept := 0
	for _, target := range ids {
		n := r.nodes[target]
		if n.Exists && n.InSource && !n.IsSourceRoot && n.Kind == "asset" {
			ids[kept] = target
			kept++
		}
	}
	ids = sortedIngestIDs(ids[:kept])
	if len(ids) != cap(ids) {
		exact := make([]uuid.UUID, len(ids))
		copy(exact, ids)
		ids = exact
	}
	return ids, nil
}
