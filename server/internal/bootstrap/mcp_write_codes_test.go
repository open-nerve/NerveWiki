package bootstrap

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// The calls that answer each code of the writes' rows of mcpToolCodes
// (M9/P2 design 3.13.1), for TestTheMCPToolsErrorsCarryTheirCodes: the
// reads' app's, a reader's of a notebook open to its workspace, and the
// parse budget's, which another app holds.

// codeCall is a tool's call and the code its error carries.
type codeCall struct {
	path, tool string
	args       map[string]any
	want       string
}

// answeredCodes are the codes each tool's calls answered.
type answeredCodes map[string]map[string]bool

func (a answeredCodes) add(tool, code string) {
	if a[tool] == nil {
		a[tool] = map[string]bool{}
	}
	a[tool][code] = true
}

// writeCodeCalls are the writes' calls in alice's notebook of r, beside
// its page Guide: it gains Guide/Sub, Locked in alice's edit session,
// linking to Target, and nine levels of pages, L1 to L9, which an MCP
// write creates.
func writeCodeCalls(t *testing.T, r readsApp, guide string) []codeCall {
	t.Helper()
	r.createPage(t, guide, "Sub", "")
	locked := r.createPage(t, "", "Locked", "[[Target]]\n")
	r.createPage(t, "", "Target", "")
	if status, got := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/pages/"+locked+"/edit-sessions", r.access, ""); status != http.StatusCreated {
		t.Fatalf("open a session of Locked = %d %s", status, got)
	}
	const chain = "L1/L2/L3/L4/L5/L6/L7/L8/L9"
	r.entryCall(t, mcpserver.NotebookEntry, "write", map[string]any{"path": chain + ".md", "content": ""})
	nb, guideAt, lockedAt := atNotebook(r.notebook), guide+":1", locked+":1"
	hidden := func(args map[string]any) map[string]any {
		args["notebook"] = r.hidden
		return args
	}
	return []codeCall{
		{nb, "write", map[string]any{"path": 7, "content": "x"}, "bad_request"},
		{nb, "write", map[string]any{"path": "Guide", "content": "x"}, "validation_failed"},
		{nb, "write", map[string]any{"path": "a/./b.md", "content": "x"}, "validation_failed"},
		{nb, "write", map[string]any{"path": "New.md", "content": "a\x00b"}, "validation_failed"},
		{nb, "write", map[string]any{"path": "Guide.md", "content": "x", "base_version": "nope"}, "validation_failed"},
		{mcpserver.Pattern, "write", hidden(map[string]any{"path": "New.md", "content": "x"}), "notebook.not_found"},
		{nb, "write", map[string]any{"path": "Missing.md", "content": "x", "base_version": guideAt}, "page.not_found"},
		{nb, "write", map[string]any{"path": "Guide.md", "content": "another"}, "page.title_taken"},
		{nb, "write", map[string]any{"path": "A/B/C/D/E/F/G/H/I/J/K.md", "content": "x"}, "page.too_deep"},
		{nb, "write", map[string]any{"path": "Guide.md", "content": "x", "base_version": guide + ":99"}, "page.revision_mismatch"},
		{nb, "write", map[string]any{"path": "Locked.md", "content": "x", "base_version": lockedAt}, "page.locked"},
		{nb, "edit", map[string]any{"path": "Guide.md", "old_string": 7, "new_string": "x"}, "bad_request"},
		{nb, "edit", map[string]any{"path": "Guide.md", "old_string": "", "new_string": "x"}, "validation_failed"},
		{mcpserver.Pattern, "edit", hidden(map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}),
			"notebook.not_found"},
		{nb, "edit", map[string]any{"path": "Missing.md", "old_string": "a", "new_string": "b"}, "page.not_found"},
		{nb, "edit", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b", "base_version": guide + ":99"},
			"page.revision_mismatch"},
		{nb, "edit", map[string]any{"path": "Locked.md", "old_string": "[[Target]]", "new_string": "x"}, "page.locked"},
		{nb, "move", map[string]any{"from": "Guide.md"}, "bad_request"},
		{nb, "move", map[string]any{"from": "Guide.md", "to": "Guide.png"}, "validation_failed"},
		{mcpserver.Pattern, "move", hidden(map[string]any{"from": "Guide.md", "to": "B.md"}), "notebook.not_found"},
		{nb, "move", map[string]any{"from": "Missing.md", "to": "B.md"}, "page.not_found"},
		{nb, "move", map[string]any{"from": "Guide/Sub.md", "to": "Guide.md"}, "page.title_taken"},
		{nb, "move", map[string]any{"from": "Guide.md", "to": "Guide/Sub/Guide.md"}, "page.cycle"},
		{nb, "move", map[string]any{"from": "Guide.md", "to": chain + "/Guide.md"}, "page.too_deep"},
		{nb, "move", map[string]any{"from": "Target.md", "to": "Target 2.md"}, "linking.pages_locked"},
		{nb, "delete", map[string]any{"path": "Guide.md", "recursive": "yes"}, "bad_request"},
		{nb, "delete", map[string]any{"path": "Guide.md"}, "validation_failed"},
		{mcpserver.Pattern, "delete", hidden(map[string]any{"path": "Guide.md"}), "notebook.not_found"},
		{nb, "delete", map[string]any{"path": "Missing.md"}, "page.not_found"},
		{nb, "delete", map[string]any{"path": "Locked.md"}, "page.locked"},
	}
}

// carolsWrites has carol, a member of acme, write through the notebook
// entry of a notebook of alice's that acme's members only view: each write
// is forbidden, the edits, moves and deletions once the page is found.
func (r readsApp) carolsWrites(t *testing.T, answered answeredCodes) {
	t.Helper()
	carol := registerAccount(t, r.contract, r.base, "carol@example.com").AccessToken
	if _, err := r.pool.Exec(context.Background(), `INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
		SELECT $1, w.id, u.id, 'member', w.created_by_id, w.created_by_id, now(), now()
		FROM workspaces w, users u WHERE w.slug = 'acme' AND u.email = 'carol@example.com'`, uuid.NewV7()); err != nil {
		t.Fatal(err)
	}
	status, got := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/workspaces/acme/notebooks", r.access,
		`{"name":"Readers","workspace_access":"viewer"}`)
	if status != http.StatusCreated {
		t.Fatalf("create Readers = %d %s", status, got)
	}
	readers := idOf(t, answer{body: got})
	if status, got := ask(t, r.contract, http.MethodPost, r.base+"/api/v0/notebooks/"+readers+"/pages", r.access,
		`{"parent_id":null,"title":"Shared","content":"s"}`); status != http.StatusCreated {
		t.Fatalf("create Shared = %d %s", status, got)
	}
	m := &mcpApp{base: r.base, pat: createToken(t, r.contract, r.base, carol)}
	for _, c := range []codeCall{
		{tool: "write", args: map[string]any{"path": "New.md", "content": "x"}},
		{tool: "edit", args: map[string]any{"path": "Shared.md", "old_string": "s", "new_string": "t"}},
		{tool: "move", args: map[string]any{"from": "Shared.md", "to": "Moved.md"}},
		{tool: "delete", args: map[string]any{"path": "Shared.md"}},
	} {
		if member := m.call(t, atNotebook(readers), c.tool, c.args).toolError(t); member["code"] != "forbidden" {
			t.Errorf("carol's %s in a notebook she views = %v, want forbidden", c.tool, member)
			continue
		}
		answered.add(c.tool, "forbidden")
	}
}

// busyWrites calls write, edit and move while the server's parse budget
// is held, in an app of its own whose budget is 64 KiB, waited for 300 ms:
// a REST write of ten times the budget keeps all of it while it waits for
// its page's lock (TestAWriteWaitingForItsLockKeepsItsFactsShare). write's
// and edit's content is parsed before their units, in the notebook whose
// row the waiting write shares; move's rewrite takes the budget without
// waiting, in its unit, so its page is in another notebook, whose row the
// move locks (M9/P2 design 3.13.1).
func busyWrites(t *testing.T, answered answeredCodes) {
	t.Helper()
	tm := newAcmeTeamWith(t, "member", "", func(c *config.Config) {
		c.Page.ParseBudgetBytes, c.Page.ParseMaxWait = 64<<10, 300*time.Millisecond
	})
	eng, other := tm.openNotebook(t, "alice", "Eng"), tm.openNotebook(t, "alice", "Other")
	held := tm.createPage(t, "alice", eng, "", "Held")
	tm.createPageWith(t, "alice", eng, "", "Edited", "e")
	tm.createPage(t, "alice", other, "", "Target")
	tm.createPageWith(t, "alice", other, "", "Linker", "[[Target]]")
	m := &mcpApp{base: tm.base, pat: createToken(t, tm.contract, tm.base, tm.tokens["alice"])}
	large := strings.Repeat("f", 100<<10)
	whileAWriteWaitsForItsLock(t, tm, held, strings.Repeat("a", 640<<10), 1, func() {
		for _, c := range []codeCall{
			{atNotebook(eng), "write", map[string]any{"path": "Busy.md", "content": large}, "server_busy"},
			{atNotebook(eng), "edit", map[string]any{"path": "Edited.md", "old_string": "e", "new_string": large}, "server_busy"},
			{atNotebook(other), "move", map[string]any{"from": "Target.md", "to": "Target 2.md"}, "server_busy"},
		} {
			if member := m.call(t, c.path, c.tool, c.args).toolError(t); member["code"] != c.want {
				t.Errorf("%s with the parse budget held = %v, want %s", c.tool, member, c.want)
				continue
			}
			answered.add(c.tool, c.want)
		}
	})
}
