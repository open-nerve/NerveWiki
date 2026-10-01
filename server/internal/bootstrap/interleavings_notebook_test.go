package bootstrap

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The interleavings of the notebooks' lock protocol (M3/P1 design 3.10),
// harnessed as the workspace's (interleavings_workspace_test.go): a
// notebook's management write shares its workspace's row, and the
// workspace's deletion holds it, so the test holds acme's row and the two
// run one after the other, in the order they came. Each ends by checking
// the notebooks' invariant.

// checkNotebooks fails t when a notebook not deleted has an active admin
// and is ownerless, or neither (M3 design 4): exactly one holds, always.
func checkNotebooks(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if n := count(t, pool, `SELECT count(*) FROM notebooks n WHERE n.deleted_at IS NULL
		AND (n.ownerless_since IS NOT NULL) = EXISTS (SELECT 1 FROM notebook_members m
			WHERE m.notebook_id = n.id AND m.role = 'admin' AND m.ended_at IS NULL AND m.deleted_at IS NULL)`); n != 0 {
		t.Errorf("%d notebooks with an admin and ownerless, or neither; want none", n)
	}
}

// createNotebook creates a notebook named name in acme as by, through the
// API, checks the invariant on it, and returns its id.
func (tm acmeTeam) createNotebook(t *testing.T, by, name string) string {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces/acme/notebooks", tm.tokens[by], `{"name":"`+name+`"}`)
	var n struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(answer), &n); status != http.StatusCreated || err != nil {
		t.Fatalf("create %s as %s = %d %s", name, by, status, answer)
	}
	checkNotebooks(t, tm.pool)
	return n.ID
}

// deletedWithAcme counts acme's notebooks deleted at acme's time, with
// their member rows; and those deleted before it.
func (tm acmeTeam) deletedWithAcme(t *testing.T) (notebooks, members, before int) {
	t.Helper()
	notebooks = count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN workspaces w ON w.id = n.workspace_id
		WHERE w.slug = 'acme' AND n.deleted_at = w.deleted_at`)
	members = count(t, tm.pool, `SELECT count(*) FROM notebook_members m JOIN notebooks n ON n.id = m.notebook_id
		JOIN workspaces w ON w.id = n.workspace_id WHERE w.slug = 'acme' AND m.deleted_at = w.deleted_at`)
	before = count(t, tm.pool, `SELECT count(*) FROM notebooks n JOIN workspaces w ON w.id = n.workspace_id
		WHERE w.slug = 'acme' AND n.deleted_at < w.deleted_at`)
	return notebooks, members, before
}

// Interleaving 15: deleting a workspace and creating a notebook in it. The
// deletion first: the creation finds the workspace deleted once it has its
// row, and answers 404, with no notebook. The creation first: the deletion
// takes the new notebook and its member with the workspace.
func TestDeletingAWorkspaceAndCreatingANotebook(t *testing.T) {
	deletion := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
	creation := request("bob", http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"Late"}`)

	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		deleted, created := tm.interleave(t, deletion, creation)
		if deleted.status != http.StatusNoContent || created.status != http.StatusNotFound || created.code != "workspace.not_found" {
			t.Errorf("DELETE acme = %d, then POST a notebook = %d %s; want 204, then 404 workspace.not_found",
				deleted.status, created.status, created.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM notebooks"); n != 0 {
			t.Errorf("%d notebooks, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the creation first", func(t *testing.T) {
		tm := newAcmeTeam(t, "member", "")
		created, deleted := tm.interleave(t, creation, deletion)
		if created.status != http.StatusCreated || deleted.status != http.StatusNoContent {
			t.Errorf("POST a notebook = %d %s, then DELETE acme = %d; want 201, then 204", created.status, created.code, deleted.status)
		}
		if notebooks, members, _ := tm.deletedWithAcme(t); notebooks != 1 || members != 1 {
			t.Errorf("%d notebooks and %d members deleted with acme, want the new one and its admin", notebooks, members)
		}
		checkNotebooks(t, tm.pool)
	})
}

// Interleaving 16: deleting a workspace and changing or deleting a notebook
// of it. The workspace's deletion first: the notebook's write finds its
// workspace deleted once it has the row, and answers 404; the notebook went
// with the workspace, unchanged. The notebook's write first: the deletion
// takes the changed notebook with the workspace, or leaves the deleted one
// at its own time.
func TestDeletingAWorkspaceAndWritingANotebook(t *testing.T) {
	deletion := request("alice", http.MethodDelete, "/api/v0/workspaces/acme", "")
	for _, tt := range []struct {
		name, method, body string
		status             int
		// before is whether the notebook's write first leaves it deleted
		// before the workspace.
		before bool
	}{
		{"a change", http.MethodPatch, `{"workspace_access":"viewer"}`, http.StatusOK, false},
		{"a deletion", http.MethodDelete, "", http.StatusNoContent, true},
	} {
		t.Run(tt.name+", the workspace's deletion first", func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			id := tm.createNotebook(t, "alice", "Notes")
			write := request("alice", tt.method, "/api/v0/notebooks/"+id, tt.body)
			deleted, written := tm.interleave(t, deletion, write)
			if deleted.status != http.StatusNoContent || written.status != http.StatusNotFound || written.code != "notebook.not_found" {
				t.Errorf("DELETE acme = %d, then %s the notebook = %d %s; want 204, then 404 notebook.not_found",
					deleted.status, tt.method, written.status, written.code)
			}
			if notebooks, members, _ := tm.deletedWithAcme(t); notebooks != 1 || members != 1 ||
				count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE workspace_access = 'none'") != 1 {
				t.Errorf("%d notebooks and %d members deleted with acme; want the notebook unchanged and its admin", notebooks, members)
			}
			checkNotebooks(t, tm.pool)
		})

		t.Run(tt.name+", the notebook's first", func(t *testing.T) {
			tm := newAcmeTeam(t, "", "")
			id := tm.createNotebook(t, "alice", "Notes")
			write := request("alice", tt.method, "/api/v0/notebooks/"+id, tt.body)
			written, deleted := tm.interleave(t, write, deletion)
			if written.status != tt.status || deleted.status != http.StatusNoContent {
				t.Errorf("%s the notebook = %d %s, then DELETE acme = %d; want %d, then 204", tt.method, written.status, written.code,
					deleted.status, tt.status)
			}
			notebooks, members, before := tm.deletedWithAcme(t)
			switch {
			case tt.before && (before != 1 || notebooks != 0):
				t.Errorf("%d notebooks deleted before acme, %d with it; want the notebook at its own time", before, notebooks)
			case !tt.before && (notebooks != 1 || members != 1 || count(t, tm.pool, "SELECT count(*) FROM notebooks WHERE workspace_access = 'viewer'") != 1):
				t.Errorf("%d notebooks and %d members deleted with acme; want the changed notebook and its admin", notebooks, members)
			}
			checkNotebooks(t, tm.pool)
		})
	}
}
