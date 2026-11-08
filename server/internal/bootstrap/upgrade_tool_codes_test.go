package bootstrap

import (
	"slices"
	"testing"
)

// toolCode is a tool and a code of its row of mcpToolCodes.
type toolCode struct{ tool, code string }

// upgradeToolCodes are the codes of mcpToolCodes' rows that an upgrade's
// freeze and guardrails answer (M11/P1 design 3.6), by tool and code: they
// fall in the rows of the five writes, so they cannot be told apart by
// tool. The fifth shard's TestTheMCPToolsErrorsCarryTheirCodes, whose app
// has no upgrade, answers mcpToolCodes but these; the seventh shard's
// TestUpgradeToolsErrorsCarryTheirCodes answers these. Both subtract from
// this one table, so that the two together answer every code of every row.
func upgradeToolCodes() map[toolCode]bool {
	return map[toolCode]bool{
		{"write", "notebook.upgrading"}: true, {"edit", "notebook.upgrading"}: true, {"move", "notebook.upgrading"}: true,
		{"delete", "notebook.upgrading"}: true, {"batch", "notebook.upgrading"}: true,
	}
}

// Each code of upgradeToolCodes is answered by a real call, in either
// entry (M11/P1 design 3.6): in a notebook being upgraded by bob, alice's
// agent's write, edit, move, delete and batch are notebook.upgrading, each
// refused before it wrote anything; and each pair listed is a code of its
// tool's row.
func TestUpgradeToolsErrorsCarryTheirCodes(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	others := tm.openNotebook(t, "alice", "Others")
	tm.createPageWith(t, "alice", others, "", "Guide", "a\n")
	tm.createPage(t, "alice", others, "", "Other")
	tm.upgrade(t, others, "bob", "upgrading")
	agent := tm.agent(t, "alice", others)
	answered := answeredCodes{}
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			tool, code string
			args       map[string]any
		}{
			{"write", "notebook.upgrading", map[string]any{"path": "New.md", "content": "x"}},
			{"edit", "notebook.upgrading", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}},
			{"move", "notebook.upgrading", map[string]any{"from": "Other.md", "to": "Moved.md"}},
			{"delete", "notebook.upgrading", map[string]any{"path": "Other.md"}},
			{"batch", "notebook.upgrading", batched(opWrite("New.md", "x"))},
		} {
			before := sourcePersistentRows(t, tm, others)
			member := sourceMCPCall(t, agent, scope, tt.tool, tt.args).toolError(t)
			if member["code"] != tt.code {
				t.Errorf("%s: %s in another's upgrade = %v, want %s", scope, tt.tool, member, tt.code)
				continue
			}
			if after := sourcePersistentRows(t, tm, others); !slices.Equal(after, before) {
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
