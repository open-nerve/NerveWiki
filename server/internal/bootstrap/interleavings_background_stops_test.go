package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The interleavings of the background restores and reverts where a job
// stops, or meets another writer of its notebook (M8/P4 design 3.10,
// items 6, 7, 8, 11, 12, 15, 17, 18, 19 and 20), through serve and River,
// the jobs writing batches of three nodes. A job is held at its first
// batch as it waits for its notebook's row, which the test shares; what
// must come between two batches queues behind the first, PostgreSQL
// handing the row to its waiters in the order they came; a later batch is
// held by a row of a node it writes, which the test locks. Each ends on
// the invariants.

// stopsTeam is acme on an app whose jobs write batches of three nodes,
// bob a member, and alice's notebook Eng.
func stopsTeam(t *testing.T) (acmeTeam, string) {
	t.Helper()
	tm := newBatchedTeam(t, "member", "")
	return tm, tm.openNotebook(t, "alice", "Eng")
}

// stopBox is a page Box of the notebook nb with n pages under it, c1 to
// cn, which alice deletes: one trash item, which a job restores in
// batches of three nodes, Box, c1 and c2, then c3, c4 and c5, and so on.
// It answers Box and its pages.
func (tm acmeTeam) stopBox(t *testing.T, nb string, n int) (string, []string) {
	t.Helper()
	box := tm.createPage(t, "alice", nb, "", "Box")
	var kids []string
	for i := range n {
		kids = append(kids, tm.createPage(t, "alice", nb, box, fmt.Sprintf("c%d", i+1)))
	}
	tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
	return box, kids
}

// stopTops imports n pages at the root of the notebook nb, as alice: the
// import's changeset C, which a job reverts in batches of three
// deletions, in its pages' ids' order. It answers C and the pages in that
// order.
func (tm acmeTeam) stopTops(t *testing.T, nb string, n int) (string, []string) {
	t.Helper()
	if j := tm.imported(t, "alice", nb, "", flatImport(t, n)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	c := tm.newest(t, nb)
	if c.Kind != "import" {
		t.Fatalf("the newest changeset is of %s, want the import", c.Kind)
	}
	return c.ID, queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL ORDER BY id", nb)
}

// stopAtFirst starts c, a background start in the notebook nb, its first
// batch held as it waits for the notebook's row, which the test shares;
// runs then with the job as it started; and lets the job go on. It
// answers the job as it started.
func (tm acmeTeam) stopAtFirst(t *testing.T, nb string, c step, then func(j backgroundJob)) backgroundJob {
	t.Helper()
	holder := tm.shareNotebook(t, nb)
	j := tm.startedJob(t, c)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	then(j)
	commit(t, holder)
	return j
}

// stopPaused starts c as stopAtFirst does, with a transaction of the
// test's own queued behind its first batch for the notebook's row: it
// takes the row once the first batch commits, before the second, which
// then waits; then runs with the job as it started, and lets the job end.
// It answers the job as it ended.
func (tm acmeTeam) stopPaused(t *testing.T, nb string, c step, then func(j backgroundJob)) backgroundJob {
	t.Helper()
	ctx := context.Background()
	pause, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pause.Rollback(ctx) })
	took := make(chan error, 1)
	j := tm.stopAtFirst(t, nb, c, func(backgroundJob) {
		go func() {
			_, err := pause.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", nb)
			took <- err
		}()
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
	})
	select {
	case err := <-took:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(interleavingWait):
		t.Fatal("the job's first batch did not end")
	}
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	then(j)
	commit(t, pause)
	return tm.endedBackground(t, c.by, j.ID)
}

// stopRoots are the roots of the notebook nb's trash items, by name.
func (tm acmeTeam) stopRoots(t *testing.T, nb string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root ORDER BY name", nb)
}

// stopAlive counts the notebook nb's nodes not deleted.
func (tm acmeTeam) stopAlive(t *testing.T, nb string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb)
}

// Interleaving 6: each job and a revert of its own changeset R between
// two batches. R exists once the first batch commits, so the revert comes
// between the second and the third: the test's transaction takes the
// notebook's row as the first batch ends, the second waits for it, and
// the revert, its unlocked reads of R done, behind the second. The
// revert puts back what the two batches wrote, a restore's part in the
// trash again, a revert's deletions alive again; the third batch finds R
// reverted as it locks it, before it writes anything: the job fails
// changeset_reverted, counting the two batches. The third batch first,
// the job is whole and the revert reverts it all. The revert job's
// second batch deletes attachments: a page it deleted the revert would
// not have parsed before its unit, which counts it changed (M8/P3 design
// 3.6), and answers 409.
func TestABackgroundJobAndARevertOfItsChangeset(t *testing.T) {
	behindSecond := func(t *testing.T, tm acmeTeam, nb string, c step) (backgroundJob, answer) {
		t.Helper()
		var got func() answer
		j := tm.stopPaused(t, nb, c, func(j backgroundJob) {
			got = tm.inBackground(t, changesetRevert("alice", *j.ChangesetID))
			pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
		})
		return j, got()
	}
	t.Run("a restore", func(t *testing.T) {
		tm, _ := stopsTeam(t)
		for try := range betweenUnits {
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Restored %d", try))
			box, _ := tm.stopBox(t, nb, 8)
			j, revert := behindSecond(t, tm, nb, trashRestoreStart("bob", nb, box))
			if revert.status != http.StatusOK {
				t.Fatalf("the revert of the job's changeset = %d %s, want 200", revert.status, revert.body)
			}
			if j.State == "succeeded" {
				if n := tm.stopAlive(t, nb); n != 0 {
					t.Errorf("the third batch first: %d nodes alive, want the revert to have put all nine back", n)
				}
				continue // the third batch first
			}
			if j.State != "failed" || j.failure() != "changeset_reverted" || j.counts()["pages"] != 6 || j.Progress.Done != 6 {
				t.Errorf("the restore = %s, want it failed changeset_reverted after its second batch", j)
			}
			if alive, roots := tm.stopAlive(t, nb), tm.stopRoots(t, nb); alive != 0 || !slices.Equal(roots, []string{"Box", "c6", "c7", "c8"}) {
				t.Errorf("%d nodes alive, trash items %q; want none alive, Box's part again and c6, c7, c8 each an item", alive, roots)
			}
			checkBackground(t, tm)
			return
		}
		t.Fatalf("the revert came after the restore's third batch, %d times of %d", betweenUnits, betweenUnits)
	})
	t.Run("a revert", func(t *testing.T) {
		tm, _ := stopsTeam(t)
		for try := range betweenUnits {
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Reverted %d", try))
			c := tm.stopMixed(t, nb)
			j, revert := behindSecond(t, tm, nb, revertStart("bob", nb, c, ""))
			if revert.status != http.StatusOK {
				t.Fatalf("the revert of the job's changeset = %d %s, want 200", revert.status, revert.body)
			}
			if j.State == "succeeded" {
				continue // the third batch first
			}
			if j.State != "failed" || j.failure() != "changeset_reverted" || j.counts()["deleted"] != 6 || j.Progress.Done != 6 {
				t.Errorf("the revert job = %s, want it failed changeset_reverted after its second batch", j)
			}
			if n := tm.stopAlive(t, nb); n != 9 || tm.reverts(t, nb) != 2 {
				t.Errorf("%d nodes alive, %d reverts; want the nine, the job's and its revert", n, tm.reverts(t, nb))
			}
			checkBackground(t, tm)
			return
		}
		t.Fatalf("the revert came after the job's third batch, %d times of %d", betweenUnits, betweenUnits)
	})
}

// stopMixed imports three pages, a0 to a2, and six attachments, b0.png to
// b5.png, at the root of the notebook nb, as alice (each level's nodes by
// name): the import's changeset C, which a job reverts in three batches,
// the pages, then the attachments three by three.
func (tm acmeTeam) stopMixed(t *testing.T, nb string) string {
	t.Helper()
	files := []vaultFile{{"a0.md", "x"}, {"a1.md", "x"}, {"a2.md", "x"}}
	for i := range 6 {
		files = append(files, vaultFile{fmt.Sprintf("b%d.png", i), pngHead})
	}
	if j := tm.imported(t, "alice", nb, "", vaultZip(t, files...)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	names := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE notebook_id = $1 ORDER BY id", nb)
	if want := []string{"a0", "a1", "a2", "b0.png", "b1.png", "b2.png", "b3.png", "b4.png", "b5.png"}; !slices.Equal(names, want) {
		t.Fatalf("the import's nodes by id = %q, want %q", names, want)
	}
	c := tm.newest(t, nb)
	if c.Kind != "import" {
		t.Fatalf("the newest changeset is of %s, want the import", c.Kind)
	}
	return c.ID
}

// Interleaving 7: a revert job and a write of a page it is to delete,
// between two batches. A write takes the notebook's row FOR SHARE, which
// the test shares as the first batch waits: it comes between the second
// batch and the third, queued behind the second as interleaving 6's
// revert. The third batch plans its own nodes again and finds the page
// changed: the job fails revert_conflict, its report naming the page, the
// two batches' deletions kept. Then the job's changeset R is reverted,
// the write too, and C whole. The third batch first, the page is in the
// trash and the write finds no page.
func TestABackgroundRevertAndAWriteOfItsPage(t *testing.T) {
	tm, _ := stopsTeam(t)
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", fmt.Sprintf("Written %d", try))
		c, tops := tm.stopTops(t, nb, 9)
		var got func() answer
		j := tm.stopPaused(t, nb, revertStart("bob", nb, c, ""), func(backgroundJob) {
			got = tm.inBackground(t, contentWrite("alice", tops[7], "changed", 1, ""))
			pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
		})
		write := got()
		if j.State == "succeeded" {
			if write.status != http.StatusNotFound {
				t.Errorf("the third batch first: the write = %d %s, want 404", write.status, write.body)
			}
			continue // the third batch first
		}
		if write.status != http.StatusOK || j.State != "failed" || j.failure() != "revert_conflict" || j.ConflictCount != 1 ||
			len(j.Conflicts) != 1 || j.Conflicts[0].NodeID != tops[7] || j.Conflicts[0].Reason != "changed" || j.counts()["deleted"] != 6 {
			t.Fatalf("the write = %d %s, the job = %s; want 200, then revert_conflict changed of %s after the second batch", write.status,
				write.body, j, tops[7])
		}
		if n := tm.stopAlive(t, nb); n != 3 {
			t.Errorf("%d pages alive, want the three the third batch was to delete", n)
		}
		w := queryStrings(t, tm.pool, "SELECT id::text FROM changesets WHERE notebook_id = $1 AND kind = 'edit' ORDER BY created_at DESC LIMIT 1", nb)
		tm.reverted(t, "alice", *j.ChangesetID)
		tm.reverted(t, "alice", w[0])
		tm.reverted(t, "alice", c)
		if n := tm.stopAlive(t, nb); n != 0 {
			t.Errorf("%d pages alive after C's revert, want none", n)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the write came after the job's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 8: a revert job and another account's edit session. Alice
// editing a page the revert deletes before it starts, the start is
// page.locked by her, and no job is made. Her session opened after the
// job planned, as its first batch waits, the batch that deletes her page
// stops the job page_locked, its report naming the page and her, the
// first batch's deletions kept. Her session over, R is reverted, then C
// whole.
func TestABackgroundRevertAndAnEditSession(t *testing.T) {
	tm, nb := stopsTeam(t)
	c, tops := tm.stopTops(t, nb, 6)
	tm.openSession(t, "alice", tops[1])
	start := revertStart("bob", nb, c, "")
	status, body := ask(t, tm.contract, start.method, tm.base+start.path, tm.tokens["bob"], start.body)
	if a := (answer{status: status, body: body, code: problemCode(t, body)}); !a.is(http.StatusConflict, "page.locked") ||
		lockHolder(a) != "alice" {
		t.Errorf("the start while alice edits = %d %s, want 409 page.locked by alice", status, body)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM transfer_jobs WHERE kind = 'revert'"); n != 0 {
		t.Errorf("%d revert jobs, want none", n)
	}
	tm.endSessions(t)

	var session string
	j := tm.stopAtFirst(t, nb, start, func(backgroundJob) { session = tm.openSession(t, "alice", tops[4]) })
	j = tm.endedBackground(t, "bob", j.ID)
	if j.State != "failed" || j.failure() != "page_locked" || j.Report.Lock == nil || j.Report.Lock.PageID != tops[4] ||
		j.Report.Lock.UserID != tm.userID(t, "alice").String() || j.Report.Lock.DisplayName != "alice" || j.counts()["deleted"] != 3 {
		t.Fatalf("the job = %s, want it failed page_locked by alice on %s after its first batch", j, tops[4])
	}
	if n := tm.stopAlive(t, nb); n != 3 {
		t.Errorf("%d pages alive, want the three the second batch was to delete", n)
	}
	if session == "" {
		t.Fatal("no session opened")
	}
	tm.endSessions(t)
	tm.reverted(t, "bob", *j.ChangesetID)
	tm.reverted(t, "bob", c)
	if n := tm.stopAlive(t, nb); n != 0 {
		t.Errorf("%d pages alive after C's revert, want none", n)
	}
	checkBackground(t, tm)
}

// endSessions ends every edit session, as their time running out would.
func (tm acmeTeam) endSessions(t *testing.T) {
	t.Helper()
	if _, err := tm.pool.Exec(context.Background(), "UPDATE edit_sessions SET created_at = now() - interval '2 minutes', "+
		"expires_at = now() - interval '1 minute' WHERE expires_at > now()"); err != nil {
		t.Fatal(err)
	}
}

// Interleaving 11: a cancel of each job as its first batch waits, which
// its heartbeat reads before the batch ends: the batch is written, and
// the job stops before the next, cancelled, its report counting the first
// batch. The revert's changeset R reverted, the notebook is as before the
// job, and C then reverts whole. The restore's changeset reverted, its
// part goes to the trash as an item of its own, beside the items the
// first batch left, each still one.
func TestCancellingABackgroundJob(t *testing.T) {
	t.Run("a restore", func(t *testing.T) {
		tm, nb := stopsTeam(t)
		box, _ := tm.stopBox(t, nb, 5)
		j := tm.stopAtFirst(t, nb, trashRestoreStart("bob", nb, box), func(j backgroundJob) { tm.cancelSeen(t, "bob", j.ID) })
		j = tm.endedBackground(t, "bob", j.ID)
		if j.State != "cancelled" || j.counts()["pages"] != 3 || j.Progress.Done != 3 || j.Progress.Total != 6 {
			t.Fatalf("the restore = %s, want it cancelled after its first batch", j)
		}
		if roots := tm.stopRoots(t, nb); !slices.Equal(roots, []string{"c3", "c4", "c5"}) {
			t.Errorf("trash items %q, want c3, c4 and c5", roots)
		}
		tm.checkTrash(t)
		tm.reverted(t, "alice", *j.ChangesetID)
		items := tm.trash(t, "bob", nb)
		var got []string
		for _, it := range items {
			got = append(got, fmt.Sprintf("%s %d", it.Name, it.Pages))
		}
		slices.Sort(got)
		if !slices.Equal(got, []string{"Box 3", "c3 1", "c4 1", "c5 1"}) {
			t.Errorf("the trash = %q, want Box's part of three pages, and c3, c4, c5 each an item", got)
		}
		checkBackground(t, tm)
	})
	t.Run("a revert", func(t *testing.T) {
		tm, nb := stopsTeam(t)
		c, _ := tm.stopTops(t, nb, 6)
		before := tm.notebookState(t, nb)
		j := tm.stopAtFirst(t, nb, revertStart("bob", nb, c, ""), func(j backgroundJob) { tm.cancelSeen(t, "bob", j.ID) })
		j = tm.endedBackground(t, "bob", j.ID)
		if j.State != "cancelled" || j.counts()["deleted"] != 3 || j.Progress.Done != 3 || j.Progress.Total != 6 {
			t.Fatalf("the revert = %s, want it cancelled after its first batch", j)
		}
		tm.checkHistory(t)
		tm.reverted(t, "alice", *j.ChangesetID)
		if now := tm.notebookState(t, nb); now != before {
			t.Errorf("the notebook after R's revert:\n%s\nwant as before the job:\n%s", now, before)
		}
		tm.reverted(t, "alice", c)
		if n := tm.stopAlive(t, nb); n != 0 {
			t.Errorf("%d pages alive after C's revert, want none", n)
		}
		checkBackground(t, tm)
	})
}

// Interleaving 12: the jobs that write a notebook's tree exclude each
// other, anyone's: while an import is queued, a restore runs or a revert
// runs, a restore's start, a revert's and an import's are transfer.busy,
// the import's before its file is sent; an export starts all the same.
func TestTheWritingJobsExcludeEachOther(t *testing.T) {
	tm, _ := stopsTeam(t)
	refused := func(t *testing.T, nb, item, c string) {
		t.Helper()
		for _, s := range []step{trashRestoreStart("bob", nb, item), revertStart("bob", nb, c, "")} {
			status, body := ask(t, tm.contract, s.method, tm.base+s.path, tm.tokens["bob"], s.body)
			if status != http.StatusConflict || problemCode(t, body) != "transfer.busy" {
				t.Errorf("%s = %d %s, want 409 transfer.busy", s.name(), status, body)
			}
		}
		if status, body := tm.startImport(t, "bob", nb, "", flatImport(t, 1)); status != http.StatusConflict ||
			problemCode(t, body) != "transfer.busy" {
			t.Errorf("an import = %d %s, want 409 transfer.busy", status, body)
		}
		tm.startExport(t, "alice", nb, "")
	}
	going := map[string]func(t *testing.T, nb, item, c string){
		"an import queued": func(t *testing.T, nb, item, c string) {
			release := tm.holdImportWorker(t)
			if status, body := tm.startImport(t, "alice", nb, "", flatImport(t, 1)); status != http.StatusAccepted {
				t.Fatalf("the import = %d %s, want 202", status, body)
			}
			refused(t, nb, item, c)
			release()
		},
		"a restore running": func(t *testing.T, nb, item, c string) {
			other, _ := tm.stopBox(t, nb, 5)
			j := tm.stopAtFirst(t, nb, trashRestoreStart("alice", nb, other), func(backgroundJob) { refused(t, nb, item, c) })
			if j = tm.endedBackground(t, "alice", j.ID); j.State != "succeeded" {
				t.Errorf("the restore = %s, want it succeeded", j)
			}
		},
		"a revert running": func(t *testing.T, nb, item, c string) {
			j := tm.stopAtFirst(t, nb, revertStart("alice", nb, c, ""), func(backgroundJob) { refused(t, nb, item, c) })
			if j = tm.endedBackground(t, "alice", j.ID); j.State != "succeeded" {
				t.Errorf("the revert = %s, want it succeeded", j)
			}
		},
	}
	for _, name := range []string{"an import queued", "a restore running", "a revert running"} {
		t.Run(name, func(t *testing.T) {
			nb := tm.openNotebook(t, "alice", name)
			item := tm.createPage(t, "alice", nb, "", "Item")
			tm.send(t, nodeDeletion("alice", item), http.StatusNoContent)
			c, _ := tm.stopTops(t, nb, 6)
			going[name](t, nb, item, c)
			checkBackground(t, tm)
		})
	}
}

// Interleaving 15: each job and its notebook's deletion between two
// batches. The deletion, behind the first batch, deletes the notebook
// with the job's row; the next batch finds no notebook, or the heartbeat
// the row gone, and the job stops writing nothing more and no end: the
// items the first batch left stay in the trash with the notebook, each a
// root at its time, beside the nodes the notebook's deletion took, none
// a root.
func TestABackgroundJobAndItsNotebooksDeletion(t *testing.T) {
	for _, kind := range []string{"trash_restore", "revert"} {
		t.Run(kind, func(t *testing.T) {
			tm, nb := stopsTeam(t)
			var start step
			var roots []string
			if kind == "trash_restore" {
				box, _ := tm.stopBox(t, nb, 5)
				start, roots = trashRestoreStart("bob", nb, box), []string{"c3", "c4", "c5"}
			} else {
				c, _ := tm.stopTops(t, nb, 6)
				start, roots = revertStart("bob", nb, c, ""), []string{"p0000", "p0001", "p0002"}
			}
			var deleted func() answer
			j := tm.stopAtFirst(t, nb, start, func(backgroundJob) {
				deleted = tm.inBackground(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""))
				pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
			})
			if a := deleted(); a.status != http.StatusNoContent {
				t.Fatalf("the notebook's deletion = %d %s, want 204", a.status, a.body)
			}
			awaitBackground(t, tm)
			if got := queryStrings(t, tm.pool, "SELECT state || ' ' || (deleted_at IS NOT NULL) FROM transfer_jobs WHERE id = $1",
				j.ID); !slices.Equal(got, []string{"running true"}) {
				t.Errorf("the job's row = %q, want it deleted with its notebook as it ran, no end written", got)
			}
			if status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+j.ID, tm.tokens["bob"], ""); status !=
				http.StatusNotFound {
				t.Errorf("the job = %d %s, want 404", status, body)
			}
			if got := tm.stopRoots(t, nb); !slices.Equal(got, roots) {
				t.Errorf("the items left = %q, want %q", got, roots)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM nodes n JOIN notebooks b ON b.id = n.notebook_id
				WHERE n.notebook_id = $1 AND n.deleted_at = b.deleted_at AND NOT n.trash_root`, nb); n != 3 {
				t.Errorf("%d nodes deleted with the notebook, want the three the first batch left alive", n)
			}
			checkBackground(t, tm)
		})
	}
}

// Interleaving 17: a revert job of a deletion whose group spans two
// batches, and the group's period or the purge between them. The group
// restorable a few seconds longer as the job starts, its first batch
// restores its top and the members with it, and the next, its period
// over, finds the rest expired: revert_conflict. A purge whose clock is
// ahead, of the versions alone, between the two batches: the next batch
// cannot read the versions of its members, changeset_trimmed. Either way
// the first batch's part is kept, the rest left in the trash.
func TestABackgroundRevertAndTheGroupsEnd(t *testing.T) {
	t.Run("its period over", func(t *testing.T) {
		tm, nb := stopsTeam(t)
		box, kids := tm.stopBox(t, nb, 5)
		deletion := tm.newest(t, nb).ID
		tm.trashedAgo(t, box, 1440*time.Hour-domain.TrashMargin-4*time.Second)
		var at time.Time
		if err := tm.pool.QueryRow(context.Background(), "SELECT deleted_at FROM nodes WHERE id = $1", box).Scan(&at); err != nil {
			t.Fatal(err)
		}
		until := at.Add(1440*time.Hour - domain.TrashMargin)
		j := tm.stopPaused(t, nb, revertStart("bob", nb, deletion, ""), func(backgroundJob) {
			if time.Now().After(until) {
				t.Fatalf("the first batch ended past the group's period, %s", until)
			}
			time.Sleep(time.Until(until) + 200*time.Millisecond)
		})
		if j.State != "failed" || j.failure() != "revert_conflict" || len(j.Conflicts) != 3 || j.Conflicts[0].Reason != "expired" ||
			j.Conflicts[0].NodeID != kids[2] || j.ConflictCount != 3 || j.counts()["restored"] != 3 {
			t.Fatalf("the job = %s, want it failed revert_conflict, c3, c4 and c5 expired, after restoring Box, c1 and c2", j)
		}
		if roots := tm.stopRoots(t, nb); !slices.Equal(roots, []string{"c3", "c4", "c5"}) || tm.stopAlive(t, nb) != 3 {
			t.Errorf("trash items %q, %d nodes alive; want c3, c4, c5 in the trash, Box, c1, c2 alive", roots, tm.stopAlive(t, nb))
		}
		checkBackground(t, tm)
	})
	t.Run("its versions purged", func(t *testing.T) {
		tm, nb := stopsTeam(t)
		box, _ := tm.stopBox(t, nb, 5)
		deletion := tm.newest(t, nb).ID
		j := tm.stopPaused(t, nb, revertStart("bob", nb, deletion, ""), func(backgroundJob) {
			if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour), "page_revisions")(); err != nil {
				t.Errorf("the purge: %v", err)
			}
		})
		if j.State != "failed" || j.failure() != "changeset_trimmed" || j.counts()["restored"] != 3 {
			t.Fatalf("the job = %s, want it failed changeset_trimmed after restoring Box, c1 and c2", j)
		}
		if roots := tm.stopRoots(t, nb); !slices.Equal(roots, []string{"c3", "c4", "c5"}) ||
			count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", box) != 1 {
			t.Errorf("trash items %q, want c3, c4, c5, Box alive", roots)
		}
		if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour))(); err != nil {
			t.Fatalf("the purge: %v", err)
		}
		checkBackground(t, tm)
	})
}

// stopUpload sends by's import into the notebook nb, its file held unsent
// until stop, and waits until it is stored, its start checked and
// admitted.
func (tm acmeTeam) stopUpload(t *testing.T, by, nb string) (stop func()) {
	t.Helper()
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		if fw, err := mw.CreateFormFile("file", "vault.zip"); err == nil {
			_, _ = fw.Write([]byte("PK\x03\x04"))
		}
	}()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/imports", pr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tm.tokens[by])
	req.Header.Set("Content-Type", mw.FormDataContentType())
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	stop = func() {
		_ = pw.CloseWithError(errors.New("stopped"))
		<-sent
	}
	t.Cleanup(stop)
	storing := func() bool {
		files, err := os.ReadDir(filepath.Join(tm.storage, "imports", ".tmp"))
		return err == nil && len(files) > 0
	}
	for deadline := time.Now().Add(interleavingWait); !storing(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the import's file is not stored")
		}
	}
	return stop
}

// Interleaving 18: an import's upload and the two starts (A-M11). While
// an import uploads into the notebook, a restore's start and a revert's
// are transfer.busy; the upload stopped, a start goes. While a start
// holds the notebook, waiting for its workspace's row which the test
// holds, an import's start is transfer.busy before it sends its file.
func TestAnUploadAndTheBackgroundStarts(t *testing.T) {
	tm, nb := stopsTeam(t)
	item := tm.createPage(t, "alice", nb, "", "Item")
	tm.send(t, nodeDeletion("alice", item), http.StatusNoContent)
	c, _ := tm.stopTops(t, nb, 6)
	stop := tm.stopUpload(t, "alice", nb)
	for _, s := range []step{trashRestoreStart("bob", nb, item), revertStart("bob", nb, c, "")} {
		status, body := ask(t, tm.contract, s.method, tm.base+s.path, tm.tokens["bob"], s.body)
		if status != http.StatusConflict || problemCode(t, body) != "transfer.busy" {
			t.Errorf("%s as the import uploads = %d %s, want 409 transfer.busy", s.name(), status, body)
		}
	}
	stop()
	for deadline := time.Now().Add(interleavingWait); ; time.Sleep(20 * time.Millisecond) {
		s := trashRestoreStart("bob", nb, item)
		status, body := ask(t, tm.contract, s.method, tm.base+s.path, tm.tokens["bob"], s.body)
		if status == http.StatusAccepted {
			var j backgroundJob
			decodeAnswer(t, body, &j)
			tm.endedBackground(t, "bob", j.ID)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the restore's start once the upload stopped = %d %s, want 202", status, body)
		}
	}

	for _, s := range []step{revertStart("bob", nb, c, "")} {
		ctx := context.Background()
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = holder.Rollback(ctx) })
		if _, err := holder.Exec(ctx, "SELECT 1 FROM workspaces WHERE slug = 'acme' FOR UPDATE"); err != nil {
			t.Fatal(err)
		}
		started := tm.inBackground(t, s)
		pgtest.WaitForLockWaitsOn(t, tm.pool, "workspaces", 1, interleavingWait)
		if status, body := tm.startImport(t, "alice", nb, "", flatImport(t, 1)); status != http.StatusConflict ||
			problemCode(t, body) != "transfer.busy" {
			t.Errorf("an import as the start holds the notebook = %d %s, want 409 transfer.busy", status, body)
		}
		if files := storedImports(t, tm.storage); len(files) != 0 {
			t.Errorf("imports stored %q, want none: refused before the file", files)
		}
		commit(t, holder)
		a := started()
		if a.status != http.StatusAccepted {
			t.Fatalf("%s = %d %s, want 202", s.name(), a.status, a.body)
		}
		var j backgroundJob
		decodeAnswer(t, a.body, &j)
		tm.endedBackground(t, "bob", j.ID)
	}
	checkBackground(t, tm)
}

// stopTree is a page Box of the notebook nb with A, B and C under it, two
// pages under each: deleted, one trash item of ten nodes, which a job
// restores in four batches, its pre-order cut in threes: Box, A, a1; a2,
// B, b1; b2, C, c1; c2. b1 and c1 are the first nodes of their batches
// that the batch before marks no root. It answers Box, b1 and c1.
func (tm acmeTeam) stopTree(t *testing.T, nb string) (box, b1, c1 string) {
	t.Helper()
	box = tm.createPage(t, "alice", nb, "", "Box")
	ids := map[string]string{}
	for _, p := range []string{"A", "B", "C"} {
		id := tm.createPage(t, "alice", nb, box, p)
		for i := range 2 {
			name := fmt.Sprintf("%c%d", p[0]+'a'-'A', i+1)
			ids[name] = tm.createPage(t, "alice", nb, id, name)
		}
	}
	return box, ids["b1"], ids["c1"]
}

// stopFolders imports F1, F2 and F3 at the root of the notebook nb, two
// pages under each, as alice: C, which a job reverts in three batches, a
// folder's deletion each, in their ids' order. It answers C and the
// folders in that order.
func (tm acmeTeam) stopFolders(t *testing.T, nb string) (string, []string) {
	t.Helper()
	var files []vaultFile
	for i := range 3 {
		files = append(files, vaultFile{fmt.Sprintf("F%d.md", i+1), ""}, vaultFile{fmt.Sprintf("F%d/a.md", i+1), "a"},
			vaultFile{fmt.Sprintf("F%d/b.md", i+1), "b"})
	}
	if j := tm.imported(t, "alice", nb, "", vaultZip(t, files...)); j.State != "succeeded" {
		t.Fatalf("the import = %+v", j)
	}
	return tm.newest(t, nb).ID, queryStrings(t, tm.pool,
		"SELECT id::text FROM nodes WHERE notebook_id = $1 AND parent_id IS NULL AND deleted_at IS NULL ORDER BY id", nb)
}

// stopAfter starts c in the notebook nb and cancels it after its batch k:
// the first held as stopAtFirst holds it, a later one by the row of the
// node held, which it writes; the cancel read as the batch waits, the
// batch ends and the job stops. It answers the job as it ended.
func (tm acmeTeam) stopAfter(t *testing.T, nb string, c step, k int, held string) backgroundJob {
	t.Helper()
	if k == 1 {
		j := tm.stopAtFirst(t, nb, c, func(j backgroundJob) { tm.cancelSeen(t, c.by, j.ID) })
		return tm.endedBackground(t, c.by, j.ID)
	}
	holder := tm.holdNode(t, held)
	j := tm.startedJob(t, c)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "nodes", 1, interleavingWait)
	tm.cancelSeen(t, c.by, j.ID)
	commit(t, holder)
	return tm.endedBackground(t, c.by, j.ID)
}

// Interleaving 19: each job stopped at each of its batches' ends (B-M12),
// cancelled after batch k. The tree's invariants hold after each. A
// revert of deletions stopped there, never in a group's middle, its
// changeset R reverted, the notebook is as before the job. A restore
// stopped there, its trash items left each restored one by one, the
// notebook is as before the deletion.
func TestABackgroundJobStoppedAtEachBatch(t *testing.T) {
	t.Run("a restore", func(t *testing.T) {
		tm, _ := stopsTeam(t)
		for k := 1; k <= 3; k++ {
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Restored after %d", k))
			box, b1, c1 := tm.stopTree(t, nb)
			before := tm.notebookState(t, nb)
			tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
			held := map[int]string{2: b1, 3: c1}[k]
			j := tm.stopAfter(t, nb, trashRestoreStart("bob", nb, box), k, held)
			if j.State != "cancelled" || j.Progress.Done != 3*k || j.counts()["pages"] != 3*k {
				t.Fatalf("after batch %d the restore = %s, want it cancelled, %d restored", k, j, 3*k)
			}
			tm.checkTrash(t)
			for roots := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root",
				nb); len(roots) > 0; roots = roots[1:] {
				if r := tm.restored(t, "bob", roots[0]); r.Renamed || r.Placed.Reason != "original" {
					t.Errorf("after batch %d, restored %+v, want it back where it was", k, r)
				}
			}
			if now := tm.notebookState(t, nb); now != before {
				t.Errorf("after batch %d, the notebook its items restored:\n%s\nwant as before the deletion:\n%s", k, now, before)
			}
		}
		checkBackground(t, tm)
	})
	t.Run("a revert", func(t *testing.T) {
		tm, _ := stopsTeam(t)
		for k := 1; k <= 2; k++ {
			nb := tm.openNotebook(t, "alice", fmt.Sprintf("Reverted after %d", k))
			c, folders := tm.stopFolders(t, nb)
			before := tm.notebookState(t, nb)
			j := tm.stopAfter(t, nb, revertStart("bob", nb, c, ""), k, folders[k-1])
			if j.State != "cancelled" || j.Progress.Done != 3*k || j.counts()["deleted"] != 3*k {
				t.Fatalf("after batch %d the revert = %s, want it cancelled, %d deleted", k, j, 3*k)
			}
			tm.checkHistory(t)
			tm.reverted(t, "alice", *j.ChangesetID)
			if now := tm.notebookState(t, nb); now != before {
				t.Errorf("after batch %d, the notebook R reverted:\n%s\nwant as before the job:\n%s", k, now, before)
			}
		}
		checkBackground(t, tm)
	})
}

// Interleaving 20: a revert job of a deletion stopped in a group's middle
// (B-M12), cancelled after its first batch, the group's top and two of its
// members restored, the rest each an item of its own. Going on, each item
// restored one by one, the group is as C's revert would leave it. R
// reverted first, its part an item again at its time, the rest's parent
// in the trash: that item restored first, then the rest, the group is so
// too. Going back, R reverted, C's revert finds the group changed.
func TestABackgroundRevertStoppedInAGroup(t *testing.T) {
	tm, _ := stopsTeam(t)
	stopped := func(t *testing.T, name string) (nb, deletion string, kids []string, before string, j backgroundJob) {
		t.Helper()
		nb = tm.openNotebook(t, "alice", name)
		box := tm.createPage(t, "alice", nb, "", "Box")
		for i := range 5 {
			kids = append(kids, tm.createPage(t, "alice", nb, box, fmt.Sprintf("c%d", i+1)))
		}
		before = tm.notebookState(t, nb)
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		deletion = tm.newest(t, nb).ID
		j = tm.stopAfter(t, nb, revertStart("bob", nb, deletion, ""), 1, "")
		if j.State != "cancelled" || j.counts()["restored"] != 3 {
			t.Fatalf("the revert = %s, want it cancelled after restoring three", j)
		}
		if roots := tm.stopRoots(t, nb); !slices.Equal(roots, []string{"c3", "c4", "c5"}) {
			t.Fatalf("trash items %q, want c3, c4, c5", roots)
		}
		tm.checkHistory(t)
		return nb, deletion, kids, before, j
	}
	restoreAll := func(t *testing.T, nb string) {
		t.Helper()
		for _, id := range queryStrings(t, tm.pool, `SELECT id::text FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root
			ORDER BY parent_id IS NOT NULL, name`, nb) {
			if r := tm.restored(t, "bob", id); r.Placed.Reason != "original" {
				t.Errorf("restored %+v, want it back where it was", r)
			}
		}
	}
	t.Run("going on", func(t *testing.T) {
		nb, _, _, before, _ := stopped(t, "On")
		restoreAll(t, nb)
		if now := tm.notebookState(t, nb); now != before {
			t.Errorf("the notebook:\n%s\nwant as before the deletion:\n%s", now, before)
		}
	})
	t.Run("R reverted first", func(t *testing.T) {
		nb, _, _, before, j := stopped(t, "Back and on")
		tm.reverted(t, "alice", *j.ChangesetID)
		if roots := tm.stopRoots(t, nb); !slices.Equal(roots, []string{"Box", "c3", "c4", "c5"}) {
			t.Fatalf("trash items %q, want Box's part and c3, c4, c5", roots)
		}
		restoreAll(t, nb)
		if now := tm.notebookState(t, nb); now != before {
			t.Errorf("the notebook:\n%s\nwant as before the deletion:\n%s", now, before)
		}
	})
	t.Run("going back", func(t *testing.T) {
		nb, deletion, _, _, j := stopped(t, "Back")
		tm.reverted(t, "alice", *j.ChangesetID)
		r := changesetRevert("alice", deletion)
		status, body := ask(t, tm.contract, r.method, tm.base+r.path, tm.tokens["alice"], "")
		a := answer{status: status, body: body, code: problemCode(t, body)}
		if conflicts, _ := conflictsOf(t, a); !a.is(http.StatusConflict, "page.revert_conflict") || len(conflicts) == 0 ||
			conflicts[0].Reason != "changed" {
			t.Errorf("C's revert after R's = %d %s, want 409 page.revert_conflict, changed", status, body)
		}
		if n := tm.stopAlive(t, nb); n != 0 {
			t.Errorf("%d nodes alive, want none", n)
		}
		tm.checkHistory(t)
	})
	checkBackground(t, tm)
}
