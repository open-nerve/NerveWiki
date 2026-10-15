package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The search's interleavings (M8/P5 design 3.8): its job against the
// units that queue what they change, a notebook's deletion, the purge,
// nervewiki reindex's queueing and another run of itself. The job never
// waits: a test stops it by holding a row of page_search, which only the
// job writes, so that its batch's write waits there, its reads done. Each
// ends with the queue emptied by a run, on the search's invariant, the
// pages' and the links'.

// holdSearchRow begins a transaction of the test's own that holds the
// search's row of the node id FOR UPDATE: the job's write of it waits,
// what it read before kept. The test ends it.
func (tm acmeTeam) holdSearchRow(t *testing.T, id string) pgx.Tx {
	t.Helper()
	return tm.holdOne(t, "SELECT count(*) FROM (SELECT 1 FROM page_search WHERE node_id = $1 FOR UPDATE) held", id)
}

// holdQueueRow begins a transaction of the test's own that holds the
// queue's row of the node id FOR UPDATE, as a unit that queues the node
// holds it until it commits. The test ends it.
func (tm acmeTeam) holdQueueRow(t *testing.T, id string) pgx.Tx {
	t.Helper()
	return tm.holdOne(t, "SELECT count(*) FROM (SELECT 1 FROM page_search_queue WHERE node_id = $1 FOR UPDATE) held", id)
}

// holdOne begins a transaction that runs query, a count of the rows it
// locks, which must be one.
func (tm acmeTeam) holdOne(t *testing.T, query, id string) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	var n int
	if err := holder.QueryRow(ctx, query, id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows of %s held: %v; want 1", n, id, err)
	}
	return holder
}

// drainInBackground runs the search's job on a goroutine of its own, and
// answers what waits for its end.
func (tm acmeTeam) drainInBackground(t *testing.T) func() page.SearchRun {
	t.Helper()
	type ended struct {
		run page.SearchRun
		err error
	}
	done := make(chan ended, 1)
	go func() {
		run, err := searchIndex(tm.pool).Drain(context.Background())
		done <- ended{run, err}
	}()
	return func() page.SearchRun {
		t.Helper()
		select {
		case e := <-done:
			if e.err != nil {
				t.Fatalf("Drain() = %+v, %v", e.run, e.err)
			}
			return e.run
		case <-time.After(interleavingWait):
			t.Fatal("the job did not end")
			return page.SearchRun{}
		}
	}
}

// builtRevision is the revision the search's row of the page id was built
// from, 0 for none.
func (tm acmeTeam) builtRevision(t *testing.T, id string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT coalesce((SELECT revision FROM page_search WHERE node_id = $1), 0)", id)
}

// checkSearchEnd runs the job until the queue is empty and checks the
// invariants.
func checkSearchEnd(t *testing.T, tm acmeTeam) {
	t.Helper()
	checkSearch(t, tm.pool)
	checkSearchBuilt(t, tm.pool)
	checkPages(t, tm.pool)
}

// 1. The job reads a page at revision 2; a save commits 3 before it writes:
// it writes 2, and does not take the page off, queued again since; the
// next run writes 3.
func TestASaveWhileTheSearchsJobBuildsItsPage(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "one")
	drainSearch(t, tm.pool)
	tm.send(t, contentWrite("alice", id, "the wombat", 1, ""), http.StatusOK)
	holder := tm.holdSearchRow(t, id)
	ran := tm.drainInBackground(t)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "page_search", 1, interleavingWait)

	tm.send(t, contentWrite("alice", id, "the dingo", 2, ""), http.StatusOK)
	commit(t, holder)
	if run := ran(); run.Written != 1 {
		t.Errorf("the run = %+v, want the page's row written", run)
	}
	if rev, queued := tm.builtRevision(t, id), count(t, tm.pool, "SELECT count(*) FROM page_search_queue WHERE node_id = $1", id); rev != 2 ||
		queued != 1 {
		t.Errorf("the row is of revision %d, the page queued %d times; want 2, still queued", rev, queued)
	}
	tm.finds(t, "bob", "dingo", "Notes")
	tm.finds(t, "bob", "wombat")
	drainSearch(t, tm.pool)
	if rev := tm.builtRevision(t, id); rev != 3 {
		t.Errorf("the row is of revision %d after the next run, want 3", rev)
	}
	checkSearchEnd(t, tm)
}

// 2. A page deleted as the job writes its row: no search lists it, its row
// a hit of the index; the next run deletes the row.
func TestADeletionWhileTheSearchsJobWritesItsRow(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "the wombat")
	drainSearch(t, tm.pool)
	tm.send(t, request("alice", http.MethodPatch, "/api/v0/nodes/"+id, `{"name":"Wombat notes"}`), http.StatusOK)
	holder := tm.holdSearchRow(t, id)
	ran := tm.drainInBackground(t)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "page_search", 1, interleavingWait)

	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	tm.finds(t, "bob", "wombat")
	commit(t, holder)
	ran()
	if n := count(t, tm.pool, "SELECT count(*) FROM page_search WHERE node_id = $1 AND name = 'Wombat notes'", id); n != 1 {
		t.Fatalf("%d rows of the deleted page by its new name, want its row, written as the job read it", n)
	}
	tm.finds(t, "bob", "wombat")
	if run := drainSearch(t, tm.pool); run.Deleted != 1 {
		t.Errorf("the next run = %+v, want the row deleted", run)
	}
	checkSearchEnd(t, tm)
}

// 3. A page deleted, its row deleted by the job, then back: from the trash,
// or by a revert of its deletion. The search finds it at once, through the
// queue, before the job builds its row again.
func TestAPageBackAfterTheSearchsJobDeletedItsRow(t *testing.T) {
	for _, back := range []struct {
		name string
		do   func(tm acmeTeam, nb, id string)
	}{
		{"from the trash", func(tm acmeTeam, _, id string) { tm.send(t, nodeRestore("alice", id), http.StatusOK) }},
		{"by a revert", func(tm acmeTeam, nb, _ string) { tm.reverted(t, "alice", tm.newest(t, nb).ID) }},
	} {
		t.Run(back.name, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			id := tm.createPageWith(t, "alice", nb, "", "Notes", "the wombat")
			tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
			if run := drainSearch(t, tm.pool); run.Written != 0 || count(t, tm.pool, "SELECT count(*) FROM page_search") != 0 {
				t.Fatalf("the run = %+v, want no row of the deleted page", run)
			}
			back.do(tm, nb, id)
			tm.finds(t, "bob", "wombat", "Notes")
			if run := drainSearch(t, tm.pool); run.Written != 1 {
				t.Errorf("the run = %+v, want the page's row built again", run)
			}
			tm.finds(t, "bob", "wombat", "Notes")
			checkSearchEnd(t, tm)
		})
	}
}

// 4. The job takes a page off the queue while a unit holds its row: it
// skips it without waiting; the next run reads it again.
func TestTheSearchsJobSkipsAQueuedRowAUnitHolds(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "the wombat")
	other := tm.createPageWith(t, "alice", nb, "", "Others", "the dingo")
	holder := tm.holdQueueRow(t, id)
	if run := tm.drainInBackground(t)(); run.Written != 2 {
		t.Errorf("the run = %+v, want both rows written", run)
	}
	if got := queryStrings(t, tm.pool, "SELECT node_id::text FROM page_search_queue"); !slices.Equal(got, []string{id}) {
		t.Errorf("queued %q after the run, want the held page alone, %s; %s taken off", got, id, other)
	}
	commit(t, holder)
	if run := drainSearch(t, tm.pool); run.Kept != 1 || run.Written != 0 {
		t.Errorf("the next run = %+v, want the page taken off, its row current", run)
	}
	checkSearchEnd(t, tm)
}

// 5. A notebook deleted while the job builds its pages: no search lists
// them; the next run deletes their rows.
func TestANotebookDeletedWhileTheSearchsJobBuildsItsPages(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	var ids []string
	for i := range 3 {
		ids = append(ids, tm.createPageWith(t, "alice", nb, "", fmt.Sprintf("Page %d", i), "one"))
	}
	drainSearch(t, tm.pool)
	for _, id := range ids {
		tm.send(t, contentWrite("alice", id, "the wombat", 1, ""), http.StatusOK)
	}
	holder := tm.holdSearchRow(t, ids[2])
	ran := tm.drainInBackground(t)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "page_search", 1, interleavingWait)

	tm.send(t, request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""), http.StatusNoContent)
	tm.finds(t, "alice", "wombat")
	commit(t, holder)
	ran()
	tm.finds(t, "alice", "wombat")
	if run := drainSearch(t, tm.pool); run.Deleted != 3 {
		t.Errorf("the next run = %+v, want the three rows deleted", run)
	}
	checkSearchEnd(t, tm)
}

// 6. nervewiki reindex's queueing stopped midway through its statement,
// holding the rows before the one the test holds, while a deletion of the
// subtree queues the same nodes: both take them in id order, so the
// deletion waits for the queueing to commit, and both end, without a
// deadlock.
func TestReindexQueueingAndADeletionQueueTheSameNodes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	root := tm.createPageWith(t, "alice", nb, "", "Root", "the wombat")
	for i := range 5 {
		tm.createPageWith(t, "alice", nb, root, fmt.Sprintf("Child %d", i), "the wombat")
	}
	ids := queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 ORDER BY id", nb)
	if ids[0] != root {
		t.Fatalf("the nodes by id %q, want the root first", ids)
	}
	holder := tm.holdQueueRow(t, ids[3])
	type requeued struct {
		n   int
		err error
	}
	done := make(chan requeued, 1)
	go func() {
		n, err := searchIndex(tm.pool).Requeue(context.Background(), uuid.MustParse(nb))
		done <- requeued{n, err}
	}()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "page_search_queue", 1, interleavingWait)
	deleted := tm.inBackground(t, nodeDeletion("alice", root))
	// The deletion's statement meets the root's key, which the queueing
	// wrote again uncommitted, and waits for its transaction with no row
	// to lock: a key's wait. The unit's writes before it wait for nothing.
	pgtest.WaitForKeyWaitOn(t, tm.pool, "page_search_queue", 1, interleavingWait)

	commit(t, holder)
	select {
	case r := <-done:
		if r.err != nil || r.n != 6 {
			t.Errorf("Requeue() = %d, %v; want the six nodes", r.n, r.err)
		}
	case <-time.After(interleavingWait):
		t.Fatal("the queueing did not end")
	}
	if a := deleted(); a.status != http.StatusNoContent {
		t.Errorf("the deletion = %d %s, want 204", a.status, a.body)
	}
	if got := queryStrings(t, tm.pool, "SELECT count(*) || ' ' || bool_or(bulk) FROM page_search_queue"); !slices.Equal(got, []string{"6 false"}) {
		t.Errorf("the queue = %q, want the six nodes exact, as the deletion queued them", got)
	}
	tm.finds(t, "bob", "wombat")
	checkSearchEnd(t, tm)
}

// 7. A search between an import's units: the first unit's pages, queued in
// bulk, are not found, and the answer says building; once the job has run,
// they are, and it does not. A unit is stopped by the lock of asset_blobs
// it waits for, holding its notebook's row; the test then asks for the
// row, which it gets once the unit commits, before the next unit, as
// betweenUnits tells.
func TestASearchBetweenAnImportsUnits(t *testing.T) {
	tm, _, _ := transferTeam(t)
	files := []vaultFile{{"a.png", pngHead}}
	for i := range 150 {
		files = append(files, vaultFile{fmt.Sprintf("p%03d.md", i), "the kakapo calls [[a.png]]"})
	}
	archive := vaultZip(t, files...)
	ctx := context.Background()
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", fmt.Sprintf("Kakapo %d", try))
		release := tm.holdAssetBlobs(t)
		j := tm.started(t, importStep(t, "bob", nb, "", archive))
		pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held := make(chan error, 1)
		go func() {
			_, err := holder.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR UPDATE", nb)
			held <- err
		}()
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		release()
		if err := <-held; err != nil {
			t.Fatal(err)
		}
		imported := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1", nb)
		if imported == 151 {
			_ = holder.Rollback(ctx)
			tm.endedJob(t, "bob", j.ID)
			continue // the next unit first
		}
		if imported != 100 {
			t.Fatalf("%d nodes imported, want the first unit's 100", imported)
		}
		a := tm.search(t, "bob", "kakapo")
		if len(a.Data) != 0 || !a.Building {
			t.Errorf("search kakapo between the units = %q, building %t; want none, building", a.names(), a.Building)
		}
		commit(t, holder)
		if ended := tm.endedJob(t, "bob", j.ID); ended.State != "succeeded" {
			t.Fatalf("the import = %+v, want it succeeded", ended)
		}
		drainSearch(t, tm.pool)
		a = tm.search(t, "bob", "kakapo")
		if len(a.Data) != 20 || a.NextCursor == nil || a.Building {
			t.Errorf("search kakapo after the job = %d hits, next %t, building %t; want a full page of them, more, built", len(a.Data),
				a.NextCursor != nil, a.Building)
		}
		checkSearchEnd(t, tm)
		checkImports(t, tm)
		return
	}
	t.Fatalf("the test's lock came after the import's next unit, %d times of %d", betweenUnits, betweenUnits)
}

// 8. A page purged while the job is stopped: its queued node is gone, and
// the next run deletes its row.
func TestThePurgeOfAPageTheSearchIndexed(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPageWith(t, "alice", nb, "", "Notes", "the wombat")
	drainSearch(t, tm.pool)
	tm.send(t, nodeDeletion("alice", id), http.StatusNoContent)
	if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour))(); err != nil {
		t.Fatal(err)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1", id) +
		count(t, tm.pool, "SELECT count(*) FROM page_search WHERE node_id = $1", id); n != 1 {
		t.Fatalf("%d of the node and its row, want the row alone, the node purged", n)
	}
	if run := drainSearch(t, tm.pool); run.Deleted != 1 {
		t.Errorf("the run = %+v, want the purged node's row deleted", run)
	}
	checkSearchEnd(t, tm)
}

// 9. Two runs of the job: while another session holds its lock, a run ends
// at once and writes nothing; once let go, a run goes as before.
func TestTheSearchsJobRunsOneAtATime(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	tm.createPageWith(t, "alice", nb, "", "Notes", "the wombat")
	ctx := context.Background()
	conn, err := tm.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(6107471706471486842)"); err != nil {
		t.Fatal(err)
	}
	if run := tm.drainInBackground(t)(); !run.Locked || run.Written != 0 {
		t.Errorf("the run while another holds the lock = %+v, want it locked out, nothing written", run)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM page_search"); n != 0 {
		t.Errorf("%d rows written, want none", n)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock(6107471706471486842)"); err != nil {
		t.Fatal(err)
	}
	if run := drainSearch(t, tm.pool); run.Written != 1 {
		t.Errorf("the run once let go = %+v, want the page's row", run)
	}
	checkSearchEnd(t, tm)
}
