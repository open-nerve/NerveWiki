package app_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// errImportsTree is a background job's call to an import's tree.
var errImportsTree = errors.New("an import's tree writes no background job")

// An import's tree checks, plans and writes no background job: jobTree
// does.

func (t *tree) CheckRestore(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", errImportsTree
}

func (t *tree) CheckRevert(context.Context, uuid.UUID, uuid.UUID, *string, func(context.Context) error) (string, error) {
	return "", errImportsTree
}

func (t *tree) PlanRestore(context.Context, app.JobSpec, uuid.UUID) (app.JobPlan, error) {
	return nil, errImportsTree
}
func (t *tree) PlanRevert(context.Context, app.JobSpec, uuid.UUID) (app.JobPlan, error) {
	return nil, errImportsTree
}

func (t *tree) RunBatch(context.Context, app.JobSpec, app.JobPlan, int, int) (app.BatchDone, error) {
	return app.BatchDone{}, errImportsTree
}

// jobTree is the page module's tree as a background restore's or
// revert's start checks it and its run plans and writes it. name is the
// restore's item's name, kind the revert's changeset's; refuse refuses a
// check before a revert's admit, late after it, onAdmit runs once admit
// let it go on. steps are the plan, planErr refuses it; planned are the
// item or changeset each plan was of. Each batch is recorded with its
// spec and actor; busy answers as many batches 503 first; halving answers
// ErrBatchTooLarge to a batch of more steps than it, none when 0; history,
// when set, answers it to a batch whose steps restore more rows of it
// than history.bound, of more than one node, as the page module does
// (M9/P3 design 3.23); fail refuses the batch from a step; onBatch runs
// as each batch starts, its number from 1; wrote are the batches written.
type jobTree struct {
	*tree
	rec          *recorder
	name, kind   string
	refuse, late error
	onAdmit      func()

	mu      sync.Mutex
	steps   []app.JobStep
	planErr error
	planned []uuid.UUID
	batches [][2]int
	wrote   [][2]int
	specs   []app.JobSpec
	actors  []shared.Actor
	busy    int
	halving int
	history *jobHistory
	fail    map[int]error
	onBatch func(n int)
}

func newJobTree(rec *recorder) *jobTree {
	return &jobTree{tree: newTree(), rec: rec, name: "Box", kind: "import", fail: map[int]error{}}
}

func (t *jobTree) CheckRestore(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	t.rec.add("Tree.CheckRestore")
	return t.name, t.refuse
}

func (t *jobTree) CheckRevert(ctx context.Context, _, _ uuid.UUID, _ *string, admit func(context.Context) error) (string, error) {
	t.rec.add("Tree.CheckRevert")
	if t.refuse != nil {
		return "", t.refuse
	}
	if err := admit(ctx); err != nil {
		return "", err
	}
	if t.onAdmit != nil {
		t.onAdmit()
	}
	return t.kind, t.late
}

func (t *jobTree) PlanRestore(_ context.Context, _ app.JobSpec, id uuid.UUID) (app.JobPlan, error) {
	return t.plan(id)
}

func (t *jobTree) PlanRevert(_ context.Context, _ app.JobSpec, id uuid.UUID) (app.JobPlan, error) {
	return t.plan(id)
}

func (t *jobTree) plan(id uuid.UUID) (app.JobPlan, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.planned = append(t.planned, id)
	if t.planErr != nil {
		return nil, t.planErr
	}
	return plan(t.steps), nil
}

// RunBatch writes the batch: each of its steps' nodes a page, the first
// batch's placing the item's root.
func (t *jobTree) RunBatch(ctx context.Context, spec app.JobSpec, p app.JobPlan, from, to int) (app.BatchDone, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return app.BatchDone{}, err
	}
	t.mu.Lock()
	t.batches, t.specs, t.actors = append(t.batches, [2]int{from, to}), append(t.specs, spec), append(t.actors, actor)
	n, onBatch := len(t.batches), t.onBatch
	t.mu.Unlock()
	if onBatch != nil {
		onBatch(n)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case t.busy > 0:
		t.busy--
		return app.BatchDone{}, shared.ServerBusy(time.Second)
	case t.halving > 0 && to-from > t.halving:
		return app.BatchDone{}, app.ErrBatchTooLarge
	case t.history.past(from, to):
		return app.BatchDone{}, fmt.Errorf("%w: past a unit's rows of history", app.ErrBatchTooLarge)
	case t.fail[from] != nil:
		return app.BatchDone{}, t.fail[from]
	}
	t.wrote = append(t.wrote, [2]int{from, to})
	done := app.BatchDone{}
	for _, s := range p.Steps()[from:to] {
		done.Nodes += s.Nodes
	}
	done.Pages = done.Nodes
	if from == 0 {
		done.Placed = &domain.Placement{Reason: "original"}
	}
	return done, nil
}

// jobHistory is the rows of history each step restores, by step, and the
// node each writes, by step, its own when not set; bound is a unit's.
type jobHistory struct {
	rows  map[int]int
	node  map[int]int
	bound int
}

// past reports whether the steps from to to restore more rows than the
// bound, and are of more than one node: a node's steps never are.
func (h *jobHistory) past(from, to int) bool {
	if h == nil {
		return false
	}
	rows, nodes := 0, map[int]bool{}
	for k := from; k < to; k++ {
		node, ok := h.node[k]
		if !ok {
			node = k
		}
		rows, nodes[node] = rows+h.rows[k], true
	}
	return rows > h.bound && len(nodes) > 1
}

// written are the batches asked so far, each its first step and the one
// after its last.
func (t *jobTree) written() [][2]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][2]int(nil), t.batches...)
}

// succeeded are the batches written so far, as written tells them.
func (t *jobTree) succeeded() [][2]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][2]int(nil), t.wrote...)
}

// plan is a background job's plan of steps.
type plan []app.JobStep

func (p plan) Steps() []app.JobStep { return p }

func (p plan) Total() int {
	n := 0
	for _, s := range p {
		n += s.Nodes
	}
	return n
}

// steps are n steps of a node each, of bytes.
func steps(n int, bytes int64) []app.JobStep {
	out := make([]app.JobStep, n)
	for i := range out {
		out[i] = app.JobStep{Nodes: 1, Bytes: bytes}
	}
	return out
}
