package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// sourceBoundaryTree delegates every operation to production. Its hook
// commits a real API change immediately around worker plan/batch boundaries.
type sourceBoundaryTree struct {
	transfer.Tree
	mu                                sync.Mutex
	beforePlan, afterPlan, afterFirst func() error
	plans, batches                    int
}

func (s *sourceBoundaryTree) PlanRevert(ctx context.Context, spec transfer.JobSpec, id uuid.UUID, cuts transfer.JobBatches) (transfer.JobPlan, error) {
	s.mu.Lock()
	s.plans++
	before, after := s.beforePlan, s.afterPlan
	s.beforePlan, s.afterPlan = nil, nil
	s.mu.Unlock()
	if before != nil {
		if err := before(); err != nil {
			return nil, err
		}
	}
	p, err := s.Tree.PlanRevert(ctx, spec, id, cuts)
	if err == nil && after != nil {
		if err = after(); err != nil {
			return nil, err
		}
	}
	return p, err
}

func (s *sourceBoundaryTree) RunBatch(ctx context.Context, spec transfer.JobSpec, p transfer.JobPlan, from, to int) (transfer.BatchDone, error) {
	s.mu.Lock()
	s.batches++
	s.mu.Unlock()
	d, err := s.Tree.RunBatch(ctx, spec, p, from, to)
	if err != nil {
		return d, err
	}
	s.mu.Lock()
	after := s.afterFirst
	s.afterFirst = nil
	s.mu.Unlock()
	if after != nil {
		if err = after(); err != nil {
			return d, err
		}
	}
	return d, nil
}

func (s *sourceBoundaryTree) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plans, s.batches
}

// A source identity changes without changing the reverted node's own
// history premise: its parent moves under raw. The request's successful
// precheck grants neither a worker pass nor a later unit pass.
func TestSourceBackgroundRechecksAtTheWorkerAndActualUnit(t *testing.T) {
	for _, boundary := range []string{"before_plan", "after_plan"} {
		t.Run(boundary, func(t *testing.T) {
			port := &sourceBoundaryTree{}
			tm := newAcmeTeamWiring(t, "member", "", wiring{batch: transfer.Batch{Nodes: 1}, transfer: func(d *transfer.Deps) { port.Tree = d.Tree; d.Tree = port }})
			nb := tm.llmWiki(t, "Source boundary")
			parent := tm.createPage(t, "alice", nb.id, "", "Folder")
			p := tm.createPageWith(t, "alice", nb.id, parent, "P", "body")
			tm.send(t, nodeRename("alice", p, "Renamed"), http.StatusOK)
			c := tm.newest(t, nb.id).ID
			move := func() error {
				q := nodeMove("alice", parent, nb.raw)
				status, body := ask(t, tm.contract, q.method, tm.base+q.path, tm.tokens[q.by], q.body)
				if status != http.StatusOK {
					return fmt.Errorf("move parent: %d %s", status, body)
				}
				return nil
			}
			port.mu.Lock()
			if boundary == "before_plan" {
				port.beforePlan = move
			} else {
				port.afterPlan = move
			}
			port.mu.Unlock()
			j := tm.background(t, revertStart("bob", nb.id, c, ""))
			plans, batches := port.counts()
			wantBatches := 0
			if boundary == "after_plan" {
				wantBatches = 1
			}
			if plans != 1 || batches != wantBatches || j.State != "failed" || j.failure() != "refused" || j.Report.Refusal == nil || j.Report.Refusal.Code != "page.source_admin_only" || j.Report.Refusal.NodeID != p || j.Progress.Done != 0 {
				t.Fatalf("%s: plans=%d batches=%d job=%s", boundary, plans, batches, j)
			}
			if tm.bgName(t, p) != "Renamed" || count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id=$1", *j.ChangesetID) != 0 {
				t.Fatalf("refused boundary wrote a change: %s", j)
			}
			tm.wrote(t, p, "body", 1)
			checkBackground(t, tm)
		})
	}
}

func TestSourceDescendantRefusalKeepsAnEarlierCommittedBatch(t *testing.T) {
	port := &sourceBoundaryTree{}
	tm := newAcmeTeamWiring(t, "member", "", wiring{batch: transfer.Batch{Nodes: 1}, transfer: func(d *transfer.Deps) { port.Tree = d.Tree; d.Tree = port }})
	nb := tm.llmWiki(t, "Source batches")
	member := tm.addNotebookMember(t, "alice", nb.id, "bob", "admin")
	a := tm.createPageWith(t, "alice", nb.id, nb.raw, "A", "one")
	b := tm.createPageWith(t, "alice", nb.id, nb.raw, "B", "two")
	c := tm.agent(t, "alice", nb.id).batchDone(t, opMove("raw/A.md", "raw/AA.md"), opMove("raw/B.md", "raw/BB.md"))
	if c.ChangesetID == nil {
		t.Fatal("rename batch did not write")
	}
	port.mu.Lock()
	port.afterFirst = func() error {
		q := request("alice", http.MethodPatch, "/api/v0/notebook-members/"+member, `{"role":"editor"}`)
		status, body := ask(t, tm.contract, q.method, tm.base+q.path, tm.tokens[q.by], q.body)
		if status != http.StatusOK {
			return fmt.Errorf("demote: %d %s", status, body)
		}
		return nil
	}
	port.mu.Unlock()
	j := tm.background(t, revertStart("bob", nb.id, *c.ChangesetID, ""))
	plans, batches := port.counts()
	if plans != 1 || batches != 2 || j.State != "failed" || j.failure() != "refused" || j.Report.Refusal == nil || j.Report.Refusal.Code != "page.source_admin_only" || j.Report.Refusal.NodeID != b || j.Progress.Done != 1 || j.counts()["put_back"] != 1 {
		t.Fatalf("plans=%d batches=%d job=%s", plans, batches, j)
	}
	if tm.bgName(t, a) != "A" || tm.bgName(t, b) != "BB" {
		t.Fatalf("batches left A=%q B=%q", tm.bgName(t, a), tm.bgName(t, b))
	}
	if count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id=$1", *j.ChangesetID) != 1 {
		t.Fatalf("job changeset did not retain exactly the first item: %s", j)
	}
	tm.wrote(t, a, "one", 1)
	tm.wrote(t, b, "two", 1)
	checkBackground(t, tm)
}

func TestSourceBackgroundRequestRefusesBeforeEnqueue(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.llmWiki(t, "Source request")
	p := tm.createPage(t, "bob", nb.id, nb.raw, "Empty")
	tm.send(t, contentWrite("bob", p, "first", 1, ""), http.StatusOK)
	c := tm.newest(t, nb.id).ID
	before := tm.guardRows(t, nb.id)
	for _, by := range []string{"bob", "alice"} {
		tm.guarded(t, nb, "source first-write undo", revertStart(by, nb.id, c, ""), cell{http.StatusConflict, "page.source_immutable"})
	}
	if tm.guardRows(t, nb.id) != before || count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id=$1", nb.id) != 0 || count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind='transfer.revert'") != 0 {
		t.Fatal("refused request queued or wrote")
	}
	tm.wrote(t, p, "first", 2)
	// The failed request released its claim: after moving the page out,
	// exactly the same changeset can start and complete normally.
	tm.send(t, nodeMove("alice", p, ""), http.StatusOK)
	j := tm.background(t, revertStart("bob", nb.id, c, ""))
	if j.State != "succeeded" || j.counts()["written"] != 1 {
		t.Fatalf("claim after refusal: %s", j)
	}
	tm.wrote(t, p, "", 3)
	checkBackground(t, tm)
}
