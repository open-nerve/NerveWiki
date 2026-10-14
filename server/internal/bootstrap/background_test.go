package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The background restores and reverts through serve (M8/P4 design 3.12;
// v0.1 design 13.1, item 21): what a request refuses as too large is
// started through the API as a job, which River runs in the imports'
// queue, its batches written through the page module's units, the links
// indexed by the linking module's observers, the attachments' rows by the
// asset module's. Each module's part reaches the job through the
// composition root: a part left out fails these tests.

// trashRestoreStart is by's background restore of the trash item of the
// notebook nb whose root is id.
func trashRestoreStart(by, nb, id string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/trash-restores", `{"node_id":"`+id+`"}`)
}

// revertStart is by's background revert of the changeset id of the
// notebook nb, with message when it is not "".
func revertStart(by, nb, id, message string) step {
	body := `{"changeset_id":"` + id + `"}`
	if message != "" {
		body = `{"changeset_id":"` + id + `","message":"` + message + `"}`
	}
	return request(by, http.MethodPost, "/api/v0/notebooks/"+nb+"/reverts", body)
}

// backgroundJob is a background job's TransferJobDetail answer, as much as
// the tests read.
type backgroundJob struct {
	ID                string  `json:"id"`
	Kind              string  `json:"kind"`
	State             string  `json:"state"`
	Name              string  `json:"name"`
	RootID            *string `json:"root_id"`
	ChangesetID       *string `json:"changeset_id"`
	TargetChangesetID *string `json:"target_changeset_id"`
	Progress          struct {
		Done  int `json:"done"`
		Total int `json:"total"`
	} `json:"progress"`
	Report *struct {
		Failure *string        `json:"failure"`
		Counts  map[string]int `json:"counts"`
		Placed  *placed        `json:"placed"`
		Lock    *struct {
			PageID      string `json:"page_id"`
			UserID      string `json:"user_id"`
			DisplayName string `json:"display_name"`
		} `json:"lock"`
	} `json:"report"`
	Conflicts     []revertConflict `json:"conflicts"`
	ConflictCount int              `json:"conflict_count"`
}

// failure is the job's failure, "" for none.
func (j backgroundJob) failure() string {
	if j.Report == nil || j.Report.Failure == nil {
		return ""
	}
	return *j.Report.Failure
}

// counts are the job's report's counts, none before it ended.
func (j backgroundJob) counts() map[string]int {
	if j.Report == nil {
		return nil
	}
	return j.Report.Counts
}

// String tells the job as a failure's message does.
func (j backgroundJob) String() string {
	data, _ := json.Marshal(j)
	return string(data)
}

// startedJob sends c, a background start, which must answer 202, and
// answers its job, queued.
func (tm acmeTeam) startedJob(t *testing.T, c step) backgroundJob {
	t.Helper()
	status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.body)
	if status != http.StatusAccepted {
		t.Fatalf("%s as %s = %d %s, want 202", c.name(), c.by, status, answer)
	}
	var j backgroundJob
	decodeAnswer(t, answer, &j)
	if j.State != "queued" || j.ChangesetID == nil {
		t.Fatalf("%s as %s = %s, want a job queued, its changeset decided", c.name(), c.by, answer)
	}
	return j
}

// endedBackground waits until the job id has ended, as by reads it.
func (tm acmeTeam) endedBackground(t *testing.T, by, id string) backgroundJob {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+id, tm.tokens[by], "")
		if status != http.StatusOK {
			t.Fatalf("read the job %s as %s = %d %s", id, by, status, answer)
		}
		var j backgroundJob
		decodeAnswer(t, answer, &j)
		if j.State != "queued" && j.State != "running" {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("the job %s is still %s", id, j.State)
		}
	}
}

// background runs c, a background start, to its job's end.
func (tm acmeTeam) background(t *testing.T, c step) backgroundJob {
	t.Helper()
	return tm.endedBackground(t, c.by, tm.startedJob(t, c).ID)
}

// awaitBackground waits until River has completed every restore and
// revert.
func awaitBackground(t *testing.T, tm acmeTeam) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); count(t, tm.pool,
		"SELECT count(*) FROM river_job WHERE kind IN ('transfer.trash_restore', 'transfer.revert') AND state <> 'completed'") != 0; time.Sleep(
		50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("background jobs not completed: %v", queryStrings(t, tm.pool,
				"SELECT kind || ' ' || state FROM river_job WHERE kind IN ('transfer.trash_restore', 'transfer.revert')"))
		}
	}
}

// checkBackground checks the invariants once River has completed every
// job: the jobs', the pages', the links', the attachments' and the
// notebooks'.
func checkBackground(t *testing.T, tm acmeTeam) {
	t.Helper()
	awaitBackground(t, tm)
	checkImports(t, tm)
	checkNotebooks(t, tm.pool)
}

// newBatchedTeam is newAcmeTeam on an app whose restores and reverts write
// batches of three nodes (M8/P4 design 3.10): a few nodes span batches.
func newBatchedTeam(t *testing.T, bobRole, carolRole string) acmeTeam {
	t.Helper()
	return newAcmeTeamWired(t, bobRole, carolRole, nil, transfer.Batch{Nodes: 3})
}

// shareNotebook begins a transaction of the test's own that holds the row
// of the notebook nb FOR SHARE: a job's start shares it as well and goes
// by, but each of its batches, and every tree's write, waits for it. The
// test ends it.
func (tm acmeTeam) shareNotebook(t *testing.T, nb string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR SHARE", nb); err != nil {
		t.Fatal(err)
	}
	return holder
}

// atFirstBatch starts c, a background start in the notebook nb, its first
// batch held as it waits for the notebook's row, which the test shares;
// runs then; and lets the job go on. It answers the job, as it started.
func (tm acmeTeam) atFirstBatch(t *testing.T, nb string, c step, then func()) backgroundJob {
	t.Helper()
	holder := tm.shareNotebook(t, nb)
	j := tm.startedJob(t, c)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	then()
	commit(t, holder)
	return j
}

// behindFirstBatch starts c as atFirstBatch does, and sends r, a tree's
// write of the notebook, behind its first batch: PostgreSQL hands the
// row to its waiters in the order they came, so r comes after the first
// batch and before the second. It answers the job as it ended, as c's
// sender reads it, and r's answer.
func (tm acmeTeam) behindFirstBatch(t *testing.T, nb string, c, r step) (backgroundJob, answer) {
	t.Helper()
	var got func() answer
	j := tm.atFirstBatch(t, nb, c, func() {
		got = tm.inBackground(t, r)
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
	})
	a := got()
	return tm.endedBackground(t, c.by, j.ID), a
}

// holdNode begins a transaction of the test's own that holds the row of the
// node id FOR UPDATE: the batch that writes it waits for it, holding the
// notebook, the batches before it written. The test ends it.
func (tm acmeTeam) holdNode(t *testing.T, id string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	if _, err := holder.Exec(ctx, "SELECT 1 FROM nodes WHERE id = $1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	return holder
}

// cancelSeen asks by's job id to cancel, and waits until its heartbeat has
// read the cancel: the job stops before its next batch.
func (tm acmeTeam) cancelSeen(t *testing.T, by, id string) {
	t.Helper()
	tm.send(t, request(by, http.MethodPost, "/api/v0/transfer-jobs/"+id+"/cancel", ""), http.StatusOK)
	for deadline := time.Now().Add(interleavingWait); count(t, tm.pool,
		"SELECT count(*) FROM transfer_jobs WHERE id = $1 AND heartbeat_at > cancel_requested_at", id) == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the job %s's heartbeat did not read its cancel", id)
		}
	}
}

// A trash item past a request's bounds, 102 pages and an attachment with
// their links, is page.restore_too_large; restored in the background, in
// two batches, it comes back where it was: the attachment's row, which
// downloads; the index, its pages' links and a page's link to it
// resolving again, the streams seeing the links that resolve otherwise.
// One changeset, the job's, a trash_restore, holds the 103 restores, each
// at its batch's time.
func TestABackgroundRestoreThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	marker := tm.createPage(t, "alice", nb, "", "Marker")
	box := tm.createPageWith(t, "alice", nb, "", "Box", "![[photo.png]]")
	for i := range 101 {
		tm.createPageWith(t, "alice", nb, box, fmt.Sprintf("P %d", i), fmt.Sprintf("[[P %d]]", (i+1)%101))
	}
	photo := tm.upload(t, "alice", nb, box, "photo.png", sevenByFive)
	linker := tm.createPageWith(t, "alice", nb, "", "Linker", "[[Box]]")
	tm.resolves(t, linker, box)
	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	tm.resolves(t, linker, "")
	c := nodeRestore("bob", box)
	if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], ""); status != http.StatusUnprocessableEntity ||
		problemCode(t, answer) != "page.restore_too_large" {
		t.Fatalf("restore Box = %d %s, want 422 page.restore_too_large", status, answer)
	}

	tm.settle(t, nb, marker)
	s := openStream(t, tm.base, "alice", tm.tokens["alice"])
	j := tm.background(t, trashRestoreStart("bob", nb, box))
	if j.Kind != "trash_restore" || j.State != "succeeded" || j.Name != "Box" || j.RootID == nil || *j.RootID != box ||
		j.TargetChangesetID != nil || j.Progress.Done != 103 || j.Progress.Total != 103 || j.counts()["pages"] != 102 ||
		j.counts()["attachments"] != 1 || j.counts()["renamed"] != 0 || j.counts()["skipped"] != 0 || j.Report.Placed == nil ||
		j.Report.Placed.at() != " original" {
		t.Fatalf("the restore = %s, want it succeeded, 102 pages and an attachment back where they were", j)
	}
	s.awaitLinks(t, nb, linker)
	awaitJob(t, tm.pool, "transfer.trash_restore")
	if got := queryStrings(t, tm.pool, "SELECT queue || ' ' || max_attempts FROM river_job WHERE kind = $1",
		"transfer.trash_restore"); !slices.Equal(got, []string{"transfer_import 1"}) {
		t.Errorf("the restore's River job = %q, want in transfer_import, one attempt", got)
	}
	tm.resolves(t, linker, box)
	if got := queryStrings(t, tm.pool, `SELECT s.kind || ' ' || count(*) || ' ' || count(DISTINCT i.created_at) FROM changesets s
		JOIN changeset_items i ON i.changeset_id = s.id AND i.restored WHERE s.id = $1 GROUP BY s.kind`, *j.ChangesetID); !slices.Equal(got,
		[]string{"trash_restore 103 2"}) {
		t.Errorf("the job's changeset = %q, want a trash_restore of 103 restores, written in two batches", got)
	}
	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/assets/"+photo.ID, tm.tokens["alice"], "")
	if status != http.StatusOK {
		t.Fatalf("GET photo.png = %d %s, want 200", status, body)
	}
	if status, file := tm.download(t, tm.asset(t, body).ContentURL); status != http.StatusOK || file != sevenByFive {
		t.Errorf("photo.png's new address = %d, want its file", status)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL", nb); n != 0 {
		t.Errorf("%d nodes in the trash, want none", n)
	}
	checkBackground(t, tm)
}

// awaitLinks reads the stream's frames until a links event of the
// notebook nb that names page among the pages whose links resolve
// otherwise.
func (s *eventStream) awaitLinks(t *testing.T, nb, page string) {
	t.Helper()
	for f := s.next(t); f.event != "EOF"; f = s.next(t) {
		if f.event != "links" {
			continue
		}
		var got struct {
			NotebookID string    `json:"notebook_id"`
			Pages      *[]string `json:"pages"`
		}
		if err := json.Unmarshal([]byte(f.data), &got); err != nil {
			t.Fatal(err)
		}
		if got.NotebookID == nb && got.Pages != nil && slices.Contains(*got.Pages, page) {
			return
		}
	}
	t.Fatalf("%s's stream ended without the links of %s", s.name, page)
}

// flatImport is a vault of n pages at its root, a page each.
func flatImport(t *testing.T, n int) []byte {
	t.Helper()
	files := make([]vaultFile, n)
	for i := range n {
		files[i] = vaultFile{fmt.Sprintf("p%04d.md", i), "x"}
	}
	return vaultZip(t, files...)
}

// An import of 1,501 pages at the root is past a revert's steps:
// page.revert_too_large; reverted in the background, its 1,501 tops each
// go to the trash, an item of its own, in batches of a hundred, one
// changeset, the job's, a revert of the import's with the message. The
// revert of that revert, past a request's steps too, restores the 1,501
// items in the background, many groups a batch: the notebook is as the
// import left it.
func TestABackgroundRevertOfAnImportThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	if j := tm.imported(t, "alice", nb, "", flatImport(t, 1501)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	imported := tm.newest(t, nb)
	if imported.Kind != "import" {
		t.Fatalf("the newest changeset is of %s, want the import", imported.Kind)
	}
	was := tm.notebookState(t, nb)
	c := changesetRevert("bob", imported.ID)
	if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], ""); status != http.StatusUnprocessableEntity ||
		problemCode(t, answer) != "page.revert_too_large" {
		t.Fatalf("revert the import = %d %s, want 422 page.revert_too_large", status, answer)
	}

	j := tm.background(t, revertStart("bob", nb, imported.ID, "Undo the vault"))
	if j.Kind != "revert" || j.State != "succeeded" || j.Name != "import" || j.RootID != nil || j.TargetChangesetID == nil ||
		*j.TargetChangesetID != imported.ID || j.Progress.Done != 1501 || j.Progress.Total != 1501 || j.counts()["deleted"] != 1501 ||
		j.counts()["restored"] != 0 || j.counts()["skipped"] != 0 || j.Report.Placed != nil || len(j.Conflicts) != 0 {
		t.Fatalf("the revert = %s, want it succeeded, 1,501 deleted", j)
	}
	r := tm.newest(t, nb)
	if r.ID != *j.ChangesetID || r.Kind != "revert" || r.Reverts == nil || r.Reverts.ID != imported.ID || r.Message == nil ||
		*r.Message != "Undo the vault" {
		t.Errorf("the log's newest = %+v, want the job's revert %s of the import, with its message", r, *j.ChangesetID)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root", nb); n != 1501 ||
		count(t, tm.pool, "SELECT count(DISTINCT deleted_at) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL", nb) != 16 {
		t.Errorf("%d trash items, want the import's 1,501, deleted in 16 batches", n)
	}
	if items := tm.trash(t, "bob", nb); len(items) != 50 {
		t.Errorf("the trash's first page holds %d items, want 50", len(items))
	}
	checkBackground(t, tm)

	again := tm.background(t, revertStart("bob", nb, r.ID, ""))
	if again.State != "succeeded" || again.Name != "revert" || again.counts()["restored"] != 1501 || again.Progress.Done != 1501 {
		t.Fatalf("the revert of the revert = %s, want it succeeded, 1,501 restored", again)
	}
	if now := tm.notebookState(t, nb); now != was {
		t.Errorf("the notebook after the revert's revert differs from the import's")
	}
	checkBackground(t, tm)
}

// folderImport is a vault of a folder's page and n pages under it.
func folderImport(t *testing.T, n int) []byte {
	t.Helper()
	files := []vaultFile{{"Folder.md", "[[p000]]"}}
	for i := range n {
		files = append(files, vaultFile{fmt.Sprintf("Folder/p%03d.md", i), fmt.Sprintf("[[p%03d]]", (i+1)%n)})
	}
	return vaultZip(t, files...)
}

// A deletion of a folder of 150 pages is past a revert's pages: reverted
// in the background, its group spans two batches, and the notebook is as
// it was before the deletion, the index too; the revert of that revert,
// one deletion, a request makes, and the notebook is as the deletion
// left it. The job's row names the changeset it wrote, which the log
// shows, and the deletion it reverted.
func TestABackgroundRevertOfADeletionThroughServe(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	if j := tm.imported(t, "alice", nb, "", folderImport(t, 150)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	folder := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'Folder'", nb)[0]
	before := tm.notebookState(t, nb)
	tm.send(t, nodeDeletion("alice", folder), http.StatusNoContent)
	deletion := tm.newest(t, nb)
	deleted := tm.notebookState(t, nb)
	c := changesetRevert("bob", deletion.ID)
	if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], ""); status != http.StatusUnprocessableEntity ||
		problemCode(t, answer) != "page.revert_too_large" {
		t.Fatalf("revert the deletion = %d %s, want 422 page.revert_too_large", status, answer)
	}

	j := tm.background(t, revertStart("bob", nb, deletion.ID, ""))
	if j.State != "succeeded" || j.Name != "edit" || *j.TargetChangesetID != deletion.ID || j.counts()["restored"] != 151 ||
		j.Progress.Done != 151 || j.Progress.Total != 151 {
		t.Fatalf("the revert = %s, want it succeeded, 151 restored", j)
	}
	if now := tm.notebookState(t, nb); now != before {
		t.Errorf("the notebook after the revert differs from before the deletion")
	}
	if r := tm.newest(t, nb); r.ID != *j.ChangesetID || r.Kind != "revert" || r.Reverts == nil || r.Reverts.ID != deletion.ID {
		t.Errorf("the log's newest = %+v, want the job's revert %s of the deletion", r, *j.ChangesetID)
	}
	if n := count(t, tm.pool, "SELECT count(DISTINCT created_at) FROM changeset_items WHERE changeset_id = $1", *j.ChangesetID); n != 2 {
		t.Errorf("the revert's items written at %d times, want two batches'", n)
	}
	checkBackground(t, tm)

	tm.reverted(t, "alice", *j.ChangesetID)
	if now := tm.notebookState(t, nb); now != deleted {
		t.Errorf("the notebook after the revert's revert differs from after the deletion")
	}
	checkBackground(t, tm)
}
