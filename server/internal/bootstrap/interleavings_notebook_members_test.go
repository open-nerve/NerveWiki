package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
)

// The interleavings of the notebook members' writes (M3/P2 design 3.9):
// both sides share acme's row, which does not order them, and lock the
// notebook's, so the test holds the notebook's row and the two run one
// after the other, in the order they came. Each ends by checking the
// notebooks' invariant, now on notebooks that outlive it.

// notebookRow is the row of notebook id, which every member write of it
// locks.
func notebookRow(id string) held {
	return held{"notebooks", "SELECT 1 FROM notebooks WHERE id = '" + id + "' FOR NO KEY UPDATE"}
}

// userID is the id of name's account.
func (tm acmeTeam) userID(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := tm.pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", name+"@example.com").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// addNotebookMember makes name a member of notebook id with role, as by,
// through the API, and returns the membership's id.
func (tm acmeTeam) addNotebookMember(t *testing.T, by, id, name, role string) string {
	t.Helper()
	status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+id+"/members", tm.tokens[by],
		`{"user_id":"`+tm.userID(t, name).String()+`","role":"`+role+`"}`)
	var m struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(answer), &m); status != http.StatusCreated || err != nil {
		t.Fatalf("add %s to %s as %s = %d %s", name, id, by, status, answer)
	}
	return m.ID
}

// twoAdmins is acme with bob a member, and a notebook of alice's whose
// admins are alice and bob: its id, and the two memberships' ids.
func twoAdmins(t *testing.T) (tm acmeTeam, id, alices, bobs string) {
	t.Helper()
	tm = newAcmeTeam(t, "member", "")
	id = tm.createNotebook(t, "alice", "Notes")
	bobs = tm.addNotebookMember(t, "alice", id, "bob", "admin")
	alices = selectText(t, tm, "SELECT id::text FROM notebook_members WHERE notebook_id = $1 AND user_id = $2", id, tm.userID(t, "alice"))
	return tm, id, alices, bobs
}

// selectText is the one text the query reads.
func selectText(t *testing.T, tm acmeTeam, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := tm.pool.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return s
}

// selectTexts are the texts the query reads, in its order.
func selectTexts(t *testing.T, tm acmeTeam, sql string, args ...any) []string {
	t.Helper()
	rows, err := tm.pool.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return texts
}

// adminsOf counts the notebook's active admins.
func adminsOf(t *testing.T, tm acmeTeam, id string) int {
	t.Helper()
	return count(t, tm.pool, "SELECT count(*) FROM notebook_members WHERE notebook_id = '"+id+
		"' AND role = 'admin' AND ended_at IS NULL AND deleted_at IS NULL")
}

// Interleaving 17: two admins of a notebook make each other a reader. The
// first to have the notebook's row does; the second, deciding under the
// lock, is a reader by then: 403. The notebook keeps an admin.
func TestTwoNotebookAdminsDemotingEachOther(t *testing.T) {
	for _, aliceFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "alice first", false: "bob first"}[aliceFirst], func(t *testing.T) {
			tm, id, alices, bobs := twoAdmins(t)
			byAlice := request("alice", http.MethodPatch, "/api/v0/notebook-members/"+bobs, `{"role":"reader"}`)
			byBob := request("bob", http.MethodPatch, "/api/v0/notebook-members/"+alices, `{"role":"reader"}`)
			first, second := byAlice, byBob
			if !aliceFirst {
				first, second = byBob, byAlice
			}

			done, refused := tm.interleaveOn(t, notebookRow(id), first, second)

			if done.status != http.StatusOK || refused.status != http.StatusForbidden || refused.code != "forbidden" {
				t.Errorf("the first = %d %s, the second = %d %s; want 200, then 403 forbidden", done.status, done.code,
					refused.status, refused.code)
			}
			if n := adminsOf(t, tm, id); n != 1 {
				t.Errorf("%d admins, want the first one's", n)
			}
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 18: the two admins of a notebook leave at once. The first
// to have the notebook's row leaves; the second counts the admins under
// the lock, finds itself alone: 409 notebook.sole_admin.
func TestTwoNotebookAdminsLeaving(t *testing.T) {
	for _, aliceFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "alice first", false: "bob first"}[aliceFirst], func(t *testing.T) {
			tm, id, _, _ := twoAdmins(t)
			first, second := request("alice", http.MethodPost, "/api/v0/notebooks/"+id+"/leave", ""),
				request("bob", http.MethodPost, "/api/v0/notebooks/"+id+"/leave", "")
			if !aliceFirst {
				first, second = second, first
			}

			left, refused := tm.interleaveOn(t, notebookRow(id), first, second)

			if left.status != http.StatusNoContent || refused.status != http.StatusConflict || refused.code != "notebook.sole_admin" {
				t.Errorf("the first = %d %s, the second = %d %s; want 204, then 409 notebook.sole_admin", left.status, left.code,
					refused.status, refused.code)
			}
			if n := adminsOf(t, tm, id); n != 1 {
				t.Errorf("%d admins, want the second one", n)
			}
			checkNotebooks(t, tm.pool)
		})
	}
}

// Interleaving 19: adding a member to a notebook and deleting it. The
// deletion first: the addition finds no notebook under its lock, 404, and
// writes no row. The addition first: the deletion takes the new row with
// the notebook, at its time.
func TestAddingANotebookMemberAndDeletingTheNotebook(t *testing.T) {
	newRow := "SELECT count(*) FROM notebook_members m JOIN users u ON u.id = m.user_id WHERE u.email = 'carol@example.com'"

	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "", "member")
		id := tm.createNotebook(t, "alice", "Notes")
		addition := request("alice", http.MethodPost, "/api/v0/notebooks/"+id+"/members",
			`{"user_id":"`+tm.userID(t, "carol").String()+`","role":"reader"}`)

		deleted, added := tm.interleaveOn(t, notebookRow(id), request("alice", http.MethodDelete, "/api/v0/notebooks/"+id, ""), addition)

		if deleted.status != http.StatusNoContent || added.status != http.StatusNotFound || added.code != "notebook.not_found" {
			t.Errorf("DELETE the notebook = %d, then add carol = %d %s; want 204, then 404 notebook.not_found",
				deleted.status, added.status, added.code)
		}
		if n := count(t, tm.pool, newRow); n != 0 {
			t.Errorf("%d rows of carol's, want none", n)
		}
		checkNotebooks(t, tm.pool)
	})

	t.Run("the addition first", func(t *testing.T) {
		tm := newAcmeTeam(t, "", "member")
		id := tm.createNotebook(t, "alice", "Notes")
		addition := request("alice", http.MethodPost, "/api/v0/notebooks/"+id+"/members",
			`{"user_id":"`+tm.userID(t, "carol").String()+`","role":"reader"}`)

		added, deleted := tm.interleaveOn(t, notebookRow(id), addition, request("alice", http.MethodDelete, "/api/v0/notebooks/"+id, ""))

		if added.status != http.StatusCreated || deleted.status != http.StatusNoContent {
			t.Errorf("add carol = %d %s, then DELETE the notebook = %d; want 201, then 204", added.status, added.code, deleted.status)
		}
		if n := count(t, tm.pool, `SELECT count(*) FROM notebook_members m JOIN users u ON u.id = m.user_id
			JOIN notebooks n ON n.id = m.notebook_id WHERE u.email = 'carol@example.com' AND m.deleted_at = n.deleted_at`); n != 1 {
			t.Errorf("%d rows of carol's deleted with the notebook, want hers", n)
		}
		checkNotebooks(t, tm.pool)
	})
}
