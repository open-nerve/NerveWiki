package bootstrap

import (
	"slices"
	"strings"
	"testing"
)

// The initiator's agent's guardrails through the whole program (M11/P1
// design 3.4, 7.1; M11/P2 design 3.6). Only MCP reaches them: REST's
// writes are frozen before them.
//
// P1's is the delete's. alice upgrades a notebook whose root holds Guide,
// Folder with Kept under it, and raw, which stands for the source root the
// start created: not of the baseline, its row protected. Her agent's
// delete of Guide, of raw, of Folder with Kept is page.upgrade_protected;
// a page it writes it may delete, and the changeset of the write records
// the upgrade. Guide moved under a page the agent created (a move P1 lets
// by), that page's delete is refused for Guide, alone and as a batch's
// second op, the batch refused whole at that op; a page under it that the
// agent created goes. Each refusal leaves the notebook's rows as they
// were.
//
// P2/S1's cells, g1 to g22 of its design's table (3.6), each in a notebook
// of its own that alice upgrades, its pages written over REST before: each
// refusal answers its code, at its op for a batch, and leaves the
// notebook's rows and the upgrade's ledger as they were; each cell done
// says what it wrote.
func TestUpgradeGuardrails(t *testing.T) {
	tm := newAcmeTeam(t, "member", "")
	t.Run("deletion", func(t *testing.T) {
		nb := tm.openNotebook(t, "alice", "Notes")
		guide := tm.createPageWith(t, "alice", nb, "", "Guide", "a\n")
		folder := tm.createPage(t, "alice", nb, "", "Folder")
		tm.createPage(t, "alice", nb, folder, "Kept")
		raw := tm.createPage(t, "alice", nb, "", "raw")
		upgrade := tm.upgrade(t, nb, "alice", "upgrading")
		tm.exec(t, `UPDATE upgrade_nodes SET baseline = false, parent_id = NULL, name = NULL, sort_order = NULL, revision = NULL,
			content_hash = NULL, byte_size = NULL, wanted = NULL, accepted_hash = NULL WHERE upgrade_id = $1 AND node_id = $2`, upgrade, raw)
		tm.exec(t, `UPDATE notebook_upgrades SET seeded_root = true, source_root_id = $2, baseline_nodes = baseline_nodes - 1,
			baseline_pages = baseline_pages - 1 WHERE id = $1`, upgrade, raw)
		g := upgradedNotebook{tm: tm, id: nb, upgrade: upgrade, agent: tm.agent(t, "alice", nb)}
		alive := func(names ...string) []string {
			return queryStrings(t, tm.pool, `SELECT name FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL AND name = ANY($2)
				ORDER BY name`, nb, names)
		}
		const protected = "page.upgrade_protected"
		g.refused(t, "the delete of Guide, of the baseline", protected, "delete", map[string]any{"path": "Guide.md"}, -1)
		g.refused(t, "the delete of raw, the protected source root", protected, "delete", map[string]any{"path": "raw.md"}, -1)
		g.refused(t, "the delete of Folder with Kept", protected, "delete", map[string]any{"path": "Folder.md", "recursive": true}, -1)
		if got := alive("Guide", "raw", "Folder", "Kept"); !slices.Equal(got, []string{"Folder", "Guide", "Kept", "raw"}) {
			t.Errorf("after the refusals, alive %v; want Folder, Guide, Kept and raw", got)
		}

		g.done(t, "the write of New", "write", map[string]any{"path": "New.md", "content": "x"})
		if got := queryStrings(t, tm.pool, `SELECT coalesce(upgrade_id::text, '') FROM changesets WHERE notebook_id = $1
			ORDER BY created_at DESC, id DESC LIMIT 1`, nb); len(got) != 1 || got[0] != upgrade {
			t.Errorf("the write's changeset is of the upgrade %v, want %s", got, upgrade)
		}
		g.done(t, "the delete of New, written since", "delete", map[string]any{"path": "New.md"})
		if got := alive("New"); len(got) != 0 {
			t.Errorf("New alive after its delete: %v", got)
		}

		g.done(t, "the write of Made/Inner", "write", map[string]any{"path": "Made/Inner.md", "content": "x"})
		g.done(t, "the move of Guide under Made", "move", map[string]any{"from": "Guide.md", "to": "Made/Guide.md"})
		g.refused(t, "the delete of Made with Guide under it", protected, "delete", map[string]any{"path": "Made.md", "recursive": true}, -1)
		g.refused(t, "a batch's delete of Made with Guide under it", protected, "batch", batched(opWrite("Extra.md", "x"),
			opDeleteAll("Made.md")), 1)
		if got := alive("Made", "Inner", "Guide", "Extra"); !slices.Equal(got, []string{"Guide", "Inner", "Made"}) {
			t.Errorf("after Made's refusals, alive %v; want Guide, Inner and Made, no Extra", got)
		}
		g.done(t, "the delete of Inner, written since", "delete", map[string]any{"path": "Made/Inner.md"})
		if got := alive("Inner", "Guide"); !slices.Equal(got, []string{"Guide"}) {
			t.Errorf("after Inner's delete, alive %v; want Guide", got)
		}
		if n := count(t, tm.pool, "SELECT count(*) FROM nodes WHERE id = $1 AND deleted_at IS NULL", guide); n != 1 {
			t.Errorf("Guide alive %d times at the end, want once", n)
		}
	})

	// g5, g6: a single move, to another parent and to another name under
	// its parent, with update_links false; g13: a batch's.
	const keepLinks = "page.upgrade_keep_links"
	t.Run("g5", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"}, upgradedPage{title: "Folder"})
		g.refused(t, "the move of Plan under Folder, update_links false", keepLinks, "move",
			map[string]any{"from": "Plan.md", "to": "Folder/Plan.md", "update_links": false}, -1)
	})
	t.Run("g6", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"})
		g.refused(t, "the rename of Plan to Map, update_links false", keepLinks, "move",
			map[string]any{"from": "Plan.md", "to": "Map.md", "update_links": false}, -1)
	})
	t.Run("g13", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"})
		g.refused(t, "a batch's rename of Plan to Map, update_links false", keepLinks, "batch",
			batched(opWrite("New.md", "x"), opWith(opMove("Plan.md", "Map.md"), "update_links", false)), 1)
	})
	checkPages(t, tm.pool)
}

// upgradedPage is a page a guarded notebook holds as its upgrade starts:
// under the page titled parent, at the root when it is "".
type upgradedPage struct{ parent, title, content string }

// upgradedNotebook is a notebook of alice's that she upgrades, her agent in
// it, and the ids of its pages by their titles.
type upgradedNotebook struct {
	tm      acmeTeam
	id      string
	upgrade string
	agent   *mcpApp
	ids     map[string]string
}

// upgradedNotebook is a new notebook of alice's holding pages, written over
// REST in their order, then upgraded by her.
func (tm acmeTeam) upgradedNotebook(t *testing.T, pages ...upgradedPage) upgradedNotebook {
	t.Helper()
	name := t.Name()[strings.LastIndex(t.Name(), "/")+1:]
	g := upgradedNotebook{tm: tm, id: tm.openNotebook(t, "alice", name), ids: map[string]string{}}
	for _, p := range pages {
		g.ids[p.title] = tm.createPageWith(t, "alice", g.id, g.ids[p.parent], p.title, p.content)
	}
	g.upgrade = tm.upgrade(t, g.id, "alice", "upgrading")
	g.agent = tm.agent(t, "alice", g.id)
	return g
}

// rows are the notebook's rows and its upgrade's: those a refusal leaves
// as they were.
func (g upgradedNotebook) rows(t *testing.T) []string {
	t.Helper()
	return append(sourcePersistentRows(t, g.tm, g.id), queryStrings(t, g.tm.pool, `SELECT node_id::text || ' ' || baseline::text || ' ' ||
		coalesce(encode(accepted_hash, 'hex'), '') FROM upgrade_nodes WHERE upgrade_id = $1 ORDER BY node_id`, g.upgrade)...)
}

// refused is the agent's call of tool with args, which must answer code,
// at the op op of a batch (none when it is negative), writing nothing.
func (g upgradedNotebook) refused(t *testing.T, what, code, tool string, args map[string]any, op int) {
	t.Helper()
	before := g.rows(t)
	member := g.agent.use(t, tool, args).toolError(t)
	if member["code"] != code {
		t.Errorf("%s = %v, want %s", what, member, code)
	}
	if op >= 0 && member["op"] != float64(op) {
		t.Errorf("%s: refused at %v, want ops[%d]", what, member["op"], op)
	}
	if after := g.rows(t); !slices.Equal(after, before) {
		t.Errorf("%s, refused, changed the notebook's rows or its upgrade's", what)
	}
}

// done is the agent's call of tool with args, which must be done.
func (g upgradedNotebook) done(t *testing.T, what, tool string, args map[string]any) {
	t.Helper()
	if r := g.agent.use(t, tool, args).result(t); r["isError"] == true {
		t.Errorf("%s = %v, want it done", what, r)
	}
}
