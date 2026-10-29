package app

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Rebuilding a bounded plan in each metadata/key/verdict pass repeats the
// whole notebook simulation. Oversized plans must still use streaming replay.
func TestSourcePrecheckBoundsItsFactReplay(t *testing.T) {
	for _, c := range []struct {
		name, keyPadding string
		nodes, steps     int
		passes           int
	}{
		{name: "one arrival", nodes: 1, steps: 1, passes: 1},
		{name: "ten thousand occurrences", nodes: 5000, steps: 2, passes: 1},
		{name: "more occurrences", nodes: 5001, steps: 2, passes: 3},
		{name: "fifteen hundred steps", nodes: 1, steps: 1500, passes: 1},
		{name: "more steps", nodes: 1, steps: 1501, passes: 3},
		{name: "empty steps within bound", nodes: 0, steps: 1500, passes: 1},
		{name: "empty steps beyond bound", nodes: 0, steps: 1501, passes: 2},
		{name: "large keys", nodes: 5000, steps: 2, keyPadding: strings.Repeat("k", 100), passes: 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			raw := probeID(900000)
			empty := probeKey(domain.KindPage, "")
			f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{}, rows: map[SourceKey][]uuid.UUID{}}
			var entering, leaving []domain.NodeFacts
			for i := range c.nodes {
				id := probeID(i + 1)
				inside := domain.NodePlace{ParentID: &raw, Key: fmt.Sprintf("page%d%s", i, c.keyPadding), InArea: true}
				outside := domain.NodePlace{Key: inside.Key}
				f.heads[id] = SourceValue{ID: id, Kind: domain.KindPage, Hash: empty.Hash[:]}
				entering = append(entering, domain.NodeFacts{ID: id, Kind: domain.KindPage, Before: &outside, After: &inside})
				leaving = append(leaving, domain.NodeFacts{ID: id, Kind: domain.KindPage, Before: &inside, After: &outside})
			}
			passes := 0
			facts := func(yield func(domain.StepFacts) bool) {
				passes++
				for i := range c.steps {
					nodes := entering
					if i%2 != 0 {
						nodes = leaving
					}
					if !yield(domain.StepFacts{Operation: domain.OpMove, Nodes: nodes}) {
						return
					}
				}
			}
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			asked := 0
			err := p.Check(t.Context(), probeID(800000), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts,
				func(ctx context.Context, action shared.Action) error {
					if ctx.Value(sourceSnapshotKey{}) != true || action != domain.ActionManageSource {
						t.Fatalf("authorization context/action = %v/%s", ctx.Value(sourceSnapshotKey{}), action)
					}
					asked++
					return nil
				})
			wantAsks := c.steps / 2
			if c.nodes == 0 {
				wantAsks = 0
			}
			if err != nil || asked != wantAsks {
				t.Fatalf("legal moves = %v, authorization steps = %d want %d", err, asked, wantAsks)
			}
			if passes != c.passes {
				t.Errorf("plan simulations = %d want %d", passes, c.passes)
			}
		})
	}
}

// A replayable iterator may reuse its scratch memory after yield returns.
// The first rewrite must remain J, so the later entrant B conflicts with A,
// even after every reference in that first yielded fact has been overwritten.
func TestSourcePrecheckFreezesYieldedFacts(t *testing.T) {
	for _, poison := range []string{"nodes", "place", "content", "hash"} {
		t.Run(poison, func(t *testing.T) {
			a, b, raw := probeID(1), probeID(2), probeID(90)
			h, j := probeKey(domain.KindPage, "H"), probeKey(domain.KindPage, "J")
			inside := domain.NodePlace{ParentID: &raw, Key: "page", InArea: true}
			outside := domain.NodePlace{Key: "page"}
			f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{
				a: {ID: a, Kind: domain.KindPage, Hash: h.Hash[:], Bytes: 1, BeforeSourceHash: h.Hash[:]},
				b: {ID: b, Kind: domain.KindPage, Hash: j.Hash[:], Bytes: 1},
			}, rows: map[SourceKey][]uuid.UUID{h: {a}}}
			passes := 0
			facts := func(yield func(domain.StepFacts) bool) {
				passes++
				parent := raw
				place := domain.NodePlace{ParentID: &parent, Key: "index", InArea: true}
				hash := append([]byte(nil), j.Hash[:]...)
				content := domain.ContentFacts{Hash: hash, Bytes: 1, BeforeBytes: 1, Merged: 1}
				nodes := []domain.NodeFacts{{ID: a, Kind: domain.KindPage, Before: &place, After: &place}}
				if !yield(domain.StepFacts{Operation: domain.OpContent, Rewrite: true, Content: &content, Nodes: nodes}) {
					return
				}
				switch poison {
				case "nodes":
					nodes[0].ID = b
				case "place":
					place = domain.NodePlace{Key: "index"}
				case "content":
					content.Hash, content.Bytes = h.Hash[:], 0
				case "hash":
					copy(hash, h.Hash[:])
				}
				yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: b, Kind: domain.KindPage, Before: &outside, After: &inside}}})
			}
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			asked := 0
			err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts,
				func(context.Context, shared.Action) error { asked++; return nil })
			var problem *shared.Error
			var refusal *domain.Refused
			if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" || problem.DuplicateOf != nil ||
				!errors.As(err, &refusal) || refusal.NodeID != b || asked != 0 {
				t.Errorf("rewrite then entrant = %v, manage asks = %d; want duplicate B without baseline target and no manage ask", err, asked)
			}
			if passes != 1 {
				t.Errorf("bounded plan simulations = %d want 1", passes)
			}
		})
	}
}

func TestSourcePrecheckFreezesParentValues(t *testing.T) {
	folder, raw, from, to := probeID(1), probeID(90), probeID(91), probeID(92)
	passes, asked := 0, 0
	facts := func(yield func(domain.StepFacts) bool) {
		passes++
		parent := from
		before, after := domain.NodePlace{ParentID: &parent, Key: "folder"}, domain.NodePlace{ParentID: &to, Key: "folder"}
		yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{
			{ID: folder, Kind: domain.KindPage, Before: &before, After: &after},
			{ID: raw, Kind: domain.KindPage},
		}})
		// Reusing the pointed-to ID must not turn a source-root ancestor's
		// parent change into a reorder and remove its management check.
		parent = to
	}
	p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}})
	err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts,
		func(context.Context, shared.Action) error { asked++; return shared.Forbidden() })
	var refusal *domain.Refused
	if !errors.Is(err, domain.ErrSourceAdminOnly) || !errors.As(err, &refusal) || refusal.NodeID != raw || asked != 1 {
		t.Errorf("moving ancestor = %v, asks = %d; want source_admin_only naming raw after one ask", err, asked)
	}
	if passes != 1 {
		t.Errorf("no-admission plan simulations = %d want 1", passes)
	}
}

func TestSourcePrecheckDoesNotReuseFactsBetweenChecks(t *testing.T) {
	a, raw := probeID(1), probeID(90)
	empty := probeKey(domain.KindPage, "")
	f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{a: {ID: a, Kind: domain.KindPage, Hash: empty.Hash[:]}}, rows: map[SourceKey][]uuid.UUID{}}
	p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
	inside := domain.NodePlace{ParentID: &raw, Key: "page", InArea: true}
	outside := domain.NodePlace{Key: "page"}
	passes := 0
	var facts iter.Seq[domain.StepFacts] = func(yield func(domain.StepFacts) bool) {
		passes++
		yield(domain.StepFacts{Operation: domain.OpMove, Nodes: []domain.NodeFacts{{ID: a, Kind: domain.KindPage, Before: &inside, After: &outside}}})
	}
	denied, asked := false, 0
	ask := func(context.Context, shared.Action) error {
		asked++
		if denied {
			return shared.Forbidden()
		}
		return nil
	}
	if err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, ask); err != nil {
		t.Fatal(err)
	}
	denied = true
	err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, ask)
	if !errors.Is(err, domain.ErrSourceAdminOnly) {
		t.Errorf("next check with revoked permission = %v want source_admin_only", err)
	}
	inside.InArea = false
	if err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts, ask); err != nil {
		t.Errorf("later ordinary move with changed facts = %v want nil", err)
	}
	if passes != 3 || asked != 2 {
		t.Errorf("three checks simulated plan %d times and asked %d times, want 3/2", passes, asked)
	}
}

func TestSourcePrecheckCountsKeysAndHashesAtTheReplayBoundary(t *testing.T) {
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("extra_byte_%d", extra), func(t *testing.T) {
			raw := probeID(900000)
			empty := probeKey(domain.KindPage, "")
			f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{}, rows: map[SourceKey][]uuid.UUID{}}
			// Ordinary title keys plus a valid 32-byte hash exactly fill the
			// 1 MiB payload budget. The second case crosses it by one byte;
			// omitting the hash from accounting would wrongly retain it.
			const nodeCount, payloadLimit = 10000, 1 << 20
			keyBytes := payloadLimit - 32 + extra
			nodes := make([]domain.NodeFacts, nodeCount)
			for i := range nodeCount {
				id := probeID(i + 1)
				size := keyBytes / nodeCount
				if i < keyBytes%nodeCount {
					size++
				}
				prefix := fmt.Sprintf("page%d", i)
				key := prefix + strings.Repeat("k", size-len(prefix))
				nodes[i] = domain.NodeFacts{ID: id, Kind: domain.KindPage, After: &domain.NodePlace{ParentID: &raw, Key: key, InArea: true}}
				f.heads[id] = SourceValue{ID: id, Kind: domain.KindPage, Hash: empty.Hash[:]}
			}
			passes := 0
			facts := func(yield func(domain.StepFacts) bool) {
				passes++
				yield(domain.StepFacts{Operation: domain.OpRestore, Nodes: nodes, Content: &domain.ContentFacts{Hash: empty.Hash[:]}})
			}
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			err := p.Check(t.Context(), probeID(800000), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts,
				func(context.Context, shared.Action) error { t.Error("arrivals requested manage_source"); return nil })
			if err != nil || len(f.headBatches) != 1 || f.headBatches[0] != nodeCount || len(f.calls) != 0 {
				t.Fatalf("empty-page arrivals = %v, metadata chunks = %v, probes = %d", err, f.headBatches, len(f.calls))
			}
			want := 1
			if extra != 0 {
				want = 3
			}
			if passes != want {
				t.Errorf("payload %d: plan simulations = %d want %d", payloadLimit+extra, passes, want)
			}
		})
	}
}

func TestSourcePrecheckReplaysTheDecisiveTailAfterOverflow(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(fmt.Sprintf("duplicate_%t", duplicate), func(t *testing.T) {
			a, b, raw := probeID(1), probeID(2), probeID(90)
			h := probeKey(domain.KindPage, "H")
			f := &sourceProbeFixture{snapshot: true, heads: map[uuid.UUID]SourceValue{
				b: {ID: b, Kind: domain.KindPage, Hash: h.Hash[:], Bytes: 1},
			}, rows: map[SourceKey][]uuid.UUID{h: {a}}}
			last := domain.StepFacts{Operation: domain.OpDelete, Nodes: []domain.NodeFacts{{ID: raw, Kind: domain.KindPage}}}
			if duplicate {
				last = domain.StepFacts{Operation: domain.OpRestore, Nodes: []domain.NodeFacts{
					{ID: b, Kind: domain.KindPage, After: &domain.NodePlace{ParentID: &raw, Key: "page", InArea: true}},
				}}
			}
			passes := 0
			facts := func(yield func(domain.StepFacts) bool) {
				passes++
				for range 1500 {
					if !yield(domain.StepFacts{}) {
						return
					}
				}
				yield(last)
			}
			p := NewSourcePrecheck(SourcePrecheckDeps{Snapshots: &sourceSnapshots{}, Sources: f})
			err := p.Check(t.Context(), probeID(80), domain.NotebookState{Mode: domain.ModeLLMWiki, SourceRootID: raw}, facts,
				func(context.Context, shared.Action) error { t.Error("refusal reached authorization"); return nil })
			var refusal *domain.Refused
			if !errors.As(err, &refusal) {
				t.Fatalf("tail was not refused: %v", err)
			}
			wantPasses := 2
			if duplicate {
				wantPasses = 3
				var problem *shared.Error
				if !errors.As(err, &problem) || problem.Code != "page.source_duplicate" || problem.DuplicateOf == nil ||
					*problem.DuplicateOf != a || refusal.NodeID != b {
					t.Errorf("tail duplicate = %v, want B refused with existing target A", err)
				}
			} else if !errors.Is(err, domain.ErrProtected) || refusal.NodeID != raw {
				t.Errorf("tail delete = %v, want protected raw", err)
			}
			if passes != wantPasses {
				t.Errorf("full plan simulations = %d want %d", passes, wantPasses)
			}
		})
	}
}
