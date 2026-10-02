package bootstrap

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The interleavings of the pages' lock protocol (M4/P1 design 3.13),
// harnessed as the workspace's (interleavings_workspace_test.go): a page
// write shares its workspace's row, then locks its notebook's; a tree's
// write locks it FOR NO KEY UPDATE, as the notebook's deletion does. The
// test holds the row both wait for, and they run in the order they came.
// Each ends by checking the pages' invariant and the notebooks'.

// checkPages fails t when the pages break an invariant (M4 design 4): a
// page not deleted under a deleted parent; two siblings not deleted with
// one title key, which the unique index refuses too, checked in case it
// changes; a page not deleted without exactly one content not deleted, or
// whose content's revision is not its latest version's; a page deeper than
// ten levels, or on a chain that loops.
func checkPages(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for what, query := range map[string]string{
		"under a deleted parent": `SELECT count(*) FROM nodes c JOIN nodes p ON p.id = c.parent_id
			WHERE c.deleted_at IS NULL AND p.deleted_at IS NOT NULL`,
		"with a sibling's title key": `SELECT count(*) FROM (SELECT 1 FROM nodes WHERE deleted_at IS NULL
			GROUP BY notebook_id, parent_id, name_key HAVING count(*) > 1) twins`,
		"without exactly one content": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NULL AND n.kind = 'page'
			AND (SELECT count(*) FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL) <> 1`,
		"whose content is not its latest version": `SELECT count(*) FROM page_contents c WHERE c.deleted_at IS NULL
			AND c.revision IS DISTINCT FROM (SELECT max(r.revision) FROM page_revisions r WHERE r.node_id = c.node_id)`,
		"deeper than ten levels, or in a loop": `WITH RECURSIVE up AS (
				SELECT id AS start, parent_id, 1 AS depth FROM nodes WHERE deleted_at IS NULL
				UNION ALL
				SELECT u.start, p.parent_id, u.depth + 1 FROM up u JOIN nodes p ON p.id = u.parent_id WHERE u.depth < 11
			) SELECT count(DISTINCT start) FROM up WHERE parent_id IS NOT NULL AND depth >= 10`,
	} {
		if n := count(t, pool, query); n != 0 {
			t.Errorf("%d pages %s, want none", n, what)
		}
	}
}

// openNotebook creates a notebook named name in acme as by, open to acme's
// members to edit, and returns its id.
func (tm acmeTeam) openNotebook(t *testing.T, by, name string) string {
	t.Helper()
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[by],
		`{"name":"`+name+`","workspace_access":"editor"}`)
	if status != http.StatusCreated {
		t.Fatalf("create %s as %s = %d %s", name, by, status, body)
	}
	return idOf(t, answer{body: body})
}

// createPage creates a page titled title in the notebook id as by, under
// the page parent, or at the root when it is "", checks the invariant, and
// returns its id.
func (tm acmeTeam) createPage(t *testing.T, by, notebookID, parent, title string) string {
	t.Helper()
	parentID := "null"
	if parent != "" {
		parentID = `"` + parent + `"`
	}
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+notebookID+"/pages", tm.tokens[by],
		`{"parent_id":`+parentID+`,"title":"`+title+`"}`)
	if status != http.StatusCreated {
		t.Fatalf("create %s as %s = %d %s", title, by, status, body)
	}
	checkPages(t, tm.pool)
	return idOf(t, answer{body: body})
}

// pageCreation is by's creation of a page titled title at the root of the
// notebook id.
func pageCreation(by, notebookID, title string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+notebookID+"/pages", `{"parent_id":null,"title":"`+title+`"}`)
}

// deletedWithNotebook counts the pages of the notebook id deleted at its
// time, with their contents.
func (tm acmeTeam) deletedWithNotebook(t *testing.T, notebookID string) (pages, contents int) {
	t.Helper()
	pages = count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
		WHERE n.id = $1 AND x.deleted_at = n.deleted_at`, notebookID)
	contents = count(t, tm.pool, `SELECT count(*) FROM page_contents c JOIN nodes x ON x.id = c.node_id JOIN notebooks n ON n.id = x.notebook_id
		WHERE n.id = $1 AND c.deleted_at = n.deleted_at`, notebookID)
	return pages, contents
}

// Interleaving 30: deleting a notebook and creating or renaming a page of
// it. The deletion first: the write finds the notebook deleted once it has
// its row: 404, nothing written. The write first: the deletion takes its
// page with the notebook, at the notebook's time.
func TestDeletingANotebookAndWritingAPage(t *testing.T) {
	for _, tt := range []struct {
		name     string
		write    func(nb, page string) step
		notFound string
		written  int // the pages the write leaves when first
	}{
		{"creating", func(nb, _ string) step { return pageCreation("bob", nb, "Late") }, "notebook.not_found", 2},
		{"renaming", func(_, page string) step {
			return request("bob", http.MethodPatch, "/api/v0/nodes/"+page, `{"name":"Renamed"}`)
		}, "page.not_found", 1},
	} {
		t.Run(tt.name+", the deletion first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			page := tm.createPage(t, "alice", nb, "", "Notes")
			deleted, written := tm.interleaveOn(t, notebookRow(nb), request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""),
				tt.write(nb, page))
			if !deleted.is(http.StatusNoContent, "") || !written.is(http.StatusNotFound, tt.notFound) {
				t.Errorf("DELETE the notebook = %d, then %s = %d %s; want 204, then 404 %s", deleted.status, tt.name, written.status,
					written.code, tt.notFound)
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE name <> 'Notes'"); n != 0 {
				t.Errorf("%d pages written after the deletion, want none", n)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(tt.name+", the write first", func(t *testing.T) {
			tm := newAcmeTeam(t, "member", "")
			nb := tm.openNotebook(t, "alice", "Eng")
			page := tm.createPage(t, "alice", nb, "", "Notes")
			written, deleted := tm.interleaveOn(t, notebookRow(nb), tt.write(nb, page),
				request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, ""))
			if written.status/100 != 2 || !deleted.is(http.StatusNoContent, "") {
				t.Errorf("%s = %d %s, then DELETE the notebook = %d; want 2xx, then 204", tt.name, written.status, written.code, deleted.status)
			}
			if pages, contents := tm.deletedWithNotebook(t, nb); pages != tt.written || contents != tt.written {
				t.Errorf("%d pages and %d contents deleted with the notebook, want %d of each", pages, contents, tt.written)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 31: deleting a workspace and creating a page in it. The
// deletion first: the creation finds the workspace deleted once it shares
// its row: 404, no page. The creation first: the deletion takes the page
// with its notebook, at the workspace's time.
func TestDeletingAWorkspaceAndCreatingAPage(t *testing.T) {
	deletion := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		deleted, created := tm.interleave(t, deletion, pageCreation("bob", nb, "Late"))
		if !deleted.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("DELETE acme = %d, then POST a page = %d %s; want 204, then 404 notebook.not_found", deleted.status, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes"); n != 0 {
			t.Errorf("%d pages, want none", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		created, deleted := tm.interleave(t, pageCreation("bob", nb, "Late"), deletion)
		if !created.is(http.StatusCreated, "") || !deleted.is(http.StatusNoContent, "") {
			t.Errorf("POST a page = %d %s, then DELETE acme = %d; want 201, then 204", created.status, created.code, deleted.status)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
			JOIN workspaces w ON w.id = n.workspace_id WHERE x.deleted_at = w.deleted_at`); n != 1 {
			t.Errorf("%d pages deleted with acme, want the new one", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// sharedNotebookRow is the row of notebook id as a page write that keeps
// the tree holds it, FOR SHARE: a step waits for it only if it locks the
// tree.
func sharedNotebookRow(id string) held {
	return held{"notebooks", "SELECT 1 FROM notebooks WHERE id = '" + id + "' FOR SHARE"}
}

// Interleaving 32: two pages of one title created at once under the same
// parent. The test holds the notebook's row FOR SHARE, so each creation
// waits for it only because it locks the tree; the second finds the
// first's title under that lock: 409, and the unique index is never
// reached.
func TestCreatingTwoPagesOfOneTitle(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), pageCreation(first, nb, "Same"), pageCreation(second, nb, "SAME"))
		if !a.is(http.StatusCreated, "") || !b.is(http.StatusConflict, "page.title_taken") {
			t.Errorf("%s created: %d %s; %s created: %d %s; want 201, then 409 page.title_taken", first, a.status, a.code, second, b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 33: bob creates a page while alice removes him from acme.
// The removal first: the creation decides under acme's lock that he sees
// the notebook no more: 404, no page. The creation first: the page stays,
// and he sees it no more.
func TestCreatingAPageAndRemovingTheWriterFromTheWorkspace(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		removed, created := tm.interleave(t, tm.removal("alice", "bob"), pageCreation("bob", nb, "Late"))
		if !removed.is(http.StatusNoContent, "") || !created.is(http.StatusNotFound, "notebook.not_found") {
			t.Errorf("remove bob = %d, then his POST a page = %d %s; want 204, then 404 notebook.not_found", removed.status, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes"); n != 0 {
			t.Errorf("%d pages, want none", n)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		created, removed := tm.interleave(t, pageCreation("bob", nb, "Late"), tm.removal("alice", "bob"))
		if !created.is(http.StatusCreated, "") || !removed.is(http.StatusNoContent, "") {
			t.Errorf("his POST a page = %d %s, then remove bob = %d; want 201, then 204", created.status, created.code, removed.status)
		}
		page := idOf(t, created)
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", page); n != 1 {
			t.Errorf("bob's page is gone, want it kept")
		}
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+page, tm.tokens["bob"], ""); status != http.StatusNotFound {
			t.Errorf("bob reads his page after his removal = %d %s, want 404", status, answer)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}
