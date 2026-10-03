package bootstrap

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The interleavings of the pages' lock protocol (M4/P1 design 3.13; M4/P2 design 3.8),
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
// ten levels, or on a chain that loops; a deleted page with a content, a
// version or an item not deleted, which would keep the purge from it; an
// edit session of a page that is not one not deleted of its notebook
// (M4/P4 design 3.2); two sessions of a page alive at the database's time,
// which the edit lock refuses (M5/P1 design 3.8).
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
		"deleted, with what follows it not deleted": `SELECT count(*) FROM nodes n WHERE n.deleted_at IS NOT NULL AND (
			EXISTS (SELECT 1 FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM page_revisions r WHERE r.node_id = n.id AND r.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM changeset_items i WHERE i.node_id = n.id AND i.deleted_at IS NULL))`,
		"deeper than ten levels, or in a loop": `WITH RECURSIVE up AS (
				SELECT id AS start, parent_id, 1 AS depth FROM nodes WHERE deleted_at IS NULL
				UNION ALL
				SELECT u.start, p.parent_id, u.depth + 1 FROM up u JOIN nodes p ON p.id = u.parent_id WHERE u.depth < 11
			) SELECT count(DISTINCT start) FROM up WHERE parent_id IS NOT NULL AND depth >= 10`,
		"with an edit session, deleted or in another notebook": `SELECT count(*) FROM edit_sessions s
			WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = s.node_id AND n.notebook_id = s.notebook_id
				AND n.kind = 'page' AND n.deleted_at IS NULL)`,
		"with two alive edit sessions": `SELECT count(*) FROM (SELECT 1 FROM edit_sessions
			WHERE ended_reason IS NULL AND expires_at > now() GROUP BY node_id HAVING count(*) > 1) locked`,
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

// The tree's writes as the interleavings send them: by's move of the node
// id under parent ("" for the root), last there; its rename; its deletion;
// a creation under parent.
func nodeMove(by, id, parent string) step {
	parentID := "null"
	if parent != "" {
		parentID = `"` + parent + `"`
	}
	return request(by, http.MethodPost, "/api/v0/nodes/"+id+"/move", `{"parent_id":`+parentID+`}`)
}

func nodeRename(by, id, name string) step {
	return request(by, http.MethodPatch, "/api/v0/nodes/"+id, `{"name":"`+name+`"}`)
}

func nodeDeletion(by, id string) step { return request(by, http.MethodDelete, "/api/v0/nodes/"+id, "") }

func childCreation(by, notebookID, parent, title string) step {
	return request(by, http.MethodPost, "/api/v0/notebooks/"+notebookID+"/pages", `{"parent_id":"`+parent+`","title":"`+title+`"}`)
}

// Interleaving 34: A moved under B while B is moved under A. Each alone is
// no cycle; the second finds the first's move under the notebook's lock:
// 409 page.cycle, and the tree has no loop.
func TestMovingTwoPagesUnderEachOther(t *testing.T) {
	orders(t, "A", "B", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		ids := map[string]string{"A": tm.createPage(t, "alice", nb, "", "A"), "B": tm.createPage(t, "alice", nb, "", "B")}
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), nodeMove("alice", ids[first], ids[second]),
			nodeMove("bob", ids[second], ids[first]))
		if !a.is(http.StatusOK, "") || !b.is(http.StatusConflict, "page.cycle") {
			t.Errorf("%s under %s = %d %s, then %s under %s = %d %s; want 200, then 409 page.cycle", first, second, a.status, a.code,
				second, first, b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 35: a move that takes a subtree deeper while a page is
// created at its bottom. Each alone stays within ten levels, both would
// not: the second finds the first's under the notebook's lock, 409
// page.too_deep.
func TestMovingASubtreeDeeperAndCreatingAtItsBottom(t *testing.T) {
	setup := func(t *testing.T) (tm acmeTeam, nb, eighth, top, bottom string) {
		tm = newAcmeTeam(t, "member", "")
		nb = tm.openNotebook(t, "alice", "Eng")
		for i := range domain.MaxDepth - 2 {
			eighth = tm.createPage(t, "alice", nb, eighth, "Level "+string(rune('A'+i)))
		}
		top = tm.createPage(t, "alice", nb, "", "Top")
		bottom = tm.createPage(t, "alice", nb, top, "Bottom")
		return tm, nb, eighth, top, bottom
	}
	t.Run("the move first", func(t *testing.T) {
		tm, nb, eighth, top, bottom := setup(t)
		moved, created := tm.interleaveOn(t, sharedNotebookRow(nb), nodeMove("alice", top, eighth), childCreation("bob", nb, bottom, "Deep"))
		if !moved.is(http.StatusOK, "") || !created.is(http.StatusConflict, "page.too_deep") {
			t.Errorf("the move = %d %s, then the creation = %d %s; want 200, then 409 page.too_deep", moved.status, moved.code,
				created.status, created.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
	t.Run("the creation first", func(t *testing.T) {
		tm, nb, eighth, top, bottom := setup(t)
		created, moved := tm.interleaveOn(t, sharedNotebookRow(nb), childCreation("bob", nb, bottom, "Deep"), nodeMove("alice", top, eighth))
		if !created.is(http.StatusCreated, "") || !moved.is(http.StatusConflict, "page.too_deep") {
			t.Errorf("the creation = %d %s, then the move = %d %s; want 201, then 409 page.too_deep", created.status, created.code,
				moved.status, moved.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 36: deleting a subtree while a page is created in it or one
// of its pages renamed. The deletion first: the parent is gone, 422
// parent_id; the page is gone, 404. The write first: what it wrote goes
// with the subtree, at the deletion's time.
func TestDeletingASubtreeAndWritingInIt(t *testing.T) {
	for _, tt := range []struct {
		name    string
		write   func(nb, child string) step
		refused [2]string // status and code when the deletion is first
		deleted int       // the pages the deletion takes when the write is first
	}{
		{"creating", func(nb, child string) step { return childCreation("bob", nb, child, "Late") },
			[2]string{"422", "validation_failed"}, 3},
		{"renaming", func(_, child string) step { return nodeRename("bob", child, "Renamed") }, [2]string{"404", "page.not_found"}, 2},
	} {
		setup := func(t *testing.T) (tm acmeTeam, nb, top, child string) {
			tm = newAcmeTeam(t, "member", "")
			nb = tm.openNotebook(t, "alice", "Eng")
			top = tm.createPage(t, "alice", nb, "", "Top")
			return tm, nb, top, tm.createPage(t, "alice", nb, top, "Child")
		}
		t.Run(tt.name+", the deletion first", func(t *testing.T) {
			tm, nb, top, child := setup(t)
			deleted, written := tm.interleaveOn(t, sharedNotebookRow(nb), nodeDeletion("alice", top), tt.write(nb, child))
			if !deleted.is(http.StatusNoContent, "") || strconv.Itoa(written.status) != tt.refused[0] || written.code != tt.refused[1] {
				t.Errorf("the deletion = %d, then %s = %d %s; want 204, then %s %s", deleted.status, tt.name, written.status, written.code,
					tt.refused[0], tt.refused[1])
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE name NOT IN ('Top', 'Child')"); n != 0 {
				t.Errorf("%d pages written after the deletion, want none", n)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
		t.Run(tt.name+", the write first", func(t *testing.T) {
			tm, nb, top, child := setup(t)
			written, deleted := tm.interleaveOn(t, sharedNotebookRow(nb), tt.write(nb, child), nodeDeletion("alice", top))
			if written.status/100 != 2 || !deleted.is(http.StatusNoContent, "") {
				t.Errorf("%s = %d %s, then the deletion = %d; want 2xx, then 204", tt.name, written.status, written.code, deleted.status)
			}
			if n := count(t, tm.pool, `SELECT count(*) FROM nodes x JOIN nodes t ON t.id = $1
				WHERE x.deleted_at = t.deleted_at AND x.deleted_at IS NOT NULL`, top); n != tt.deleted {
				t.Errorf("%d pages deleted with Top at its time, want %d", n, tt.deleted)
			}
			checkPages(t, tm.pool)
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 37: a page renamed to a title while another page of that
// title is moved under the same parent. Each alone is no clash; the
// second finds the first's under the notebook's lock: 409
// page.title_taken.
func TestRenamingAndMovingIntoOneTitle(t *testing.T) {
	orders(t, "rename", "move", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "member", "")
		nb := tm.openNotebook(t, "alice", "Eng")
		parent := tm.createPage(t, "alice", nb, "", "Parent")
		keep := tm.createPage(t, "alice", nb, parent, "Keep")
		merge := tm.createPage(t, "alice", nb, "", "Merge")
		steps := map[string]step{"rename": nodeRename("alice", keep, "MERGE"), "move": nodeMove("bob", merge, parent)}
		a, b := tm.interleaveOn(t, sharedNotebookRow(nb), steps[first], steps[second])
		if a.status != http.StatusOK || !b.is(http.StatusConflict, "page.title_taken") {
			t.Errorf("the %s = %d %s, then the %s = %d %s; want 200, then 409 page.title_taken", first, a.status, a.code, second,
				b.status, b.code)
		}
		checkPages(t, tm.pool)
		checkNotebooks(t, tm.pool)
	})
}
