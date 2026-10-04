package bootstrap

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// A page's task items through serve (M5/P6 design 3.2–3.4).

// taskToggle is by's toggle of the page id's task item at offset, on base.
func taskToggle(by, id string, base, offset int, checked bool) step {
	return request(by, http.MethodPost, "/api/v0/pages/"+id+"/toggle-task",
		fmt.Sprintf(`{"base_revision":%d,"offset":%d,"checked":%t}`, base, offset, checked))
}

// The reading view's checkboxes carry their byte positions in the content,
// a CRLF's two bytes counted: serve's Markdown registers the extension
// (markdownExtensions).
func TestAReadingViewsCheckboxesCarryTheirPositions(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Eng")
	id := tm.createPage(t, "alice", nb, "", "Tasks")
	tm.send(t, contentWrite("alice", id, "intro\r\n\r\n- [ ] a\r\n- [x] b\r\n", 1, ""), http.StatusOK)

	status, body := ask(t, tm.contract, http.MethodGet, tm.base+"/api/v0/pages/"+id+"/view", tm.tokens["bob"], "")
	if status != http.StatusOK {
		t.Fatalf("bob's reading view = %d %s", status, body)
	}
	var v struct {
		HTML string `json:"html"`
	}
	decodeAnswer(t, body, &v)
	for _, want := range []string{
		`<li><input disabled="" type="checkbox" data-task="12"> a</li>`,
		`<li><input checked="" disabled="" type="checkbox" data-task="21"> b</li>`,
	} {
		if !strings.Contains(v.HTML, want) {
			t.Errorf("the reading view %q holds no %s", v.HTML, want)
		}
	}
}

// A toggle's last hop (M5/P6 design 3.3): through serve, it goes by the
// page's lock and reaches the streams as a content write does. While
// alice's session holds the page, bob's toggle and her own, in no session,
// are page.locked by her; once it ends, bob's ticks the item, the one
// byte changed, and the stream sees the page's new revision. A toggle of
// an item in that state already writes nothing: no event.
func TestATaskToggleGoesByTheLockAndReachesTheStreams(t *testing.T) {
	tm, nb, marker, s := eventsTeam(t)
	id := tm.createPage(t, "alice", nb, "", "Tasks")
	s.tree(t, nb, map[string]int{id: 1})
	tm.send(t, contentWrite("alice", id, "intro\r\n\r\n- [ ] a\r\n- [x] b\r\n", 1, ""), http.StatusOK)
	s.pages(t, nb)

	session := tm.openSession(t, "alice", id)
	s.lock(t, id, session)
	for _, by := range []string{"bob", "alice"} {
		c := taskToggle(by, id, 2, 12, true)
		status, body := ask(t, tm.contract, c.method, tm.base+c.path, tm.tokens[by], c.body)
		if status != http.StatusConflict || !strings.Contains(body, `"code":"page.locked"`) || lockHolder(answer{body: body}) != "alice" {
			t.Errorf("%s's toggle while alice edits = %d %s, want 409 page.locked by alice", by, status, body)
		}
	}
	tm.send(t, request("alice", http.MethodDelete, "/api/v0/edit-sessions/"+session, ""), http.StatusNoContent)
	s.lock(t, id, session)

	tm.send(t, taskToggle("bob", id, 2, 12, true), http.StatusOK)
	if p := s.pages(t, nb); p.Tree || fmt.Sprint(p.written()) != fmt.Sprint(map[string]int{id: 3}) {
		t.Errorf("a toggle: tree %v, written %v; want %s at 3 alone", p.Tree, p.written(), id)
	}
	if got, want := tm.content(t, "bob", id).Content, "intro\r\n\r\n- [x] a\r\n- [x] b\r\n"; got != want {
		t.Errorf("toggled to %q, want %q", got, want)
	}
	tm.send(t, taskToggle("bob", id, 3, 21, true), http.StatusOK)
	tm.quiet(t, nb, marker, s)
}
