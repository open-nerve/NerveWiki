package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The background restores' and reverts' interleavings with the tree's
// other writes (M8/P4 design 3.10), through serve and River, the jobs'
// batches of three nodes: a write between two batches of a job, which
// the next batch finds under the notebook's lock. The test holds the
// notebook's row so that a job's batch waits for it, sends the write to
// wait behind it, and lets go: PostgreSQL hands the row to its waiters in
// the order they came. A write that needs what the first batch restored
// is sent once it is written, behind the second batch, so it comes
// before the third (bgBetweenBatches). Where the job's next batch could
// still take the row first (a lock asked anew, M7 closeout), the test
// checks that end too and tries again in a notebook of its own
// (betweenUnits). Each ends on the invariants.

// bgBetweenBatches starts c, a background start in the notebook nb, and
// holds its job after its first batch. The test shares the notebook's
// row; the job's first batch waits for it, then each of queued, sent
// then, behind it in turn; the test's hold grows to a tree's write's, and
// a second hold of the test's, FOR SHARE, waits behind them all. Let go,
// the first batch is written, then queued, and the second batch waits for
// the second hold: then runs, and a write it sends that waits for the
// notebook comes after the second batch, before the third. It answers the
// job as it ended, as c's sender reads it, and queued's answers.
func (tm acmeTeam) bgBetweenBatches(t *testing.T, nb string, c step, queued []step, then func()) (backgroundJob, []answer) {
	t.Helper()
	ctx := context.Background()
	first := tm.shareNotebook(t, nb)
	j := tm.startedJob(t, c)
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	var answers []func() answer
	for i, q := range queued {
		answers = append(answers, tm.inBackground(t, q))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2+i, interleavingWait)
	}
	// The test's own share grows to a write's: the second hold, which
	// shares, must wait behind the batch and the writes that wait for it.
	if _, err := first.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR NO KEY UPDATE", nb); err != nil {
		t.Fatal(err)
	}
	second, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Rollback(ctx) })
	held := make(chan error, 1)
	go func() {
		_, err := second.Exec(ctx, "SELECT 1 FROM notebooks WHERE id = $1 FOR SHARE", nb)
		held <- err
	}()
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2+len(queued), interleavingWait)
	commit(t, first)
	select {
	case err := <-held:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(interleavingWait):
		t.Fatal("the test's second hold of the notebook did not come")
	}
	got := make([]answer, len(answers))
	for i, a := range answers {
		got[i] = a()
	}
	// The second batch waits for the second hold.
	pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
	then()
	commit(t, second)
	return tm.endedBackground(t, c.by, j.ID), got
}

// bgAfterSecondBatch sends r, which waits for the notebook behind the
// job's second batch, as bgBetweenBatches' then: r comes after it, before
// the third. It answers what waits for r's answer.
func (tm acmeTeam) bgAfterSecondBatch(t *testing.T, r step) (then func(), got func() answer) {
	t.Helper()
	then = func() {
		got = tm.inBackground(t, r)
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
	}
	return then, func() answer { return got() }
}

// bgBox creates the page Box at the notebook nb's root, and under it the
// pages named, and answers Box's id and theirs.
func (tm acmeTeam) bgBox(t *testing.T, nb string, names ...string) (string, []string) {
	t.Helper()
	box := tm.createPage(t, "alice", nb, "", "Box")
	ids := make([]string, len(names))
	for i, name := range names {
		ids[i] = tm.createPage(t, "alice", nb, box, name)
	}
	return box, ids
}

// bgSeven are the names of Box's seven pages: with Box, a group of eight
// a job restores in three batches, Box, C1 and C2; C3 to C5; C6 and C7.
func bgSeven() []string {
	return []string{"C1", "C2", "C3", "C4", "C5", "C6", "C7"}
}

// bgName is the name of the node id.
func (tm acmeTeam) bgName(t *testing.T, id string) string {
	t.Helper()
	names := queryStrings(t, tm.pool, "SELECT name FROM nodes WHERE id = $1", id)
	if len(names) != 1 {
		t.Fatalf("the node %s: %v", id, names)
	}
	return names[0]
}

// bgTrashItems are the notebook nb's trash items, each its root's name and
// pages, by name.
func (tm acmeTeam) bgTrashItems(t *testing.T, nb string) []string {
	t.Helper()
	items := queryStrings(t, tm.pool, "SELECT name || ' ' || trash_pages FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root",
		nb)
	slices.Sort(items)
	return items
}

// bgLastOf is the node of the changeset id with the greatest id: a revert
// deletes the tops of an import in their ids' order, so its last.
func (tm acmeTeam) bgLastOf(t *testing.T, id string) string {
	t.Helper()
	return queryStrings(t, tm.pool, "SELECT node_id::text FROM changeset_items WHERE changeset_id = $1 ORDER BY node_id DESC LIMIT 1", id)[0]
}

// Interleaving 1: a background restore and a page created, by a name of
// the item's, under its root, which the job's first batch restored. The
// creation between the second batch and the third: the third restores
// the item's page of that name numbered, renamed. The third first: the
// creation is page.title_taken.
func TestARestoreJobAndANameTakenBetweenItsBatches(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Named "+strconv.Itoa(try))
		box, kids := tm.bgBox(t, nb, bgSeven()...)
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		then, created := tm.bgAfterSecondBatch(t, childCreation("alice", nb, box, "C6"))
		j, _ := tm.bgBetweenBatches(t, nb, trashRestoreStart("bob", nb, box), nil, then)
		a := created()
		if a.is(http.StatusConflict, "page.title_taken") {
			if j.State != "succeeded" || j.counts()["renamed"] != 0 || tm.bgName(t, kids[5]) != "C6" {
				t.Errorf("the third batch first: the restore = %s, C6 named %q; want it whole, C6 its name", j, tm.bgName(t, kids[5]))
			}
			continue
		}
		if a.status != http.StatusCreated || j.State != "succeeded" || j.counts()["pages"] != 8 || j.counts()["renamed"] != 1 ||
			j.counts()["skipped"] != 0 {
			t.Fatalf("the creation = %d %s, the restore = %s; want 201, then the restore of 8 pages, one renamed", a.status, a.body, j)
		}
		if got := tm.bgName(t, kids[5]); got != "C6 2" {
			t.Errorf("the item's C6 restored as %q, want C6 2", got)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the creation came after the restore's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 2: a background restore and the deletion of what its first
// batches restored, under which its third batch restores: A, under Box,
// and five of A's six pages, the batches' order being the group's
// pre-order. The deletion between the second batch and the third: the
// third finds A gone, the job fails tree_changed, the six pages written
// kept; A is a trash item of its own with what it held, and the two pages
// of A the first batch left in the trash are each one still, their counts
// right. The third first: the deletion takes A with all of its pages.
func TestARestoreJobWhoseRestoredPartIsDeleted(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Deleted "+strconv.Itoa(try))
		box := tm.createPage(t, "alice", nb, "", "Box")
		a := tm.createPage(t, "alice", nb, box, "A")
		for i := range 6 {
			tm.createPage(t, "alice", nb, a, fmt.Sprintf("a%d", i+1))
		}
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		then, deleted := tm.bgAfterSecondBatch(t, nodeDeletion("alice", a))
		j, _ := tm.bgBetweenBatches(t, nb, trashRestoreStart("bob", nb, box), nil, then)
		if d := deleted(); d.status != http.StatusNoContent {
			t.Fatalf("the deletion of A = %d %s, want 204", d.status, d.body)
		}
		if j.State == "succeeded" {
			if got := tm.bgTrashItems(t, nb); !slices.Equal(got, []string{"A 7"}) {
				t.Errorf("the third batch first: the trash = %q, want A with its six pages", got)
			}
			continue
		}
		if j.State != "failed" || j.failure() != "tree_changed" || j.counts()["pages"] != 6 || j.Progress.Done != 6 {
			t.Fatalf("the restore = %s, want it failed tree_changed after two batches", j)
		}
		if got := tm.bgTrashItems(t, nb); !slices.Equal(got, []string{"A 5", "a5 1", "a6 1"}) {
			t.Errorf("the trash = %q, want A with the four pages restored under it, a5 and a6 each an item", got)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the deletion came after the restore's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 3: a background restore and a move of what its first
// batches restored, deeper: Box, its pages L1 to L4 and D, under X8,
// eight levels deep, puts them at the tenth, the deepest pages go; D's
// page, which the third batch restores, would be at the eleventh. The
// move between the second batch and the third: the third finds it too
// deep, and the job fails tree_changed. The third first: the move is
// page.too_deep.
func TestARestoreJobWhoseRestoredPartMovesTooDeep(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Moved "+strconv.Itoa(try))
		x := ""
		for i := range 8 {
			x = tm.createPage(t, "alice", nb, x, fmt.Sprintf("X%d", i+1))
		}
		box, kids := tm.bgBox(t, nb, "L1", "L2", "L3", "L4", "D")
		tm.createPage(t, "alice", nb, kids[4], "d")
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		then, moved := tm.bgAfterSecondBatch(t, nodeMove("alice", box, x))
		j, _ := tm.bgBetweenBatches(t, nb, trashRestoreStart("bob", nb, box), nil, then)
		m := moved()
		if m.is(http.StatusConflict, "page.too_deep") {
			if j.State != "succeeded" || j.counts()["pages"] != 7 {
				t.Errorf("the third batch first: the restore = %s, want it whole", j)
			}
			continue
		}
		if m.status != http.StatusOK || j.State != "failed" || j.failure() != "tree_changed" || j.counts()["pages"] != 6 {
			t.Fatalf("the move = %d %s, the restore = %s; want 200, then the restore failed tree_changed after two batches", m.status,
				m.body, j)
		}
		if got := tm.bgTrashItems(t, nb); !slices.Equal(got, []string{"d 1"}) {
			t.Errorf("the trash = %q, want d alone, an item of its own", got)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the move came after the restore's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 4: a background restore and a restore, through a request,
// of a page of its item that its first batch left in the trash, an item
// of its own. Between the second batch and the third: the third skips
// it, the job succeeds, skipped counted. The third first: the request is
// page.not_in_trash.
func TestARestoreJobSkipsAnItemRestoredBetweenItsBatches(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Skipped "+strconv.Itoa(try))
		box, kids := tm.bgBox(t, nb, bgSeven()...)
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		then, restored := tm.bgAfterSecondBatch(t, nodeRestore("bob", kids[5]))
		j, _ := tm.bgBetweenBatches(t, nb, trashRestoreStart("bob", nb, box), nil, then)
		r := restored()
		if r.is(http.StatusConflict, "page.not_in_trash") {
			if j.State != "succeeded" || j.counts()["pages"] != 8 || j.counts()["skipped"] != 0 {
				t.Errorf("the third batch first: the restore = %s, want it whole", j)
			}
			continue
		}
		if r.status != http.StatusOK || j.State != "succeeded" || j.counts()["pages"] != 7 || j.counts()["skipped"] != 1 ||
			j.Progress.Done != 8 || j.Progress.Total != 8 {
			t.Fatalf("the request's restore = %d %s, the job = %s; want 200, then the job's of 7 pages, C6 skipped", r.status, r.body, j)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb); n != 8 {
			t.Errorf("%d nodes alive, want Box and its seven pages", n)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the request came after the restore's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 5: a background restore and the purge. The purge at its
// cutoff between the first batch and the second takes an item past the
// period, Old, and leaves the item the job restores, within it, the
// nodes the first batch left in the trash included: the second batch
// restores them. An item moved before its job runs to between the end of
// its period and the purge's cutoff: the job fails restore_expired,
// writing nothing, and the same purge leaves the item, an hour clear.
func TestARestoreJobAndThePurge(t *testing.T) {
	retention := 24 * time.Hour
	tm := newAcmeTeamWired(t, "member", "", func(c *config.Config) { c.Jobs.PurgeRetention = retention }, transfer.Batch{Nodes: 3})
	t.Run("between two batches", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Between")
		old := tm.createPage(t, "alice", nb, "", "Old")
		tm.send(t, nodeDeletion("alice", old), http.StatusNoContent)
		tm.trashedAgo(t, old, retention+time.Hour)
		box, _ := tm.bgBox(t, nb, "C1", "C2", "C3", "C4")
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		j, _ := tm.bgBetweenBatches(t, nb, trashRestoreStart("bob", nb, box), nil, func() {
			if err := tm.purgeBefore(t, time.Now().Add(-retention))(); err != nil {
				t.Errorf("the purge: %v", err)
			}
		})
		if j.State != "succeeded" || j.counts()["pages"] != 5 {
			t.Errorf("the restore = %s, want it whole", j)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1", old); n != 0 {
			t.Errorf("Old kept, want it purged")
		}
		checkBackground(t, tm)
	})
	t.Run("past the period", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Past")
		box, _ := tm.bgBox(t, nb, "C1", "C2", "C3", "C4")
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		release := tm.holdImportWorker(t)
		j := tm.startedJob(t, trashRestoreStart("bob", nb, box))
		tm.trashedAgo(t, box, retention-30*time.Minute)
		if err := tm.purgeBefore(t, time.Now().Add(-retention))(); err != nil {
			t.Fatalf("the purge: %v", err)
		}
		release()
		ended := tm.endedBackground(t, "bob", j.ID)
		if ended.State != "failed" || ended.failure() != "restore_expired" || ended.counts()["pages"] != 0 || ended.Progress.Done != 0 {
			t.Errorf("the restore = %s, want it failed restore_expired, nothing written", ended)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL", nb); n != 5 {
			t.Errorf("%d nodes in the trash, want the item's five, the purge's later", n)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1", *ended.ChangesetID); n != 0 {
			t.Errorf("the job's changeset written, want none")
		}
		checkBackground(t, tm)
	})
}

// Interleaving 9: a background revert of an import of four pages at the
// root, which deletes them three a batch, and a page created under the
// last, which its second batch deletes. The creation between the batches:
// the second batch finds the page under it, the job fails revert_conflict,
// children_added, the three deleted kept. The second first: the creation
// finds no parent.
func TestARevertJobAndAPageCreatedUnderATopItDeletes(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Added "+strconv.Itoa(try))
		if j := tm.imported(t, "alice", nb, "", flatImport(t, 4)); j.State != "succeeded" {
			t.Fatalf("the import = %+v", j)
		}
		imported := tm.newest(t, nb).ID
		last := tm.bgLastOf(t, imported)
		j, created := tm.behindFirstBatch(t, nb, revertStart("bob", nb, imported, ""), childCreation("alice", nb, last, "Added"))
		if created.status != http.StatusCreated {
			if j.State != "succeeded" || j.counts()["deleted"] != 4 {
				t.Errorf("the second batch first: the revert = %s, want it whole", j)
			}
			continue
		}
		if j.State != "failed" || j.failure() != "revert_conflict" || j.counts()["deleted"] != 3 || j.ConflictCount != 1 ||
			len(j.Conflicts) != 1 || j.Conflicts[0].Reason != "children_added" {
			t.Fatalf("the revert = %s, want it failed revert_conflict, children_added, after a batch", j)
		}
		if count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", last) != 1 {
			t.Errorf("%s deleted, want it kept with the page under it", tm.bgName(t, last))
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the creation came after the revert's second batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 10: a background revert of the deletion of Box and its
// seven pages, a group it restores across three batches, and a page
// created under Box, which its first batch restored, by the name of the
// group's C6. Between the second batch and the third: C6, a member of the
// group's later batches, goes back under its name or not at all, and the
// job fails revert_conflict, title_taken, naming the page that holds the
// name. The third first: the creation is page.title_taken.
func TestARevertJobAndANameTakenInItsGroup(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Group "+strconv.Itoa(try))
		box, kids := tm.bgBox(t, nb, bgSeven()...)
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		deletion := tm.newest(t, nb).ID
		then, created := tm.bgAfterSecondBatch(t, childCreation("alice", nb, box, "C6"))
		j, _ := tm.bgBetweenBatches(t, nb, revertStart("bob", nb, deletion, ""), nil, then)
		a := created()
		if a.is(http.StatusConflict, "page.title_taken") {
			if j.State != "succeeded" || j.counts()["restored"] != 8 {
				t.Errorf("the third batch first: the revert = %s, want it whole", j)
			}
			continue
		}
		if a.status != http.StatusCreated {
			t.Fatalf("the creation = %d %s, want 201", a.status, a.body)
		}
		var holder struct {
			ID string `json:"id"`
		}
		decodeAnswer(t, a.body, &holder)
		if j.State != "failed" || j.failure() != "revert_conflict" || j.counts()["restored"] != 6 || j.ConflictCount != 1 ||
			len(j.Conflicts) != 1 || j.Conflicts[0].NodeID != kids[5] || j.Conflicts[0].Reason != "title_taken" ||
			j.Conflicts[0].Name != "C6" || j.Conflicts[0].HolderID == nil || *j.Conflicts[0].HolderID != holder.ID {
			t.Fatalf("the revert = %s, want it failed revert_conflict, C6's title taken by %s, after two batches", j, holder.ID)
		}
		if got := tm.bgTrashItems(t, nb); !slices.Equal(got, []string{"C6 1", "C7 1"}) {
			t.Errorf("the trash = %q, want C6 and C7, each an item", got)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the creation came after the revert's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 13: a node a background revert's later batch deleted
// between a changeset C's deletion of it and a restore r of it, whose
// revert deleted it again (M8/P4 design 3.7). The revert J of an import
// of seven pages under P, three a batch, the last, N, alone in its third,
// makes its changeset in its first; C deletes N after it, and a restore
// brings N back under P before J's third batch deletes it there. Then P
// is deleted, r restores N at the root, P being in the trash, r's revert
// deletes it there, and P is restored. N is in the trash by a revert of a
// restore that followed C's deletion, but J's deletion came between: N's
// item is not C's, and N, at the root, is not where C left it: C's revert
// is page.revert_conflict, changed. By the changesets' times, J's being
// before C's, N would be compared by its group alone and put back under P.
func TestARevertJobsDeletionBetweenADeletionAndARestore(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Between "+strconv.Itoa(try))
		p := tm.createPage(t, "alice", nb, "", "P")
		if j := tm.imported(t, "alice", nb, p, flatImport(t, 7)); j.State != "succeeded" {
			t.Fatalf("the import = %+v", j)
		}
		imported := tm.newest(t, nb).ID
		n := tm.bgLastOf(t, imported)
		then, back := tm.bgAfterSecondBatch(t, nodeRestore("bob", n))
		j, answers := tm.bgBetweenBatches(t, nb, revertStart("bob", nb, imported, ""), []step{nodeDeletion("alice", n)}, then)
		if d := answers[0]; d.status != http.StatusNoContent {
			t.Fatalf("C, the deletion of N = %d %s, want 204", d.status, d.body)
		}
		if b := back(); b.status != http.StatusOK || j.State != "succeeded" {
			// The third batch first: J finds N deleted, changed, and the
			// restore then brings it back.
			if j.State != "failed" || j.failure() != "revert_conflict" {
				t.Errorf("the third batch first: the revert = %s, want it failed revert_conflict", j)
			}
			continue
		}
		if j.counts()["deleted"] != 7 {
			t.Fatalf("the revert = %s, want the import's seven deleted", j)
		}
		c := queryStrings(t, tm.pool, `SELECT s.id::text FROM changesets s JOIN changeset_items i ON i.changeset_id = s.id
			WHERE s.kind = 'edit' AND i.node_id = $1 AND i.after_name IS NULL`, n)
		if len(c) != 1 || count(t, tm.pool, "SELECT count(*) FROM changesets j, changesets c WHERE j.id = $1 AND c.id = $2 AND j.created_at < c.created_at",
			*j.ChangesetID, c[0]) != 1 {
			t.Fatalf("C = %v, want one deletion of N after J's changeset was made", c)
		}
		tm.send(t, nodeDeletion("alice", p), http.StatusNoContent)
		r := tm.restored(t, "bob", n)
		if r.Placed.at() != " parent_in_trash" {
			t.Fatalf("r restored N %+v, want it at the root, P in the trash", r)
		}
		tm.reverted(t, "alice", r.ChangesetID)
		tm.restored(t, "alice", p)
		if between := count(t, tm.pool, `SELECT count(*) FROM changeset_items x WHERE x.node_id = $1 AND x.changeset_id = $2
			AND x.after_name IS NULL AND x.updated_at > (SELECT updated_at FROM changeset_items WHERE changeset_id = $3 AND node_id = $1)
			AND x.updated_at < (SELECT created_at FROM changeset_items WHERE changeset_id = $4 AND node_id = $1)`,
			n, *j.ChangesetID, c[0], r.ChangesetID); between != 1 {
			t.Fatalf("J's deletion of N between C's and r's restore: %d, want it", between)
		}
		conflicted(t, "C's revert", tm.inBackground(t, changesetRevert("bob", c[0]))(), n, "changed")
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the restore came after the revert's third batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 14: a background restore and the revert, through a
// request, of the deletion C that made its item (M8/P4 design 3.10,
// A-M10). C's revert after the job's plan, before its first batch: it
// restores the group whole, and the first batch finds the item changed,
// trash_changed, writing nothing. C's revert after the first batch: it
// finds the item's root restored, page.revert_conflict, changed, and the
// job goes on to the end.
func TestARestoreJobAndTheRevertOfItsDeletion(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	t.Run("before the first batch", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Before")
		box, _ := tm.bgBox(t, nb, "C1", "C2", "C3", "C4")
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		deletion := tm.newest(t, nb).ID
		holder := tm.shareNotebook(t, nb)
		reverted := tm.inBackground(t, changesetRevert("alice", deletion))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		// The start shares the notebook's row with the test; its job plans,
		// and its first batch waits behind the revert.
		j := tm.startedJob(t, trashRestoreStart("bob", nb, box))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 2, interleavingWait)
		commit(t, holder)
		if r := reverted(); r.status != http.StatusOK {
			t.Fatalf("C's revert = %d %s, want 200", r.status, r.body)
		}
		ended := tm.endedBackground(t, "bob", j.ID)
		if ended.State != "failed" || ended.failure() != "trash_changed" || ended.counts()["pages"] != 0 {
			t.Errorf("the restore = %s, want it failed trash_changed, nothing written", ended)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1", *ended.ChangesetID); n != 0 {
			t.Errorf("the job's changeset written, want none")
		}
		checkBackground(t, tm)
	})
	t.Run("after the first batch", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "After")
		box, _ := tm.bgBox(t, nb, "C1", "C2", "C3", "C4")
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		deletion := tm.newest(t, nb).ID
		j, r := tm.behindFirstBatch(t, nb, trashRestoreStart("bob", nb, box), changesetRevert("alice", deletion))
		conflicts, _ := conflictsOf(t, r)
		if !r.is(http.StatusConflict, "page.revert_conflict") || !slices.ContainsFunc(conflicts, func(c revertConflict) bool {
			return c.NodeID == box && c.Reason == "changed"
		}) {
			t.Errorf("C's revert = %d %s, want 409 page.revert_conflict, Box changed", r.status, r.body)
		}
		if j.State != "succeeded" || j.counts()["pages"] != 5 {
			t.Errorf("the restore = %s, want it whole", j)
		}
		checkBackground(t, tm)
	})
}

// A background revert of R1, the revert of an import of ten pages under
// P: ten groups of a page each, four batches of three nodes at most. The
// last group's page restored where it was by a request between the job's
// first and second batches, then deleted again, unchanged, between the
// second and the third: the fourth batch plans its group at the time it
// is deleted at now and reads its content there, as a request's revert of
// R1 would, and the job restores all ten (M8/P4A review A-M1). The fourth
// batch before the deletion: the page alive where it was is changed,
// page.revert_conflict, as a request's revert has it. Its groups are a
// page each, so it pins the batch's reads at the new time; that later
// batches read a group there, TestARevertJobOfAGroupDeletedAgainSinceThePlan
// (page/app) holds.
func TestARevertJobOfAGroupDeletedAgainSinceItsPlan(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Again "+strconv.Itoa(try))
		p := tm.createPage(t, "alice", nb, "", "P")
		if j := tm.imported(t, "alice", nb, p, flatImport(t, 10)); j.State != "succeeded" {
			t.Fatalf("the import = %+v", j)
		}
		r1 := tm.reverted(t, "alice", tm.newest(t, nb).ID)
		g := tm.bgLastOf(t, r1)
		then, deleted := tm.bgAfterSecondBatch(t, nodeDeletion("bob", g))
		j, answers := tm.bgBetweenBatches(t, nb, revertStart("bob", nb, r1, ""), []step{nodeRestore("bob", g)}, then)
		if a := answers[0]; a.status != http.StatusOK {
			t.Fatalf("the restore of %s = %d %s, want 200", g, a.status, a.body)
		}
		if d := deleted(); d.status != http.StatusNoContent {
			t.Fatalf("the deletion of %s = %d %s, want 204", g, d.status, d.body)
		}
		if j.failure() == "revert_conflict" && len(j.Conflicts) == 1 && j.Conflicts[0].NodeID == g &&
			j.Conflicts[0].Reason == "changed" && j.Progress.Done == 9 {
			continue
		}
		if j.State != "succeeded" || j.counts()["restored"] != 10 || j.Progress.Done != 10 {
			t.Fatalf("the revert of R1 = %s; want it whole, %s back where it was", j, g)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the deletion came after the revert's fourth batch, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 16: a background revert of the deletion of Box and its
// seven pages, a group it restores across three batches, and a restore,
// through a request, of C6, which its first batch left in the trash, an
// item of its own: C6 goes back where the deletion took it from. Between
// the second batch and the third: the third skips it, and the job
// succeeds, the notebook as it was before the deletion. The third first:
// the request is page.not_in_trash.
func TestARevertJobSkipsAMemberRestoredBetweenItsBatches(t *testing.T) {
	tm := newBatchedTeam(t, "member", "")
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", "Restored "+strconv.Itoa(try))
		box, kids := tm.bgBox(t, nb, bgSeven()...)
		before := tm.notebookState(t, nb)
		tm.send(t, nodeDeletion("alice", box), http.StatusNoContent)
		deletion := tm.newest(t, nb).ID
		then, restored := tm.bgAfterSecondBatch(t, nodeRestore("bob", kids[5]))
		j, _ := tm.bgBetweenBatches(t, nb, revertStart("bob", nb, deletion, ""), nil, then)
		r := restored()
		if r.is(http.StatusConflict, "page.not_in_trash") {
			if j.State != "succeeded" || j.counts()["restored"] != 8 || j.counts()["skipped"] != 0 {
				t.Errorf("the third batch first: the revert = %s, want it whole", j)
			}
			continue
		}
		if r.status != http.StatusOK || j.State != "succeeded" || j.counts()["restored"] != 7 || j.counts()["skipped"] != 1 ||
			j.Progress.Done != 8 {
			t.Fatalf("the request's restore = %d %s, the revert = %s; want 200, then the revert of the rest, C6 skipped", r.status, r.body, j)
		}
		if now := tm.notebookState(t, nb); now != before {
			t.Errorf("the notebook after the revert:\n%s\nwant as before the deletion:\n%s", now, before)
		}
		checkBackground(t, tm)
		return
	}
	t.Fatalf("the request came after the revert's third batch, %d times of %d", betweenUnits, betweenUnits)
}
