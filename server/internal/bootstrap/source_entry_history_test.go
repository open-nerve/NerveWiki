package bootstrap

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// A historical place carries today's descendants. Both synchronous undo and
// the job's start must reject a source entry that has grown beyond the bound.
func TestSourceEntryHistoryRefusesBeforeSyncWriteAndEnqueue(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Source history bound")
	root := tm.sourceEntryHistoryTree(t, wiki)
	tm.send(t, nodeMove("alice", root, ""), http.StatusOK)
	changed := tm.newest(t, wiki.id).ID
	late := tm.createPage(t, "alice", wiki.id, root, "Late")
	before := sourceEntryRows(t, tm.pool)
	for _, c := range []step{changesetRevert("alice", changed), revertStart("alice", wiki.id, changed, "")} {
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.body)
		var problem struct{ Code string }
		decodeAnswer(t, body, &problem)
		if status != http.StatusUnprocessableEntity || problem.Code != "page.source_too_large" {
			t.Fatalf("historical source-entry overflow = %d %s, want 422 page.source_too_large", status, body)
		}
		if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
			t.Fatal("refused history request changed persisted state")
		}
	}
	if count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id) != 0 ||
		count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind = 'transfer.revert'") != 0 {
		t.Fatal("source-entry refusal queued a background revert")
	}
	// The failed start released its claim and left the original change intact.
	tm.send(t, nodeDeletion("alice", late), http.StatusNoContent)
	j := tm.background(t, revertStart("alice", wiki.id, changed, ""))
	if j.State != "succeeded" || j.counts()["put_back"] != 1 {
		t.Fatalf("retry at 10000 nodes = %s", j)
	}
	if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2", root, wiki.raw) != 1 {
		t.Fatal("successful retry did not put the subtree back in sources")
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkBackground(t, tm)
}

func TestSourceEntryBackgroundRechecksAtWorkerAndActualUnit(t *testing.T) {
	for _, boundary := range []string{"before_plan", "after_plan"} {
		t.Run(boundary, func(t *testing.T) {
			port := &sourceBoundaryTree{}
			assets := &sourceEntryAttachments{}
			tm := newAcmeTeamWiring(t, "member", "", wiring{
				page:     func(d *page.Deps) { assets.AttachmentInfos, d.Attachments = d.Attachments, assets },
				transfer: func(d *transfer.Deps) { port.Tree = d.Tree; d.Tree = port },
			})
			wiki := tm.llmWiki(t, "Source entry "+boundary)
			root := tm.sourceEntryHistoryTree(t, wiki)
			tm.send(t, nodeMove("alice", root, ""), http.StatusOK)
			changed := tm.newest(t, wiki.id).ID
			grow := func() error {
				assets.calls.Store(0)
				assets.active.Store(true)
				return tm.growSourceEntryTree(t, wiki.id, root)
			}
			port.mu.Lock()
			if boundary == "before_plan" {
				port.beforePlan = grow
			} else {
				port.afterPlan = grow
			}
			port.mu.Unlock()
			j := tm.background(t, revertStart("alice", wiki.id, changed, ""))
			assets.active.Store(false)
			plans, batches := port.counts()
			wantBatches := 0
			if boundary == "after_plan" {
				wantBatches = 1
			}
			if plans != 1 || batches != wantBatches || j.State != "failed" || j.failure() != "refused" ||
				j.Report.Refusal == nil || j.Report.Refusal.Code != "page.source_too_large" ||
				j.Report.Refusal.NodeID != root || j.Progress.Done != 0 {
				t.Fatalf("%s: plans=%d batches=%d job=%s", boundary, plans, batches, j)
			}
			if boundary == "after_plan" && assets.calls.Load() != 0 {
				t.Errorf("locked overflow reached attachment metadata %d times", assets.calls.Load())
			}
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE parent_id = $1 AND name = 'Late' AND deleted_at IS NULL", root) != 1 ||
				count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id IS NULL", root) != 1 ||
				j.ChangesetID == nil || count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1", *j.ChangesetID) != 0 {
				t.Fatalf("growth or refused first-batch persistence is wrong: %s", j)
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkBackground(t, tm)
		})
	}
}

func TestSourceEntryLaterBatchKeepsEarlierCommittedWork(t *testing.T) {
	port := &sourceBoundaryTree{}
	assets := &sourceEntryAttachments{}
	tm := newAcmeTeamWiring(t, "member", "", wiring{
		batch:    transfer.Batch{Nodes: 1},
		page:     func(d *page.Deps) { assets.AttachmentInfos, d.Attachments = d.Attachments, assets },
		transfer: func(d *transfer.Deps) { port.Tree = d.Tree; d.Tree = port },
	})
	wiki := tm.llmWiki(t, "Source entry later batch")
	a := tm.createPage(t, "alice", wiki.id, "", "A")
	root := tm.sourceEntryHistoryTree(t, wiki)
	c := tm.agent(t, "alice", wiki.id).batchDone(t, opMove("A.md", "AA.md"), opMove("raw/Big.md", "Big.md"))
	if c.ChangesetID == nil {
		t.Fatal("the original two-operation batch did not write a changeset")
	}
	beforeRoot := queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id = $1", root)[0]
	port.mu.Lock()
	port.afterFirst = func() error {
		var done bool
		if err := tm.pool.QueryRow(context.Background(), "SELECT name = 'A' FROM nodes WHERE id = $1", a).Scan(&done); err != nil {
			return err
		}
		if !done {
			return fmt.Errorf("first committed batch did not restore A before growing Big")
		}
		assets.calls.Store(0)
		assets.active.Store(true)
		return tm.growSourceEntryTree(t, wiki.id, root)
	}
	port.mu.Unlock()
	j := tm.background(t, revertStart("alice", wiki.id, *c.ChangesetID, ""))
	assets.active.Store(false)
	plans, batches := port.counts()
	if plans != 1 || batches != 2 || j.State != "failed" || j.failure() != "refused" ||
		j.Report.Refusal == nil || j.Report.Refusal.Code != "page.source_too_large" ||
		j.Report.Refusal.NodeID != root || j.Progress.Done != 1 || j.counts()["put_back"] != 1 {
		t.Fatalf("later batch: plans=%d batches=%d job=%s", plans, batches, j)
	}
	if assets.calls.Load() != 0 {
		t.Errorf("later overflow read attachment metadata %d times", assets.calls.Load())
	}
	if tm.bgName(t, a) != "A" || queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id = $1", root)[0] != beforeRoot ||
		count(t, tm.pool, "SELECT count(*) FROM nodes WHERE parent_id = $1 AND name = 'Late' AND deleted_at IS NULL", root) != 1 {
		t.Fatal("the earlier batch was lost, the growth was absent, or the refused root was written")
	}
	if j.ChangesetID == nil || count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1 AND node_id = $2", *j.ChangesetID, a) != 1 ||
		count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1", *j.ChangesetID) != 1 {
		t.Fatalf("job history did not retain exactly the first item: %s", j)
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkBackground(t, tm)
}

// The target parent is absent initially. Restoring it makes a later placement
// a source entry; looking only at the original live tree would miss the cap.
func TestSourceEntryHistoryCountsAfterRestoringItsParent(t *testing.T) {
	for _, restoreChild := range []bool{false, true} {
		t.Run(fmt.Sprint(restoreChild), func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			wiki := tm.llmWiki(t, "Source entry restored parent")
			parent := tm.createPage(t, "alice", wiki.id, wiki.raw, "P")
			root := tm.sourceEntryHistoryTree(t, wiki)
			tm.send(t, nodeMove("alice", root, parent), http.StatusOK)
			ops := []map[string]any{opMove("raw/P/Big.md", "Big.md"), opDelete("raw/P.md")}
			restored := 1
			if restoreChild {
				name := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE parent_id = $1 AND kind = 'page' ORDER BY id LIMIT 1", root)[0]
				ops = append(ops, opDelete("Big/"+name+".md"))
				restored++
			}
			c := tm.agent(t, "alice", wiki.id).batchDone(t, ops...)
			if c.ChangesetID == nil {
				t.Fatal("move-out and parent deletion did not write a changeset")
			}
			late := tm.createPage(t, "alice", wiki.id, root, "Late")
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL", parent) != 1 {
				t.Fatal("historical source parent is not in the trash")
			}
			if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL", root); got != 10002-restored {
				t.Fatalf("before restoring %d groups, Big has %d nodes; want %d before its projected 10001-node placement", restored, got, 10002-restored)
			}
			before := sourceEntryRows(t, tm.pool)
			for _, request := range []step{changesetRevert("alice", *c.ChangesetID), revertStart("alice", wiki.id, *c.ChangesetID, "")} {
				status, body := ask(t, tm.contract, request.method, tm.base+request.path, tm.tokens[request.by], request.body)
				var problem struct{ Code string }
				decodeAnswer(t, body, &problem)
				if status != http.StatusUnprocessableEntity || problem.Code != "page.source_too_large" {
					t.Fatalf("entry after parent restore = %d %s, want 422 source_too_large", status, body)
				}
				if after := sourceEntryRows(t, tm.pool); !maps.Equal(after, before) {
					t.Fatal("refused delayed placement changed the tree, trash, content or history")
				}
			}
			if count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE notebook_id = $1", wiki.id) != 0 ||
				count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind = 'transfer.revert'") != 0 {
				t.Fatal("refused delayed placement queued work")
			}
			// The same schedule is valid at 10000: restoring one parent is not itself
			// a single source-entry move, and must not consume the later move's allowance.
			tm.send(t, nodeDeletion("alice", late), http.StatusNoContent)
			j := tm.background(t, revertStart("alice", wiki.id, *c.ChangesetID, ""))
			if j.State != "succeeded" || j.counts()["restored"] != restored || j.counts()["put_back"] != 1 {
				t.Fatalf("retry with restored parent and 10000 carried nodes = %s", j)
			}
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2 AND deleted_at IS NULL", parent, wiki.raw) != 1 ||
				count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND parent_id = $2 AND deleted_at IS NULL", root, parent) != 1 {
				t.Fatal("successful retry did not restore the parent and then place its subtree")
			}
			if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL", root) != 10000 {
				t.Fatal("retry did not carry exactly 10000 nodes, including any restored child")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkBackground(t, tm)
		})
	}
}

func (tm acmeTeam) sourceEntryHistoryTree(t *testing.T, wiki guardedNotebook) string {
	t.Helper()
	root := tm.createPage(t, "alice", wiki.id, wiki.raw, "Big")
	tm.addSourceEntryPages(t, root, 9998)
	tm.upload(t, "alice", wiki.id, root, "empty.bin", "")
	if got := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE (id = $1 OR parent_id = $1) AND deleted_at IS NULL", root); got != 10000 {
		t.Fatalf("history source-entry fixture has %d nodes, want 10000", got)
	}
	assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
	checkPages(t, tm.pool)
	return root
}

// This real request changes only the carried descendants, not the historical
// premise on the moved root. The decorator invokes it after the chosen real
// worker boundary, never by returning a fabricated plan.
func (tm acmeTeam) growSourceEntryTree(t *testing.T, nb, root string) error {
	t.Helper()
	body := fmt.Sprintf(`{"parent_id":%q,"title":"Late","content":""}`, root)
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/pages", tm.tokens["alice"], body)
	if status != http.StatusCreated {
		return fmt.Errorf("grow source-entry subtree: %d %s", status, answer)
	}
	return nil
}
