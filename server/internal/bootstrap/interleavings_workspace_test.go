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

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
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
// carol, whose memberships are written through SQL (the matrix's reason,
// M2/P3 design 3.10: what a test aims at is known before it runs), on an
// app of its own; and dana, an account of no workspace.
type acmeTeam struct {
	url      string
	base     string
	pool     *pgxpool.Pool
	contract *apitest.Contract
	tokens   map[string]string    // by name
	members  map[string]uuid.UUID // the memberships, by name
}

// newAcmeTeam is acme with bob and carol of the roles given; "" is none.
func newAcmeTeam(t *testing.T, bobRole, carolRole string) acmeTeam {
	t.Helper()
	return newAcmeTeamWith(t, bobRole, carolRole, nil)
}

// newAcmeTeamWith is newAcmeTeam on an app whose configuration change
// alters, when it is not nil.
func newAcmeTeamWith(t *testing.T, bobRole, carolRole string, change func(*config.Config)) acmeTeam {
	t.Helper()
	url := pgtest.NewDatabase(t)
	cfg := testConfig(t, url, false)
	if change != nil {
		change(&cfg)
	}
	tm := acmeTeam{
		url: url, base: startApp(t, cfg, migrations.FS()), pool: connect(t, url), contract: apitest.Load(t),
		tokens: map[string]string{}, members: map[string]uuid.UUID{},
	}
	for _, name := range []string{"alice", "bob", "carol", "dana"} {
		tm.tokens[name] = registerAccount(t, tm.contract, tm.base, name+"@example.com").AccessToken
	}
	if status, answer := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/workspaces", tm.tokens["alice"],
		`{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("create acme = %d %s", status, answer)
	}
	var alice uuid.UUID
	if err := tm.pool.QueryRow(context.Background(), "SELECT id FROM workspace_members").Scan(&alice); err != nil {
		t.Fatal(err)
	}
	tm.members["alice"] = alice
	for name, role := range map[string]string{"bob": bobRole, "carol": carolRole} {
		if role != "" {
			tm.join(t, name, role)
		}
	}
	return tm
}

// join writes name's membership of acme with role through SQL.
func (tm acmeTeam) join(t *testing.T, name, role string) {
	t.Helper()
	id := uuid.NewV7()
	if _, err := tm.pool.Exec(context.Background(), `INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
		SELECT $1, w.id, u.id, $3, w.created_by_id, w.created_by_id, now(), now()
		FROM workspaces w, users u WHERE w.slug = 'acme' AND u.email = $2`, id, name+"@example.com", role); err != nil {
		t.Fatal(err)
	}
	tm.members[name] = id
}

// step is a request of an interleaving: who sends what; or, when command
// is set, a command run in process.
type step struct {
	by, method, path, body string
	command                func() error
}

// answer is what a step got: a request's answer, or a command's error.
type answer struct {
	req    *http.Request
	res    *http.Response
	status int
	code   string
	body   string
	err    error
}

// held is the row an interleaving's test holds: the statement that locks
// it, in a transaction of the test's own, and the table whose row lock the
// steps wait for.
type held struct{ table, lock string }

// acmeRow is acme's workspace row, which every change of it or of its
// members and invitations locks.
func acmeRow() held {
	return held{"workspaces", "SELECT 1 FROM workspaces WHERE slug = 'acme' FOR NO KEY UPDATE"}
}

// interleave is interleaveOn acme's row.
func (tm acmeTeam) interleave(t *testing.T, first, second step) (answer, answer) {
	t.Helper()
	return tm.interleaveOn(t, acmeRow(), first, second)
}

// interleaveOn holds h's row while it sends first, then second, each once
// the one before waits for the row; then lets them run, and returns their
// answers, a request's checked against the contract.
func (tm acmeTeam) interleaveOn(t *testing.T, h held, first, second step) (answer, answer) {
	t.Helper()
	return tm.interleaveBehind(t, h, first, second, h.table)
}

// interleaveBehind is interleaveOn where second waits for a row of the
// table secondOn rather than for h's: a row first holds while it waits for
// h's.
func (tm acmeTeam) interleaveBehind(t *testing.T, h held, first, second step, secondOn string) (answer, answer) {
	t.Helper()
	ctx := context.Background()
	holder, err := tm.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, h.lock); err != nil {
		t.Fatal(err)
	}
	steps := []step{first, second}
	answers := make([]chan answer, 2)
	for i, c := range steps {
		send := tm.sender(t, c)
		answers[i] = make(chan answer, 1)
		go func() { answers[i] <- send() }()
		table, waiting := h.table, i+1
		if i == 1 && secondOn != h.table {
			table, waiting = secondOn, 1
		}
		pgtest.WaitForLockWaitsOn(t, tm.pool, table, waiting, interleavingWait)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var got [2]answer
	for i := range got {
		select {
		case got[i] = <-answers[i]:
		case <-time.After(interleavingWait):
			t.Fatalf("%s did not answer", steps[i].name())
		}
		if steps[i].command != nil {
			continue
		}
		if got[i].res == nil {
			t.Fatalf("%s failed: %v", steps[i].name(), got[i].err)
		}
		tm.contract.CheckResponse(t, got[i].req, got[i].res)
	}
	return got[0], got[1]
}

// request is by's request.
func request(by, method, path, body string) step {
	return step{by: by, method: method, path: path, body: body}
}

// removal is by's removal of name from acme.
func (tm acmeTeam) removal(by, name string) step {
	return request(by, http.MethodDelete, "/api/v0/workspace-members/"+tm.members[name].String(), "")
}

// send sends c at once, checked against the contract, and fails t unless it
// answers want.
func (tm acmeTeam) send(t *testing.T, c step, want int) {
	t.Helper()
	if status, answer := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[c.by], c.body); status != want {
		t.Fatalf("%s as %s = %d %s, want %d", c.name(), c.by, status, answer, want)
	}
}

func (c step) name() string {
	if c.command != nil {
		return "the command of " + c.by
	}
	return c.method + " " + c.path
}

// sender is what sends c, or runs its command, and returns what it got:
// the request is built here, on the test's goroutine; the sender runs on
// one of its own, and reports rather than fail the test.
func (tm acmeTeam) sender(t *testing.T, c step) func() answer {
	t.Helper()
	if c.command != nil {
		return func() answer { return answer{err: c.command()} }
	}
	var body []byte
	if c.body != "" {
		body = []byte(c.body)
	}
	req := newRequest(t, c.method, tm.base+c.path, tm.tokens[c.by], body)
	return func() answer {
		res, err := client().Do(req)
		got := answer{req: req, res: res, err: err}
		if err == nil {
			payload, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			res.Body = io.NopCloser(bytes.NewReader(payload))
			got.status, got.body = res.StatusCode, string(payload)
			var p struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(payload, &p) == nil {
				got.code = p.Code
			}
		}
		return got
	}
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
		leave := func(by string) step { return request(by, http.MethodPost, "/api/v0/workspaces/acme/leave", "") }

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
			return request(by, http.MethodPatch, "/api/v0/workspace-members/"+tm.members[whom].String(), `{"role":"member"}`)
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
		remove := request("alice", http.MethodDelete, "/api/v0/workspace-members/"+tm.members["bob"].String(), "")
		del := request("bob", http.MethodDelete, "/api/v0/workspaces/acme", "")

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
		remove := request("alice", http.MethodDelete, "/api/v0/workspace-members/"+tm.members["bob"].String(), "")
		del := request("bob", http.MethodDelete, "/api/v0/workspaces/acme", "")

		a, b := tm.interleave(t, del, remove)

		if !a.is(http.StatusNoContent, "") || !b.is(http.StatusNotFound, "workspace.member_not_found") {
			t.Errorf("deletion: %d %s; removal: %d %s; want 204, then 404 workspace.member_not_found", a.status, a.code, b.status, b.code)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM workspace_members WHERE deleted_at IS NULL OR ended_at IS NOT NULL"); n != 0 {
			t.Errorf("%d memberships not deleted, or ended, want every one deleted and none ended", n)
		}
	})
}
