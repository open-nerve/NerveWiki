package app_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// This history is built by the existing module job model: x and y leave
// G before its deletion, G' takes its name, then y moves under G'. The
// restore group is connected (G, K, p1, p2); raw is a separate live root.
// Its compact tail has external parents K and G, both restored earlier.
func TestSourceJobAsideFactsMatchActualBatches(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(map[bool]string{false: "one_batch", true: "segment_then_tail"}[split], func(t *testing.T) {
			s := newJobScene()
			raw := s.node("raw", nil, false)
			g := s.node("G", &raw, false)
			k := s.node("K", &g, false)
			p1 := s.node("p1", &k, false)
			p2 := s.node("p2", &g, false)
			x := s.node("x", &k, false)
			y := s.node("y", &k, false)
			asset := s.node("asset", &y, true)
			e := s.begin()
			if !e.move(x, nil, "x") || !e.move(y, nil, "y") || !e.del(g) {
				t.Fatal("scene must move x/y out before deleting the connected G group")
			}
			g2 := s.node("G'", &raw, false)
			if !e.move(y, &g2, "y") {
				t.Fatal("scene must put y under the replacement G'")
			}
			f, c := s.load()
			spy := sourceJobFixture(f, raw)
			w, spec := f.jobs(now(), 0, 0), f.jobSpec()
			cuts := [][2]int{{0, 8}}
			if split {
				cuts = [][2]int{{0, 6}, {6, 8}}
			}
			batches := func([]domain.JobStep) [][2]int { return cuts }
			want := []string{
				"move y/page G':true->/:false asset/asset y:true->y:false",
				"delete G'/page raw:true->-",
				"restore G/page -->raw:true K/page -->G:true p1/page -->K:true p2/page -->G:true",
				"move x/page /:false->K:true",
				"move y/page /:false->K:true asset/asset y:false->y:true",
			}
			if split {
				want[2] = "restore G/page -->raw:true K/page -->G:true"
				want = append(want, "restore p1/page -->K:true p2/page -->G:true")
			}
			facts, err := w.SourceCheckedJobFacts(f.asJob(), spec, c.ID, raw, batches)
			if err != nil {
				t.Fatal(err)
			}
			assertSourceJobTrace(t, "checked facts", s.labels, facts, want)
			plan, err := w.PlanRevert(f.asJob(), spec, c.ID, batches)
			if err != nil {
				t.Fatal(err)
			}
			steps, _ := plan.RevertSteps()
			compact := []string{"aside y+", "delete G' [G']+", "restore G+", "restore K+", "place x+", "place y", "restore p1", "restore p2"}
			if got := s.describe(steps, plan.Steps()); !slices.Equal(got, compact) {
				t.Fatalf("compact plan=%v, want %v", got, compact)
			}
			if len(spy.steps) != 0 || len(f.store.changesets) != 0 {
				t.Fatal("observing and planning must not guard or persist a write")
			}
			f.rec.calls = nil
			for i, cut := range cuts {
				if _, err := w.RunBatch(f.asJob(), spec, plan, cut[0], cut[1]); err != nil {
					t.Fatalf("batch %v: %v", cut, err)
				}
				if split && i == 0 {
					for _, id := range []uuid.UUID{p1, p2} {
						if _, alive := f.store.nodes[id]; alive || !f.store.trash[id].TrashRoot {
							t.Fatalf("tail %s must remain a trash root after the segment", s.labels[id])
						}
					}
					for _, id := range []uuid.UUID{x, y} {
						if !sameParent(f.store.nodes[id].ParentID, &k) {
							t.Fatalf("%s must already be under restored K before the tail", s.labels[id])
						}
					}
				}
			}
			assertSourceJobGuardTrace(t, s.labels, spy, want)
			persist := []string{"GuardWrite move in tx", "RelocateNode in tx", "GuardWrite delete in tx", "DeleteNodes in tx",
				"GuardWrite restore in tx", "RestoreNodes in tx", "GuardWrite move in tx", "MoveNode in tx",
				"GuardWrite move in tx", "RelocateNode in tx"}
			if split {
				persist = append(persist, "GuardWrite restore in tx", "RestoreNodes in tx")
				last := spy.steps[len(spy.steps)-1]
				if last.Lines == nil || len(last.Lines.AfterParents) != 2 {
					t.Fatalf("tail must retain its two external parents' current lines: %+v", last.Lines)
				}
				for parent, ids := range map[uuid.UUID][]uuid.UUID{k: {raw, g, k}, g: {raw, g}} {
					var got []uuid.UUID
					for _, a := range last.Lines.AfterParents[parent] {
						got = append(got, a.ID)
					}
					if !slices.Equal(got, ids) {
						t.Errorf("tail parent %s line=%v, want %v", s.labels[parent], got, ids)
					}
				}
			}
			assertSourceJobPersistence(t, f, persist)
			for id, parent := range map[uuid.UUID]uuid.UUID{g: raw, k: g, p1: k, p2: g, x: k, y: k, asset: y} {
				n, ok := f.store.nodes[id]
				if !ok || !sameParent(n.ParentID, &parent) {
					t.Errorf("persisted %s=%+v present=%t, want parent %s", s.labels[id], n, ok, s.labels[parent])
				}
			}
			if len(f.store.trash) != 1 || !f.store.trash[g2].TrashRoot {
				t.Errorf("trash=%v, want replacement G' alone", f.store.trash)
			}
			if diff := s.p.compare(s.t0, s.p.modelOf(f)); diff != "" {
				t.Fatalf("replayed scene differs from the original: %s", diff)
			}
		})
	}
}

// m (with an attachment) returns to raw before the newly-created N is
// deleted. The deletion must include N's current children in level order,
// and exclude both m and its attachment after their preceding move.
func TestSourceJobPlaceThenDeleteFactsUseCurrentSubtrees(t *testing.T) {
	s := newJobScene()
	raw := s.node("raw", nil, false)
	m := s.node("m", &raw, false)
	asset := s.node("asset", &m, true)
	e := s.begin()
	n := s.node("N", nil, false)
	a := s.node("a", &n, false)
	s.node("x", &a, true)
	b := s.node("b", &n, false)
	s.node("y", &b, true)
	s.order(b, -1)
	if !e.move(m, &n, "m") {
		t.Fatal("scene must move m under the newly-created N")
	}
	f, c := s.load()
	spy := sourceJobFixture(f, raw)
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	want := []string{
		"move m/page N:false->raw:true asset/asset m:false->m:true",
		"delete N/page /:false->- b/page N:false->- a/page N:false->- x/asset a:false->- y/asset b:false->-",
	}
	facts, err := w.SourceCheckedJobFacts(f.asJob(), spec, c.ID, raw, allJobBatches)
	if err != nil {
		t.Fatal(err)
	}
	assertSourceJobTrace(t, "checked facts", s.labels, facts, want)
	plan, err := w.PlanRevert(f.asJob(), spec, c.ID, allJobBatches)
	if err != nil {
		t.Fatal(err)
	}
	steps, _ := plan.RevertSteps()
	if got, want := s.describe(steps, plan.Steps()), []string{"place m", "delete N [N b a x y]"}; !slices.Equal(got, want) {
		t.Fatalf("compact plan=%v, want %v", got, want)
	}
	f.rec.calls = nil
	if done, err := w.RunBatch(f.asJob(), spec, plan, 0, len(plan.Steps())); err != nil || done.PutBack != 1 || done.Deleted != 5 {
		t.Fatalf("RunBatch=%+v, %v; want m placed and N's five current nodes deleted", done, err)
	}
	assertSourceJobGuardTrace(t, s.labels, spy, want)
	assertSourceJobPersistence(t, f, []string{"GuardWrite move in tx", "MoveNode in tx", "GuardWrite delete in tx", "DeleteNodes in tx"})
	if len(f.store.trash) != 5 || !sameParent(f.store.nodes[m].ParentID, &raw) || !sameParent(f.store.nodes[asset].ParentID, &m) {
		t.Fatalf("persisted subtree is wrong: nodes=%v trash=%v", f.store.nodes, f.store.trash)
	}
	if diff := s.p.compare(s.t0, s.p.modelOf(f)); diff != "" {
		t.Fatal(diff)
	}
}

// This is a pure action-simulator check, not a claim that nominal job cuts
// produce this interleaving. The original trash group is connected. After
// F/P/Q return, P moves out of raw, then the remaining A/B forest returns.
// Runtime reachability with another job restoring a parent is covered by
// TestSourceRevertContinuationKeepsEachRootLineAfterASkippedParent.
func TestSourceJobSimulatorUsesCurrentLinesForEachRestoredParent(t *testing.T) {
	raw, f, p, a, q, b := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	node := func(id uuid.UUID, name string, parent *uuid.UUID, kind domain.Kind) domain.Node {
		return domain.Node{ID: id, Kind: kind, Name: name, NameKey: strings.ToLower(name), ParentID: parent}
	}
	group := []domain.TrashMember{
		{Node: node(f, "F", &raw, domain.KindPage)},
		{Node: node(p, "P", &f, domain.KindPage), Level: 1},
		{Node: node(a, "A", &p, domain.KindPage), Level: 2},
		{Node: node(q, "Q", &f, domain.KindPage), Level: 1},
		{Node: node(b, "B.png", &q, domain.KindAsset), Level: 2},
	}
	state := domain.RevertState{Tree: []domain.Node{node(raw, "raw", nil, domain.KindPage)},
		Groups: map[uuid.UUID][]domain.TrashMember{f: group}}
	actions := []domain.RevertOp{
		{Kind: domain.RevertRestore, NodeID: f, Group: []domain.TrashMember{group[0], group[1], group[3]},
			States: []domain.TreeState{group[0].Node.State(), group[1].Node.State(), group[3].Node.State()}},
		{Kind: domain.RevertPlace, NodeID: p, Move: true, To: domain.TreeState{Name: "P"}},
		{Kind: domain.RevertRestore, NodeID: a, Group: []domain.TrashMember{group[2], group[4]},
			States: []domain.TreeState{group[2].Node.State(), group[4].Node.State()}},
	}
	facts := slices.Collect(domain.RevertActionFacts(state, slices.Values(actions), raw))
	assertSourceJobTrace(t, "pure action simulation", map[uuid.UUID]string{raw: "raw", f: "F", p: "P", a: "A", q: "Q", b: "B"}, facts, []string{
		"restore F/page -->raw:true P/page -->F:true Q/page -->F:true",
		"move P/page F:true->/:false",
		"restore A/page -->P:false B/asset -->Q:true",
	})
}

func sourceJobFixture(f *fixture, raw uuid.UUID) *guard {
	f.grant(domain.ActionManageSource)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw}}
	spy := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{app.NewModeGuard(f.auth, f.store), spy}
	return spy
}

func assertSourceJobGuardTrace(t *testing.T, names map[uuid.UUID]string, spy *guard, want []string) {
	t.Helper()
	var facts []domain.StepFacts
	for _, step := range spy.steps {
		facts = append(facts, app.SourceStepFacts(step))
	}
	assertSourceJobTrace(t, "actual guard facts", names, facts, want)
}

// Only renders already-computed facts. In particular, it does not walk a
// parent graph or infer source membership; all expected lines are literals.
func assertSourceJobTrace(t *testing.T, label string, names map[uuid.UUID]string, facts []domain.StepFacts, want []string) {
	t.Helper()
	name := func(id uuid.UUID) string {
		if s, ok := names[id]; ok {
			return s
		}
		return "unknown:" + id.String()
	}
	place := func(p *domain.NodePlace) string {
		if p == nil {
			return "-"
		}
		parent := "/"
		if p.ParentID != nil {
			parent = name(*p.ParentID)
		}
		return fmt.Sprintf("%s:%t", parent, p.InArea)
	}
	var got []string
	for _, step := range facts {
		if step.Kind != domain.ChangesetRevert || step.Content != nil || step.Rewrite || step.EditSession {
			t.Errorf("%s has unexpected non-tree facts: %+v", label, step)
		}
		line := string(step.Operation)
		for _, node := range step.Nodes {
			line += fmt.Sprintf(" %s/%s %s->%s", name(node.ID), node.Kind, place(node.Before), place(node.After))
		}
		got = append(got, line)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s:\n%s\nwant:\n%s", label, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func assertSourceJobPersistence(t *testing.T, f *fixture, want []string) {
	t.Helper()
	var got []string
	for _, call := range f.rec.calls {
		if strings.HasPrefix(call, "GuardWrite ") || strings.HasPrefix(call, "MoveNode ") || strings.HasPrefix(call, "RelocateNode ") ||
			strings.HasPrefix(call, "RenameNode ") || strings.HasPrefix(call, "DeleteNodes ") || strings.HasPrefix(call, "RestoreNodes ") ||
			strings.HasPrefix(call, "WriteContent ") {
			got = append(got, call)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("guard/persistence order=%v, want %v", got, want)
	}
}
