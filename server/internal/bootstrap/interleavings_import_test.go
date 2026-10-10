package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The imports' interleavings (M7/P6 design 3.16; v0.1 design 13.4, item
// 4), through serve and River: an import's unit and a save of its
// notebook; an import's unit and a new page of a name it creates, in both
// orders; a job's creation and its notebook's deletion, in both orders; a
// cancel of a running import; the place it goes deleted as it runs. A unit
// is stopped by a lock of asset_blobs, which it writes as it creates an
// attachment, holding its notebook's row FOR NO KEY UPDATE. Each ends on
// the invariants: the tree, the links, each live attachment's one row, no
// live job left queued or running, no import's archive but a running
// one's.

// importStep is by's import of archive into the notebook nb, under its
// page parent when it is not "".
func importStep(t *testing.T, by, nb, parent string, archive []byte) step {
	t.Helper()
	contentType, body := importBody(t, parent, "vault.zip", archive)
	return step{by: by, method: http.MethodPost, path: "/api/v0/notebooks/" + nb + "/imports", body: body, contentType: contentType}
}

// started starts c, an import, and answers its job, queued.
func (tm acmeTeam) started(t *testing.T, c step) transferJob {
	t.Helper()
	status, answer := askTyped(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.contentType, c.body)
	if status != http.StatusAccepted {
		t.Fatalf("%s as %s = %d %s", c.name(), c.by, status, answer)
	}
	var j transferJob
	decodeAnswer(t, answer, &j)
	return j
}

// inBackground sends c on a goroutine of its own, and answers what waits
// for its answer.
func (tm acmeTeam) inBackground(t *testing.T, c step) func() answer {
	t.Helper()
	send, done := tm.sender(t, c), make(chan answer, 1)
	go func() { done <- send() }()
	return func() answer {
		t.Helper()
		select {
		case a := <-done:
			if a.res == nil {
				t.Fatalf("%s failed: %v", c.name(), a.err)
			}
			tm.contract.CheckResponse(t, a.req, a.res)
			return a
		case <-time.After(interleavingWait):
			t.Fatalf("%s did not answer", c.name())
			return answer{}
		}
	}
}

// awaitImports waits until River has completed every import.
func awaitImports(t *testing.T, tm acmeTeam) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); count(t, tm.pool,
		"SELECT count(*) FROM river_job WHERE kind = 'transfer.import' AND state <> 'completed'") != 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("imports not completed: %v", queryStrings(t, tm.pool, "SELECT state FROM river_job WHERE kind = 'transfer.import'"))
		}
	}
}

// checkImports checks the invariants once River has completed every
// import.
func checkImports(t *testing.T, tm acmeTeam) {
	t.Helper()
	awaitImports(t, tm)
	awaitExports(t, tm)
	checkTransfers(t, tm)
	checkPages(t, tm.pool)
	checkLinks(t, tm.pool)
	checkAssets(t, tm.pool, tm.storage)
}

// manyPages is a vault of an attachment, a.png, then n pages: the
// attachment and the first 99 pages are the first unit's.
func manyPages(t *testing.T, n int) []byte {
	t.Helper()
	files := []vaultFile{{"a.png", pngHead}}
	for i := range n {
		files = append(files, vaultFile{fmt.Sprintf("p%03d.md", i), "[[a.png]]"})
	}
	return vaultZip(t, files...)
}

// An import's unit and a save of its place at once: the unit, held as it
// writes an attachment's row, holds the notebook's row; the save waits for
// it, then both are written.
func TestAnImportsUnitAndASaveAtOnce(t *testing.T) {
	tm, nb, spec := transferTeam(t)
	release := tm.holdAssetBlobs(t)
	j := tm.started(t, importStep(t, "bob", nb, spec, vaultZip(t, vaultFile{"A.md", "a"}, vaultFile{"A/x.png", pngHead})))
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	saved := tm.inBackground(t, contentWrite("alice", spec, "after", 1, ""))
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	release()
	if a := saved(); a.status != http.StatusOK {
		t.Errorf("the save = %d %s, want 200", a.status, a.body)
	}
	ended := tm.endedJob(t, "bob", j.ID)
	if ended.State != "succeeded" || ended.Report.Counts["pages"] != 1 || ended.Report.Counts["attachments"] != 1 {
		t.Errorf("the import = %+v, want succeeded", ended)
	}
	if got := queryStrings(t, tm.pool, "SELECT content FROM page_contents WHERE node_id = $1", spec); len(got) != 1 || got[0] != "after" {
		t.Errorf("Spec's content = %q, want the save's", got)
	}
	checkImports(t, tm)
}

// An import and a new page of a name it creates. The page first: the
// import's is numbered, reported renamed. The import's unit first, held:
// the new page waits for the notebook's row, then finds the name taken.
func TestAnImportAndANewPageOfItsName(t *testing.T) {
	t.Run("the page first", func(t *testing.T) {
		tm, nb, _ := transferTeam(t)
		tm.createPage(t, "alice", nb, "", "a")
		ended := tm.imported(t, "bob", nb, "", vaultZip(t, vaultFile{"a.md", "mine"}))
		if ended.State != "succeeded" || len(ended.Problems) != 1 || ended.Problems[0].Code != "renamed" {
			t.Errorf("the import = %+v, want a renamed", ended)
		}
		if got := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE notebook_id = $1 AND parent_id IS NULL AND name LIKE 'a%' ORDER BY name",
			nb); len(got) != 2 || got[0] != "a" || got[1] != "a 2" {
			t.Errorf("the root's pages = %q, want a and the import's a 2", got)
		}
		checkImports(t, tm)
	})
	t.Run("the import first", func(t *testing.T) {
		tm, nb, _ := transferTeam(t)
		release := tm.holdAssetBlobs(t)
		j := tm.started(t, importStep(t, "bob", nb, "", vaultZip(t, vaultFile{"a.md", "mine"}, vaultFile{"z.png", pngHead})))
		pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)
		created := tm.inBackground(t, request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/pages", `{"parent_id":null,"title":"a"}`))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		release()
		if a := created(); !a.is(http.StatusConflict, "page.title_taken") {
			t.Errorf("the new page = %d %s, want 409 page.title_taken", a.status, a.body)
		}
		if ended := tm.endedJob(t, "bob", j.ID); ended.State != "succeeded" || len(ended.Problems) != 0 {
			t.Errorf("the import = %+v, want succeeded, a its own", ended)
		}
		checkImports(t, tm)
	})
}

// holdImportWorker takes the imports' one worker (jobs.import_workers):
// alice's import into a notebook of hers, Ops, which River runs and holds
// at its start, on its row, which a transaction of the test's own locks
// until release, which the test's end calls too. Its archive is not in
// the store: it fails once let go.
func (tm acmeTeam) holdImportWorker(t *testing.T) (release func()) {
	t.Helper()
	ctx := context.Background()
	ops := tm.openNotebook(t, "alice", "Ops")
	id := uuid.NewV7()
	if _, err := tm.pool.Exec(ctx, `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
		SELECT $1, $2, 'import', 'queued', 'ops.zip', id, 'api', now() FROM users WHERE email = 'alice@example.com'`, id, ops); err != nil {
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
		return inserter.InsertTx(ctx, tx, riveradapter.ImportArgs{JobID: id}, &river.InsertOpts{Queue: transfer.QueueImport, MaxAttempts: 1})
	})
	if err != nil {
		t.Fatal(err)
	}
	pgtest.WaitForLockWaitsOn(t, tm.pool, "transfer_jobs", 1, interleavingWait)
	return release
}

// An import's start and its notebook's deletion. The deletion first: the
// start finds the notebook deleted once it shares its row, 404, no job,
// its archive deleted. The start first, the imports' worker held: the job
// is deleted with the notebook, at its time, still queued; let go, its
// worker finds it deleted and deletes its archive.
func TestDeletingANotebookAndStartingAnImport(t *testing.T) {
	deletion := func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") }
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		deleted, started := tm.interleaveOn(t, notebookRow(nb), deletion(nb), importStep(t, "bob", nb, "", vaultZip(t, vaultFile{"a.md", "a"})))
		if !deleted.is(http.StatusNoContent, "") || !started.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("DELETE the notebook = %d, then the import = %d %s; want 204, then 404 notebook.not_found", deleted.status,
				started.status, started.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs"); n != 0 {
			t.Errorf("%d jobs, want none", n)
		}
		checkImports(t, tm)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the start first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		release := tm.holdImportWorker(t)
		j := tm.started(t, importStep(t, "bob", nb, "", vaultZip(t, vaultFile{"a.md", "a"})))
		tm.send(t, deletion(nb), http.StatusNoContent)
		release()
		awaitImports(t, tm)
		if n := count(t, tm.pool, `SELECT count(*) FROM transfer_jobs j JOIN notebooks n ON n.id = j.notebook_id
			WHERE j.id = $1 AND j.deleted_at = n.deleted_at AND j.state = 'queued'`, j.ID); n != 1 {
			t.Errorf("%d jobs deleted with the notebook at its time, never started; want the import", n)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", nb); n != 0 {
			t.Errorf("%d nodes, want none imported", n)
		}
		checkImports(t, tm)
		checkNotebooks(t, tm.pool)
	})
}

// A cancel of an import running, held in its first unit: the cancel
// answers at once, the job still running; once its heartbeat has read the
// cancel, let go, the unit is written, the next is not. The job ends
// cancelled, its report counting the first unit's, its archive deleted.
func TestCancellingARunningImport(t *testing.T) {
	tm, nb, _ := transferTeam(t)
	release := tm.holdAssetBlobs(t)
	j := tm.started(t, importStep(t, "bob", nb, "", manyPages(t, 150)))
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/transfer-jobs/"+j.ID+"/cancel", tm.tokens["bob"], "")
	var asked transferJob
	decodeAnswer(t, answer, &asked)
	if status != http.StatusOK || asked.State != "running" {
		t.Fatalf("cancel = %d %s, want 200, running", status, answer)
	}
	for deadline := time.Now().Add(interleavingWait); count(t, tm.pool,
		"SELECT count(*) FROM transfer_jobs WHERE id = $1 AND heartbeat_at > cancel_requested_at", j.ID) == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the heartbeat did not read the cancel")
		}
	}
	release()
	ended := tm.endedJob(t, "bob", j.ID)
	if ended.State != "cancelled" || ended.Report == nil || ended.Report.Failure != nil || ended.Report.Counts["pages"] != 99 ||
		ended.Report.Counts["attachments"] != 1 || ended.Progress.Done != 100 || ended.Progress.Total != 151 {
		t.Errorf("the import = %+v, report %s; want cancelled after its first unit", ended, describe(ended.Report))
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name LIKE 'p%'", nb); n != 99 {
		t.Errorf("%d pages imported, want the first unit's 99", n)
	}
	checkImports(t, tm)
}

// The place an import goes deleted as it runs: the deletion waits for the
// first unit, held, then deletes the place with what it holds; the next
// unit finds the place gone, and the job fails root_not_found.
func TestAnImportWhosePlaceIsDeleted(t *testing.T) {
	tm, nb, _ := transferTeam(t)
	place := tm.createPage(t, "alice", nb, "", "Place")
	release := tm.holdAssetBlobs(t)
	j := tm.started(t, importStep(t, "bob", nb, place, manyPages(t, 150)))
	pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)

	deleted := tm.inBackground(t, request("alice", http.MethodDelete, "/api/v0/nodes/"+place, ""))
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	release()
	if a := deleted(); a.status != http.StatusNoContent && a.status != http.StatusOK {
		t.Errorf("the place's deletion = %d %s, want it done", a.status, a.body)
	}
	ended := tm.endedJob(t, "bob", j.ID)
	if ended.State != "failed" || ended.Report == nil || ended.Report.Failure == nil || *ended.Report.Failure != "root_not_found" ||
		ended.Report.Counts["pages"] != 99 {
		t.Errorf("the import = %+v, report %s; want failed root_not_found after its first unit", ended, describe(ended.Report))
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN nodes p ON p.id = $1
		WHERE n.parent_id = p.id AND n.deleted_at = p.deleted_at`, place); n != 100 {
		t.Errorf("%d of the first unit's nodes deleted with their place, want 100", n)
	}
	checkImports(t, tm)
}
