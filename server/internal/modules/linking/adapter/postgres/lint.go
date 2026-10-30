package postgresadapter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// LintReads reads bounded projections in the caller's transaction.
type LintReads struct {
	store           *Store
	maxPayloadBytes int64
}

// NewLintReads takes the page module's fixed content limit, not user configuration.
func NewLintReads(pool *pgxpool.Pool, maxContentBytes int) *LintReads {
	return &LintReads{store: New(pool), maxPayloadBytes: app.MaxLintPayloadBytes(maxContentBytes)}
}

var _ app.LintReadStore = (*LintReads)(nil)

// Heads keeps all stored versions so stale pages can be counted once.
func (r *LintReads) Heads(ctx context.Context, notebook uuid.UUID) ([]app.IndexedHead, error) {
	rows, err := r.store.planned(ctx).LintHeads(ctx, notebook)
	if err != nil {
		return nil, err
	}
	out := make([]app.IndexedHead, len(rows))
	for i, v := range rows {
		out[i] = app.IndexedHead{ID: v.NodeID, Extractor: int(v.Extractor), FrontmatterValid: v.FrontmatterValid}
	}
	return out, nil
}

func lintTake(take int, after *app.EdgePos) error {
	if take < 1 || take > app.LintReadRows {
		return errors.New("lint reads: take must be between 1 and 512")
	}
	if after != nil && after.Start < 0 {
		return errors.New("lint reads: negative cursor position")
	}
	return nil
}
func lintPositions(positions []app.EdgePos) ([]uuid.UUID, []int32, error) {
	if len(positions) > app.LintReadRows {
		return nil, nil, errors.New("lint reads: too many positions")
	}
	ids := make([]uuid.UUID, len(positions))
	starts := make([]int32, len(positions))
	for i, p := range positions {
		if p.Start < 0 || (i > 0 && positions[i-1].Compare(p) >= 0) {
			return nil, nil, errors.New("lint reads: positions must be nonnegative, unique and sorted")
		}
		ids[i] = p.SourceID
		starts[i] = p.Start
	}
	return ids, starts, nil
}
func lintIDs(ids []uuid.UUID) error {
	if len(ids) > app.LintReadRows {
		return errors.New("lint reads: too many page IDs")
	}
	for i, id := range ids {
		if i > 0 && bytes.Compare(ids[i-1][:], id[:]) >= 0 {
			return errors.New("lint reads: page IDs must be unique and sorted")
		}
	}
	return nil
}

// WantedEdgeHeads reads only short heads, with a separate initial statement.
func (r *LintReads) WantedEdgeHeads(ctx context.Context, notebook uuid.UUID, excluded []uuid.UUID, after *app.EdgePos, take int) ([]app.EdgeHead, error) {
	if err := lintTake(take, after); err != nil {
		return nil, err
	}
	var out []app.EdgeHead
	if after == nil {
		rows, err := r.store.planned(ctx).LintWantedEdgeHeadsFirst(ctx, gen.LintWantedEdgeHeadsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: int32(take)})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out = append(out, app.EdgeHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, TargetBytes: v.TargetBytes})
		}
	} else {
		rows, err := r.store.planned(ctx).LintWantedEdgeHeadsAfter(ctx, gen.LintWantedEdgeHeadsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: int32(take), AfterSource: after.SourceID, AfterStart: after.Start})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out = append(out, app.EdgeHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, TargetBytes: v.TargetBytes})
		}
	}
	for _, h := range out {
		if h.TargetBytes < 1 || h.TargetBytes > r.maxPayloadBytes {
			return nil, fmt.Errorf("lint reads: target exceeds derived bound in notebook %s source %s", notebook, h.SourceID)
		}
	}
	return out, nil
}

// AmbiguousEdgeHeads reads only short heads, with a separate initial statement.
func (r *LintReads) AmbiguousEdgeHeads(ctx context.Context, notebook uuid.UUID, excluded []uuid.UUID, after *app.EdgePos, take int) ([]app.EdgeHead, error) {
	if err := lintTake(take, after); err != nil {
		return nil, err
	}
	var out []app.EdgeHead
	if after == nil {
		rows, err := r.store.planned(ctx).LintAmbiguousEdgeHeadsFirst(ctx, gen.LintAmbiguousEdgeHeadsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: int32(take)})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out = append(out, app.EdgeHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, TargetBytes: v.TargetBytes})
		}
	} else {
		rows, err := r.store.planned(ctx).LintAmbiguousEdgeHeadsAfter(ctx, gen.LintAmbiguousEdgeHeadsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, ExcludedIds: excluded, Take: int32(take), AfterSource: after.SourceID, AfterStart: after.Start})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			out = append(out, app.EdgeHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, TargetBytes: v.TargetBytes})
		}
	}
	for _, h := range out {
		if h.TargetBytes < 1 || h.TargetBytes > r.maxPayloadBytes {
			return nil, fmt.Errorf("lint reads: target exceeds derived bound in notebook %s source %s", notebook, h.SourceID)
		}
	}
	return out, nil
}

// Edges fetches paired primary keys and rechecks notebook ownership.
func (r *LintReads) Edges(ctx context.Context, notebook uuid.UUID, positions []app.EdgePos) ([]app.LintEdge, error) {
	ids, starts, err := lintPositions(positions)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.store.planned(ctx).LintEdgesByPositions(ctx, gen.LintEdgesByPositionsParams{NotebookID: notebook, SourceIds: ids, Starts: starts})
	if err != nil {
		return nil, err
	}
	out := make([]app.LintEdge, len(rows))
	for i, v := range rows {
		out[i] = app.LintEdge{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, Kind: v.Kind, Target: v.Target, TargetKey: v.TargetKey, AltKey: v.TargetAltKey, ResolvedID: v.ResolvedID, ResolvedAsset: v.ResolvedAsset, Ambiguous: v.Ambiguous}
	}
	return out, nil
}

// SourcePropertyHeads probes only the first ASCII sources property per page.
// JSON serialization here and exact retrieval are two necessary DB reads.
func (r *LintReads) SourcePropertyHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) ([]app.SourcePropertyHead, error) {
	if err := lintIDs(ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.store.planned(ctx).LintFirstSourceHeads(ctx, gen.LintFirstSourceHeadsParams{NotebookID: notebook, Extractor: domain.Extractor, PageIds: ids})
	if err != nil {
		return nil, err
	}
	out := make([]app.SourcePropertyHead, len(rows))
	for i, v := range rows {
		if v.ValueBytes < 0 || v.ValueBytes > r.maxPayloadBytes {
			return nil, fmt.Errorf("lint reads: JSON exceeds derived bound in notebook %s source %s", notebook, v.NodeID)
		}
		out[i] = app.SourcePropertyHead{SourcePropertyPos: app.SourcePropertyPos{PageID: v.NodeID, Position: v.Position}, ValueBytes: v.ValueBytes}
	}
	return out, nil
}

// SourceProperties is an exact paired read, not another first-key probe.
func (r *LintReads) SourceProperties(ctx context.Context, notebook uuid.UUID, positions []app.SourcePropertyPos) ([]app.SourceProperty, error) {
	pairs := make([]app.EdgePos, len(positions))
	for i, p := range positions {
		pairs[i] = app.EdgePos{SourceID: p.PageID, Start: p.Position}
	}
	ids, starts, err := lintPositions(pairs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.store.planned(ctx).LintFirstSourceValues(ctx, gen.LintFirstSourceValuesParams{NotebookID: notebook, Extractor: domain.Extractor, PageIds: ids, Positions: starts})
	if err != nil {
		return nil, err
	}
	out := make([]app.SourceProperty, len(rows))
	for i, v := range rows {
		out[i] = app.SourceProperty{SourcePropertyPos: app.SourcePropertyPos{PageID: v.NodeID, Position: v.Position}, Key: v.Key, JSON: []byte(v.Value)}
	}
	return out, nil
}

func lintSpecs(specs []app.SourceSpec) (gen.LintSourceSlotHeadsFirstParams, error) {
	var p gen.LintSourceSlotHeadsFirstParams
	if len(specs) > app.LintReadRows {
		return p, errors.New("lint reads: too many source specs")
	}
	for i, s := range specs {
		if len(s.Key) != 7 || strings.ToLower(s.Key) != "sources" || s.Items < 0 || s.Items > 10000 || (!s.Array && s.Items != 0) {
			return p, errors.New("lint reads: invalid source spec shape")
		}
		if i > 0 && bytes.Compare(specs[i-1].PageID[:], s.PageID[:]) >= 0 {
			return p, errors.New("lint reads: source specs must be unique and sorted")
		}
		p.PageIds = append(p.PageIds, s.PageID)
		p.FirstKeys = append(p.FirstKeys, s.Key)
		p.IsArray = append(p.IsArray, s.Array)
		p.ItemCounts = append(p.ItemCounts, s.Items)
	}
	if len(specs) > 0 {
		p.BatchFirstSource = specs[0].PageID
		p.BatchLastSource = specs[len(specs)-1].PageID
	}
	return p, nil
}

// SourceSlotHeads restricts projected paths before materializing them and
// bounds the query to this JSON batch's disjoint source interval.
func (r *LintReads) SourceSlotHeads(ctx context.Context, notebook uuid.UUID, specs []app.SourceSpec, after *app.EdgePos, take int) ([]app.SourceSlotHead, error) {
	if err := lintTake(take, after); err != nil {
		return nil, err
	}
	p, err := lintSpecs(specs)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	p.NotebookID = notebook
	p.Extractor = domain.Extractor
	p.Take = int32(take)
	var out []app.SourceSlotHead
	if after == nil {
		rows, err := r.store.planned(ctx).LintSourceSlotHeadsFirst(ctx, p)
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			if v.PropertyKey == nil {
				return nil, errors.New("lint reads: slot head has no property key")
			}
			out = append(out, app.SourceSlotHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, PropertyKey: *v.PropertyKey, TargetBytes: v.TargetBytes})
		}
	} else {
		rows, err := r.store.planned(ctx).LintSourceSlotHeadsAfter(ctx, gen.LintSourceSlotHeadsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, Take: int32(take), PageIds: p.PageIds, FirstKeys: p.FirstKeys, IsArray: p.IsArray, ItemCounts: p.ItemCounts, BatchFirstSource: p.BatchFirstSource, BatchLastSource: p.BatchLastSource, AfterSource: after.SourceID, AfterStart: after.Start})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			if v.PropertyKey == nil {
				return nil, errors.New("lint reads: slot head has no property key")
			}
			out = append(out, app.SourceSlotHead{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, PropertyKey: *v.PropertyKey, TargetBytes: v.TargetBytes})
		}
	}
	for _, h := range out {
		if h.TargetBytes < 1 || h.TargetBytes > r.maxPayloadBytes {
			return nil, fmt.Errorf("lint reads: slot target exceeds derived bound in notebook %s source %s", notebook, h.SourceID)
		}
	}
	return out, nil
}

// SourceSlotEdges retains every matched edge's nullable origin/resolution.
// It does not qualify a target or collapse conflicting paths.
func (r *LintReads) SourceSlotEdges(ctx context.Context, notebook uuid.UUID, positions []app.EdgePos) ([]app.SourceSlotEdge, error) {
	ids, starts, err := lintPositions(positions)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.store.planned(ctx).LintSourceSlotEdges(ctx, gen.LintSourceSlotEdgesParams{NotebookID: notebook, PageIds: ids, Starts: starts})
	if err != nil {
		return nil, err
	}
	out := make([]app.SourceSlotEdge, len(rows))
	for i, v := range rows {
		out[i] = app.SourceSlotEdge{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, PropertyKey: v.PropertyKey, Target: v.Target, Kind: v.Kind, ResolvedID: v.ResolvedID, ResolvedAsset: v.ResolvedAsset, Ambiguous: v.Ambiguous}
	}
	return out, nil
}

// BodyReferences reads only short current-format body references. Emitters
// is the complete unique set; the SQL materializes it once per statement.
func (r *LintReads) BodyReferences(ctx context.Context, notebook uuid.UUID, emitters []uuid.UUID, after *app.EdgePos, take int) ([]app.BodyReference, error) {
	if err := lintTake(take, after); err != nil {
		return nil, err
	}
	if len(emitters) == 0 {
		return nil, nil
	}
	var out []app.BodyReference
	if after == nil {
		rows, err := r.store.planned(ctx).LintBodyReferencesFirst(ctx, gen.LintBodyReferencesFirstParams{NotebookID: notebook, Extractor: domain.Extractor, WikiPageIds: emitters, Take: int32(take)})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			if v.ResolvedID == nil {
				return nil, errors.New("lint reads: body reference has no target ID")
			}
			out = append(out, app.BodyReference{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, ResolvedID: *v.ResolvedID, ResolvedAsset: v.ResolvedAsset})
		}
	} else {
		rows, err := r.store.planned(ctx).LintBodyReferencesAfter(ctx, gen.LintBodyReferencesAfterParams{NotebookID: notebook, Extractor: domain.Extractor, WikiPageIds: emitters, Take: int32(take), AfterSource: after.SourceID, AfterStart: after.Start})
		if err != nil {
			return nil, err
		}
		for _, v := range rows {
			if v.ResolvedID == nil {
				return nil, errors.New("lint reads: body reference has no target ID")
			}
			out = append(out, app.BodyReference{EdgePos: app.EdgePos{SourceID: v.SourceID, Start: v.RangeStart}, ResolvedID: *v.ResolvedID, ResolvedAsset: v.ResolvedAsset})
		}
	}
	return out, nil
}
