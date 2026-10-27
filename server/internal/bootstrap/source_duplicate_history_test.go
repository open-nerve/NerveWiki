package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// A real source appears after admission, before the worker's plan or after it.
// Each boundary must refuse at its own layer and persist the committed target.
func TestSourceBackgroundRefusalsNameTheirActualLayer(t *testing.T) {
	for _, kind := range []string{"page", "asset"} {
		for _, boundary := range []string{"before_plan", "after_plan", "after_first"} {
			t.Run(kind+"/"+boundary, func(t *testing.T) {
				port := &sourceBoundaryTree{}
				tm := newAcmeTeamWiring(t, "member", "", wiring{
					batch: transfer.Batch{Nodes: 1},
					transfer: func(d *transfer.Deps) {
						port.Tree, d.Tree = d.Tree, port
					},
				})
				wiki := tm.llmWiki(t, "Late duplicate")
				// A's earlier UUID ensures the independent rename is the first
				// committed undo batch in the after_first case.
				a := tm.createPage(t, "alice", wiki.id, "", "A")
				const content = "same late source bytes"
				var original, originalPath, knownName string
				var createKnown step
				if kind == "page" {
					original = tm.createPageWith(t, "alice", wiki.id, wiki.raw, "Original", content)
					originalPath, knownName = "Original.md", "Known"
					body, err := json.Marshal(map[string]any{"parent_id": wiki.raw, "title": knownName, "content": content})
					if err != nil {
						t.Fatal(err)
					}
					createKnown = request("alice", http.MethodPost, "/api/v0/notebooks/"+wiki.id+"/pages", string(body))
				} else {
					original = tm.upload(t, "alice", wiki.id, wiki.raw, "original.bin", content).ID
					originalPath, knownName = "original.bin", "known.bin"
					createKnown = assetUpload(t, "alice", wiki.id, wiki.raw, knownName, content)
				}
				var changed string
				if boundary == "after_first" {
					c := tm.agent(t, "alice", wiki.id).batchDone(t,
						opMove("A.md", "AA.md"), opMove("raw/"+originalPath, originalPath))
					if c.ChangesetID == nil {
						t.Fatal("original two-operation batch wrote no changeset")
					}
					changed = *c.ChangesetID
				} else {
					tm.send(t, nodeMove("alice", original, ""), http.StatusOK)
					changed = tm.newest(t, wiki.id).ID
				}
				before := queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id = $1", original)[0]
				send := tm.sender(t, createKnown)
				create := func() error {
					if boundary == "after_first" {
						var restored bool
						if err := tm.pool.QueryRow(context.Background(), "SELECT name = 'A' FROM nodes WHERE id = $1", a).Scan(&restored); err != nil {
							return err
						}
						if !restored {
							return fmt.Errorf("A was not committed before the later duplicate appeared")
						}
					}
					got := send()
					if got.err != nil {
						return got.err
					}
					if got.status != http.StatusCreated {
						return fmt.Errorf("create committed duplicate target = %d %s", got.status, got.body)
					}
					return nil
				}
				port.mu.Lock()
				switch boundary {
				case "before_plan":
					port.beforePlan = create
				case "after_plan":
					port.afterPlan = create
				case "after_first":
					port.afterFirst = create
				}
				port.mu.Unlock()
				j := tm.background(t, revertStart("alice", wiki.id, changed, ""))
				plans, batches := port.counts()
				wantBatches, done := 0, 0
				switch boundary {
				case "after_plan":
					wantBatches = 1
				case "after_first":
					wantBatches, done = 2, 1
				}
				targets := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = $2 AND deleted_at IS NULL", wiki.id, knownName)
				if len(targets) != 1 {
					t.Fatalf("the committed target is missing: %v", targets)
				}
				if plans != 1 || batches != wantBatches || j.State != "failed" || j.failure() != "refused" ||
					j.Progress.Done != done || j.counts()["put_back"] != done || j.Report == nil || j.Report.Refusal == nil ||
					j.Report.Refusal.Code != "page.source_duplicate" || j.Report.Refusal.NodeID != original ||
					j.Report.Refusal.DuplicateOf == nil || *j.Report.Refusal.DuplicateOf != targets[0] {
					t.Errorf("%s: plans=%d batches=%d target=%s job=%s", boundary, plans, batches, targets[0], j)
				}
				if after := queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id = $1", original)[0]; after != before {
					t.Error("refused worker changed the original node")
				}
				if j.ChangesetID == nil {
					t.Fatal("job has no assigned changeset ID")
				}
				if got := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1", *j.ChangesetID); got != done {
					t.Errorf("job persisted %d changesets, want %d", got, done)
				}
				if got := count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1", *j.ChangesetID); got != done {
					t.Errorf("job persisted %d items, want %d", got, done)
				}
				if boundary == "after_first" && (tm.bgName(t, a) != "A" ||
					count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1 AND node_id = $2", *j.ChangesetID, a) != 1) {
					t.Error("the later refusal lost or misidentified the first committed batch")
				}
				assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
				checkBackground(t, tm)
				checkAssets(t, tm.pool, tm.storage)
			})
		}
	}
}
