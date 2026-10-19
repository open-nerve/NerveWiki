package bootstrap

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The edit lock's every cell over MCP (M9 design 4.10, 9; M9/P2 design
// 3.13.3), as TestTheEditLockReachesEveryPath has REST's: an agent opens
// no session, and a session in the web, its caller's own too, refuses its
// writes, edits and deletions, each refusal telling the locked pages'
// paths and who holds them.

// lockOf is l as the cells tell it: its page, its path, its holder, and
// whether the caller holds it.
func lockOf(l mcpLock) string {
	path := "no path"
	if l.Path != nil {
		path = *l.Path
	}
	return l.PageID + " " + path + " " + l.DisplayName + " " + strconv.FormatBool(l.You)
}

// With bob's session holding Notes (cells 1–6): alice's write, edit and
// delete are page.locked by bob, the write's and the edit's lock with
// Notes' path, the delete's in its locks; bob's own are page.locked by
// him, told as his, and his session lives on. A write of the content the
// page holds writes nothing on an old version (7); a page created under
// Notes, its rename and its move pass (8, 9). With Notes2 and Late in
// bob's sessions and Child in alice's (10), alice's deletion of Beside is
// page.locked by all three in the subtree's order, one level after
// another (11); once they end (12), it puts the four pages in the trash
// (13). No session is ever an agent's: counted before cell 12, while
// every session the test opened is there, as ending a session and
// deleting its page both remove its row (M9/P2 S1 review C-M1).
func TestTheEditLockReachesEveryMCPWrite(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	beside := tm.createPage(t, "alice", nb, "", "Beside")
	notes := tm.createPageWith(t, "alice", nb, "", "Notes", "Draft\n")
	session := tm.openSession(t, "bob", notes)
	alice, bob := tm.agent(t, "alice", nb), tm.agent(t, "bob", nb)
	token := versionOf(notes, 1)
	for _, c := range []struct {
		cell int
		m    *mcpApp
		tool string
		args map[string]any
		lock string
		says string
	}{
		{1, alice, "write", map[string]any{"path": "Notes.md", "content": "Mine\n", "base_version": token}, notes + " Notes.md bob false",
			`"Notes.md" is being edited by bob in the web editor`},
		{2, alice, "edit", map[string]any{"path": "Notes.md", "old_string": "Draft", "new_string": "Mine"}, notes + " Notes.md bob false",
			`"Notes.md" is being edited by bob in the web editor`},
		{4, bob, "write", map[string]any{"path": "Notes.md", "content": "His\n", "base_version": token}, notes + " Notes.md bob true",
			`"Notes.md" is being edited by you, in a browser`},
		{5, bob, "edit", map[string]any{"path": "Notes.md", "old_string": "Draft", "new_string": "His"}, notes + " Notes.md bob true",
			`"Notes.md" is being edited by you, in a browser`},
	} {
		r := refusalOf(t, c.m.use(t, c.tool, c.args))
		if r.Code != "page.locked" || r.Lock == nil || lockOf(*r.Lock) != c.lock || !strings.Contains(r.Message, c.says) {
			t.Errorf("cell %d: %s = %+v, want page.locked, its lock %s, saying %s", c.cell, c.tool, r, c.lock, c.says)
		}
	}
	for _, c := range []struct {
		cell int
		m    *mcpApp
		lock string
	}{
		{3, alice, notes + " Notes.md bob false"},
		{6, bob, notes + " Notes.md bob true"},
	} {
		r := refusalOf(t, c.m.use(t, "delete", map[string]any{"path": "Notes.md"}))
		if r.Code != "page.locked" || r.LocksCount != 1 || len(r.Locks) != 1 || lockOf(r.Locks[0]) != c.lock || r.Lock == nil ||
			r.Lock.PageID != notes || r.lockPath {
			t.Errorf("cell %d: delete = %+v, want page.locked, its locks %s alone, its lock Notes without a path", c.cell, r, c.lock)
		}
	}
	if reason, there := tm.endOf(t, session); reason != "" || !there {
		t.Errorf("cell 6: bob's session after his agent's delete ended %q (there %v), want alive", reason, there)
	}
	if c := tm.content(t, "alice", notes); c.Content != "Draft\n" || c.Revision != 1 {
		t.Errorf("cells 1–6 wrote Notes: %+v", c)
	}

	tm.send(t, contentWrite("bob", notes, "Saved\n", 1, session), http.StatusOK)
	if w := answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": "Notes.md", "content": "Saved\n", "base_version": token})); w.Changed ||
		w.Version != versionOf(notes, 2) {
		t.Errorf("cell 7: a write of what Notes holds, on its first version = %+v, want nothing written at version 2", w)
	}
	child := answerOf[mcpWritten](t, alice.use(t, "write", map[string]any{"path": "Notes/Child.md", "content": "child\n"}))
	if !child.Changed || child.Path != "Notes/Child.md" {
		t.Errorf("cell 8: a write of a page under Notes = %+v, want it created", child)
	}
	for _, step := range [][2]string{{"Notes.md", "Notes2.md"}, {"Notes2.md", "Beside/Notes2.md"}} {
		if moved := answerOf[mcpMoved](t, alice.use(t, "move", map[string]any{"from": step[0], "to": step[1]})); moved.Path != step[1] ||
			moved.NodeID != notes {
			t.Errorf("cell 9: move of %s to %s = %+v", step[0], step[1], moved)
		}
	}

	late := answerOf[mcpWritten](t, bob.use(t, "write", map[string]any{"path": "Beside/Late.md", "content": "late\n"})).NodeID
	sessions := []string{session, tm.openSession(t, "bob", late), tm.openSession(t, "alice", child.NodeID)}
	r := refusalOf(t, alice.use(t, "delete", map[string]any{"path": "Beside.md", "recursive": true}))
	var locks []string
	for _, l := range r.Locks {
		locks = append(locks, lockOf(l))
	}
	want := []string{notes + " Beside/Notes2.md bob false", late + " Beside/Late.md bob false", child.NodeID + " Beside/Notes2/Child.md alice true"}
	if r.Code != "page.locked" || r.LocksCount != 3 || !slices.Equal(locks, want) || r.Lock == nil || r.Lock.PageID != notes || r.lockPath ||
		!strings.Contains(r.Message, "3 pages being edited in the web editor, by bob and by you, in a browser") {
		t.Errorf("cell 11: delete of Beside = %+v, locks %q; want page.locked by %q, its lock Notes2 without a path", r, locks, want)
	}

	if n := count(t, tm.pool, "SELECT count(*) FROM edit_sessions WHERE client LIKE 'mcp:%'"); n != 0 {
		t.Errorf("%d edit sessions are an agent's, want none", n)
	}
	for i, by := range []string{"bob", "bob", "alice"} {
		tm.send(t, request(by, http.MethodDelete, "/api/v0/edit-sessions/"+sessions[i], ""), http.StatusNoContent)
	}
	if gone := answerOf[mcpDeleted](t, alice.use(t, "delete", map[string]any{"path": "Beside.md", "recursive": true})); gone.NodeID != beside ||
		gone.Pages != 4 {
		t.Errorf("cell 13: delete of Beside once the sessions ended = %+v, want its four pages", gone)
	}
	checkPages(t, tm.pool)
}
