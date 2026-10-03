package bootstrap

import (
	"net/http"
	"strings"
	"testing"
)

// A page's task items through serve (M5/P6 design 3.2–3.4).

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
