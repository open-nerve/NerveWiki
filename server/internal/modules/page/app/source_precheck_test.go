package app

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type sourceSnapshotKey struct{}
type sourceSnapshots struct {
	calls int
	fail  error
}

func (s *sourceSnapshots) WithinSnapshot(ctx context.Context, do func(context.Context) error) error {
	s.calls++
	if s.fail != nil {
		return s.fail
	}
	return do(context.WithValue(ctx, sourceSnapshotKey{}, true))
}
func sourceSequence(steps ...domain.StepFacts) iter.Seq[domain.StepFacts] {
	return func(yield func(domain.StepFacts) bool) {
		for _, s := range steps {
			if !yield(s) {
				return
			}
		}
	}
}
func TestSourcePrecheckUsesSnapshotAndWholeExclusionSet(t *testing.T) {
	key := probeKey(domain.KindPage, "same")
	a, b, q := probeID(1), probeID(2), probeID(3)
	raw := probeID(90)
	inside, outside := domain.NodePlace{InArea: true, ParentID: &raw}, domain.NodePlace{}
	f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{a: {ID: a, Kind: key.Kind, Hash: key.Hash[:], Bytes: 4, BeforeSourceHash: key.Hash[:]}, b: {ID: b, Kind: key.Kind, Hash: key.Hash[:], Bytes: 4}}, rows: map[SourceKey][]uuid.UUID{key: {a, q, probeID(4), probeID(5), probeID(6)}}}
	snaps := &sourceSnapshots{}
	p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: snaps, Sources: f})
	facts := sourceSequence(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: a, Kind: key.Kind, Before: &inside, After: &outside}}}, domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: b, Kind: key.Kind, Before: &outside, After: &inside}}})
	asked := 0
	err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, func(context.Context, shared.Action) error { asked++; return nil })
	var problem *shared.Error
	var refusal *domain.Refused
	if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" || problem.DuplicateOf == nil || *problem.DuplicateOf != q || !errors.As(err, &refusal) || refusal.NodeID != b {
		t.Errorf("snapshot duplicate=%#v, want target Q and refused B", err)
	}
	if snaps.calls != 1 || f.returned != 2 || asked != 1 {
		t.Errorf("snapshots/rows/manage asks=%d/%d/%d want 1/2/1", snaps.calls, f.returned, asked)
	}
	// Removing Q leaves only A, which the full plan moves out before B enters.
	f.rows[key] = []uuid.UUID{a}
	f.calls = nil
	f.returned = 0
	if err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, func(context.Context, shared.Action) error { return nil }); err != nil {
		t.Errorf("whole-plan exclusion rejected legal handoff: %v", err)
	}
}
func TestSourcePrecheckKeepsSnapshotBaselineAcrossSimulatedWrites(t *testing.T) {
	key := probeKey(domain.KindPage, "same")
	a, b := probeID(1), probeID(2)
	raw := probeID(90)
	inside := domain.NodePlace{InArea: true, ParentID: &raw}
	outside := domain.NodePlace{}
	for _, baseline := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary baseline", true: "source baseline"}[baseline], func(t *testing.T) {
			f := &sourceProbeFixture{heads: map[uuid.UUID]SourceValue{a: {ID: a, Kind: key.Kind, Hash: key.Hash[:], Bytes: 4}, b: {ID: b, Kind: key.Kind, Hash: key.Hash[:], Bytes: 4}}, rows: map[SourceKey][]uuid.UUID{}}
			var steps []domain.StepFacts
			if baseline {
				v := f.heads[a]
				v.BeforeSourceHash = key.Hash[:]
				f.heads[a] = v
				f.rows[key] = []uuid.UUID{a}
				steps = append(steps, domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: a, Kind: key.Kind, Before: &inside, After: &outside}}})
			}
			steps = append(steps, domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: a, Kind: key.Kind, Before: &outside, After: &inside}, {ID: b, Kind: key.Kind, Before: &outside, After: &inside}}})
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, sourceSequence(steps...), func(context.Context, shared.Action) error { return nil })
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != "page.source_duplicate" {
				t.Fatalf("same-step duplicate accepted: %v", err)
			}
			if (e.DuplicateOf != nil) != baseline || baseline && *e.DuplicateOf != a {
				t.Errorf("baseline target=%v, want eligible=%v/A", e.DuplicateOf, baseline)
			}
		})
	}
}
func TestSourcePrecheckNormalDoesNoSnapshotOrMetadata(t *testing.T) {
	snaps := &sourceSnapshots{fail: errors.New("unexpected snapshot")}
	p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: snaps})
	if err := p.Check(t.Context(), uuid.UUID{}, domain.NotebookState{}, sourceSequence(), nil); err != nil || snaps.calls != 0 {
		t.Errorf("normal = %v, calls %d", err, snaps.calls)
	}
}

func TestSourcePrecheckExcludesTheWholePlanAcrossMetadataChunks(t *testing.T) {
	for _, dense := range []bool{false, true} {
		for _, external := range []bool{false, true} {
			t.Run(fmt.Sprintf("dense_%v/external_%v", dense, external), func(t *testing.T) {
				count := 10001
				if dense {
					count = 60000
				}
				key := probeKey(domain.KindPage, "same")
				empty := probeKey(domain.KindPage, "")
				raw := probeID(900000)
				inside := domain.NodePlace{InArea: true, ParentID: &raw}
				outside := domain.NodePlace{}
				f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{}, rows: map[SourceKey][]uuid.UUID{}}
				var nodes []domain.NodeFacts
				for i := range count {
					id := probeID(i)
					v := SourceValue{ID: id, Kind: key.Kind, Hash: empty.Hash[:]}
					before := outside
					if dense || i == count-1 {
						v.Hash, v.Bytes, v.BeforeSourceHash = key.Hash[:], 4, key.Hash[:]
						before = inside
						f.rows[key] = append(f.rows[key], id)
					}
					f.heads[id] = v
					nodes = append(nodes, domain.NodeFacts{ID: id, Kind: key.Kind, Before: &before, After: &outside})
				}
				entrant, target := probeID(count+1), probeID(count+2)
				f.heads[entrant] = SourceValue{ID: entrant, Kind: key.Kind, Hash: key.Hash[:], Bytes: 4}
				if external {
					f.rows[key] = append(f.rows[key], target)
				}
				facts := sourceSequence(domain.StepFacts{Operation: domain.OpMove, Nodes: nodes}, domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: entrant, Kind: key.Kind, Before: &outside, After: &inside}}})
				p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
				err := p.Check(t.Context(), probeID(800000), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, func(context.Context, shared.Action) error { return nil })
				if external {
					var e *shared.Error
					if !errors.As(err, &e) || e.Code != "page.source_duplicate" || e.DuplicateOf == nil || *e.DuplicateOf != target {
						t.Errorf("cross-chunk witness=%v want external Q %s", err, target)
					}
				} else if err != nil {
					t.Errorf("all excluded source keys rejected: %v", err)
				}
				wantRows := 1
				if dense {
					wantRows = count
				}
				if external {
					wantRows++
				}
				if f.returned != wantRows {
					t.Errorf("returned rows=%d want bounded %d", f.returned, wantRows)
				}
				total := 0
				for _, size := range f.headBatches {
					total += size
				}
				if total != count+1 || len(f.headBatches) != (count+domain.MaxUnitNodes)/domain.MaxUnitNodes {
					t.Errorf("metadata batches=%v total=%d want %d IDs", f.headBatches, total, count+1)
				}
			})
		}
	}
}

func TestSourcePlansSimulateEveryHashChange(t *testing.T) {
	for _, scenario := range []string{"remove one converged member", "rewrite changes key", "rewrite returns to baseline"} {
		t.Run(scenario, func(t *testing.T) {
			h, j := probeKey(domain.KindPage, "H"), probeKey(domain.KindPage, "J")
			a, b, c, raw := probeID(1), probeID(2), probeID(3), probeID(90)
			inside := domain.NodePlace{InArea: true, ParentID: &raw}
			outside := domain.NodePlace{}
			f := &sourceProbeFixture{heads: map[uuid.UUID]SourceValue{a: {ID: a, Kind: h.Kind, Hash: h.Hash[:], Bytes: 1, BeforeSourceHash: h.Hash[:]}, b: {ID: b, Kind: h.Kind, Hash: h.Hash[:], Bytes: 1, BeforeSourceHash: h.Hash[:]}, c: {ID: c, Kind: h.Kind, Hash: h.Hash[:], Bytes: 1}}, rows: map[SourceKey][]uuid.UUID{h: {a, b}}}
			var steps []domain.StepFacts
			var want *uuid.UUID
			content := func(key SourceKey) domain.StepFacts {
				return domain.StepFacts{Operation: domain.OpContent, Rewrite: true, Content: &domain.ContentFacts{Hash: key.Hash[:], Bytes: 1, BeforeBytes: 1, Merged: 1}, Nodes: []domain.NodeFacts{{ID: a, Kind: h.Kind, Before: &inside, After: &inside}}}
			}
			if scenario == "remove one converged member" {
				steps = append(steps, domain.StepFacts{Operation: domain.OpDelete, Nodes: []domain.NodeFacts{{ID: a, Kind: h.Kind, Before: &inside}}})
				want = &b
			} else {
				delete(f.heads, b)
				f.rows[h] = []uuid.UUID{a}
				steps = append(steps, content(j))
				v := f.heads[c]
				v.Hash = j.Hash[:]
				f.heads[c] = v
				if scenario == "rewrite returns to baseline" {
					steps = append(steps, content(h))
					v.Hash = h.Hash[:]
					f.heads[c] = v
					want = &a
				}
			}
			steps = append(steps, domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: c, Kind: h.Kind, Before: &outside, After: &inside}}})
			if scenario == "remove one converged member" {
				steps = append(steps, domain.StepFacts{Operation: domain.OpRename, Nodes: []domain.NodeFacts{{ID: b, Kind: h.Kind, Before: &inside, After: &inside}}})
			}
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, sourceSequence(steps...), func(context.Context, shared.Action) error { return nil })
			var e *shared.Error
			if !errors.As(err, &e) || e.Code != "page.source_duplicate" {
				t.Fatalf("simulated state accepted a duplicate: %v", err)
			}
			if (e.DuplicateOf == nil) != (want == nil) || want != nil && *e.DuplicateOf != *want {
				t.Errorf("simulated baseline target=%v want %v", e.DuplicateOf, want)
			}
		})
	}
}
