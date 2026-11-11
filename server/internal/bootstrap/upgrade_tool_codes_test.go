package bootstrap

import (
	"slices"
	"testing"
	"time"
	"uuid"
)

// toolCode is a tool and a code of its row of mcpToolCodes.
type toolCode struct{ tool, code string }

// upgradeToolCodes are the codes of mcpToolCodes' rows that an upgrade's
// freeze and guardrails answer (M11/P1 design 3.6, M11/P2 design 3.6), by
// tool and code: they fall in the rows of the five writes, so they cannot
// be told apart by tool; and every code of the upgrade's tools' rows. The
// fifth shard's TestTheMCPToolsErrorsCarryTheirCodes, whose app has no
// upgrade, answers mcpToolCodes but these; the eighth shard's
// TestUpgradeToolsErrorsCarryTheirCodes answers these. Both subtract from
// this one table, so that the two together answer every code of every row.
func upgradeToolCodes() map[toolCode]bool {
	pairs := map[toolCode]bool{
		{"write", "notebook.upgrading"}: true, {"edit", "notebook.upgrading"}: true, {"move", "notebook.upgrading"}: true,
		{"delete", "notebook.upgrading"}: true, {"batch", "notebook.upgrading"}: true,
		{"delete", "page.upgrade_protected"}: true, {"batch", "page.upgrade_protected"}: true,
		{"write", "page.upgrade_immutable"}: true, {"edit", "page.upgrade_immutable"}: true, {"batch", "page.upgrade_immutable"}: true,
		{"batch", "page.upgrade_batch_rewrite"}: true, {"move", "page.upgrade_keep_links"}: true, {"batch", "page.upgrade_keep_links"}: true,
	}
	// The upgrade's tools' rows whole (M11/P2 design 3.15): the fifth
	// shard's app has no upgrade to check or submit.
	for _, tool := range []string{"upgrade_verify", "upgrade_submit"} {
		for _, code := range mcpToolCodes()[tool] {
			pairs[toolCode{tool, code}] = true
		}
	}
	return pairs
}

// Each code of upgradeToolCodes is answered by a real call, in either
// entry (M11/P1 design 3.6, M11/P2 design 3.15): in a notebook being
// upgraded by bob, alice's agent's write, edit, move, delete and batch
// are notebook.upgrading; in one being upgraded by alice, whose baseline
// has Guide, her agent's delete of Guide, alone or in a batch, is
// page.upgrade_protected; her agent's write and edit of Guide's body,
// alone or in a batch, are page.upgrade_immutable; her agent's batch that
// moves Other, then edits the frontmatter of Linked, which links to it,
// folding the move's rewrite in, is page.upgrade_batch_rewrite; her
// agent's move of Other with update_links false, alone or in a batch, is
// page.upgrade_keep_links; each refused before it wrote anything. The
// upgrade's tools answer each code of their rows: a cursor that does not
// decode and an argument submit does not know, bad_request; a limit
// without a rule, validation_failed; a notebook alice does not see,
// notebook.not_found (the account entry's: the notebook entry's is the
// edge's 404); bob's agent's, an editor's, forbidden; an unknown rule;
// a notebook not upgraded, wiki.upgrade_state; the app's one lint slot
// held (heldRules), server_busy; a page's index stale, and the gates
// not passed (no AGENTS), each submission's. Each pair listed is a code
// of its tool's row.
func TestUpgradeToolsErrorsCarryTheirCodes(t *testing.T) {
	r := upgradeRunsOn(t, nil, 1, 2*time.Second, 0)
	tm := r.tm
	others := tm.openNotebook(t, "alice", "Others")
	tm.createPageWith(t, "alice", others, "", "Guide", "a\n")
	tm.createPage(t, "alice", others, "", "Other")
	tm.upgrade(t, others, "bob", "upgrading")
	own := tm.openNotebook(t, "alice", "Own")
	guide := tm.createPageWith(t, "alice", own, "", "Guide", "a\n")
	tm.createPage(t, "alice", own, "", "Other")
	tm.createPageWith(t, "alice", own, "", "Linked", "---\ntype: a\n---\nSee [[Other]].\n")
	tm.upgrade(t, own, "alice", "upgrading")
	plain := tm.openNotebook(t, "alice", "Plain")
	tm.createPage(t, "alice", plain, "", "Page")
	stale := tm.openNotebook(t, "alice", "Stale")
	tm.createPage(t, "alice", stale, "", "Page")
	tm.upgrade(t, stale, "alice", "upgrading")
	tm.exec(t, "UPDATE indexed_pages SET extractor = extractor - 1 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)", stale)
	none := uuid.NewV7().String()
	agents := map[string]*mcpApp{"alice": tm.agent(t, "alice", others), "bob": tm.agent(t, "bob", own)}
	answered := answeredCodes{}
	for _, scope := range []string{"notebook", "account"} {
		for _, tt := range []struct {
			by, notebook, tool, code string
			args                     map[string]any
		}{
			{"alice", others, "write", "notebook.upgrading", map[string]any{"path": "New.md", "content": "x"}},
			{"alice", others, "edit", "notebook.upgrading", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}},
			{"alice", others, "move", "notebook.upgrading", map[string]any{"from": "Other.md", "to": "Moved.md"}},
			{"alice", others, "delete", "notebook.upgrading", map[string]any{"path": "Other.md"}},
			{"alice", others, "batch", "notebook.upgrading", batched(opWrite("New.md", "x"))},
			{"alice", own, "delete", "page.upgrade_protected", map[string]any{"path": "Guide.md"}},
			{"alice", own, "batch", "page.upgrade_protected", batched(opWrite("New.md", "x"), opDelete("Guide.md"))},
			{"alice", own, "write", "page.upgrade_immutable", map[string]any{"path": "Guide.md", "content": "b\n", "base_version": guide + ":1"}},
			{"alice", own, "edit", "page.upgrade_immutable", map[string]any{"path": "Guide.md", "old_string": "a", "new_string": "b"}},
			{"alice", own, "batch", "page.upgrade_immutable", batched(opWrite("New.md", "x"), opEdit("Guide.md", "a", "b"))},
			{"alice", own, "batch", "page.upgrade_batch_rewrite", batched(opMove("Other.md", "Moved.md"), opEdit("Linked.md", "type: a", "type: b"))},
			{"alice", own, "move", "page.upgrade_keep_links", map[string]any{"from": "Other.md", "to": "Moved.md", "update_links": false}},
			{"alice", own, "batch", "page.upgrade_keep_links", batched(opWith(opMove("Other.md", "Moved.md"), "update_links", false))},
			{"alice", own, "upgrade_verify", "bad_request", map[string]any{"rule": "orphans", "cursor": "bad"}},
			{"alice", own, "upgrade_submit", "bad_request", map[string]any{"nonesuch": 1}},
			{"alice", own, "upgrade_verify", "validation_failed", map[string]any{"limit": 10}},
			{"alice", none, "upgrade_verify", "notebook.not_found", map[string]any{}},
			{"alice", none, "upgrade_submit", "notebook.not_found", map[string]any{}},
			{"bob", own, "upgrade_verify", "forbidden", map[string]any{}},
			{"bob", own, "upgrade_submit", "forbidden", map[string]any{}},
			{"alice", own, "upgrade_verify", "wiki.lint_rule_not_found", map[string]any{"rule": "nonesuch"}},
			{"alice", plain, "upgrade_verify", "wiki.upgrade_state", map[string]any{}},
			{"alice", plain, "upgrade_submit", "wiki.upgrade_state", map[string]any{}},
			{"alice", stale, "upgrade_submit", "wiki.upgrade_stale_index", map[string]any{}},
			{"alice", own, "upgrade_submit", "wiki.upgrade_not_ready", map[string]any{}},
		} {
			if tt.notebook == none && scope == "notebook" {
				continue // the edge's 404, before the tool
			}
			m := *agents[tt.by]
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
		// The one lint slot held by a read of another upgrade's items: each
		// tool waits its budget, about 2 s, and is server_busy, the
		// submission upgrading again.
		letGo := r.held(t, checkStep("alice", stale, "/missing_type"))
		for _, tool := range []string{"upgrade_verify", "upgrade_submit"} {
			m := *agents["alice"]
			m.notebook = own
			if member := sourceMCPCall(t, &m, scope, tool, map[string]any{}).toolError(t); member["code"] != "server_busy" ||
				member["retry_after_seconds"] != float64(1) {
				t.Errorf("%s: %s with the slot held = %v, want server_busy", scope, tool, member)
				continue
			}
			answered.add(tool, "server_busy")
		}
		letGo()
		if got := tm.submission(t, own); got != backToUpgrading {
			t.Errorf("%s: the submissions refused left %s", scope, got)
		}
	}
	tm.exec(t, "UPDATE indexed_pages SET extractor = extractor + 1 WHERE node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)", stale)
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
