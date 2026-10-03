package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// pagesData is a pages frame's data.
type pagesData struct {
	NotebookID string `json:"notebook_id"`
	Tree       bool   `json:"tree"`
	Pages      *[]struct {
		ID       string `json:"id"`
		Revision int    `json:"revision"`
	} `json:"pages"`
}

// written is the pages the frame lists as written, page id → revision;
// nil when it lists none.
func (p pagesData) written() map[string]int {
	if p.Pages == nil {
		return nil
	}
	out := map[string]int{}
	for _, w := range *p.Pages {
		out[w.ID] = w.Revision
	}
	return out
}

// pages fails t unless the stream's next frame is a pages event of the
// notebook nb, and returns its data.
func (s *eventStream) pages(t *testing.T, nb string) pagesData {
	t.Helper()
	f := s.expect(t, "pages")
	var p pagesData
	if err := json.Unmarshal([]byte(f.data), &p); err != nil || p.NotebookID != nb {
		t.Fatalf("%s's stream: pages %s, %v; want one of %s", s.name, f.data, err, nb)
	}
	return p
}

// tree fails t unless the stream's next frame is a pages event of nb that
// changed the tree and wrote the pages written, page id → revision.
func (s *eventStream) tree(t *testing.T, nb string, written map[string]int) {
	t.Helper()
	if p := s.pages(t, nb); !p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(written) {
		t.Fatalf("%s's stream: tree %v, written %v; want the tree changed, %v written", s.name, p.Tree, p.written(), written)
	}
}

// lock fails t unless the stream's next frame is a lock event of the page
// and the session.
func (s *eventStream) lock(t *testing.T, page, session string) {
	t.Helper()
	if f := s.expect(t, "lock"); f.field(t, "page_id") != page || f.field(t, "session_id") != session {
		t.Fatalf("%s's stream: lock %s, want page %s, session %s", s.name, f.data, page, session)
	}
}

// quiet fails t unless nothing reached the streams since their last
// frames: one write of the page marker's content, by alice, is the next
// frame of each.
func (tm acmeTeam) quiet(t *testing.T, nb, marker string, streams ...*eventStream) {
	t.Helper()
	rev := tm.content(t, "alice", marker).Revision
	tm.send(t, contentWrite("alice", marker, fmt.Sprintf("# Marker %d", rev+1), rev, ""), http.StatusOK)
	for _, s := range streams {
		if p := s.pages(t, nb); p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(map[string]int{marker: rev + 1}) {
			t.Fatalf("%s's stream: %v, want the marker's write: something else came before it", s.name, p.written())
		}
	}
}

// eventsTeam is acme with bob a member, alice's notebook Eng open to its
// members to edit, a marker page in it, and bob's event stream.
func eventsTeam(t *testing.T) (tm acmeTeam, nb, marker string, bob *eventStream) {
	t.Helper()
	tm = newAcmeTeam(t, "member", "")
	nb = tm.openNotebook(t, "alice", "Eng")
	marker = tm.createPage(t, "alice", nb, "", "Marker")
	tm.settle(t, nb, marker)
	return tm, nb, marker, openStream(t, tm.base, "bob", tm.tokens["bob"])
}

// The observer's last hop (M4/P1 handoff item 4; M5 design 8): each write
// unit of the pages reaches a stream that sees the notebook as one pages
// event, through the whole program, NOTIFY and the listener included. A
// creation, a rename, a move and a deletion change the tree; a creation,
// with a content or without, lists the page at revision 1, and a content
// written, alone or in an edit session, the page and its new revision. An
// opening, a heartbeat, an end and a save of the content the page holds
// write nothing: no pages event.
func TestEveryPageWriteReachesTheStreams(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)

	a := tm.createPage(t, "alice", nb, "", "A")
	s.tree(t, nb, map[string]int{a: 1})
	status, body := ask(t, tm.contract, http.MethodPost, tm.base+"/api/v0/notebooks/"+nb+"/pages", tm.tokens["alice"],
		`{"parent_id":null,"title":"B","content":"# B"}`)
	if status != http.StatusCreated {
		t.Fatalf("create B = %d %s", status, body)
	}
	b := idOf(t, answer{body: body})
	s.tree(t, nb, map[string]int{b: 1})
	tm.send(t, nodeRename("alice", a, "A2"), http.StatusOK)
	s.tree(t, nb, map[string]int{})
	tm.send(t, nodeMove("alice", a, b), http.StatusOK)
	s.tree(t, nb, map[string]int{})

	tm.send(t, contentWrite("alice", a, "# A", 1, ""), http.StatusOK)
	if p := s.pages(t, nb); p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(map[string]int{a: 2}) {
		t.Errorf("a content written: tree %v, written %v; want %s at 2 alone", p.Tree, p.written(), a)
	}
	session := tm.openSession(t, "alice", a)
	s.lock(t, a, session)
	tm.send(t, contentWrite("alice", a, "# A in a session", 2, session), http.StatusOK)
	if p := s.pages(t, nb); p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(map[string]int{a: 3}) {
		t.Errorf("a content written in a session: tree %v, written %v; want %s at 3 alone", p.Tree, p.written(), a)
	}
	tm.send(t, contentWrite("alice", a, "# A in a session", 3, session), http.StatusOK)
	tm.send(t, heartbeat("alice", session), http.StatusOK)
	tm.quiet(t, nb, marker, s)
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)
	s.lock(t, a, session)

	tm.send(t, nodeDeletion("alice", b), http.StatusNoContent)
	s.tree(t, nb, map[string]int{})
	tm.quiet(t, nb, marker, s)
}

// The session subscriber's last hop (M4/P4 handoff item 8): each opening
// and each end of an alive session reaches the stream as a lock event of
// its page and session: an opening, a take-over (the end, then the
// opening), a forced unlock, the owner's end, a deletion of the page and
// of a subtree. A tombstone's end and an expired session's are told by no
// one.
func TestEverySessionChangeReachesTheStreams(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)
	a := tm.createPage(t, "alice", nb, "", "A")
	s.tree(t, nb, map[string]int{a: 1})

	first := tm.openSession(t, "alice", a)
	s.lock(t, a, first)
	second := tm.openSessionWith(t, "alice", sessionTakeOver("alice", a))
	s.lock(t, a, first)
	s.lock(t, a, second)
	tm.send(t, unlock("alice", a), http.StatusNoContent)
	s.lock(t, a, second)
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+second, ""), http.StatusNoContent)
	tm.quiet(t, nb, marker, s)

	third := tm.openSession(t, "bob", a)
	s.lock(t, a, third)
	// Expired a minute ago, whatever the clocks of the server and the database.
	if _, err := tm.pool.Exec(context.Background(),
		"UPDATE edit_sessions SET created_at = created_at - interval '2 minutes', expires_at = created_at - interval '1 minute' WHERE id = $1", third); err != nil {
		t.Fatal(err)
	}
	fourth := tm.openSession(t, "alice", a)
	s.lock(t, a, fourth)
	tm.quiet(t, nb, marker, s)

	tm.send(t, nodeDeletion("alice", a), http.StatusNoContent)
	s.lock(t, a, fourth)
	s.tree(t, nb, map[string]int{})

	top := tm.createPage(t, "alice", nb, "", "Top")
	s.tree(t, nb, map[string]int{top: 1})
	under := tm.createPage(t, "alice", nb, top, "Under")
	s.tree(t, nb, map[string]int{under: 1})
	fifth := tm.openSession(t, "alice", under)
	s.lock(t, under, fifth)
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+fifth, ""), http.StatusNoContent)
	s.lock(t, under, fifth)
	sixth := tm.openSession(t, "alice", under)
	s.lock(t, under, sixth)
	tm.send(t, nodeDeletion("alice", top), http.StatusNoContent)
	s.lock(t, under, sixth)
	s.tree(t, nb, map[string]int{})
	tm.quiet(t, nb, marker, s)
}
