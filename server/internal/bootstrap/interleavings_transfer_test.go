package bootstrap

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The exports' interleavings (M7/P5 design 3.14; v0.1 design 13.4, item
// 4), through serve and River: a job's creation and its notebook's
// deletion, in both orders; a cancel of a running export; an export's
// snapshot and a save at once. A job is stopped in its snapshot by a lock
// of asset_blobs, which it reads there, after the pages and their
// contents' sizes, and which no page's write touches. Each ends on the
// invariant: no job left running, no archive but a live export's.

// transferTeam is acme with Eng, open to its members, and alice's page
// Spec, its content "before", and an attachment under it, once serve's
// first jobs have run: none of them then reads asset_blobs as a test
// holds it. Its pool has room for an export held in its snapshot, its
// heartbeat, the requests and River's.
func transferTeam(t *testing.T) (tm acmeTeam, nb, spec string) {
	t.Helper()
	tm = newAcmeTeamWith(t, "member", "", func(c *config.Config) { c.Database.MaxConns = 10 })
	nb = tm.openNotebook(t, "alice", "Eng")
	spec = tm.createPageWith(t, "alice", nb, "", "Spec", "before")
	tm.upload(t, "alice", nb, spec, "x.png", pngHead)
	awaitJob(t, tm.pool, "asset.sweep_orphan_files")
	awaitJob(t, tm.pool, jobs.PurgeKind)
	return tm, nb, spec
}

// holdAssetBlobs locks asset_blobs in a transaction of the test's own
// until release, which the test's end calls too.
func (tm acmeTeam) holdAssetBlobs(t *testing.T) (release func()) {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(ctx, "LOCK TABLE asset_blobs IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	release = func() { _ = holder.Rollback(ctx) }
	t.Cleanup(release)
	return release
}

// checkTransfers fails t when the jobs break an invariant (M7/P5 design
// 3.14): a job not deleted still queued or running once River's every
// export has completed; an archive in the store that no live export that
// succeeded keeps.
func checkTransfers(t *testing.T, tm acmeTeam) {
	t.Helper()
	if n := count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind = 'transfer.export' AND state <> 'completed'"); n != 0 {
		t.Errorf("%d exports not completed by River", n)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE state IN ('queued', 'running') AND deleted_at IS NULL"); n != 0 {
		t.Errorf("%d jobs left queued or running", n)
	}
	live := map[string]bool{}
	for _, id := range queryStrings(t, tm.pool, "SELECT id::text FROM transfer_jobs WHERE state = 'succeeded' AND deleted_at IS NULL") {
		live[id] = true
	}
	for id := range storedArchives(t, tm.storage) {
		if !live[id] {
			t.Errorf("the archive of %s, which no live export keeps", id)
		}
	}
}

// awaitExports waits until River has completed every export.
func awaitExports(t *testing.T, tm acmeTeam) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); count(t, tm.pool,
		"SELECT count(*) FROM river_job WHERE kind = 'transfer.export' AND state <> 'completed'") != 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("exports not completed: %v", queryStrings(t, tm.pool, "SELECT state FROM river_job WHERE kind = 'transfer.export'"))
		}
	}
}

// An export's start and its notebook's deletion. The deletion first: the
// start finds the notebook deleted once it shares its row, 404, no job.
// The start first: the job is deleted with the notebook, at its time; its
// worker finds it deleted, or stops at its next heartbeat, and keeps no
// archive.
func TestDeletingANotebookAndStartingAnExport(t *testing.T) {
	deletion := func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") }
	start := func(nb string) step { return request("bob", http.MethodPost, "/api/v0/notebooks/"+nb+"/exports", `{}`) }
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		deleted, started := tm.interleaveOn(t, notebookRow(nb), deletion(nb), start(nb))
		if !deleted.is(http.StatusNoContent, "") || !started.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("DELETE the notebook = %d, then the export = %d %s; want 204, then 404 notebook.not_found", deleted.status,
				started.status, started.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs"); n != 0 {
			t.Errorf("%d jobs, want none", n)
		}
		checkTransfers(t, tm)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the start first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		tm.createPageWith(t, "alice", nb, "", "Spec", "spec")
		started, deleted := tm.interleaveOn(t, notebookRow(nb), start(nb), deletion(nb))
		if !started.is(http.StatusAccepted, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("the export = %d %s, then DELETE the notebook = %d; want 202, then 204", started.status, started.code, deleted.status)
		}
		awaitExports(t, tm)
		if n := count(t, tm.pool, `SELECT count(*) FROM transfer_jobs j JOIN notebooks n ON n.id = j.notebook_id
			WHERE j.deleted_at = n.deleted_at AND j.state IN ('queued', 'running')`); n != 1 {
			t.Errorf("%d jobs deleted with the notebook at its time, never ended; want the export", n)
		}
		if status, _ := tm.transferJobOf(t, "bob", idOf(t, started)); status != http.StatusNotFound {
			t.Errorf("read the job = %d, want 404", status)
		}
		checkTransfers(t, tm)
		checkNotebooks(t, tm.pool)
	})
}

// A cancel of an export running, held in its snapshot: the cancel answers
// at once, the job still running, its cancel asked; the job's heartbeat
// reads it within a second and stops the job, its read cut short, though
// the lock is still held. It ends cancelled, its report written, its
// archive not kept.
func TestCancellingARunningExport(t *testing.T) {
	tm, nb, _ := transferTeam(t)
	release := tm.holdAssetBlobs(t)
	j := tm.startExport(t, "bob", nb, "")
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/transfer-jobs/"+j.ID+"/cancel", tm.tokens["bob"], "")
	var asked transferJob
	decodeAnswer(t, answer, &asked)
	if status != http.StatusOK || asked.State != "running" {
		t.Fatalf("cancel = %d %s, want 200, running", status, answer)
	}
	ended := tm.endedJob(t, "bob", j.ID)
	release()
	if ended.State != "cancelled" || ended.Report == nil || ended.Report.Failure != nil || ended.Download != nil {
		t.Errorf("the export = %+v, want cancelled, its report without a failure", ended)
	}
	awaitExports(t, tm)
	checkTransfers(t, tm)
}

// An export's snapshot and a save at once: held in its snapshot, the job
// has read the pages; a save of Spec commits meanwhile; let go, the job
// writes Spec as it was when its snapshot began.
func TestAnExportsSnapshotAndASaveAtOnce(t *testing.T) {
	tm, nb, spec := transferTeam(t)
	release := tm.holdAssetBlobs(t)
	j := tm.startExport(t, "bob", nb, "")
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	tm.send(t, contentWrite("alice", spec, "after", 1, ""), http.StatusOK)
	release()
	ended := tm.endedJob(t, "bob", j.ID)
	if ended.State != "succeeded" || ended.Download == nil {
		t.Fatalf("the export = %+v, want succeeded", ended)
	}
	_, entries, _ := tm.archiveAt(t, ended.Download.URL)
	if got := entries["Eng/Spec.md"].data; got != "before" {
		t.Errorf("Spec.md = %q, want the content of the snapshot, before", got)
	}
	var content string
	if err := tm.pool.QueryRow(context.Background(), "SELECT content FROM page_contents WHERE node_id = $1", spec).Scan(&content); err != nil ||
		content != "after" {
		t.Errorf("Spec's content = %q, %v; want the save's", content, err)
	}
	awaitExports(t, tm)
	checkTransfers(t, tm)
}
