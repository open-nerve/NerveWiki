package bootstrap

import (
	"net/http"
	"testing"
)

// The page module's part in a notebook's deletion reaches it on each of
// the deletion's three paths through serve (M4/P1 design 3.9; v0.1 design
// 13.1, item 21): a notebook's deletion by its admin, its workspace's
// deletion, and an ownerless notebook's deletion by the workspace's admin.
// Each takes the notebook's tree of three levels, what follows its pages
// and its changesets at the notebook's time; the pages read 404 after.
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
			child := tm.createPage(t, writer, nb, root, "Child")
			pages := []string{root, child, tm.createPage(t, writer, nb, child, "Grandchild")}
			if tt.before != nil {
				tt.before(t, tm)
			}

			tm.send(t, tt.deletion(nb), http.StatusNoContent)

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
