package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// The edit lock's last hop (M5/P1 design 5): pageRegistrants(pool) hands
// the lock to serve as the openings' vetoer and the writes' guard, and
// each path reaches it through the whole program. With bob's session
// holding Notes: another opening, alice's or his own, is page.locked by
// bob, a take-over passes his own lock alone; a content write without his
// session and alice's deletion are page.locked; his save in it, a write of
// the content Notes holds, a page created under it, with a content or
// without, its rename and its move pass. Once Notes is under Beside,
// alice's deletion of Beside is page.locked at Notes, and his own passes,
// and ends his session. Each answer is checked against the contract, the
// lock and ended_by members among them.
func TestTheEditLockReachesEveryPath(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	beside := tm.createPage(t, "alice", nb, "", "Beside")
	id := tm.createPage(t, "alice", nb, "", "Notes")
	session := tm.openSession(t, "bob", id)
	locked := func(by string, c step) {
		t.Helper()
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.body)
		if status != http.StatusConflict || !strings.Contains(body, `"code":"page.locked"`) || lockHolder(answer{body: body}) != "bob" ||
			!strings.Contains(body, `"page_id":"`+id+`"`) {
			t.Errorf("%s as %s = %d %s, want 409 page.locked at Notes by bob", c.name(), by, status, body)
		}
	}
	locked("alice", sessionOpening("alice", id))
	locked("alice", sessionTakeOver("alice", id))
	locked("bob", sessionOpening("bob", id))
	locked("alice", contentWrite("alice", id, "# Mine", 1, ""))
	locked("alice", nodeDeletion("alice", id))
	tm.send(t, contentWrite("bob", id, "# His", 1, session), http.StatusOK)
	tm.send(t, contentWrite("alice", id, "# His", 1, ""), http.StatusOK)
	tm.send(t, childCreation("alice", nb, id, "Under"), http.StatusCreated)
	tm.send(t, request("alice", http.MethodPost, "/api/v0/notebooks/"+nb+"/pages",
		`{"parent_id":"`+id+`","title":"Written","content":"# New"}`), http.StatusCreated)
	tm.send(t, nodeRename("alice", id, "Renamed"), http.StatusOK)
	tm.send(t, nodeMove("alice", id, beside), http.StatusOK)

	taken := tm.openSessionWith(t, "bob", sessionTakeOver("bob", id))
	if reason, _ := tm.endOf(t, session); reason != "taken_over" {
		t.Errorf("bob's first session ended %q, want taken_over", reason)
	}
	tm.send(t, unlock("alice", id), http.StatusNoContent)
	c := contentWrite("bob", id, "# Again", 2, taken)
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens["bob"], c.body)
	if status != http.StatusConflict || !strings.Contains(body, `"code":"page.edit_session_unlocked"`) || endedBy(answer{body: body}) != "alice" {
		t.Errorf("bob's save after the unlock = %d %s, want 409 page.edit_session_unlocked by alice", status, body)
	}
	again := tm.openSession(t, "bob", id)
	locked("alice", nodeDeletion("alice", beside))
	tm.send(t, nodeDeletion("bob", beside), http.StatusNoContent)
	if n := count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE id = $1", again); n != 0 {
		t.Error("bob's deletion kept his session")
	}
	checkPages(t, tm.pool)
}

// openSessionWith opens by's edit session through c and returns its id.
func (tm acmeTeam) openSessionWith(t *testing.T, by string, c step) string {
	t.Helper()
	status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.body)
	if status != http.StatusCreated {
		t.Fatalf("%s as %s = %d %s", c.name(), by, status, body)
	}
	return idOf(t, answer{body: body})
}
