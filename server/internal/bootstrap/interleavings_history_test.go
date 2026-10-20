package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The interleavings of the revert (M8/P3 design 3.10), harnessed as the
// trash's (interleavings_trash_test.go): a revert reads and plans
// unlocked, then locks its notebook's row FOR NO KEY UPDATE as every
// tree's write does, and reads and plans again there, which decides. Each
// ends by checking the pages', the links', the attachments' and the
// notebooks' invariants.

// reverts counts the reverts' changesets of the notebook nb.
func (tm acmeTeam) reverts(t *testing.T, nb string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM changesets WHERE notebook_id = $1 AND kind = 'revert'", nb)
}

// conflicted fails t unless a is page.revert_conflict of the one node id,
// for reason.
func conflicted(t *testing.T, what string, a answer, id, reason string) {
	t.Helper()
	conflicts, n := conflictsOf(t, a)
	if !a.is(http.StatusConflict, "page.revert_conflict") || n != 1 || len(conflicts) != 1 || conflicts[0].NodeID != id ||
		conflicts[0].Reason != reason {
		t.Errorf("%s = %d %s, want 409 page.revert_conflict, %s %s", what, a.status, a.body, id, reason)
	}
}

// Interleaving 1: a revert and a write of a page whose content its
// changeset wrote. The write first: the revert finds the page changed
// under the lock, page.revert_conflict, and writes nothing. The revert
// first: the write's base is gone, page.revision_mismatch.
func TestRevertingAndWritingAPageOfIt(t *testing.T) {
	orders(t, "revert", "write", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPageWith(t, "alice", nb, "", "P", "one")
		tm.send(t, contentWrite("alice", p, "two", 1, ""), http.StatusOK)
		c := tm.newest(t, nb).ID
		steps := map[string]step{"revert": changesetRevert("bob", c), "write": contentWrite("alice", p, "three", 2, "")}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		revert, write := got["revert"], got["write"]
		content := tm.content(t, "alice", p).Content
		switch first {
		case "write":
			conflicted(t, "the revert after the write", revert, p, "changed")
			if write.status != http.StatusOK || content != "three" || tm.reverts(t, nb) != 0 {
				t.Errorf("the write = %d, P %q; want 200, three, and no revert", write.status, content)
			}
		case "revert":
			if revert.status != http.StatusOK || !write.is(http.StatusConflict, "page.revision_mismatch") || content != "one" {
				t.Errorf("the revert = %d %s, the write = %d %s, P %q; want 200, then 409 page.revision_mismatch, one", revert.status,
					revert.body, write.status, write.code, content)
			}
		}
		tm.checkHistory(t)
	})
}

// Interleaving 2: a revert of a rename and another rename of its node. The
// rename first: the revert finds the node changed under the lock. The
// revert first: the rename renames the node from the name the revert put
// back.
func TestRevertingAndRenamingANodeOfIt(t *testing.T) {
	orders(t, "revert", "rename", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		x := tm.createPage(t, "alice", nb, "", "A")
		tm.send(t, nodeRename("alice", x, "B"), http.StatusOK)
		c := tm.newest(t, nb).ID
		steps := map[string]step{"revert": changesetRevert("bob", c), "rename": nodeRename("alice", x, "C")}
		a, b := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: a, second: b}
		revert, rename := got["revert"], got["rename"]
		switch first {
		case "rename":
			conflicted(t, "the revert after the rename", revert, x, "changed")
			if rename.status != http.StatusOK || tm.reverts(t, nb) != 0 {
				t.Errorf("the rename = %d %s, want 200, and no revert", rename.status, rename.body)
			}
		case "revert":
			if revert.status != http.StatusOK || rename.status != http.StatusOK || count(t, tm.pool, `SELECT count(*) FROM changeset_items i
				JOIN changesets c ON c.id = i.changeset_id WHERE i.node_id = $1 AND c.kind = 'edit' AND i.before_name = 'A'
				AND i.after_name = 'C'`, x) != 1 {
				t.Errorf("the revert = %d %s, the rename = %d %s; want both 200, the rename from A", revert.status, revert.body,
					rename.status, rename.body)
			}
		}
		tm.checkHistory(t)
	})
}

// Interleaving 3: a revert of a changeset whose pages are being edited (M5
// handoff, item 3). C renamed T, writing the links of P and Q again; while
// alice's session holds P, bob's revert of C is page.locked by her, and
// writes nothing: T's name, P and Q are as C left them. While bob's own
// session elsewhere holds P, his revert is page.locked by him. A revert of
// a creation whose page alice is editing is page.locked by her.
func TestRevertingWhileAPageOfItIsEdited(t *testing.T) {
	refused := func(t *testing.T, tm acmeTeam, nb, c, holder string) {
		t.Helper()
		was := tm.notebookState(t, nb)
		r := changesetRevert("bob", c)
		status, body := ask(t, tm.contract, r.method, tm.base+r.path, tm.tokens["bob"], "")
		if a := (answer{status: status, body: body, code: problemCode(t, body)}); !a.is(http.StatusConflict, "page.locked") ||
			lockHolder(a) != holder {
			t.Errorf("the revert = %d %s, want 409 page.locked by %s", status, body, holder)
		}
		if now := tm.notebookState(t, nb); now != was || tm.reverts(t, nb) != 0 {
			t.Errorf("the refused revert changed the notebook:\n%s\nwas\n%s", now, was)
		}
		tm.checkHistory(t)
	}
	for _, holder := range []string{"alice", "bob"} {
		t.Run("a rename's rewrite, P held by "+holder, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			target := tm.createPage(t, "alice", nb, "", "T")
			p := tm.createPageWith(t, "alice", nb, "", "P", "[[T]]")
			tm.createPageWith(t, "alice", nb, "", "Q", "see [[T]]")
			tm.send(t, nodeRename("alice", target, "U"), http.StatusOK)
			c := tm.newest(t, nb)
			if c.ContentCount != 2 {
				t.Fatalf("the rename = %+v, want P and Q written again in it", c)
			}
			tm.openSession(t, holder, p)
			refused(t, tm, nb, c.ID, holder)
		})
	}
	t.Run("a creation of a page being edited", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		n := tm.createPage(t, "alice", nb, "", "N")
		c := tm.newest(t, nb).ID
		tm.openSession(t, "alice", n)
		refused(t, tm, nb, c, "alice")
	})
}

// Interleaving 4: a revert and the purge that trims its changeset. C
// deleted B's subtree. While the revert, its unlocked reads done, waits
// for the notebook, a purge whose clock is ahead takes C's items and
// versions, marking C: under the lock the revert reads the fewer items,
// then the mark, page.changeset_trimmed, and writes nothing (unmarked, an
// empty plan would have answered changed false). The revert first: B is
// back, and the purge takes nothing of it. A transaction holding C's row
// keeps the purge off C's items and mark; once it ends, the purge takes
// and marks them. The server's own purge runs once as its jobs start, at a
// moment River picks (2-3.6 s after the start here), and its last
// statement deletes, whatever their time, the changesets a purge marked
// and emptied of items and versions: the cases whose purge marks C wait
// for it first, or it could take C between their purge and their check.
func TestRevertingAndPurgingTheChangeset(t *testing.T) {
	deleted := func(t *testing.T) (tm acmeTeam, nb, b, c string) {
		t.Helper()
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		b = tm.createPageWith(t, "alice", nb, "", "B", "# B")
		tm.createPage(t, "alice", nb, b, "Under")
		tm.send(t, nodeDeletion("alice", b), http.StatusNoContent)
		return tm, nb, b, tm.newest(t, nb).ID
	}
	ahead := func(t *testing.T, tm acmeTeam) {
		t.Helper()
		if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour), "changeset_items", "page_revisions")(); err != nil {
			t.Errorf("the purge: %v", err)
		}
	}
	itemsOf := func(t *testing.T, tm acmeTeam, c string) (items int, trimmed bool) {
		t.Helper()
		return count(t, tm.pool, "SELECT count(*) FROM changeset_items WHERE changeset_id = $1", c),
			count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1 AND trimmed", c) == 1
	}
	t.Run("the purge first", func(t *testing.T) {
		tm, nb, b, c := deleted(t)
		awaitJob(t, tm.pool, jobs.PurgeKind)
		holder := tm.holdNotebook(t, nb)
		got := tm.waitingFor(t, "notebooks", changesetRevert("bob", c), func() {
			ahead(t, tm)
			commit(t, holder)
		})
		if !got.is(http.StatusConflict, "page.changeset_trimmed") {
			t.Errorf("the revert = %d %s, want 409 page.changeset_trimmed", got.status, got.body)
		}
		if items, trimmed := itemsOf(t, tm, c); items != 0 || !trimmed || tm.reverts(t, nb) != 0 ||
			count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NOT NULL", b) != 1 {
			t.Errorf("C has %d items, trimmed %v; want none, trimmed, B still in the trash and no revert", items, trimmed)
		}
		tm.checkHistory(t)
	})
	t.Run("the revert first", func(t *testing.T) {
		tm, _, b, c := deleted(t)
		tm.reverted(t, "bob", c)
		const rows = "SELECT (SELECT count(*) FROM changeset_items) + (SELECT count(*) FROM page_revisions)"
		before := count(t, tm.pool, rows)
		ahead(t, tm)
		if after := count(t, tm.pool, rows); after != before || count(t, tm.pool, "SELECT count(*) FROM changesets WHERE trimmed") != 0 {
			t.Errorf("the purge after the revert left %d rows of %d, or marked a changeset; want it to take nothing", after, before)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", b); n != 1 {
			t.Errorf("B is not back")
		}
		tm.checkHistory(t)
	})
	t.Run("C's row held", func(t *testing.T) {
		tm, _, _, c := deleted(t)
		awaitJob(t, tm.pool, jobs.PurgeKind)
		ctx := context.Background()
		holder, err := tm.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = holder.Rollback(ctx) })
		if _, err := holder.Exec(ctx, "SELECT 1 FROM changesets WHERE id = $1 FOR NO KEY UPDATE", c); err != nil {
			t.Fatal(err)
		}
		was, _ := itemsOf(t, tm, c)
		ahead(t, tm)
		if items, trimmed := itemsOf(t, tm, c); items != was || trimmed {
			t.Errorf("C held, the purge left %d of its %d items, marked %v; want them all, unmarked", items, was, trimmed)
		}
		commit(t, holder)
		ahead(t, tm)
		if items, trimmed := itemsOf(t, tm, c); items != 0 || !trimmed {
			t.Errorf("C let go, the purge left %d items, marked %v; want none, marked", items, trimmed)
		}
		tm.checkHistory(t)
	})
}

// threeBatches is a vault an import writes in three units: 100 pages,
// then an attachment and 99 pages, then an attachment and 10 pages (each
// level's nodes by name, 100 a unit).
func threeBatches(t *testing.T) []byte {
	t.Helper()
	var files []vaultFile
	for i := range 100 {
		files = append(files, vaultFile{fmt.Sprintf("p%03d.md", i), "first"})
	}
	files = append(files, vaultFile{"q.png", pngHead})
	for i := range 99 {
		files = append(files, vaultFile{fmt.Sprintf("q%03d.md", i), "second"})
	}
	files = append(files, vaultFile{"r.png", pngHead})
	for i := range 10 {
		files = append(files, vaultFile{fmt.Sprintf("r%03d.md", i), "third"})
	}
	return vaultZip(t, files...)
}

// Interleaving 5: a revert of a changeset an import is merging its units
// into (as TestAnImportWhosePlaceIsDeleted). The import's second unit is
// held as it writes its attachment's row, holding the notebook; the
// revert of the import's changeset C, whose unlocked reads find the first
// unit's, waits for the notebook behind it. Let go, the revert comes
// before the third unit: under the lock it reads both units' and reverts
// them, their top nodes in the trash; the third unit finds C reverted as
// it locks C in Run, before writing anything, and the job fails
// changeset_reverted, reporting the two units' counts, the third unit's
// file deleted (checkAssets: no file in the store without its row). The
// third unit first, the revert reverts the whole import, and the job
// succeeds. C's id is the job's changeset_id, which its row has from its
// start (M8/P4 design 3.2); the first unit made C with it.
func TestRevertingAnImportBeingMerged(t *testing.T) {
	tm, _, _ := transferTeam(t)
	for try := range betweenUnits {
		nb := tm.openNotebook(t, "alice", fmt.Sprintf("Merged %d", try))
		release := tm.holdAssetBlobs(t)
		j := tm.started(t, importStep(t, "bob", nb, "", threeBatches(t)))
		pgtest.WaitForTableLockWaits(t, tm.pool, "asset_blobs", 1, interleavingWait)
		status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/transfer-jobs/"+j.ID, tm.tokens["bob"], "")
		var row backgroundJob
		decodeAnswer(t, answer, &row)
		if status != http.StatusOK || row.ChangesetID == nil {
			t.Fatalf("the import's job = %d %s, want its changeset", status, answer)
		}
		if changesets := queryStrings(t, tm.pool, "SELECT id::text FROM changesets WHERE notebook_id = $1 AND kind = 'import'", nb); !slices.Equal(
			changesets, []string{*row.ChangesetID}) {
			t.Fatalf("the import's changesets = %v, want the first unit's, the job's %s", changesets, *row.ChangesetID)
		}

		reverted := tm.inBackground(t, changesetRevert("alice", *row.ChangesetID))
		pgtest.WaitForLockWaitsOn(t, tm.pool, "notebooks", 1, interleavingWait)
		release()
		if a := reverted(); a.status != http.StatusOK {
			t.Fatalf("the revert = %d %s, want 200", a.status, a.body)
		}
		ended := tm.endedJob(t, "bob", j.ID)
		alive := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb)
		tops := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL AND trash_root", nb)
		if ended.State == "succeeded" {
			if alive != 0 || tops != 211 {
				t.Errorf("the third unit first: %d nodes alive, %d trash items; want none, the import's 211", alive, tops)
			}
			continue // the third unit first
		}
		if ended.State != "failed" || ended.Report == nil || ended.Report.Failure == nil || *ended.Report.Failure != "changeset_reverted" ||
			ended.Report.Counts["pages"] != 199 || ended.Report.Counts["attachments"] != 1 {
			t.Errorf("the import = %+v, report %s; want failed changeset_reverted after its second unit", ended, describe(ended.Report))
		}
		if alive != 0 || tops != 200 || count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND name LIKE 'r%'", nb) != 0 {
			t.Errorf("%d nodes alive, %d trash items; want none, the two units' 200, and none of the third's", alive, tops)
		}
		checkImports(t, tm)
		checkNotebooks(t, tm.pool)
		return
	}
	t.Fatalf("the revert came after the import's third unit, %d times of %d", betweenUnits, betweenUnits)
}

// Interleaving 6: two reverts of one changeset. The second finds it
// reverted under the lock, its node changed: page.revert_conflict; one
// revert's changeset.
func TestTwoRevertsOfAChangeset(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	x := tm.createPage(t, "alice", nb, "", "A")
	tm.send(t, nodeRename("alice", x, "B"), http.StatusOK)
	c := tm.newest(t, nb).ID
	a, b := tm.interleaveOn(t, notebookRow(nb), changesetRevert("alice", c), changesetRevert("bob", c))
	if a.status != http.StatusOK {
		t.Errorf("the first revert = %d %s, want 200", a.status, a.body)
	}
	conflicted(t, "the second revert", b, x, "changed")
	if n := tm.reverts(t, nb); n != 1 {
		t.Errorf("%d reverts' changesets, want 1", n)
	}
	tm.checkHistory(t)
}

// Interleaving 7: a revert of a deletion and a restore of its item. The
// restore first: the revert finds D alive, changed. The revert first: the
// restore reads its item again under the lock, its root no longer in the
// trash, page.trash_changed (M8/P2 design 3.5, step 1).
func TestRevertingADeletionAndRestoringItsItem(t *testing.T) {
	orders(t, "revert", "restore", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		d := tm.createPage(t, "alice", nb, "", "D")
		tm.send(t, nodeDeletion("alice", d), http.StatusNoContent)
		c := tm.newest(t, nb).ID
		steps := map[string]step{"revert": changesetRevert("bob", c), "restore": nodeRestore("alice", d)}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		revert, restore := got["revert"], got["restore"]
		switch first {
		case "restore":
			conflicted(t, "the revert after the restore", revert, d, "changed")
			if restore.status != http.StatusOK {
				t.Errorf("the restore = %d %s, want 200", restore.status, restore.body)
			}
		case "revert":
			if revert.status != http.StatusOK || !restore.is(http.StatusConflict, "page.trash_changed") {
				t.Errorf("the revert = %d %s, the restore = %d %s; want 200, then 409 page.trash_changed", revert.status, revert.body,
					restore.status, restore.code)
			}
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", d); n != 1 {
			t.Errorf("D is not back once")
		}
		tm.checkHistory(t)
	})
}

// Interleaving 8: a revert of a deletion and the deletion of the parent it
// goes back under. The revert first: D goes back under P, then goes with
// it, at its time, a node of P's item, not a root. The deletion first:
// D's parent is gone, parent_gone.
func TestRevertingADeletionAndDeletingItsParent(t *testing.T) {
	orders(t, "revert", "deletion", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPage(t, "alice", nb, "", "P")
		d := tm.createPage(t, "alice", nb, p, "D")
		tm.send(t, nodeDeletion("alice", d), http.StatusNoContent)
		c := tm.newest(t, nb).ID
		steps := map[string]step{"revert": changesetRevert("bob", c), "deletion": nodeDeletion("alice", p)}
		x, y := tm.interleaveOn(t, notebookRow(nb), steps[first], steps[second])
		got := map[string]answer{first: x, second: y}
		revert, deletion := got["revert"], got["deletion"]
		if deletion.status != http.StatusNoContent {
			t.Fatalf("the deletion = %d %s, want 204", deletion.status, deletion.body)
		}
		switch first {
		case "revert":
			if revert.status != http.StatusOK || count(t, tm.pool, `SELECT count(*) FROM nodes d JOIN nodes p ON p.id = d.parent_id
				WHERE d.id = $1 AND p.id = $2 AND d.deleted_at = p.deleted_at AND NOT d.trash_root AND p.trash_pages = 2`, d, p) != 1 {
				t.Errorf("the revert = %d %s; want D back under P, then deleted with it", revert.status, revert.body)
			}
		case "deletion":
			conflicted(t, "the revert after the deletion", revert, d, "parent_gone")
		}
		tm.checkHistory(t)
	})
}

// Interleaving 9: a revert of an edit session's own changeset (M8 design
// 4.8, a changeset being written). While alice's session holds P, bob's
// revert of C, its writes, is page.locked by her. Her session expired,
// the revert puts P's bytes back; her old tab's save is
// page.edit_session_ended, and her new session writes a changeset of its
// own. A session that wrote P's bytes back changed nothing: the revert
// answers changed false and writes nothing, and the session's next save
// still merges into C, which nothing reverted.
func TestRevertingAnEditSessionsChangeset(t *testing.T) {
	session := func(t *testing.T, tm acmeTeam, nb, p string, contents ...string) (string, string) {
		t.Helper()
		s := tm.openSession(t, "alice", p)
		rev := tm.content(t, "alice", p).Revision
		for _, c := range contents {
			tm.send(t, contentWrite("alice", p, c, rev, s), http.StatusOK)
			rev++
		}
		return s, tm.newest(t, nb).ID
	}
	t.Run("its session alive, then expired", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPageWith(t, "alice", nb, "", "P", "before")
		s, c := session(t, tm, nb, p, "during")
		r := changesetRevert("bob", c)
		status, body := ask(t, tm.contract, r.method, tm.base+r.path, tm.tokens["bob"], "")
		if a := (answer{status: status, body: body, code: problemCode(t, body)}); !a.is(http.StatusConflict, "page.locked") ||
			lockHolder(a) != "alice" {
			t.Errorf("the revert while alice edits = %d %s, want 409 page.locked by alice", status, body)
		}
		if _, err := tm.pool.Exec(context.Background(), "UPDATE edit_sessions SET created_at = now() - interval '2 minutes', "+
			"expires_at = now() - interval '1 minute' WHERE id = $1", s); err != nil {
			t.Fatal(err)
		}
		tm.reverted(t, "bob", c)
		got := tm.content(t, "alice", p)
		if got.Content != "before" {
			t.Errorf("P = %q, want before", got.Content)
		}
		late := contentWrite("alice", p, "late", got.Revision, s)
		if status, body := ask(t, tm.contract, late.method, tm.base+late.path, tm.tokens["alice"], late.body); status != http.StatusConflict ||
			problemCode(t, body) != "page.edit_session_ended" {
			t.Errorf("the old tab's save = %d %s, want 409 page.edit_session_ended", status, body)
		}
		session(t, tm, nb, p, "anew")
		if l, _ := tm.history(t, "alice", nb, "limit=2"); len(l) != 2 || l[0].Kind != "edit" || l[0].ID == c || l[1].Kind != "revert" {
			t.Errorf("the log = %+v, want the new session's changeset, then the revert", l)
		}
		tm.checkHistory(t)
	})
	t.Run("its session net zero", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		p := tm.createPageWith(t, "alice", nb, "", "P", "before")
		s, c := session(t, tm, nb, p, "during", "before")
		r := changesetRevert("bob", c)
		status, body := ask(t, tm.contract, r.method, tm.base+r.path, tm.tokens["bob"], "")
		var got revertAnswer
		decodeAnswer(t, body, &got)
		if status != http.StatusOK || got.Changed || got.ChangesetID != nil || tm.reverts(t, nb) != 0 {
			t.Errorf("the revert of a net-zero session = %d %s, want 200, changed false, nothing written", status, body)
		}
		rev := tm.content(t, "alice", p).Revision
		tm.send(t, contentWrite("alice", p, "after", rev, s), http.StatusOK)
		if cs := tm.changeset(t, "alice", c); tm.newest(t, nb).ID != c || len(cs.RevertedBy) != 0 {
			t.Errorf("the save after the revert went elsewhere than C, or C is reverted: %+v", cs)
		}
		tm.checkHistory(t)
	})
}

// Interleaving 10: a revert of a deletion while the purge is midway
// through its group (as TestAGroupThePurgeIsMidwayThrough). While the
// revert waits for the notebook, a purge whose clock is ahead takes the
// versions alone, leaving C's items and C unmarked: under the lock the
// revert cannot read the hashes of its pages' versions, Missing,
// page.changeset_trimmed, and writes nothing. A purge of the attachments'
// rows alone: the restore of the group counts fewer rows than it brings
// back, page.trash_changed in the unit, page.changeset_trimmed here,
// rolled back. These are the whole program's paths to the two.
func TestRevertingADeletionThePurgeIsMidwayThrough(t *testing.T) {
	for _, table := range []string{"page_revisions", "asset_blobs"} {
		t.Run(table, func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			r := tm.createPageWith(t, "alice", nb, "", "R", "# R")
			tm.upload(t, "alice", nb, r, "a.txt", "file")
			tm.send(t, nodeDeletion("alice", r), http.StatusNoContent)
			c := tm.newest(t, nb).ID
			holder := tm.holdNotebook(t, nb)
			got := tm.waitingFor(t, "notebooks", changesetRevert("bob", c), func() {
				if err := tm.purgeBefore(t, time.Now().Add(2*time.Hour), table)(); err != nil {
					t.Errorf("the purge: %v", err)
				}
				commit(t, holder)
			})
			if !got.is(http.StatusConflict, "page.changeset_trimmed") {
				t.Errorf("the revert = %d %s, want 409 page.changeset_trimmed", got.status, got.body)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL", nb) +
				tm.reverts(t, nb) + count(t, tm.pool, "SELECT count(*) FROM changesets WHERE id = $1 AND trimmed", c); n != 0 {
				t.Errorf("%d nodes back, reverts, or C marked; want none", n)
			}
			tm.checkHistory(t)
		})
	}
}
