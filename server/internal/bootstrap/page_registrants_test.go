package bootstrap

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// The page module's part in a notebook's deletion reaches it on each of
// the deletion's three paths through serve (M4/P1 design 3.9; v0.1 design
// 13.1, item 21): a notebook's deletion by its admin, its workspace's
// deletion, and an ownerless notebook's deletion by the workspace's admin.
// Each takes the notebook's tree of three levels, what follows its pages
// and its changesets at the notebook's time, the pages' edit sessions
// (M4/P4 design 3.7) and their index (M6/P3 design 3.5); the pages read
// 404 after.
func TestDeletingANotebookDeletesItsPages(t *testing.T) {
	for _, tt := range []struct {
		name string
		// setup is a team with a notebook by writer.
		setup func(t *testing.T) (tm acmeTeam, nb, writer string)
		// before runs after the pages are written, before the deletion.
		before   func(t *testing.T, tm acmeTeam)
		deletion func(nb string) step
	}{
		{
			name: "by its admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "member", "")
				return tm, tm.createNotebook(t, "alice", "Eng"), "alice"
			},
			deletion: func(nb string) step { return request("alice", http.MethodDelete, "/api/v0/notebooks/"+nb, "") },
		},
		{
			name: "with its workspace",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "member", "")
				return tm, tm.createNotebook(t, "bob", "Eng"), "bob"
			},
			deletion: func(string) step { return request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "") },
		},
		{
			name: "ownerless, by the workspace's admin",
			setup: func(t *testing.T) (acmeTeam, string, string) {
				tm := newAcmeTeam(t, "admin", "member")
				return tm, tm.createNotebook(t, "carol", "Plans"), "carol"
			},
			before:   func(t *testing.T, tm acmeTeam) { tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent) },
			deletion: func(nb string) step { return ownerlessDeletion("alice", nb) },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tm, nb, writer := tt.setup(t)
			root := tm.createPage(t, writer, nb, "", "Root")
			child := tm.createPageWith(t, writer, nb, root, "Child", "---\naliases: [C]\ntags: [t]\n---\n[[Root]]")
			pages := []string{root, child, tm.createPage(t, writer, nb, child, "Grandchild")}
			for _, page := range pages {
				tm.openSession(t, writer, page)
			}
			if tt.before != nil {
				tt.before(t, tm)
			}
			indexed := func() int {
				return count(t, tm.pool, `SELECT (SELECT count(*) FROM indexed_pages WHERE notebook_id = $1)
					+ (SELECT count(*) FROM page_links WHERE notebook_id = $1) + (SELECT count(*) FROM page_tags WHERE notebook_id = $1)
					+ (SELECT count(*) FROM page_properties WHERE notebook_id = $1) + (SELECT count(*) FROM page_aliases WHERE notebook_id = $1)`, nb)
			}
			if n := indexed(); n != 3+1+1+2+1 {
				t.Fatalf("the notebook's index holds %d rows, want the three pages', the child's link, tag, properties and alias", n)
			}

			tm.send(t, tt.deletion(nb), http.StatusNoContent)

			if n := indexed(); n != 0 {
				t.Errorf("the deleted notebook's index holds %d rows, want none", n)
			}

			for table, join := range map[string]string{
				"nodes":           "nodes x",
				"page_contents":   "page_contents x JOIN nodes p ON p.id = x.node_id",
				"page_revisions":  "page_revisions x JOIN nodes p ON p.id = x.node_id",
				"changeset_items": "changeset_items x JOIN nodes p ON p.id = x.node_id",
				"changesets":      "changesets x",
			} {
				notebookOfRow := "x.notebook_id"
				if table != "nodes" && table != "changesets" {
					notebookOfRow = "p.notebook_id"
				}
				if n := count(t, tm.pool, "SELECT count(*) FROM "+join+" JOIN notebooks n ON n.id = "+notebookOfRow+
					" WHERE n.id = $1 AND n.deleted_at IS NOT NULL AND x.deleted_at = n.deleted_at", nb); n != 3 {
					t.Errorf("%d rows of %s deleted at the notebook's time, want the three pages'", n, table)
				}
			}
			if n := count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE notebook_id = $1", nb); n != 0 {
				t.Errorf("%d edit sessions of the deleted notebook's pages, want none", n)
			}
			for _, page := range pages {
				if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+page, tm.tokens["alice"], ""); status != http.StatusNotFound ||
					problemCode(t, answer) != "page.not_found" {
					t.Errorf("GET a page of the deleted notebook = %d %s, want 404 page.not_found", status, answer)
				}
			}
			checkPages(t, tm.pool)
		})
	}
}

// The pages' activity reaches the ownerless list through serve (M4/P4
// design 3.10): an ownerless notebook's size is its pages' bytes, and its
// last activity the latest write, which a session's second save moves on
// in the changeset it keeps.
func TestTheOwnerlessListShowsThePagesActivity(t *testing.T) {
	tm := newAcmeTeam(t, "admin", "member")
	nb := tm.createNotebook(t, "carol", "Plans")
	notes := tm.createPage(t, "carol", nb, "", "Notes")
	tm.createPage(t, "carol", nb, "", "Empty")
	session := tm.openSession(t, "carol", notes)
	written := func() time.Time {
		t.Helper()
		var at time.Time
		if err := tm.pool.QueryRow(context.Background(), `SELECT c.updated_at FROM changesets c JOIN edit_sessions s ON s.changeset_id = c.id
			WHERE s.id = $1`, session).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	tm.send(t, contentWrite("carol", notes, "# Notes\n", 1, session), http.StatusOK)
	first := written()
	content := "# Notes\n\nMore of them.\n"
	tm.send(t, contentWrite("carol", notes, content, 2, session), http.StatusOK)
	second := written()
	tm.send(t, tm.removal("alice", "carol"), http.StatusNoContent)

	status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/workspaces/acme/ownerless-notebooks", tm.tokens["alice"], "")
	var list struct {
		Data []struct {
			ID             string    `json:"id"`
			LastActivityAt time.Time `json:"last_activity_at"`
			SizeBytes      int       `json:"size_bytes"`
		} `json:"data"`
	}
	decodeAnswer(t, answer, &list)
	if status != http.StatusOK || len(list.Data) != 1 || list.Data[0].ID != nb {
		t.Fatalf("the ownerless list = %d %s, want Plans alone", status, answer)
	}
	if got := list.Data[0]; got.SizeBytes != len(content) || !got.LastActivityAt.Equal(second) || !second.After(first) {
		t.Errorf("Plans is listed with %d bytes, last active %s; want %d, at the second save %s, after the first %s",
			got.SizeBytes, got.LastActivityAt, len(content), second, first)
	}
	checkPages(t, tm.pool)
}
