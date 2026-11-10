package bootstrap

import (
	"slices"
	"testing"
)

// toolCode is a tool and a code of its row of mcpToolCodes.
type toolCode struct{ tool, code string }

// upgradeToolCodes are the codes of mcpToolCodes' rows that an upgrade's
// freeze and guardrails answer (M11/P1 design 3.6, M11/P2 design 3.6), by
// tool and code: they
// fall in the rows of the five writes, so they cannot be told apart by
// tool. The fifth shard's TestTheMCPToolsErrorsCarryTheirCodes, whose app
// has no upgrade, answers mcpToolCodes but these; the eighth shard's
// TestUpgradeToolsErrorsCarryTheirCodes answers these. Both subtract from
// this one table, so that the two together answer every code of every row.
func upgradeToolCodes() map[toolCode]bool {
	return map[toolCode]bool{
		{"write", "notebook.upgrading"}: true, {"edit", "notebook.upgrading"}: true, {"move", "notebook.upgrading"}: true,
		{"delete", "notebook.upgrading"}: true, {"batch", "notebook.upgrading"}: true,
		{"delete", "page.upgrade_protected"}: true, {"batch", "page.upgrade_protected"}: true,
		{"write", "page.upgrade_immutable"}: true, {"edit", "page.upgrade_immutable"}: true, {"batch", "page.upgrade_immutable"}: true,
		{"batch", "page.upgrade_batch_rewrite"}: true, {"move", "page.upgrade_keep_links"}: true, {"batch", "page.upgrade_keep_links"}: true,
	}
}

// Each code of upgradeToolCodes is answered by a real call, in either
// entry (M11/P1 design 3.6): in a notebook being upgraded by bob, alice's
// agent's write, edit, move, delete and batch are notebook.upgrading; in
// one being upgraded by alice, whose baseline has Guide, her agent's
// delete of Guide, alone or in a batch, is page.upgrade_protected; her
// agent's write and edit of Guide's body, alone or in a batch, are
// page.upgrade_immutable; her agent's batch that moves Other, then edits
// the frontmatter of Linked, which links to it, folding the move's
// rewrite in, is page.upgrade_batch_rewrite; her agent's move of Other
// with update_links false, alone or in a batch, is
// page.upgrade_keep_links; each refused
// before it wrote anything; and each pair listed is a code of its tool's
// row.
func TestUpgradeToolsErrorsCarryTheirCodes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	others := tm.openNotebook(t, "alice", "Others")
	tm.createPageWith(t, "alice", others, "", "Guide", "a\n")
	tm.createPage(t, "alice", others, "", "Other")
	tm.upgrade(t, others, "bob", "upgrading")
	own := tm.openNotebook(t, "alice", "Own")
	guide := tm.createPageWith(t, "alice", own, "", "Guide", "a\n")
	tm.createPage(t, "alice", own, "", "Other")
	tm.createPageWith(t, "alice", own, "", "Linked", "---\ntype: a\n---\nSee [[Other]].\n")
	tm.upgrade(t, own, "alice", "upgrading")
	agent := tm.agent(t, "alice", others)
	answered := answeredCodes{}
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			notebook, tool, code string
			args                 map[string]any
		}{
			{others, "write", "notebook.upgrading", map[string]any{"path": "New.md", "content": "x"}},
			{others, "edit", "notebook.upgrading", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}},
			{others, "move", "notebook.upgrading", map[string]any{"from": "Other.md", "to": "Moved.md"}},
			{others, "delete", "notebook.upgrading", map[string]any{"path": "Other.md"}},
			{others, "batch", "notebook.upgrading", batched(opWrite("New.md", "x"))},
			{own, "delete", "page.upgrade_protected", map[string]any{"path": "Guide.md"}},
			{own, "batch", "page.upgrade_protected", batched(opWrite("New.md", "x"), opDelete("Guide.md"))},
			{own, "write", "page.upgrade_immutable", map[string]any{"path": "Guide.md", "content": "b\n", "base_version": guide + ":1"}},
			{own, "edit", "page.upgrade_immutable", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}},
			{own, "batch", "page.upgrade_immutable", batched(opWrite("New.md", "x"), opEdit("Guide.md", "a", "b"))},
			{own, "batch", "page.upgrade_batch_rewrite", batched(opMove("Other.md", "Moved.md"), opEdit("Linked.md", "type: a", "type: b"))},
			{own, "move", "page.upgrade_keep_links", map[string]any{"from": "Other.md", "to": "Moved.md", "update_links": false}},
			{own, "batch", "page.upgrade_keep_links", batched(opWith(opMove("Other.md", "Moved.md"), "update_links", false))},
		} {
			m := *agent
			m.notebook = tt.notebook
			before := sourcePersistentRows(t, tm, tt.notebook)
			member := sourceMCPCall(t, &m, scope, tt.tool, tt.args).toolError(t)
			if member["code"] != tt.code {
				t.Errorf("%s: %s = %v, want %s", scope, tt.tool, member, tt.code)
				continue
			}
			if after := sourcePersistentRows(t, tm, tt.notebook); !slices.Equal(after, before) {
				t.Errorf("%s: %s, refused, changed the notebook's rows", scope, tt.tool)
			}
			answered.add(tt.tool, tt.code)
		}
	}
	rows := mcpToolCodes()
	for pair := range upgradeToolCodes() {
		if !answered[pair.tool][pair.code] {
			t.Errorf("upgradeToolCodes lists %s's %s, which no call here answers", pair.tool, pair.code)
		}
		if !slices.Contains(rows[pair.tool], pair.code) {
			t.Errorf("upgradeToolCodes lists %s's %s, which its row of mcpToolCodes does not", pair.tool, pair.code)
		}
	}
	checkPages(t, tm.pool)
}
