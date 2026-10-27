package app_test

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The original job restores preorder F, P, A, Q, B. Between its batches,
// another job restores Q alone and a real move puts Q under raw, leaving
// B in the trash. The original continuation then skips Q and restores
// the forest A, B: each root needs its own current ancestor line.
func TestSourceRestorePartKeepsEachRootLineAfterASkippedParent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionDelete, domain.ActionRestore, domain.ActionMove)
	raw := f.page("raw", nil, 0)
	root := f.page("F", nil, 1)
	p := f.page("P", &root.ID, 0)
	a := f.page("A", &p.ID, 0)
	q := f.page("Q", &root.ID, 1)
	b := f.page("B", &q.ID, 0)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{
		f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw.ID},
	}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	if err := f.remove(root.ID); err != nil {
		t.Fatal(err)
	}
	w, spec := f.jobs(now(), 0, 0), f.jobSpec()
	plan, err := w.PlanRestore(f.asJob(), spec, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps()) != 5 || plan.Total() != 5 {
		t.Fatalf("original plan has %d steps, total %d; want F, P, A, Q, B", len(plan.Steps()), plan.Total())
	}
	first, err := w.RunBatch(f.asJob(), spec, plan, 0, 2)
	if err != nil || first.Nodes != 2 || first.Pages != 2 || first.Skipped != 0 {
		t.Fatalf("restore F/P = %+v, %v; want two pages restored", first, err)
	}
	for _, n := range []domain.Node{root, p} {
		if _, ok := f.store.nodes[n.ID]; !ok {
			t.Fatalf("%s was not restored by the first batch", n.Name)
		}
	}
	for _, n := range []domain.Node{a, q} {
		if trashed, ok := f.store.trash[n.ID]; !ok || !trashed.TrashRoot {
			t.Fatalf("%s = %+v, present=%t; want a remaining trash root", n.Name, trashed, ok)
		}
	}

	otherSpec := f.jobSpec()
	otherPlan, err := w.PlanRestore(f.asJob(), otherSpec, q.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherPlan.Steps()) != 2 || otherPlan.Total() != 2 {
		t.Fatalf("Q's plan has %d steps, total %d; want Q, B", len(otherPlan.Steps()), otherPlan.Total())
	}
	other, err := w.RunBatch(f.asJob(), otherSpec, otherPlan, 0, 1)
	if err != nil || other.Nodes != 1 || other.Pages != 1 || other.Skipped != 0 {
		t.Fatalf("restore Q alone = %+v, %v; want one page restored", other, err)
	}
	if trashed, ok := f.store.trash[b.ID]; !ok || !trashed.TrashRoot {
		t.Fatalf("B = %+v, present=%t; want the second job to leave B as a trash root", trashed, ok)
	}
	if _, err := f.move(q.ID, app.Destination{ParentID: &raw.ID}); err != nil {
		t.Fatal(err)
	}
	if got := f.store.nodes[q.ID]; !sameParent(got.ParentID, &raw.ID) {
		t.Fatalf("Q after move = %+v; want a live child of raw", got)
	}

	g.steps = nil
	f.rec.calls = nil
	out, err := w.RunBatch(f.asJob(), spec, plan, 2, 5)
	if err != nil || out.Nodes != 2 || out.Pages != 2 || out.Assets != 0 || out.Skipped != 1 || out.Placed != nil {
		t.Fatalf("restore A/Q/B = %+v, %v; want A/B restored and Q skipped", out, err)
	}
	reads := 0
	for _, call := range f.rec.calls {
		if strings.HasPrefix(call, "Ancestors") {
			reads++
		}
	}
	if reads != 2 {
		t.Fatalf("ancestor reads=%d, want existing P/Q placement reads only: %v", reads, f.rec.calls)
	}
	if len(g.steps) != 1 || g.steps[0].Operation != domain.OpRestore {
		t.Fatalf("continuation guard steps = %+v; want one restore", g.steps)
	}
	s := g.steps[0]
	var changed []uuid.UUID
	for _, change := range s.Changes {
		changed = append(changed, change.NodeID)
		if change.Before != nil || change.After == nil || change.Restored == nil {
			t.Fatalf("continuation change = %+v; want a restoration", change)
		}
	}
	if want := []uuid.UUID{a.ID, b.ID}; !slices.Equal(changed, want) {
		t.Fatalf("continuation changes = %v; want A/B %v, without skipped Q", changed, want)
	}
	for _, n := range []domain.Node{a, b} {
		got, ok := f.store.nodes[n.ID]
		if !ok || !sameParent(got.ParentID, n.ParentID) {
			t.Errorf("%s after continuation = %+v, present=%t; want its original parent %v", n.Name, got, ok, n.ParentID)
		}
	}
	if len(f.store.trash) != 0 || !sameParent(f.store.nodes[q.ID].ParentID, &raw.ID) {
		t.Errorf("remaining trash=%v, Q=%+v; want all restored and Q still under raw", f.store.trash, f.store.nodes[q.ID])
	}
	facts := app.SourceStepFacts(s)
	if len(facts.Nodes) != 2 {
		t.Fatalf("restore facts = %+v; want exactly A and B", facts)
	}
	for i, want := range []struct {
		node   domain.Node
		inArea bool
	}{{a, false}, {b, true}} {
		got := facts.Nodes[i]
		if got.ID != want.node.ID || got.Kind != domain.KindPage || got.Before != nil || got.After == nil {
			t.Fatalf("facts[%d] = %+v; want restored page %s", i, got, want.node.Name)
		}
		if !sameParent(got.After.ParentID, want.node.ParentID) || got.After.InArea != want.inArea {
			t.Errorf("%s after facts = %+v; want parent %v, InArea=%t", want.node.Name, got.After, want.node.ParentID, want.inArea)
		}
	}
}

// The revert's continuation has the same forest after Q was restored by
// another job. Q stays at its original place, which the revert checks;
// moving the already-restored P outside raw makes only A leave the area.
func TestSourceRevertContinuationKeepsEachRootLineAfterASkippedParent(t *testing.T) {
	scene := newJobScene()
	raw := scene.node("raw", nil, false)
	root := scene.node("F", &raw, false)
	p := scene.node("P", &root, false)
	a := scene.node("A", &p, false)
	q := scene.node("Q", &root, false)
	b := scene.node("B", &q, false)
	if !scene.begin().del(root) {
		t.Fatal("the initial legal deletion of F was refused")
	}
	f, changeset := scene.load()
	f.grant(domain.ActionRestore, domain.ActionMove)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{
		f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: raw},
	}
	g := &guard{recorder: f.rec}
	f.guards = []app.WriteGuard{g}
	w, spec, plan := f.revertJob(t, changeset.ID, nil, now())
	steps, _ := plan.RevertSteps()
	if got, want := scene.describe(steps, plan.Steps()), []string{"restore F", "restore P", "restore A", "restore Q", "restore B"}; !slices.Equal(got, want) {
		t.Fatalf("compact steps = %v; want %v", got, want)
	}
	first, err := w.RunBatch(f.asJob(), spec, plan, 0, 2)
	if err != nil || first.Nodes != 2 || first.Restored != 2 || first.Skipped != 0 {
		t.Fatalf("revert F/P = %+v, %v; want two restored nodes", first, err)
	}
	for _, id := range []uuid.UUID{root, p} {
		if _, ok := f.store.nodes[id]; !ok {
			t.Fatalf("%s was not restored by the first batch", scene.labels[id])
		}
	}
	for _, id := range []uuid.UUID{a, q} {
		if trashed, ok := f.store.trash[id]; !ok || !trashed.TrashRoot {
			t.Fatalf("%s = %+v, present=%t; want a remaining trash root", scene.labels[id], trashed, ok)
		}
	}

	otherSpec := f.jobSpec()
	otherPlan, err := w.PlanRestore(f.asJob(), otherSpec, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherPlan.Steps()) != 2 || otherPlan.Total() != 2 {
		t.Fatalf("Q's plan has %d steps, total %d; want Q, B", len(otherPlan.Steps()), otherPlan.Total())
	}
	other, err := w.RunBatch(f.asJob(), otherSpec, otherPlan, 0, 1)
	if err != nil || other.Nodes != 1 || other.Pages != 1 || other.Skipped != 0 {
		t.Fatalf("restore Q alone = %+v, %v; want one restored page", other, err)
	}
	if trashed, ok := f.store.trash[b]; !ok || !trashed.TrashRoot {
		t.Fatalf("B = %+v, present=%t; want the other job to leave B as a trash root", trashed, ok)
	}
	if _, err := f.move(p, app.Destination{}); err != nil {
		t.Fatal(err)
	}
	if got := f.store.nodes[p]; got.ParentID != nil {
		t.Fatalf("P after move = %+v; want a notebook root outside raw", got)
	}
	if got := f.store.nodes[q]; !sameParent(got.ParentID, &root) {
		t.Fatalf("Q after the other restore = %+v; want its original parent F", got)
	}

	g.steps = nil
	f.rec.calls = nil
	out, err := w.RunBatch(f.asJob(), spec, plan, 2, 5)
	if err != nil || out.Nodes != 2 || out.Restored != 2 || out.Skipped != 1 {
		t.Fatalf("revert A/Q/B = %+v, %v; want A/B restored and Q skipped", out, err)
	}
	reads := 0
	for _, call := range f.rec.calls {
		if strings.HasPrefix(call, "Ancestors") {
			reads++
		}
	}
	if reads != 2 {
		t.Fatalf("ancestor reads=%d, want existing P/Q placement reads only: %v", reads, f.rec.calls)
	}
	if len(g.steps) != 1 || g.steps[0].Operation != domain.OpRestore {
		t.Fatalf("continuation guard steps = %+v; want one restore", g.steps)
	}
	s := g.steps[0]
	var changed []uuid.UUID
	for _, change := range s.Changes {
		changed = append(changed, change.NodeID)
		if change.Before != nil || change.After == nil || change.Restored == nil {
			t.Fatalf("continuation change = %+v; want a restoration", change)
		}
	}
	if want := []uuid.UUID{a, b}; !slices.Equal(changed, want) {
		t.Fatalf("continuation changes = %v; want A/B %v, without skipped Q", changed, want)
	}
	if len(f.store.trash) != 0 || f.store.nodes[p].ParentID != nil || !sameParent(f.store.nodes[q].ParentID, &root) ||
		!sameParent(f.store.nodes[root].ParentID, &raw) {
		t.Errorf("remaining trash=%v, P=%+v, Q=%+v, F=%+v; want P outside raw and Q under F under raw",
			f.store.trash, f.store.nodes[p], f.store.nodes[q], f.store.nodes[root])
	}
	facts := app.SourceStepFacts(s)
	if len(facts.Nodes) != 2 {
		t.Fatalf("restore facts = %+v; want exactly A and B", facts)
	}
	for i, want := range []struct {
		id, parent uuid.UUID
		inArea     bool
	}{{a, p, false}, {b, q, true}} {
		persisted, ok := f.store.nodes[want.id]
		if !ok || !sameParent(persisted.ParentID, &want.parent) {
			t.Errorf("%s after continuation = %+v, present=%t; want parent %s", scene.labels[want.id], persisted, ok, scene.labels[want.parent])
		}
		got := facts.Nodes[i]
		if got.ID != want.id || got.Kind != domain.KindPage || got.Before != nil || got.After == nil {
			t.Fatalf("facts[%d] = %+v; want restored page %s", i, got, scene.labels[want.id])
		}
		if !sameParent(got.After.ParentID, &want.parent) || got.After.InArea != want.inArea {
			t.Errorf("%s after facts = %+v; want parent %s, InArea=%t", scene.labels[want.id], got.After, scene.labels[want.parent], want.inArea)
		}
	}
}
