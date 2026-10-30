package app_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// lintData replaces only database/asset ports. The production bounded streams,
// domain judgments, and Prepare remain real. Fixtures provide stored rows,
// never finished rule items. Unexpected scopes or unbounded reads are refused.
type lintData struct {
	nb          uuid.UUID
	heads       []app.IndexedHead
	edges       []app.LintEdge
	properties  []app.SourceProperty
	slots       []app.SourceSlotEdge
	bodies      []app.BodyReference
	sqlIDs      map[domain.Rule][]uuid.UUID
	candidates  map[domain.Rule][]uuid.UUID
	excluded    []uuid.UUID
	parents     []uuid.UUID
	assetBytes  map[uuid.UUID]int64
	calls       []string
	assetCalls  [][]uuid.UUID
	valueSizes  []int
	propertyIDs map[uuid.UUID]int
	fail        string
	boom        error
	onCall      func(string)
}

func lintID(n uint64) uuid.UUID {
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], n)
	return id
}
func lintPtr[T any](v T) *T { return &v }

func lintNode(n uint64, parent *uuid.UUID, key string, kind domain.NodeKind, size int64) app.CompactNode {
	return app.CompactNode{ID: lintID(n), ParentID: parent, Kind: kind, NameKey: key,
		Linkable: kind == domain.NodeAsset, PageBytes: size,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second)}
}

func lintFixture(nodes ...app.CompactNode) (*lintData, app.ReadInput) {
	f := &lintData{nb: lintID(900000), sqlIDs: map[domain.Rule][]uuid.UUID{},
		candidates: map[domain.Rule][]uuid.UUID{}, assetBytes: map[uuid.UUID]int64{},
		propertyIDs: map[uuid.UUID]int{}, boom: errors.New("the data port failed")}
	for _, n := range nodes {
		if n.Kind == domain.NodePage {
			f.heads = append(f.heads, app.IndexedHead{ID: n.ID, Extractor: 2, FrontmatterValid: true})
		}
	}
	return f, app.ReadInput{NotebookID: f.nb, EffectiveMode: domain.ModeStandard, Nodes: nodes}
}

func (f *lintData) rules() *app.Rules {
	return app.NewRules(app.NewLintReads(f, 5<<20), f, f)
}
func (f *lintData) enter(ctx context.Context, name string, nb uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if nb != f.nb {
		return errors.New("fake rejected a foreign notebook")
	}
	f.calls = append(f.calls, name)
	if f.onCall != nil {
		f.onCall(name)
	}
	if f.fail == name {
		return f.boom
	}
	return nil
}
func lintSorted(ids []uuid.UUID, max int) bool {
	if max > 0 && len(ids) > max {
		return false
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1].Compare(ids[i]) >= 0 {
			return false
		}
	}
	return true
}
func (f *lintData) current(id uuid.UUID) bool {
	for _, h := range f.heads {
		if h.ID == id {
			return h.Extractor == 2
		}
	}
	return false
}
func (f *lintData) Heads(ctx context.Context, nb uuid.UUID) ([]app.IndexedHead, error) {
	if err := f.enter(ctx, "heads", nb); err != nil {
		return nil, err
	}
	return slices.Clone(f.heads), nil
}
func (f *lintData) edgeHeads(ctx context.Context, nb uuid.UUID, excluded []uuid.UUID, after *app.EdgePos, take int, ambiguous bool) ([]app.EdgeHead, error) {
	if err := f.enter(ctx, "edge-heads", nb); err != nil {
		return nil, err
	}
	if take != app.LintReadRows || !lintSorted(excluded, 0) {
		return nil, errors.New("fake rejected edge parameters")
	}
	if f.excluded != nil && !slices.Equal(excluded, f.excluded) {
		return nil, errors.New("fake rejected incomplete emitter exclusion")
	}
	var out []app.EdgeHead
	for _, e := range f.edges {
		if !f.current(e.SourceID) || slices.Contains(excluded, e.SourceID) ||
			after != nil && e.Compare(*after) <= 0 {
			continue
		}
		if ambiguous {
			if !e.Ambiguous || e.ResolvedID != nil && !e.ResolvedAsset && !f.current(*e.ResolvedID) {
				continue
			}
		} else if e.ResolvedID != nil || e.TargetKey == nil {
			continue
		}
		out = append(out, app.EdgeHead{EdgePos: e.EdgePos, TargetBytes: int64(len(e.Target))})
		if len(out) == take {
			break
		}
	}
	return out, nil
}
func (f *lintData) WantedEdgeHeads(ctx context.Context, nb uuid.UUID, excluded []uuid.UUID, after *app.EdgePos, take int) ([]app.EdgeHead, error) {
	return f.edgeHeads(ctx, nb, excluded, after, take, false)
}
func (f *lintData) AmbiguousEdgeHeads(ctx context.Context, nb uuid.UUID, excluded []uuid.UUID, after *app.EdgePos, take int) ([]app.EdgeHead, error) {
	return f.edgeHeads(ctx, nb, excluded, after, take, true)
}
func (f *lintData) Edges(ctx context.Context, nb uuid.UUID, positions []app.EdgePos) ([]app.LintEdge, error) {
	if err := f.enter(ctx, "edges", nb); err != nil {
		return nil, err
	}
	var out []app.LintEdge
	size := 0
	for _, p := range positions {
		for _, e := range f.edges {
			if e.EdgePos == p {
				out = append(out, e)
				size += len(e.Target)
			}
		}
	}
	f.valueSizes = append(f.valueSizes, size)
	return out, nil
}
func (f *lintData) SourcePropertyHeads(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) ([]app.SourcePropertyHead, error) {
	if err := f.enter(ctx, "source-heads", nb); err != nil {
		return nil, err
	}
	if !lintSorted(ids, app.LintReadRows) {
		return nil, errors.New("fake rejected source ids")
	}
	var out []app.SourcePropertyHead
	for _, p := range f.properties {
		if slices.Contains(ids, p.PageID) && f.current(p.PageID) {
			f.propertyIDs[p.PageID]++
			if f.propertyIDs[p.PageID] != 1 {
				return nil, errors.New("first sources value was requested twice")
			}
			out = append(out, app.SourcePropertyHead{SourcePropertyPos: p.SourcePropertyPos, ValueBytes: int64(len(p.JSON))})
		}
	}
	slices.SortFunc(out, func(a, b app.SourcePropertyHead) int { return a.PageID.Compare(b.PageID) })
	return out, nil
}
func (f *lintData) SourceProperties(ctx context.Context, nb uuid.UUID, positions []app.SourcePropertyPos) ([]app.SourceProperty, error) {
	if err := f.enter(ctx, "source-values", nb); err != nil {
		return nil, err
	}
	var out []app.SourceProperty
	for _, pos := range positions {
		for _, p := range f.properties {
			if pos == p.SourcePropertyPos {
				out = append(out, p)
			}
		}
	}
	return out, nil
}
func lintSlotMatches(key string, spec app.SourceSpec) bool {
	if !spec.Array {
		return key == spec.Key
	}
	suffix, ok := strings.CutPrefix(key, spec.Key+".")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(suffix)
	return err == nil && n >= 0 && n < int(spec.Items) && strconv.Itoa(n) == suffix
}
func (f *lintData) SourceSlotHeads(ctx context.Context, nb uuid.UUID, specs []app.SourceSpec, after *app.EdgePos, take int) ([]app.SourceSlotHead, error) {
	if err := f.enter(ctx, "slot-heads", nb); err != nil {
		return nil, err
	}
	if len(specs) > app.LintReadRows || take != app.LintReadRows {
		return nil, errors.New("fake rejected slot batch")
	}
	var out []app.SourceSlotHead
	for _, e := range f.slots {
		if !f.current(e.SourceID) || e.PropertyKey == nil || after != nil && e.Compare(*after) <= 0 {
			continue
		}
		for _, spec := range specs {
			if e.SourceID == spec.PageID && lintSlotMatches(*e.PropertyKey, spec) {
				out = append(out, app.SourceSlotHead{EdgePos: e.EdgePos, PropertyKey: *e.PropertyKey, TargetBytes: int64(len(e.Target))})
			}
		}
		if len(out) == take {
			break
		}
	}
	return out, nil
}
func (f *lintData) SourceSlotEdges(ctx context.Context, nb uuid.UUID, positions []app.EdgePos) ([]app.SourceSlotEdge, error) {
	if err := f.enter(ctx, "slot-values", nb); err != nil {
		return nil, err
	}
	var out []app.SourceSlotEdge
	for _, p := range positions {
		for _, e := range f.slots {
			if p == e.EdgePos {
				out = append(out, e)
			}
		}
	}
	return out, nil
}
func (f *lintData) BodyReferences(ctx context.Context, nb uuid.UUID, emitters []uuid.UUID, after *app.EdgePos, take int) ([]app.BodyReference, error) {
	if err := f.enter(ctx, "bodies", nb); err != nil {
		return nil, err
	}
	if take != app.LintReadRows || !lintSorted(emitters, 0) {
		return nil, errors.New("fake rejected body parameters")
	}
	var out []app.BodyReference
	for _, e := range f.bodies {
		if f.current(e.SourceID) && slices.Contains(emitters, e.SourceID) &&
			(after == nil || e.Compare(*after) > 0) {
			out = append(out, e)
			if len(out) == take {
				break
			}
		}
	}
	return out, nil
}
func (f *lintData) pageRows(ctx context.Context, in app.PageRuleInput, rule domain.Rule) ([]uuid.UUID, error) {
	if err := f.enter(ctx, "page-"+string(rule), in.NotebookID); err != nil {
		return nil, err
	}
	if !lintSorted(in.CandidateIDs, 0) || !lintSorted(in.ExcludedEmitters, 0) {
		return nil, errors.New("fake rejected page relations")
	}
	if want, ok := f.candidates[rule]; ok && !slices.Equal(want, in.CandidateIDs) {
		return nil, fmt.Errorf("fake rejected %s candidates: got %v, want %v", rule, in.CandidateIDs, want)
	}
	var out []uuid.UUID
	for _, id := range f.sqlIDs[rule] {
		if slices.Contains(in.CandidateIDs, id) && f.current(id) {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, func(a, b uuid.UUID) int { return a.Compare(b) })
	return out, nil
}
func (f *lintData) pageCount(ctx context.Context, in app.PageRuleInput, r domain.Rule) (domain.Count, error) {
	ids, err := f.pageRows(ctx, in, r)
	return domain.Count{Value: int64(min(len(ids), 10000)), Capped: len(ids) >= 10000}, err
}
func (f *lintData) pageItems(ctx context.Context, in app.PageRuleInput, r domain.Rule, after *uuid.UUID, take int) ([]uuid.UUID, error) {
	if take < 1 || take > 101 {
		return nil, errors.New("fake rejected page take")
	}
	ids, err := f.pageRows(ctx, in, r)
	var out []uuid.UUID
	for _, id := range ids {
		if after == nil || id.Compare(*after) > 0 {
			out = append(out, id)
			if len(out) == take {
				break
			}
		}
	}
	return out, err
}
func (f *lintData) LintOrphanCount(c context.Context, i app.PageRuleInput) (domain.Count, error) {
	return f.pageCount(c, i, domain.Orphans)
}
func (f *lintData) LintOrphanItems(c context.Context, i app.PageRuleInput, a *uuid.UUID, n int) ([]uuid.UUID, error) {
	return f.pageItems(c, i, domain.Orphans, a, n)
}
func (f *lintData) LintFrontmatterCount(c context.Context, i app.PageRuleInput) (domain.Count, error) {
	return f.pageCount(c, i, domain.InvalidFrontmatter)
}
func (f *lintData) LintFrontmatterItems(c context.Context, i app.PageRuleInput, a *uuid.UUID, n int) ([]uuid.UUID, error) {
	return f.pageItems(c, i, domain.InvalidFrontmatter, a, n)
}
func (f *lintData) LintMissingTypeCount(c context.Context, i app.PageRuleInput) (domain.Count, error) {
	return f.pageCount(c, i, domain.MissingType)
}
func (f *lintData) LintMissingTypeItems(c context.Context, i app.PageRuleInput, a *uuid.UUID, n int) ([]uuid.UUID, error) {
	return f.pageItems(c, i, domain.MissingType, a, n)
}
func (f *lintData) LintMissingSummaryCount(c context.Context, i app.PageRuleInput) (domain.Count, error) {
	return f.pageCount(c, i, domain.MissingSummary)
}
func (f *lintData) LintMissingSummaryItems(c context.Context, i app.PageRuleInput, a *uuid.UUID, n int) ([]uuid.UUID, error) {
	return f.pageItems(c, i, domain.MissingSummary, a, n)
}
func (f *lintData) LintPendingAssets(ctx context.Context, in app.PendingAssetInput, ids []uuid.UUID) ([]uuid.UUID, error) {
	if err := f.enter(ctx, "pending-assets", in.NotebookID); err != nil {
		return nil, err
	}
	if !lintSorted(ids, app.LintReadRows) || !lintSorted(in.ParentCandidates, 0) {
		return nil, errors.New("fake rejected pending relations")
	}
	if f.parents != nil && !slices.Equal(in.ParentCandidates, f.parents) {
		return nil, errors.New("fake rejected incomplete potential parents")
	}
	var out []uuid.UUID
	for _, id := range ids {
		found := false
		for _, e := range f.bodies {
			if e.ResolvedID == id && e.ResolvedAsset && slices.Contains(in.ParentCandidates, e.SourceID) && f.current(e.SourceID) {
				found = true
			}
		}
		if !found {
			out = append(out, id)
		}
	}
	return out, nil
}
func (f *lintData) BytesOf(ctx context.Context, nb uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	if err := f.enter(ctx, "asset-bytes", nb); err != nil {
		return nil, err
	}
	if len(ids) == 0 || !lintSorted(ids, 100) {
		return nil, errors.New("fake rejected asset projection")
	}
	f.assetCalls = append(f.assetCalls, slices.Clone(ids))
	out := map[uuid.UUID]int64{}
	for _, id := range ids {
		if size, ok := f.assetBytes[id]; ok {
			out[id] = size
		}
	}
	return out, nil
}
