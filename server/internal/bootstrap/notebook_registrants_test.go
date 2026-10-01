package bootstrap

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The notebook module's part in a workspace's deletion reaches it through
// serve (M2 handoff to M3, item 1; v0.1 design 13.1, item 21): the
// composition check proves only that serve reaches the registrants; this
// proves they are handed over. The other paths of the membership's end and
// restore come with their registrants (M3/P3).
func TestDeletingAWorkspaceDeletesItsNotebooks(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	var ids []string
	for _, name := range []string{"alice", "bob"} {
		status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[name],
			`{"name":"Notes of `+name+`"}`)
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(answer), &created); status != http.StatusCreated || err != nil {
			t.Fatalf("create %s's notebook = %d %s", name, status, answer)
		}
		ids = append(ids, created.ID)
	}

	if status, answer := ask(t, tm.contract, http.MethodDelete, tm.base+"/api/v0/workspaces/acme", tm.tokens["alice"], ""); status != http.StatusNoContent {
		t.Fatalf("DELETE acme = %d %s", status, answer)
	}

	// The notebooks and their members, deleted with acme's time.
	if n := count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN workspaces w ON w.id = n.workspace_id
		WHERE w.slug = 'acme' AND n.deleted_at = w.deleted_at AND n.updated_by_id = w.updated_by_id`); n != 2 {
		t.Errorf("%d of acme's notebooks deleted with it, want both", n)
	}
	if n := count(t, tm.pool, `SELECT count(*) FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id
		JOIN workspaces w ON w.id = n.workspace_id WHERE w.slug = 'acme' AND m.deleted_at = w.deleted_at`); n != 2 {
		t.Errorf("%d of the notebooks' members deleted with acme, want both admins", n)
	}
	for _, id := range ids {
		if status, answer := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/notebooks/"+id, tm.tokens["alice"], ""); status != http.StatusNotFound ||
			problemCode(t, answer) != "notebook.not_found" {
			t.Errorf("GET a notebook of the deleted acme = %d %s, want 404 notebook.not_found", status, answer)
		}
	}
}
