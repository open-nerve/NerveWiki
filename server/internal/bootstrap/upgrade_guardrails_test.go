package bootstrap

import (
	"slices"
	"testing"
)

// The initiator's agent's guardrails through the whole program (M11/P1
// design 3.4, 7.1; P2/S1 adds the rest). P1's is the delete's, which only
// MCP reaches: REST's writes are frozen before it.
//
// alice upgrades a notebook whose root holds Guide, Folder with Kept under
// it, and raw, which stands for the source root the start created: not of
// the baseline, its row protected. Her agent's delete of Guide, of raw, of
// Folder with Kept is page.upgrade_protected; a page it writes it may
// delete, and the changeset of the write records the upgrade. Guide moved
// under a page the agent created (a move P1 lets by), that page's delete
// is refused for Guide, alone and as a batch's second op, the batch
// refused whole at that op; a page under it that the agent created goes.
// Each refusal leaves the notebook's rows as they were.
func TestUpgradeGuardrails(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	nb := tm.openNotebook(t, "alice", "Notes")
	guide := tm.createPageWith(t, "alice", nb, "", "Guide", "a\n")
	folder := tm.createPage(t, "alice", nb, "", "Folder")
	tm.createPage(t, "alice", nb, folder, "Kept")
	raw := tm.createPage(t, "alice", nb, "", "raw")
	upgrade := tm.upgrade(t, nb, "alice", "upgrading")
	tm.exec(t, `UPDATE upgrade_nodes SET baseline = false, parent_id = NULL, name = NULL, sort_order = NULL, revision = NULL,
		content_hash = NULL, byte_size = NULL, wanted = NULL, accepted_hash = NULL WHERE upgrade_id = $1 AND node_id = $2`, upgrade, raw)
	agent := tm.agent(t, "alice", nb)
	alive := func(names ...string) []string {
		return queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND name = ANY($2)
			ORDER BY name`, nb, names)
	}
	refused := func(what, tool string, args map[string]any, op int) {
		t.Helper()
		before := sourcePersistentRows(t, tm, nb)
		member := agent.use(t, tool, args).toolError(t)
		if member["code"] != "page.upgrade_protected" {
			t.Errorf("%s = %v, want page.upgrade_protected", what, member)
		}
		if op >= 0 && member["op"] != float64(op) {
			t.Errorf("%s: refused at %v, want ops[%d]", what, member["op"], op)
		}
		if after := sourcePersistentRows(t, tm, nb); !slices.Equal(after, before) {
			t.Errorf("%s, refused, changed the notebook's rows", what)
		}
	}
	done := func(what, tool string, args map[string]any) {
		t.Helper()
		if r := agent.use(t, tool, args).result(t); r["isError"] == true {
			t.Errorf("%s = %v, want it done", what, r)
		}
	}

	refused("the delete of Guide, of the baseline", "delete", map[string]any{"path": "Guide.md"}, -1)
	refused("the delete of raw, the protected source root", "delete", map[string]any{"path": "raw.md"}, -1)
	refused("the delete of Folder with Kept", "delete", map[string]any{"path": "Folder.md", "recursive": true}, -1)
	if got := alive("Guide", "raw", "Folder", "Kept"); !slices.Equal(got, []string{"Folder", "Guide", "Kept", "raw"}) {
		t.Errorf("after the refusals, alive %v; want Folder, Guide, Kept and raw", got)
	}

	done("the write of New", "write", map[string]any{"path": "New.md", "content": "x"})
	if got := queryStrings(t, tm.pool, `SELECT coalesce(upgrade_id::text, '') FROM changesets WHERE notebook_id = $1
		ORDER BY created_at DESC, id DESC LIMIT 1`, nb); len(got) != 1 || got[0] != upgrade {
		t.Errorf("the write's changeset is of the upgrade %v, want %s", got, upgrade)
	}
	done("the delete of New, written since", "delete", map[string]any{"path": "New.md"})
	if got := alive("New"); len(got) != 0 {
		t.Errorf("New alive after its delete: %v", got)
	}

	done("the write of Made/Inner", "write", map[string]any{"path": "Made/Inner.md", "content": "x"})
	done("the move of Guide under Made", "move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"})
	refused("the delete of Made with Guide under it", "delete", map[string]any{"path": "Made.md", "recursive": true}, -1)
	refused("a batch's delete of Made with Guide under it", "batch", batched(opWrite("Extra.md", "x"), opDeleteAll("Made.md")), 1)
	if got := alive("Made", "Inner", "Guide", "Extra"); !slices.Equal(got, []string{"Guide", "Inner", "Made"}) {
		t.Errorf("after Made's refusals, alive %v; want Guide, Inner and Made, no Extra", got)
	}
	done("the delete of Inner, written since", "delete", map[string]any{"path": "Made/Inner.md"})
	if got := alive("Inner", "Guide"); !slices.Equal(got, []string{"Guide"}) {
		t.Errorf("after Inner's delete, alive %v; want Guide", got)
	}
	if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", guide); n != 1 {
		t.Errorf("Guide alive %d times at the end, want once", n)
	}
	checkPages(t, tm.pool)
}
