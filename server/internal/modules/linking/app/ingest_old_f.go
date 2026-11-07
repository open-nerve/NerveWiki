package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"
)

// ingestOldBodyBatches is private to the locked pre-Replace capture. One native
// result is fully closed before it is buffered here; each app prefix remains
// at most LintReadRows. Public Bodies retains its own 512-row query boundary.
func ingestOldBodyBatches(notebook uuid.UUID, emitters []uuid.UUID,
	fetch func(context.Context, uuid.UUID, []uuid.UUID, *EdgePos, int) ([]BodyReference, error),
) *LintBatches[BodyReference] {
	ids := slices.Clone(emitters)
	var buffered []BodyReference
	var cursor EdgePos
	haveCursor, done := false, false
	return &LintBatches[BodyReference]{next: func(ctx context.Context) ([]BodyReference, error) {
		if done {
			return nil, nil
		}
		if len(buffered) == 0 {
			var after *EdgePos
			if haveCursor {
				// One owned value per query, never an address into returned rows
				// or one escaping allocation for every body reference.
				p := cursor
				after = &p
			}
			var err error
			buffered, err = fetch(ctx, notebook, ids, after, IngestOldBodyReadRows)
			if err != nil {
				return nil, err
			}
			if len(buffered) > IngestOldBodyReadRows {
				return nil, fmt.Errorf("ingest old bodies: %d body references exceed batch", len(buffered))
			}
			if len(buffered) == 0 {
				done = true
				buffered = nil
				return nil, nil
			}
		}
		n := min(LintReadRows, len(buffered))
		out := buffered[:n:n]
		// Keep logical row validation at the existing app-prefix boundary.
		// Only native prefetch errors gain the larger physical lookahead.
		for _, row := range out {
			if row.Start < 0 || haveCursor && row.Compare(cursor) <= 0 {
				return nil, errors.New("lint reads: body references do not advance")
			}
			cursor, haveCursor = row.EdgePos, true
		}
		buffered = buffered[n:]
		if len(buffered) == 0 {
			buffered = nil
		}
		return out, nil
	}}
}

// referenceIngestBodyAssets consumes every body row of the complete source
// relation, retaining only asset-flagged target IDs for one source at a time.
// emit must load every such ID before applying actual target eligibility.
func referenceIngestBodyAssets(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID, oldBodies IngestOldBodyReads,
	emit func(uuid.UUID, []uuid.UUID) error,
) error {
	if len(ids) == 0 {
		return nil
	}
	batches := ingestOldBodyBatches(notebook, ids, oldBodies.OldBodyReferences)
	var body []BodyReference
	done := false
	next := func() error {
		if len(body) == 0 && !done {
			var err error
			body, err = batches.Next(ctx)
			if err != nil {
				return err
			}
			done = len(body) == 0
		}
		return nil
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		assets := make([]uuid.UUID, 0)
		for {
			if err := next(); err != nil {
				return err
			}
			if len(body) == 0 || body[0].SourceID != id {
				break
			}
			row := body[0]
			body = body[1:]
			if row.ResolvedAsset {
				assets = append(assets, row.ResolvedID)
			}
		}
		if len(body) > 0 && body[0].SourceID.Compare(id) < 0 {
			return errors.New("ingest: unexpected body page")
		}
		if err := emit(id, assets); err != nil {
			return err
		}
	}
	if err := next(); err != nil {
		return err
	}
	if !done {
		return errors.New("ingest: references outside requested pages")
	}
	return ctx.Err()
}
