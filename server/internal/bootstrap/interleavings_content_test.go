package bootstrap

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// The interleavings of a page's content and its edit sessions (M4/P4
// design 3.11), harnessed as the pages' (interleavings_page_test.go): a
// content's write shares its workspace's row and its notebook's, then
// locks the page's content row, its gate, FOR NO KEY UPDATE; an edit
// session's opening passes the same gate; a write in a session locks the
// session's row after it. A tree's write and a notebook's change lock the
// notebook's row, which the test holds where one of those meets a save.
// Each ends by checking the pages' invariant and the notebooks'.

// contentRow is the page id's content row, the gate of its content's
// writes and of its sessions' openings.
func contentRow(id string) held {
	return held{"page_contents", "SELECT 1 FROM page_contents WHERE node_id = '" + id + "' FOR NO KEY UPDATE"}
}

// sessionRow is the edit session id's row, which a save in it holds FOR
// UPDATE.
func sessionRow(id string) held {
	return held{"edit_sessions", "SELECT 1 FROM edit_sessions WHERE id = '" + id + "' FOR UPDATE"}
}

// revisions counts the page id's versions, and those deleted at its time.
func (tm acmeTeam) revisions(t *testing.T, id string) (all, deletedWithIt int) {
	t.Helper()
	all = count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id = $1", id)
	deletedWithIt = count(t, tm.pool, `SELECT count(*) FROM page_revisions r JOIN nodes n ON n.id = r.node_id
		WHERE n.id = $1 AND r.deleted_at = n.deleted_at`, id)
	return all, deletedWithIt
}

// sessionsOf counts the page id's edit sessions.
func (tm acmeTeam) sessionsOf(t *testing.T, id string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE node_id = $1", id)
}

// Interleaving 38: two saves on the same base revision. Both share the
// notebook's row and wait at the page's gate; the second finds the
// first's revision: 409 page.revision_mismatch.
func TestTwoSavesOnOneBase(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		id := tm.createPage(t, "alice", nb, "", "Notes")
		a, b := tm.interleaveOn(t, contentRow(id), contentWrite(first, id, "# "+first, 1, ""), contentWrite(second, id, "# "+second, 1, ""))
		if !a.is(http.StatusOK, "") || !b.is(http.StatusConflict, "page.revision_mismatch") {
			t.Errorf("%s saved: %d %s; %s saved: %d %s; want 200, then 409 page.revision_mismatch", first, a.status, a.code,
				second, b.status, b.code)
		}
		if got := tm.content(t, "alice", id); got.Content != "# "+first || got.Revision != 2 {
			t.Errorf("the content is %+v, want %s's at revision 2", got, first)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 39: a save while its page is deleted. Both wait for the
// notebook's row, the save to share it, the deletion to lock it. The
// deletion first: the save finds the page deleted, 404 page.not_found. The
// save first: its version goes with the page, at the deletion's time.
func TestSavingAndDeletingAPage(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, nb, id string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		return tm, nb, tm.createPage(t, "alice", nb, "", "Notes")
	}
	t.Run("the deletion first", func(t *testing.T) {
		tm, nb, id := setup(t)
		deleted, saved := tm.interleaveOn(t, notebookRow(nb), nodeDeletion("alice", id), contentWrite("bob", id, "# Late", 1, ""))
		if !deleted.is(http.StatusNoContent, "") || !saved.is(http.StatusNotFound, "page.not_found") {
			t.Errorf("the deletion = %d, then the save = %d %s; want 204, then 404 page.not_found", deleted.status, saved.status, saved.code)
		}
		if all, _ := tm.revisions(t, id); all != 1 {
			t.Errorf("%d versions, want the first alone", all)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the save first", func(t *testing.T) {
		tm, nb, id := setup(t)
		saved, deleted := tm.interleaveOn(t, notebookRow(nb), contentWrite("bob", id, "# Late", 1, ""), nodeDeletion("alice", id))
		if !saved.is(http.StatusOK, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("the save = %d %s, then the deletion = %d; want 200, then 204", saved.status, saved.code, deleted.status)
		}
		if all, deletedWithIt := tm.revisions(t, id); all != 2 || deletedWithIt != 2 {
			t.Errorf("%d versions, %d deleted at the page's time; want both", all, deletedWithIt)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 40: a save in an edit session while its notebook, or its
// workspace, is deleted: the test holds the row the deletion locks, which
// the save shares. The deletion first: 404 page.not_found. The save first:
// the page goes at the notebook's time, its new version with it. Either
// way, the session goes with the page.
func TestSavingAndDeletingTheNotebook(t *testing.T) {
	for _, tt := range []struct {
		name     string
		held     func(nb string) held
		deletion func(nb string) step
	}{
		{"its notebook", notebookRow, func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") }},
		{"its workspace", func(string) held { return acmeRow() },
			func(string) step { return request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "") }},
	} {
		setup := func(t *testing.T) (tm acmeTeam, nb, id, session string) {
			tm = newAcmeTeam(t, "member", "")
			nb = tm.openNotebook(t, "alice", "Eng")
			id = tm.createPage(t, "alice", nb, "", "Notes")
			return tm, nb, id, tm.openSession(t, "bob", id)
		}
		t.Run(tt.name+", the deletion first", func(t *testing.T) {
			tm, nb, id, session := setup(t)
			deleted, saved := tm.interleaveOn(t, tt.held(nb), tt.deletion(nb), contentWrite("bob", id, "# Late", 1, session))
			if !deleted.is(http.StatusNoContent, "") || !saved.is(http.StatusNotFound, "page.not_found") {
				t.Errorf("the deletion = %d, then the save = %d %s; want 204, then 404 page.not_found", deleted.status, saved.status, saved.code)
			}
			if all, _ := tm.revisions(t, id); all != 1 || tm.sessionsOf(t, id) != 0 {
				t.Errorf("%d versions and %d sessions, want the first version and no session", all, tm.sessionsOf(t, id))
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(tt.name+", the save first", func(t *testing.T) {
			tm, nb, id, session := setup(t)
			saved, deleted := tm.interleaveOn(t, tt.held(nb), contentWrite("bob", id, "# Late", 1, session), tt.deletion(nb))
			if !saved.is(http.StatusOK, "") || !deleted.is(http.StatusNoContent, "") {
				t.Errorf("the save = %d %s, then the deletion = %d; want 200, then 204", saved.status, saved.code, deleted.status)
			}
			pages, contents := tm.deletedWithNotebook(t, nb)
			if _, deletedWithIt := tm.revisions(t, id); pages != 1 || contents != 1 || deletedWithIt != 2 || tm.sessionsOf(t, id) != 0 {
				t.Errorf("%d pages, %d contents and %d versions deleted with the notebook, %d sessions; want 1, 1, 2 and none",
					pages, contents, deletedWithIt, tm.sessionsOf(t, id))
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 41: bob, who writes only by the workspace's default role,
// saves while alice closes the notebook to the workspace; both wait for
// the notebook's row. The change first: bob sees the page no more, 404
// page.not_found. The save first: 200, and he reads it no more.
func TestSavingAndClosingTheNotebook(t *testing.T) {
	closing := func(nb string) step {
		return request("alice", http.MethodPatch, "/api/v0/notebooks/"+nb, `{"workspace_access":"none"}`)
	}
	setup := func(t *testing.T) (tm acmeTeam, nb, id string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		return tm, nb, tm.createPage(t, "alice", nb, "", "Notes")
	}
	t.Run("the change first", func(t *testing.T) {
		tm, nb, id := setup(t)
		changed, saved := tm.interleaveOn(t, notebookRow(nb), closing(nb), contentWrite("bob", id, "# Late", 1, ""))
		if !changed.is(http.StatusOK, "") || !saved.is(http.StatusNotFound, "page.not_found") {
			t.Errorf("the change = %d %s, then bob's save = %d %s; want 200, then 404 page.not_found", changed.status, changed.code,
				saved.status, saved.code)
		}
		if got := tm.content(t, "alice", id); got.Revision != 1 {
			t.Errorf("the content is at revision %d, want 1", got.Revision)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the save first", func(t *testing.T) {
		tm, nb, id := setup(t)
		saved, changed := tm.interleaveOn(t, notebookRow(nb), contentWrite("bob", id, "# Late", 1, ""), closing(nb))
		if !saved.is(http.StatusOK, "") || !changed.is(http.StatusOK, "") {
			t.Errorf("bob's save = %d %s, then the change = %d %s; want 200, then 200", saved.status, saved.code, changed.status, changed.code)
		}
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/content", tm.tokens["bob"], ""); status != http.StatusNotFound {
			t.Errorf("bob reads the content after the change = %d %s, want 404", status, answer)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 42: two openings of a session of one page wait at its
// gate, and both open: M4 does not make them exclusive, and both went
// through the gate, where M5's vetoer decides. An opening while the page
// is deleted, both waiting for the notebook's row: the deletion first, 404
// page.not_found; the opening first, its session goes with the page.
func TestOpeningSessionsOfAPage(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		id := tm.createPage(t, "alice", nb, "", "Notes")
		a, b := tm.interleaveOn(t, contentRow(id), sessionOpening(first, id), sessionOpening(second, id))
		if !a.is(http.StatusCreated, "") || !b.is(http.StatusCreated, "") || tm.sessionsOf(t, id) != 2 {
			t.Errorf("%s opened: %d %s; %s opened: %d %s; %d sessions; want 201 twice, two sessions", first, a.status, a.code,
				second, b.status, b.code, tm.sessionsOf(t, id))
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	setup := func(t *testing.T) (tm acmeTeam, nb, id string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		return tm, nb, tm.createPage(t, "alice", nb, "", "Notes")
	}
	t.Run("the deletion first", func(t *testing.T) {
		tm, nb, id := setup(t)
		deleted, opened := tm.interleaveOn(t, notebookRow(nb), nodeDeletion("alice", id), sessionOpening("bob", id))
		if !deleted.is(http.StatusNoContent, "") || !opened.is(http.StatusNotFound, "page.not_found") || tm.sessionsOf(t, id) != 0 {
			t.Errorf("the deletion = %d, then the opening = %d %s, %d sessions; want 204, then 404 page.not_found, none", deleted.status,
				opened.status, opened.code, tm.sessionsOf(t, id))
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the opening first", func(t *testing.T) {
		tm, nb, id := setup(t)
		opened, deleted := tm.interleaveOn(t, notebookRow(nb), sessionOpening("bob", id), nodeDeletion("alice", id))
		if !opened.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") || tm.sessionsOf(t, id) != 0 {
			t.Errorf("the opening = %d %s, then the deletion = %d, %d sessions; want 201, then 204, none", opened.status, opened.code,
				deleted.status, tm.sessionsOf(t, id))
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 43: the expired sessions' cleanup while a session's row is
// held, as a save in it holds it. The cleanup skips the row and runs on,
// leaving it; a run after its release deletes it. A heartbeat of it is then
// 404 page.edit_session_not_found, a save in it 409
// page.edit_session_ended. The cleanup is the job serve runs, here every
// second; the session expires by SQL rather than by a minute's wait.
func TestCleaningUpAHeldSession(t *testing.T) {
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) { c.Page.EditSessionCleanupInterval = time.Second })
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPage(t, "alice", nb, "", "Notes")
	session := tm.openSession(t, "bob", id)
	ctx := context.Background()
	if _, err := tm.pool.Exec(ctx, "UPDATE edit_sessions SET created_at = now() - interval '2 minutes', expires_at = now() - interval '1 minute' "+
		"WHERE id = $1", session); err != nil {
		t.Fatal(err)
	}
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, sessionRow(session).lock); err != nil {
		t.Fatal(err)
	}
	const runs = "SELECT count(*) FROM river_job WHERE kind = 'page.cleanup_expired_edit_sessions' AND state = 'completed'"
	held := count(t, tm.pool, runs)
	for deadline := time.Now().Add(interleavingWait); count(t, tm.pool, runs) < held+2; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no two runs of the cleanup completed within %s of holding the session", interleavingWait)
		}
	}
	if tm.sessionsOf(t, id) != 1 {
		t.Error("the cleanup deleted the session it found held")
	}
	if err := holder.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(interleavingWait); tm.sessionsOf(t, id) != 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the expired session is still there %s after its release", interleavingWait)
		}
	}
	for _, c := range []struct {
		step step
		want cell
	}{
		{request("bob", http.MethodPost, "/api/v0/edit-sessions/"+session+"/heartbeat", ""), cell{http.StatusNotFound, "page.edit_session_not_found"}},
		{contentWrite("bob", id, "# Late", 1, session), cell{http.StatusConflict, "page.edit_session_ended"}},
	} {
		status, answer := ask(t, tm.contract, c.step.method, tm.base+c.step.path, tm.tokens["bob"], c.step.body)
		if status != c.want.status || problemCode(t, answer) != c.want.code {
			t.Errorf("%s after the cleanup = %d %s, want %s", c.step.name(), status, answer, c.want)
		}
	}
	checkPages(t, tm.pool)
	checkNotebooks(t, tm.pool)
}
