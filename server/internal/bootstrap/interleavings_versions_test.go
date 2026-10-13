package bootstrap

import (
	"net/http"
	"testing"
)

// The interleavings of a version's restore (M8/P1 design 3.7), harnessed
// as the contents' (interleavings_content_test.go): a restore is a content
// write, which shares its notebook's row and locks the page's content row;
// its look-ups before the unit are unlocked. Each ends by checking the
// pages' invariant and the notebooks'.

// A restore and a save in bob's edit session, both at the page's content
// row. The save first: the restore finds its revision, 409
// page.revision_mismatch. The restore first: bob's alive session holds the
// page's lock, 409 page.locked by bob; the save then writes.
func TestRestoringAVersionAndSavingInASession(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, id, session string) {
		tm = newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		id = tm.createPageWith(t, "alice", nb, "", "Notes", "one")
		tm.send(t, contentWrite("alice", id, "two", 1, ""), http.StatusOK)
		return tm, id, tm.openSession(t, "bob", id)
	}
	t.Run("the save first", func(t *testing.T) {
		tm, id, session := setup(t)
		saved, restored := tm.interleaveOn(t, contentRow(id), contentWrite("bob", id, "three", 2, session), versionRestore("alice", id, 1, 2))
		if !saved.is(http.StatusOK, "") || !restored.is(http.StatusConflict, "page.revision_mismatch") {
			t.Errorf("the save = %d %s, then the restore = %d %s; want 200, then 409 page.revision_mismatch", saved.status, saved.code,
				restored.status, restored.code)
		}
		if got := tm.content(t, "alice", id); got.Content != "three" || got.Revision != 3 {
			t.Errorf("the content is %+v, want bob's at revision 3", got)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the restore first", func(t *testing.T) {
		tm, id, session := setup(t)
		restored, saved := tm.interleaveOn(t, contentRow(id), versionRestore("alice", id, 1, 2), contentWrite("bob", id, "three", 2, session))
		if !restored.is(http.StatusConflict, "page.locked") || lockHolder(restored) != "bob" || !saved.is(http.StatusOK, "") {
			t.Errorf("the restore = %d %s, then the save = %d %s; want 409 page.locked by bob, then 200", restored.status,
				restored.body, saved.status, saved.code)
		}
		if got := tm.content(t, "alice", id); got.Content != "three" || got.Revision != 3 {
			t.Errorf("the content is %+v, want bob's at revision 3", got)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// A restore while its page is deleted, both at the notebook's row, which
// the restore shares and the deletion locks. The deletion first: the
// restore, which found the page and its version unlocked, finds the page
// gone in its unit, 404 page.not_found. The restore first: its version
// goes with the page, at the deletion's time.
func TestRestoringAVersionAndDeletingThePage(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, nb, id string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		id = tm.createPageWith(t, "alice", nb, "", "Notes", "one")
		tm.send(t, contentWrite("alice", id, "two", 1, ""), http.StatusOK)
		return tm, nb, id
	}
	t.Run("the deletion first", func(t *testing.T) {
		tm, nb, id := setup(t)
		deleted, restored := tm.interleaveOn(t, notebookRow(nb), nodeDeletion("alice", id), versionRestore("bob", id, 1, 2))
		if !deleted.is(http.StatusNoContent, "") || !restored.is(http.StatusNotFound, "page.not_found") {
			t.Errorf("the deletion = %d, then the restore = %d %s; want 204, then 404 page.not_found", deleted.status, restored.status,
				restored.code)
		}
		if all, deletedWithIt := tm.revisions(t, id); all != 2 || deletedWithIt != 2 {
			t.Errorf("%d versions, %d deleted at the page's time; want the two before", all, deletedWithIt)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the restore first", func(t *testing.T) {
		tm, nb, id := setup(t)
		restored, deleted := tm.interleaveOn(t, notebookRow(nb), versionRestore("bob", id, 1, 2), nodeDeletion("alice", id))
		if !restored.is(http.StatusOK, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("the restore = %d %s, then the deletion = %d; want 200, then 204", restored.status, restored.code, deleted.status)
		}
		if all, deletedWithIt := tm.revisions(t, id); all != 3 || deletedWithIt != 3 {
			t.Errorf("%d versions, %d deleted at the page's time; want all three", all, deletedWithIt)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}
