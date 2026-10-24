package bootstrap

import (
	"slices"
	"strings"
	"testing"
)

// The mode guard reaches MCP's writes through the notebook entry (M10/P1
// design 3.24): an LLM Wiki's write of index/x.md, which creates index at
// its root on the way, is page.virtual_file; the move and the deletion of
// AGENTS.md are page.protected; a writer's move of raw.md is
// page.source_admin_only; each answered as REST's, its text led by its
// code, structuredContent.error's code the same, the rows as they were.
// An edit of AGENTS.md is written as any page's, of the agent's client.
// A batch that edits Notes, then deletes AGENTS.md, is refused at its
// second op as a whole: Notes as it was.
func TestTheModeGuardReachesEveryMCPWrite(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	wiki := tm.llmWiki(t, "Wiki")
	tm.createPageWith(t, "alice", wiki.id, "", "Notes", "draft\n")
	alice, bob := tm.agent(t, "alice", wiki.id), tm.agent(t, "bob", wiki.id)
	alice.handshakeAs(t, "whole-program")
	for _, tt := range []struct {
		who  string
		m    *mcpApp
		tool string
		args map[string]any
		code string
	}{
		{"alice", alice, "write", map[string]any{"path": "index/x.md", "content": "x"}, "page.virtual_file"},
		{"alice", alice, "move", map[string]any{"from": "AGENTS.md", "to": "Notes/AGENTS.md"}, "page.protected"},
		{"bob", bob, "move", map[string]any{"from": "raw.md", "to": "Notes/raw.md"}, "page.source_admin_only"},
		{"alice", alice, "delete", map[string]any{"path": "AGENTS.md"}, "page.protected"},
	} {
		before := tm.guardRows(t, wiki.id)
		member := tt.m.use(t, tt.tool, tt.args).toolError(t)
		if message, _ := member["message"].(string); member["code"] != tt.code || !strings.HasPrefix(message, tt.code+": ") {
			t.Errorf("%s's %s %v = %v, want %s", tt.who, tt.tool, tt.args, member, tt.code)
		}
		if after := tm.guardRows(t, wiki.id); after != before {
			t.Errorf("%s's %s %v, refused, left the rows %s, want them as they were, %s", tt.who, tt.tool, tt.args, after, before)
		}
	}

	edited := alice.use(t, "edit", map[string]any{"path": "AGENTS.md", "old_string": "# How this LLM Wiki is kept",
		"new_string": "# How this wiki is kept"})
	if r := edited.result(t); r["isError"] == true {
		t.Fatalf("alice's edit of AGENTS.md = %v, want it written", r)
	}
	if got := queryStrings(t, tm.pool, `SELECT c.kind || ' ' || c.client FROM changesets c
		WHERE c.notebook_id = $1 AND c.client LIKE 'mcp%' ORDER BY c.created_at`, wiki.id); !slices.Equal(got, []string{"edit mcp:whole-program"}) {
		t.Errorf("the agent's changesets are %q, want its edit of AGENTS", got)
	}
	if content := tm.rootOf(t, wiki.id)[1].Content; !strings.HasPrefix(content, "# How this wiki is kept") {
		t.Errorf("AGENTS after the edit = %.60q, want it edited", content)
	}

	before := tm.guardRows(t, wiki.id)
	a := alice.use(t, "batch", batched(map[string]any{"op": "edit", "path": "Notes.md", "old_string": "draft", "new_string": "final"},
		map[string]any{"op": "delete", "path": "AGENTS.md"}))
	member := a.toolError(t)
	if message, _ := member["message"].(string); member["code"] != "page.protected" || member["op"] != 1.0 ||
		!strings.HasPrefix(message, "page.protected: ops[1] (delete): ") {
		t.Errorf("the batch that deletes AGENTS.md = %.400s, want page.protected at ops[1] (delete)", a.body)
	}
	if after, notes := tm.guardRows(t, wiki.id), queryStrings(t, tm.pool, `SELECT c.content FROM page_contents c
		JOIN nodes n ON n.id = c.node_id WHERE n.notebook_id = $1 AND n.name = 'Notes'`, wiki.id); after != before ||
		!slices.Equal(notes, []string{"draft\n"}) {
		t.Errorf("after the refused batch: rows %s, Notes %q; want the rows %s, Notes as it was", after, notes, before)
	}
	checkPages(t, tm.pool)
}
