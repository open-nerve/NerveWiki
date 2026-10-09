package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The exports' interleavings (M7/P5 design 3.14; v0.1 design 13.4, item
// 4), through serve and River: a job's creation and its notebook's
// deletion, in both orders; a cancel of a running export; an export's
// snapshot and a save at once; an export's success and its notebook's
// deletion. A job is stopped in its snapshot by a lock of asset_blobs,
// which it reads there, after the pages and their contents' sizes, and
// which no page's write touches. Each ends on the invariant: no live job
// left queued or running, no archive but a succeeded export's, live or
// deleted with its notebook and not yet purged.

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
// 3.14; M7/P6 design 3.16): a job not deleted still queued or running once
// River's every export and import has completed; an archive in the store
// that no export that succeeded keeps, live or deleted with its notebook
// (the sweep's, a day later); an import's that no import queued or running
// keeps, not deleted.
func checkTransfers(t *testing.T, tm acmeTeam) {
	t.Helper()
	if n := count(t, tm.pool, "SELECT count(*) FROM river_job WHERE kind IN ('transfer.export', 'transfer.import') AND state <> 'completed'"); n != 0 {
		t.Errorf("%d jobs not completed by River", n)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE state IN ('queued', 'running') AND deleted_at IS NULL"); n != 0 {
		t.Errorf("%d jobs left queued or running", n)
	}
	live := map[string]bool{}
	for _, id := range queryStrings(t, tm.pool, "SELECT id::text FROM transfer_jobs WHERE state = 'succeeded'") {
		live[id] = true
	}
	for id := range storedArchives(t, tm.storage) {
		if !live[id] {
			t.Errorf("the archive of %s, which no live export keeps", id)
		}
	}
	importing := map[string]bool{}
	for _, id := range queryStrings(t, tm.pool, "SELECT id::text FROM transfer_jobs WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL") {
		importing[id] = true
	}
	for _, path := range storedImports(t, tm.storage) {
		if id := strings.TrimSuffix(filepath.Base(path), ".zip"); !importing[id] {
			t.Errorf("the archive of %s, which no import queued or running keeps", id)
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

// holdExportWorker takes the exports' one worker (jobs.export_workers):
// alice's export of a notebook of hers, Ops, which River runs and holds at
// its start, on its row, which a transaction of the test's own locks until
// release, which the test's end calls too.
func (tm acmeTeam) holdExportWorker(t *testing.T) (release func()) {
	t.Helper()
	ctx := context.Background()
	ops := tm.openNotebook(t, "alice", "Ops")
	tm.createPageWith(t, "alice", ops, "", "Runbook", "runbook")
	id := uuid.NewV7()
	if _, err := tm.pool.Exec(ctx, `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
		SELECT $1, $2, 'export', 'queued', 'Ops', id, 'api', now() FROM users WHERE email = 'alice@example.com'`, id, ops); err != nil {
		t.Fatal(err)
	}
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	release = func() { _ = holder.Rollback(ctx) }
	t.Cleanup(release)
	if _, err := holder.Exec(ctx, "SELECT 1 FROM transfer_jobs WHERE id = $1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	inserter, err := jobs.NewInserter(tm.pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	err = pgx.BeginFunc(ctx, tm.pool, func(tx pgx.Tx) error {
		return inserter.InsertTx(ctx, tx, riveradapter.ExportArgs{JobID: id}, &river.InsertOpts{Queue: transfer.QueueExport, MaxAttempts: 1})
	})
	if err != nil {
		t.Fatal(err)
	}
	pgtest.WaitForLockWaitsOn(t, tm.pool, "transfer_jobs", 1, interleavingWait)
	return release
}

// An export's start and its notebook's deletion. The deletion first: the
// start finds the notebook deleted once it shares its row, 404, no job.
// The start first, the exports' worker held: the job is deleted with the
// notebook, at its time, still queued; let go, its worker finds it deleted
// and does nothing. A job deleted as it runs stops at its next heartbeat
// (the module's tests).
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
		release := tm.holdExportWorker(t)
		j := tm.startExport(t, "bob", nb, "")
		tm.send(t, deletion(nb), http.StatusNoContent)
		release()
		awaitExports(t, tm)
		if n := count(t, tm.pool, `SELECT count(*) FROM transfer_jobs j JOIN notebooks n ON n.id = j.notebook_id
			WHERE j.id = $1 AND j.deleted_at = n.deleted_at AND j.state = 'queued'`, j.ID); n != 1 {
			t.Errorf("%d jobs deleted with the notebook at its time, never started; want the export", n)
		}
		if status, _ := tm.transferJobOf(t, "bob", j.ID); status != http.StatusNotFound {
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

// An export's success and its notebook's deletion. A deletion locks the
// notebook's row, then deletes the jobs' rows, an earlier success's among
// them; the success, held in its snapshot until then, locks the
// notebook's row first too, before its own row and the earlier one it
// expires, and waits: neither waits for a job's row the other holds, no
// deadlock. The deletion commits; the job, deleted, ends unwritten, its
// River job completed, its archive dropped. The test's transaction takes
// the deletion's locks in its order: it stops between the earlier job's
// row and the rest.
func TestAnExportsSuccessAndItsNotebooksDeletion(t *testing.T) {
	tm, nb, _ := transferTeam(t)
	ctx := context.Background()
	earlier := tm.endedJob(t, "alice", tm.startExport(t, "alice", nb, "").ID)
	if earlier.State != "succeeded" {
		t.Fatalf("the earlier export = %+v, want succeeded", earlier)
	}
	awaitExports(t, tm)
	release := tm.holdAssetBlobs(t)
	j := tm.startExport(t, "alice", nb, "")
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	deletion, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deletion.Rollback(ctx) })
	at := time.Now().UTC()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"UPDATE notebooks SET deleted_at = $2 WHERE id = $1", []any{nb, at}},
		{"UPDATE transfer_jobs SET deleted_at = $2 WHERE id = $1", []any{earlier.ID, at}},
	} {
		if _, err := deletion.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	release()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	if _, err := deletion.Exec(ctx, "UPDATE transfer_jobs SET deleted_at = $2 WHERE notebook_id = $1 AND deleted_at IS NULL", nb, at); err != nil {
		t.Fatalf("the deletion of the jobs: %v", err)
	}
	if err := deletion.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	awaitExports(t, tm)
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE id = $1 AND state = 'running' AND deleted_at IS NOT NULL", j.ID); n != 1 {
		t.Errorf("%d jobs deleted while running, want the export, its end unwritten", n)
	}
	if got := storedArchives(t, tm.storage); got[j.ID] != "" {
		t.Errorf("the deleted export's archive is kept: %v", got)
	}
	checkTransfers(t, tm)
}
