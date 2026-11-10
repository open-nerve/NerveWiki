package bootstrap

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
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
// P2/S1's cells, g1 to g22 of its design's table (3.6), g23 to g25 of
// decision 268 and g27 of decision 270, each in a notebook
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

	// g1: a baseline page's frontmatter written, its body as it was: done,
	// the ledger the new content's hash. g2, g3: an edit and a write of its
	// body.
	const immutable = "page.upgrade_immutable"
	t.Run("g1", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n\nText.\n"})
		const written = "---\ntype: note\nsummary: The plan.\n---\n# Plan\n\nText.\n"
		g.done(t, "the write of Plan's frontmatter", "write", map[string]any{"path": "Plan.md", "content": written,
			"base_version": g.version(t, "Plan")})
		g.holds(t, "Plan", written)
		g.accepted(t, "Plan")
	})
	t.Run("g2", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n\nText.\n"})
		g.refused(t, "an edit of Plan's body", immutable, "edit", map[string]any{"path": "Plan.md", "old_string": "Text",
			"new_string": "TexT"}, -1)
	})
	t.Run("g3", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n\nText.\n"})
		g.refused(t, "a write of Plan's body", immutable, "write", map[string]any{"path": "Plan.md",
			"content": "---\ntype: note\n---\n# Plan\n\nOther text.\n", "base_version": g.version(t, "Plan")}, -1)
	})
	// g4: a rename of a page that two baseline pages link to: done, the
	// system's rewrite of their links let by, unread, and the ledger each
	// one's hash after it.
	t.Run("g4", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"},
			upgradedPage{title: "Notes", content: "---\ntype: note\n---\nSee [[Plan]].\n"},
			upgradedPage{title: "Guide", content: "Also [[Plan]].\n"})
		g.done(t, "the rename of Plan to Map", "move", map[string]any{"from": "Plan.md", "to": "Map.md"})
		g.holds(t, "Notes", "---\ntype: note\n---\nSee [[Map]].\n")
		g.holds(t, "Guide", "Also [[Map]].\n")
		g.accepted(t, "Notes", "Guide", "Plan")
	})
	// g7: a page the agent created, its body written again; g8: the body
	// of the source root the start seeded, whose protection row is not of
	// the baseline.
	t.Run("g7", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"})
		g.done(t, "the write of New", "write", map[string]any{"path": "New.md", "content": "x\n"})
		g.done(t, "a write of New's body", "write", map[string]any{"path": "New.md", "content": "y\n", "base_version": g.version(t, "New")})
		g.holds(t, "New", "y\n")
	})
	t.Run("g8", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"}, upgradedPage{title: "raw", content: "Sources.\n"})
		g.seeded(t, "raw")
		g.done(t, "a write of raw's body", "write", map[string]any{"path": "raw.md", "content": "Other sources.\n",
			"base_version": g.version(t, "raw")})
		g.holds(t, "raw", "Other sources.\n")
		if got := queryStrings(t, tm.pool, `SELECT coalesce(encode(accepted_hash, 'hex'), '-') FROM upgrade_nodes WHERE upgrade_id = $1
			AND node_id = $2`, g.upgrade, g.ids["raw"]); !slices.Equal(got, []string{"-"}) {
			t.Errorf("raw's protection row has the accepted hash %v, want none", got)
		}
	})
	// g14: a page opening with a lone "---", a valid frontmatter written
	// above it: the old page whole is the body, its bytes kept. g15: an
	// invalid frontmatter with its closing line fixed in place; another
	// pushed below a new block, its body then the old block.
	t.Run("g14", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Old", content: "---\nText\n"})
		g.done(t, "a frontmatter written above Old's lone opening line", "write", map[string]any{"path": "Old.md",
			"content": "---\ntype: x\n---\n---\nText\n", "base_version": g.version(t, "Old")})
		g.holds(t, "Old", "---\ntype: x\n---\n---\nText\n")
		g.accepted(t, "Old")
	})
	t.Run("g15", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Broken", content: "---\na: [\n---\nText\n"},
			upgradedPage{title: "Other", content: "---\na: [\n---\nText\n"})
		g.done(t, "Broken's frontmatter fixed in place", "write", map[string]any{"path": "Broken.md",
			"content": "---\nraw: \"a: [\"\n---\nText\n", "base_version": g.version(t, "Broken")})
		g.holds(t, "Broken", "---\nraw: \"a: [\"\n---\nText\n")
		g.accepted(t, "Broken")
		g.refused(t, "a frontmatter written above Other's", immutable, "write", map[string]any{"path": "Other.md",
			"content": "---\ntype: x\n---\n---\na: [\n---\nText\n", "base_version": g.version(t, "Other")}, -1)
	})
	// g16: a write of the bytes a baseline page holds is done, writing
	// nothing.
	t.Run("g16", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"})
		before := g.rows(t)
		g.done(t, "a write of Plan's bytes", "write", map[string]any{"path": "Plan.md", "content": "# Plan\n",
			"base_version": g.version(t, "Plan")})
		if after := g.rows(t); !slices.Equal(after, before) {
			t.Errorf("a write of Plan's bytes changed the notebook's rows or its upgrade's")
		}
	})
	// g17, g18: a baseline page's body changed by SQL, past the units (the
	// tripwire of M11 design 4.6): the agent's write of its frontmatter
	// alone, which the guard lets by, its body being the one there, and a
	// rename that the system rewrites it for, which the guard does not
	// read, are both a defect's error, internal_error, the ledger's write
	// finding the hash written over not the one it accepted (M11/P2 design
	// 3.5); nothing written, the ledger as it was.
	t.Run("g17", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "---\ntype: a\n---\nBody.\n"})
		g.bypass(t, "Plan", "---\ntype: a\n---\nBody bypassed.\n")
		g.refused(t, "the write of Plan's frontmatter, its body changed past the units", "internal_error", "write",
			map[string]any{"path": "Plan.md", "content": "---\ntype: b\n---\nBody bypassed.\n", "base_version": g.version(t, "Plan")}, -1)
	})
	t.Run("g18", func(t *testing.T) {
		g := tm.upgradedNotebook(t, upgradedPage{title: "Plan", content: "# Plan\n"},
			upgradedPage{title: "Notes", content: "---\ntype: a\n---\nSee [[Plan]].\n"})
		g.bypass(t, "Notes", "---\ntype: a\n---\nSee [[Plan]]. Bypassed.\n")
		g.refused(t, "the rename of Plan, Notes changed past the units", "internal_error", "move",
			map[string]any{"from": "Plan.md", "to": "Map.md"}, -1)
	})

	// g9: a batch's move of A, then an edit of the frontmatter of P, a
	// baseline page linking to A: the move's rewrite folded into P's
	// content, page.upgrade_batch_rewrite at P's last content op. g10: the
	// two the other way round, done, P rewritten by the batch's deferred
	// rewrite, the ledger its hash. g11: a move P links to none of, then an
	// edit of P's body: page.upgrade_immutable, nothing folded. g12: two
	// edits of P's body, the second back: done, the batch writing its last
	// content alone (decision 208).
	const batchRewrite = "page.upgrade_batch_rewrite"
	linked := []upgradedPage{{title: "A", content: "# A\n"}, {title: "B", content: "# B\n"},
		{title: "P", content: "---\ntype: a\n---\nSee [[A]].\n"}}
	t.Run("g9", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of A, then an edit of P's frontmatter", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opEdit("P.md", "type: a", "type: b"), opEdit("P.md", "type: b", "type: c")), 2)
	})
	t.Run("g10", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.done(t, "a batch's edit of P's frontmatter, then a move of A", "batch",
			batched(opEdit("P.md", "type: a", "type: b"), opMove("A.md", "Map.md")))
		g.holds(t, "P", "---\ntype: b\n---\nSee [[Map]].\n")
		g.accepted(t, "P")
	})
	t.Run("g11", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of B, then an edit of P's body", immutable, "batch",
			batched(opMove("B.md", "Map.md"), opEdit("P.md", "See", "Read")), 1)
	})
	t.Run("g12", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.done(t, "a batch's two edits of P's body, the second back", "batch",
			batched(opEdit("P.md", "See", "Read"), opEdit("P.md", "Read", "See")))
		g.holds(t, "P", "---\ntype: a\n---\nSee [[A]].\n")
	})
	// g19-g21: the coarse judgment of decisions 265 and 268. g19: a move of
	// A, then an edit of P's body: page.upgrade_batch_rewrite; split, the
	// edit is page.upgrade_immutable. g20: a write of P's body linking to A
	// and B, their moves, an edit of its frontmatter:
	// page.upgrade_batch_rewrite; split, the write is
	// page.upgrade_immutable. g21: a move of A, then a write of P of a new
	// frontmatter and the rewritten body: page.upgrade_batch_rewrite, the
	// move having reached P, whatever the write; in two batches, the move
	// then the write, done.
	t.Run("g19", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of A, then an edit of P's body", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opEdit("P.md", "See", "Read")), 1)
		g.refused(t, "a batch's edit of P's body, then a move of A", immutable, "batch",
			batched(opEdit("P.md", "See", "Read"), opMove("A.md", "Map.md")), 0)
	})
	t.Run("g20", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		written := opWriteOn("P.md", "---\ntype: a\n---\nNew [[A]] and [[B]].\n", g.version(t, "P"))
		g.refused(t, "a batch's write of P's body, moves of A and B, an edit of P's frontmatter", batchRewrite, "batch",
			batched(written, opMove("A.md", "Map.md"), opMove("B.md", "Key.md"), opEdit("P.md", "type: a", "type: b")), 3)
		g.refused(t, "a batch's write of P's body alone", immutable, "batch", batched(written), 0)
	})
	t.Run("g21", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		const content = "---\ntype: b\n---\nSee [[Map]].\n"
		g.refused(t, "a batch's move of A, then a write of P's new frontmatter and rewritten body", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opWriteOn("P.md", content, g.version(t, "P"))), 1)
		g.done(t, "a batch's move of A", "batch", batched(opMove("A.md", "Map.md")))
		g.done(t, "then a batch's write of P", "batch", batched(opWriteOn("P.md", content, g.version(t, "P"))))
		g.holds(t, "P", content)
		g.accepted(t, "P")
	})
	// g23-g25: decision 268, a baseline page a rename earlier in the batch
	// rewrote, then written, whatever the write, is
	// page.upgrade_batch_rewrite at its last content op, nothing written;
	// else the batch's deferred rewrite would not follow the rename, which
	// comes before the page's last content op, and the page's links would
	// stay on the old name. g23: the link broken: a write of P's new
	// frontmatter over the body it holds, and a write of the very bytes it
	// holds, which writes nothing; split, the write and then the move, the
	// rename rewrites P. g24: the link led elsewhere: a new A.md written
	// after the rename, then P written the same. g25: the edit undoing the
	// rewrite folded into P, its content then the bytes P holds.
	unwritten := "---\ntype: b\n---\nSee [[A]].\n"
	t.Run("g23", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of A, then a write of P's new frontmatter over its body", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opWriteOn("P.md", unwritten, g.version(t, "P"))), 1)
		g.refused(t, "a batch's move of A, then a write of the bytes P holds", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opWriteOn("P.md", "---\ntype: a\n---\nSee [[A]].\n", g.version(t, "P"))), 1)
		g.done(t, "a batch's write of P's new frontmatter, then the move of A", "batch",
			batched(opWriteOn("P.md", unwritten, g.version(t, "P")), opMove("A.md", "Map.md")))
		g.holds(t, "P", "---\ntype: b\n---\nSee [[Map]].\n")
		g.accepted(t, "P")
	})
	t.Run("g24", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of A, a new A, then a write of P", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opWrite("A.md", "# Another A\n"), opWriteOn("P.md", unwritten, g.version(t, "P"))), 2)
	})
	t.Run("g25", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		g.refused(t, "a batch's move of A, then an edit of P undoing the rewrite", batchRewrite, "batch",
			batched(opMove("A.md", "Map.md"), opEdit("P.md", "[[Map]]", "[[A]]")), 1)
	})
	// g26: N, a page the agent wrote since the start, not of the baseline,
	// linking to A: a batch's move of A, then N's own bytes, which the unit
	// writes not but has the guards judge, other bytes, an edit undoing the
	// rewrite folded in, each done, N holding what the agent wrote; P, of
	// the baseline, which the batch does not write, rewritten by its
	// deferred rewrite. A guard refusing a reached page before it reads
	// whether it is of the baseline fails here (review P2S1C-N5).
	t.Run("g26", func(t *testing.T) {
		const n = "See [[A]].\n"
		for _, tt := range []struct {
			what  string
			op    func(g upgradedNotebook) map[string]any
			holds string
		}{
			{"N's own bytes", func(g upgradedNotebook) map[string]any { return opWriteOn("N.md", n, g.version(t, "N")) }, n},
			{"other bytes", func(g upgradedNotebook) map[string]any {
				return opWriteOn("N.md", "---\ntype: x\n---\n"+n, g.version(t, "N"))
			}, "---\ntype: x\n---\n" + n},
			{"an edit undoing the rewrite", func(upgradedNotebook) map[string]any { return opEdit("N.md", "[[Map]]", "[[A]]") }, n},
		} {
			g := tm.upgradedNotebook(t, linked...)
			g.done(t, "N written", "write", map[string]any{"path": "N.md", "content": n})
			g.done(t, "a batch's move of A, then "+tt.what+" of N", "batch", batched(opMove("A.md", "Map.md"), tt.op(g)))
			g.holds(t, "N", tt.holds)
			g.holds(t, "P", "---\ntype: a\n---\nSee [[Map]].\n")
		}
	})
	// g27: decision 270, P's link taken out by an edit, A renamed, the link
	// put back: the rename reaches no link P holds in the batch then, but
	// one it had before it, by the index; page.upgrade_batch_rewrite at
	// P's last op, nothing written, the link broken, or led to a new A
	// written after the rename. Put back before the rename, the batch is
	// done, the rename rewriting P.
	t.Run("g27", func(t *testing.T) {
		g := tm.upgradedNotebook(t, linked...)
		out, back := opEdit("P.md", "[[A]]", "(A)"), opEdit("P.md", "(A)", "[[A]]")
		g.refused(t, "a batch taking P's link out, renaming A, putting it back", batchRewrite, "batch",
			batched(out, opMove("A.md", "Map.md"), back), 2)
		g.refused(t, "the same, a new A written after the rename", batchRewrite, "batch",
			batched(out, opMove("A.md", "Map.md"), opWrite("A.md", "# Another A\n"), back), 3)
		g.done(t, "a batch taking P's link out and putting it back, then renaming A", "batch",
			batched(out, back, opMove("A.md", "Map.md")))
		g.holds(t, "P", "---\ntype: a\n---\nSee [[Map]].\n")
		g.accepted(t, "P")
		// A link to A the batch adds, in Q's frontmatter, which the index
		// has not before the batch: the contents' pass alone marks Q, the
		// rename rewriting the link in the batch (decision 268); an edit
		// after the rename, or a write of the link on the old name,
		// page.upgrade_batch_rewrite at Q's last op.
		q := tm.upgradedNotebook(t, append(slices.Clone(linked), upgradedPage{title: "Q", content: "---\ntype: a\n---\nQ.\n"})...)
		add, linking := opEdit("Q.md", "type: a", "type: a\nsee: \"[[A]]\""), "---\ntype: a\nsee: \"[[A]]\"\n---\nQ.\n"
		q.refused(t, "a batch adding a link to A to Q, renaming A, editing Q", batchRewrite, "batch",
			batched(add, opMove("A.md", "Map.md"), opEdit("Q.md", "type: a", "type: b")), 2)
		q.refused(t, "the same, writing Q's link on the old name", batchRewrite, "batch",
			batched(add, opMove("A.md", "Map.md"), opWriteOn("Q.md", linking, q.version(t, "Q"))), 2)
	})

	// g22: a batch's blind writes of three baseline pages, each taking off
	// a frontmatter of some 5 MiB, which the guard would read whole: their
	// heads pass the reads' bound at the third, page.batch_too_large there,
	// nothing written (M11/P2 decision 266); the same batch in a notebook
	// not upgraded is done. The pages are written by SQL, a request's
	// parse of 15 MiB being past the test's deadline under the race
	// detector (05-lessons 1).
	t.Run("g22", func(t *testing.T) {
		big := func(name string) sqlPage {
			body := "\n---\nBody.\n"
			return sqlPage{"", name, "---\nk: " + strings.Repeat("x", page.MaxContentBytes-len("---\nk: ")-len(body)-100) + body}
		}
		ops := func(ids []string) []map[string]any {
			var out []map[string]any
			for i, id := range ids {
				out = append(out, opWriteOn(fmt.Sprint("P", i, ".md"), "---\ntype: x\n---\nBody.\n", id+":1"))
			}
			return out
		}
		seeded := func(nb string) []string {
			seedPagesSQL(t, tm.pool, nb, []sqlPage{big("P0"), big("P1"), big("P2")})
			return queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 ORDER BY name", nb)
		}
		g := upgradedNotebook{tm: tm, id: tm.openNotebook(t, "alice", "g22"), ids: map[string]string{}}
		ids := seeded(g.id)
		g.upgrade = tm.upgrade(t, g.id, "alice", "upgrading")
		g.agent = tm.agent(t, "alice", g.id)
		g.refused(t, "a batch's blind writes of three baseline pages of 5 MiB", "page.batch_too_large", "batch", batched(ops(ids)...), 2)
		standard := tm.openNotebook(t, "alice", "g22 standard")
		m := *g.agent
		m.notebook = standard
		if r := m.use(t, "batch", batched(ops(seeded(standard))...)).result(t); r["isError"] == true {
			t.Errorf("the same batch in a notebook not upgraded = %v, want it done", r)
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

// version is the version token of the page titled title now.
func (g upgradedNotebook) version(t *testing.T, title string) string {
	t.Helper()
	id := g.idOf(t, title)
	return id + ":" + queryStrings(t, g.tm.pool, "SELECT revision::text FROM page_contents WHERE node_id = $1", id)[0]
}

// idOf is the id of the page titled title, alive in the notebook.
func (g upgradedNotebook) idOf(t *testing.T, title string) string {
	t.Helper()
	if id, ok := g.ids[title]; ok {
		return id
	}
	ids := queryStrings(t, g.tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = $2 AND deleted_at IS NULL", g.id, title)
	if len(ids) != 1 {
		t.Fatalf("the pages titled %s: %v", title, ids)
	}
	g.ids[title] = ids[0]
	return ids[0]
}

// holds fails t unless the page titled title holds content.
func (g upgradedNotebook) holds(t *testing.T, title, content string) {
	t.Helper()
	if got := queryStrings(t, g.tm.pool, "SELECT content FROM page_contents WHERE node_id = $1", g.idOf(t, title)); len(got) != 1 ||
		got[0] != content {
		t.Errorf("%s holds %q, want %q", title, got, content)
	}
}

// accepted fails t unless the upgrade's ledger accepted the hash of the
// content each page titled titles holds.
func (g upgradedNotebook) accepted(t *testing.T, titles ...string) {
	t.Helper()
	for _, title := range titles {
		if n := count(t, g.tm.pool, `SELECT count(*) FROM upgrade_nodes b JOIN page_contents c ON c.node_id = b.node_id
			WHERE b.upgrade_id = $1 AND b.node_id = $2 AND b.accepted_hash = c.content_hash`, g.upgrade, g.idOf(t, title)); n != 1 {
			t.Errorf("the ledger accepted %s's hash %d times, want once", title, n)
		}
	}
}

// seeded has the page titled title stand for the source root the
// upgrade's start created (M11/P1 design 3.12): its row protects it, not
// of the baseline.
func (g upgradedNotebook) seeded(t *testing.T, title string) {
	t.Helper()
	g.tm.exec(t, `UPDATE upgrade_nodes SET baseline = false, parent_id = NULL, name = NULL, sort_order = NULL, revision = NULL,
		content_hash = NULL, byte_size = NULL, wanted = NULL, accepted_hash = NULL WHERE upgrade_id = $1 AND node_id = $2`,
		g.upgrade, g.ids[title])
	g.tm.exec(t, `UPDATE notebook_upgrades SET seeded_root = true, source_root_id = $2, baseline_nodes = baseline_nodes - 1,
		baseline_pages = baseline_pages - 1 WHERE id = $1`, g.upgrade, g.ids[title])
}

// bypass writes content to the page titled title by SQL, past the units:
// its revision, its index and the ledger as they were.
func (g upgradedNotebook) bypass(t *testing.T, title, content string) {
	t.Helper()
	g.tm.exec(t, `UPDATE page_contents SET content = $2, content_hash = sha256(convert_to($2, 'UTF8')), byte_size = octet_length($2)
		WHERE node_id = $1`, g.ids[title], content)
}

// done is the agent's call of tool with args, which must be done.
func (g upgradedNotebook) done(t *testing.T, what, tool string, args map[string]any) {
	t.Helper()
	if r := g.agent.use(t, tool, args).result(t); r["isError"] == true {
		t.Errorf("%s = %v, want it done", what, r)
	}
}
