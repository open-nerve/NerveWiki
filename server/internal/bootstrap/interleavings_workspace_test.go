package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The interleavings of the workspace's lock protocol (M2/P2 design 3.9),
// through the wired app, its access module and PostgreSQL, with no seam in
// the use cases: the test holds the workspace's row in a transaction of its
// own, sends A's request and waits until it waits for the row, then B's,
// then commits. PostgreSQL hands a row's lock to its waiters in the order
// they came (the first holds the tuple lock, the next waits behind it), so
// A runs before B, each deciding under the lock what the other committed.
// Each interleaving runs in both orders. Every wait has a deadline.

// interleavingWait bounds every wait of these tests.
const interleavingWait = 10 * time.Second

// acmeTeam is acme, created through the API by alice, its admin, and bob and
// carol, whose memberships are written through SQL (they join by invitation
// from M2/P3 on), on an app of its own.
type acmeTeam struct {
	base     string
	pool     *pgxpool.Pool
	contract *apitest.Contract
	tokens   map[string]string    // by name
	members  map[string]uuid.UUID // the memberships, by name
}

func newAcmeTeam(t *testing.T, bobRole, carolRole string) acmeTeam {
	t.Helper()
	url := pgtest.NewDatabase(t)
	tm := acmeTeam{
		base: startApp(t, testConfig(t, url, false), migrations.FS()), pool: connect(t, url), contract: apitest.Load(t),
		tokens: map[string]string{}, members: map[string]uuid.UUID{},
	}
	for _, name := range []string{"alice", "bob", "carol"} {
		tm.tokens[name] = registerAccount(t, tm.contract, tm.base, name+"@example.com").AccessToken
	}
	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces", tm.tokens["alice"],
		`{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("create acme = %d %s", status, answer)
	}
	ctx := context.Background()
	for name, role := range map[string]string{"bob": bobRole, "carol": carolRole} {
		if _, err := tm.pool.Exec(ctx, `INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
			SELECT $1, w.id, u.id, $3, w.created_by_id, w.created_by_id, now(), now()
			FROM workspaces w, users u WHERE w.slug = 'acme' AND u.email = $2`, uuid.NewV7(), name+"@example.com", role); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := tm.pool.Query(ctx, `SELECT split_part(u.email, '@', 1), m.id FROM workspace_members m JOIN users u ON u.id = m.user_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var id uuid.UUID
		if err := rows.Scan(&name, &id); err != nil {
			t.Fatal(err)
		}
		tm.members[name] = id
	}
	return tm
}

// step is a request of an interleaving: who sends what.
type step struct {
	by, method, path, body string
}

// answer is what a step got.
type answer struct {
	req    *http.Request
	res    *http.Response
	status int
	code   string
}

// interleave holds acme's row while it sends first, then second, each once
// the one before waits for the row; then lets them run, and returns their
// answers, checked against the contract.
func (tm acmeTeam) interleave(t *testing.T, first, second step) (answer, answer) {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, "SELECT 1 FROM workspaces WHERE slug = 'acme' FOR NO KEY UPDATE"); err != nil {
		t.Fatal(err)
	}
	answers := make([]chan answer, 2)
	for i, c := range []step{first, second} {
		var body []byte
		if c.body != "" {
			body = []byte(c.body)
		}
		req := newRequest(t, c.method, tm.base+c.path, tm.tokens[c.by], body)
		answers[i] = make(chan answer, 1)
		go func() {
			res, err := client().Do(req)
			got := answer{req: req, res: res}
			if err == nil {
				payload, _ := io.ReadAll(res.Body)
				_ = res.Body.Close()
				res.Body = io.NopCloser(bytes.NewReader(payload))
				got.status = res.StatusCode
				var p struct {
					Code string `json:"code"`
				}
				if json.Unmarshal(payload, &p) == nil {
					got.code = p.Code
				}
			}
			answers[i] <- got
		}()
		pgtest.WaitForLockWaitsOn(t, tm.pool, "workspaces", i+1, interleavingWait)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var got [2]answer
	for i := range got {
		select {
		case got[i] = <-answers[i]:
		case <-time.After(interleavingWait):
			t.Fatalf("%s %s did not answer", []step{first, second}[i].method, []step{first, second}[i].path)
		}
		if got[i].res == nil {
			t.Fatalf("%s %s failed", got[i].req.Method, got[i].req.URL.Path)
		}
		tm.contract.CheckResponse(t, got[i].req, got[i].res)
	}
	return got[0], got[1]
}

// activeAdmins counts acme's active admins.
func (tm acmeTeam) activeAdmins(t *testing.T) int {
	t.Helper()
	return count(t, tm.pool, `SELECT count(*) FROM workspace_members m JOIN workspaces w ON w.id = m.workspace_id
		WHERE w.slug = 'acme' AND m.role = 'admin' AND m.ended_at IS NULL AND m.deleted_at IS NULL`)
}

func (a answer) is(status int, code string) bool { return a.status == status && a.code == code }

// orders runs f with the two callers in each order: the one that goes
// first, and the other.
func orders(t *testing.T, a, b string, f func(t *testing.T, first, second string)) {
	t.Helper()
	for _, o := range [][2]string{{a, b}, {b, a}} {
		t.Run(o[0]+" first", func(t *testing.T) { f(t, o[0], o[1]) })
	}
}

// Two admins leave at once: the second counts the first gone and is the
// only admin left (interleaving 1).
func TestTwoAdminsLeavingAtOnceLeaveOne(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "admin", "member")
		leave := func(by string) step { return step{by, http.MethodPost, "/api/v0/workspaces/acme/leave", ""} }

		a, b := tm.interleave(t, leave(first), leave(second))

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusConflict, "workspace.sole_admin") {
			t.Errorf("%s left: %d %s; %s left: %d %s; want 204, then 409 workspace.sole_admin", first, a.status, a.code, second, b.status, b.code)
		}
		if n := tm.activeAdmins(t); n != 1 {
			t.Errorf("acme has %d active admins, want 1", n)
		}
	})
}

// Two admins demote each other at once: the second is a member when it
// decides (interleaving 2).
func TestTwoAdminsDemotingEachOtherLeaveOne(t *testing.T) {
	orders(t, "alice", "bob", func(t *testing.T, first, second string) {
		tm := newAcmeTeam(t, "admin", "member")
		demote := func(by, whom string) step {
			return step{by, http.MethodPatch, "/api/v0/workspace-members/" + tm.members[whom].String(), `{"role":"member"}`}
		}

		a, b := tm.interleave(t, demote(first, second), demote(second, first))

		if !a.is(http.StatusOK, "") || !b.is(http.StatusForbidden, "forbidden") {
			t.Errorf("%s demoted %s: %d %s; the other way: %d %s; want 200, then 403 forbidden", first, second, a.status, a.code, b.status, b.code)
		}
		if n := tm.activeAdmins(t); n != 1 {
			t.Errorf("acme has %d active admins, want 1", n)
		}
	})
}

// Alice removes bob, an admin, while bob deletes acme: whichever commits
// first, the other finds nothing to act on (interleaving 3).
func TestRemovingAnAdminWhileTheyDeleteTheWorkspace(t *testing.T) {
	t.Run("the removal first", func(t *testing.T) {
		tm := newAcmeTeam(t, "admin", "member")
		remove := step{"alice", http.MethodDelete, "/api/v0/workspace-members/" + tm.members["bob"].String(), ""}
		del := step{"bob", http.MethodDelete, "/api/v0/workspaces/acme", ""}

		a, b := tm.interleave(t, remove, del)

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.not_found") {
			t.Errorf("removal: %d %s; deletion: %d %s; want 204, then 404 workspace.not_found", a.status, a.code, b.status, b.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM workspaces WHERE slug = 'acme' AND deleted_at IS NULL"); n != 1 {
			t.Errorf("acme deleted, want it kept")
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM workspace_members WHERE id = $1 AND ended_at IS NOT NULL", tm.members["bob"]); n != 1 {
			t.Errorf("bob's membership not ended, want it ended")
		}
	})
	t.Run("the deletion first", func(t *testing.T) {
		tm := newAcmeTeam(t, "admin", "member")
		remove := step{"alice", http.MethodDelete, "/api/v0/workspace-members/" + tm.members["bob"].String(), ""}
		del := step{"bob", http.MethodDelete, "/api/v0/workspaces/acme", ""}

		a, b := tm.interleave(t, del, remove)

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.member_not_found") {
			t.Errorf("deletion: %d %s; removal: %d %s; want 204, then 404 workspace.member_not_found", a.status, a.code, b.status, b.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM workspace_members WHERE deleted_at IS NULL OR ended_at IS NOT NULL"); n != 0 {
			t.Errorf("%d memberships not deleted, or ended, want every one deleted and none ended", n)
		}
	})
}
