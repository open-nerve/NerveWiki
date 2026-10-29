package bootstrap

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// A committed asset is a valid target; two ordinary trashed siblings become
// sources only in the simulated restore, so neither is a valid rollback link.
func sourceDuplicateReportTargets(t *testing.T) {
	for _, kind := range []string{"committed-asset", "restored-internal"} {
		t.Run(kind, func(t *testing.T) {
			port := &sourceBoundaryTree{}
			tm := newAcmeTeamWiring(t, "member", "", wiring{transfer: func(d *transfer.Deps) { port.Tree, d.Tree = d.Tree, port }})
			wiki := tm.llmWiki(t, "Report target")
			var root, refused string
			var change step
			if kind == "committed-asset" {
				root = tm.upload(t, "alice", wiki.id, wiki.raw, "deleted.bin", "").ID
				refused = root
				change = assetUpload(t, "alice", wiki.id, wiki.raw, "known.bin", "")
			} else {
				g := tm.createPage(t, "alice", wiki.id, "", "G")
				root = tm.createPage(t, "alice", wiki.id, g, "F")
				tm.createPageWith(t, "alice", wiki.id, root, "P", "same restored source bytes")
				refused = tm.createPageWith(t, "alice", wiki.id, root, "Q", "same restored source bytes")
				change = nodeMove("alice", g, wiki.raw)
			}
			tm.send(t, nodeDeletion("alice", root), http.StatusNoContent)
			before := queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id=$1", refused)[0]
			send := tm.sender(t, change)
			port.mu.Lock()
			port.beforePlan = func() error {
				got := send()
				if got.err != nil {
					return got.err
				}
				want := http.StatusOK
				if kind == "committed-asset" {
					want = http.StatusCreated
				}
				if got.status != want {
					return fmt.Errorf("change before real restore plan = %d %s, want %d", got.status, got.body, want)
				}
				return nil
			}
			port.mu.Unlock()
			j := tm.background(t, trashRestoreStart("alice", wiki.id, root))
			plans, batches := port.counts()
			if plans != 1 || batches != 0 || j.State != "failed" || j.failure() != "refused" || j.Progress.Done != 0 {
				t.Errorf("restore report: plans=%d batches=%d job=%s, want plan refusal before any batch", plans, batches, j)
			}
			var target *string
			if kind == "committed-asset" {
				ids := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id=$1 AND name='known.bin' AND deleted_at IS NULL", wiki.id)
				if len(ids) != 1 {
					t.Fatalf("committed target IDs = %v", ids)
				}
				target = &ids[0]
			}
			assertSourceRefusalReport(t, tm, wiki.id, j.ID, refused, target)
			if after := queryStrings(t, tm.pool, "SELECT to_jsonb(n)::text FROM nodes n WHERE id=$1", refused)[0]; after != before {
				t.Error("refused worker changed a trashed member")
			}
			if j.ChangesetID == nil || count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id=$1", *j.ChangesetID) != 0 {
				t.Error("refused worker created its assigned changeset")
			}
			assertSourceHashesMatchRebuild(t, t.Context(), tm.pool)
			checkBackground(t, tm)
			checkAssets(t, tm.pool, tm.storage)
		})
	}
}

func assertSourceRefusalReport(t *testing.T, tm acmeTeam, nb, job, node string, target *string) {
	t.Helper()
	check := func(where string, report map[string]any, required bool) {
		t.Helper()
		refusal, _ := report["refusal"].(map[string]any)
		got, present := refusal["duplicate_of"]
		var want any
		if target != nil {
			want = *target
		}
		if report["failure"] != "refused" || refusal["code"] != "page.source_duplicate" || refusal["node_id"] != node || required && !present || got != want {
			t.Errorf("%s refusal = %v, want node %s and required duplicate_of=%v", where, report, node, want)
		}
	}
	var persisted string
	if err := tm.pool.QueryRow(t.Context(), "SELECT report::text FROM transfer_jobs WHERE id=$1", job).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	decodeAnswer(t, persisted, &report)
	check("persisted JSON", report, false)
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+job, tm.tokens["alice"], "")
	var detail struct {
		Report map[string]any `json:"report"`
	}
	decodeAnswer(t, body, &detail)
	if status != http.StatusOK {
		t.Errorf("report detail = %d %s", status, body)
	}
	check("detail HTTP", detail.Report, true)
	status, body = ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+nb+"/transfer-jobs", tm.tokens["alice"], "")
	var list struct {
		Data []struct {
			ID     string         `json:"id"`
			Report map[string]any `json:"report"`
		} `json:"data"`
	}
	decodeAnswer(t, body, &list)
	found := 0
	for _, row := range list.Data {
		if row.ID == job {
			found++
			check("list HTTP", row.Report, true)
		}
	}
	if status != http.StatusOK || found != 1 {
		t.Errorf("report list = %d %s, want the finished job exactly once", status, body)
	}
}
