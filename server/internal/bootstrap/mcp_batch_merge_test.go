package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

// itemsIn are the items of the changeset id, each its node, its place
// before and after as parent/name ("-" for none) and its revision ("-").
func (tm acmeTeam) itemsIn(t *testing.T, id string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT node_id::text || ' ' || coalesce(coalesce(before_parent_id::text, '') || '/' || before_name, '-')
			|| ' ' || coalesce(coalesce(after_parent_id::text, '') || '/' || after_name, '-') || ' ' || coalesce(revision::text, '-')
		FROM changeset_items WHERE changeset_id = $1 ORDER BY 1`, id)
}

// versionsIn are the versions the changeset id holds, each its page, its
// base ("-" for none), its revision and its content.
func (tm acmeTeam) versionsIn(t *testing.T, id string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT node_id::text || ' ' || coalesce(base_revision::text, '-') || ' ' || revision || ' ' || content
		FROM page_revisions WHERE changeset_id = $1 ORDER BY 1`, id)
}

// Each kind of a batch's merges holds on the database (M9/P3 design
// 3.14.2): through MCP's batch, its changeset's items and versions, read
// back. A page created and moved is one item of its creation where it
// ended, one version, the write's created at its path after the batch, as a
// folder's created and renamed; created and edited, one version of the
// edit; renamed and written, or written and renamed, one item and one
// version on the revision before the batch; written and deleted, the
// deletion's item at the batch's revision. A page's descendant deleted,
// then the page, is one deletion, its root the page, the first op merged
// into the second. The deferred rewrite of a page the batch wrote updates
// its version, two past the batch's base, which the edit answers; of one it
// created, its version 2, its item 1. A page renamed and back is no item
// and, alone, no change; with another page's edit, the edit alone, no
// reorder in the history; with its own edit between, the edit, its index of
// its new content. A write of the bytes a page holds on a stale version
// changes nothing, as a write alone. The invariants hold after each, and no
// line is logged at ERROR.
func TestTheMergesOfABatchHoldOnTheDatabase(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	notebooks := 0
	each := func(name string, pages []restPage, test func(t *testing.T, ids map[string]string, m *mcpApp)) {
		t.Run(name, func(t *testing.T) {
			notebooks++
			nb := tm.openNotebook(t, "alice", fmt.Sprint("Eng ", notebooks))
			test(t, tm.seed(t, nb, pages...), agent.in(nb))
			checkPages(t, tm.pool)
			checkLinks(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
	// in fails t unless the batch's items and versions are those of want.
	in := func(t *testing.T, b mcpBatched, items, versions []string) {
		t.Helper()
		if b.ChangesetID == nil {
			t.Fatalf("the batch = %+v, want a changeset", b)
		}
		slices.Sort(items)
		slices.Sort(versions)
		if got := tm.itemsIn(t, *b.ChangesetID); !slices.Equal(got, items) {
			t.Errorf("the changeset's items are %q, want %q", got, items)
		}
		if got := tm.versionsIn(t, *b.ChangesetID); !slices.Equal(got, versions) {
			t.Errorf("the changeset's versions are %q, want %q", got, versions)
		}
	}

	each("created, then renamed and moved", []restPage{{"F", ""}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opWrite("X.md", "x"), opMove("X.md", "F/Y.md"))
		x := b.Results[0].NodeID
		in(t, b, []string{x + " - " + ids["F"] + "/Y 1"}, []string{x + " - 1 x"})
		sum := sha256.Sum256([]byte("x"))
		if got := queryStrings(t, tm.pool, "SELECT encode(content_hash, 'hex') FROM page_revisions WHERE node_id = $1", x); !slices.Equal(got,
			[]string{hex.EncodeToString(sum[:])}) {
			t.Errorf("the version's hash is %q, want the content's", got)
		}
		if b.Results[0].Path != "F/Y.md" || b.Results[1].Path != "F/Y.md" {
			t.Errorf("the results' paths are %q and %q, want F/Y.md", b.Results[0].Path, b.Results[1].Path)
		}
		if c := b.Results[0].Created; len(c) != 1 || c[0].Path != "F/Y.md" || c[0].NodeID != x {
			t.Errorf("the write created %+v, want X at F/Y.md, where the batch left it (M9/P3 review C-M1)", c)
		}
	})
	each("a folder created, then renamed", nil, func(t *testing.T, _ map[string]string, m *mcpApp) {
		b := m.batchDone(t, opWrite("N/X.md", "x"), opMove("N.md", "M.md"))
		var paths []string
		for _, c := range b.Results[0].Created {
			paths = append(paths, c.Path)
		}
		if !slices.Equal(paths, []string{"M.md", "M/X.md"}) || b.Results[0].Path != "M/X.md" || b.Results[1].Path != "M.md" {
			t.Errorf("the write created %v at %s, the move %s; want M.md and M/X.md, where the batch left them", paths,
				b.Results[0].Path, b.Results[1].Path)
		}
	})
	each("created, then edited", nil, func(t *testing.T, _ map[string]string, m *mcpApp) {
		b := m.batchDone(t, opWrite("X.md", "a"), opEdit("X.md", "a", "b"))
		x := b.Results[0].NodeID
		in(t, b, []string{x + " - /X 1"}, []string{x + " - 1 b"})
	})
	each("renamed, then written", []restPage{{"A", "a"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opMove("A.md", "B.md"), opWriteOn("B.md", "b", versionOf(ids["A"], 1)))
		in(t, b, []string{ids["A"] + " /A /B -"}, []string{ids["A"] + " 1 2 b"})
	})
	each("written, then renamed", []restPage{{"A", "a"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opEdit("A.md", "a", "b"), opMove("A.md", "B.md"))
		in(t, b, []string{ids["A"] + " /A /B -"}, []string{ids["A"] + " 1 2 b"})
	})
	each("written, then deleted", []restPage{{"A", "a"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opEdit("A.md", "a", "b"), opDelete("A.md"))
		in(t, b, []string{ids["A"] + " /A - 2"}, []string{ids["A"] + " 1 2 b"})
	})
	each("a descendant deleted, then its ancestor", []restPage{{"F", ""}, {"F/X", ""}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opDelete("F/X.md"), opDeleteAll("F.md"))
		if r := b.Results; r[0].MergedInto == nil || *r[0].MergedInto != 1 || r[1].MergedInto != nil || *r[1].Pages != 2 {
			t.Errorf("the results = %+v, want the first merged into the second, which deletes both", r)
		}
		f, x := ids["F"], ids["F/X"]
		if got := queryStrings(t, tm.pool, `SELECT x.trash_root || ' ' || f.trash_root || ' ' || (x.deleted_at = f.deleted_at) || ' ' || f.trash_pages
			FROM nodes x, nodes f WHERE x.id = $1 AND f.id = $2`, x, f); got[0] != "false true true 2" {
			t.Errorf("X's trash root, F's, deleted at one time, F's pages = %s; want one deletion of F's, with X", got[0])
		}
		in(t, b, []string{f + " /F - 1", x + " " + f + "/X - 1"}, nil)
	})
	each("the deferred rewrite of a page the batch wrote", []restPage{{"A", ""}, {"P", "p"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opEdit("P.md", "p", "p [[A]]"), opMove("A.md", "B.md"))
		in(t, b, []string{ids["A"] + " /A /B -"}, []string{ids["P"] + " 1 3 p [[B]]"})
		if v := b.Results[0].Version; v == nil || *v != versionOf(ids["P"], 3) {
			t.Errorf("the edit answered the version %v, want P's after the rewrite, 3 (M9/P3 review C-N2)", v)
		}
	})
	each("the deferred rewrite of a page the batch created", []restPage{{"A", ""}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opWrite("N.md", "[[A]]"), opMove("A.md", "B.md"))
		n := b.Results[0].NodeID
		in(t, b, []string{n + " - /N 1", ids["A"] + " /A /B -"}, []string{n + " - 2 [[B]]"})
	})
	each("renamed and back", []restPage{{"A", ""}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		nb := m.notebook
		logs, sets := len(structured(t, m.use(t, "log", map[string]any{}))["changesets"].([]any)), tm.changesetsIn(t, nb)
		b := m.batchDone(t, opMove("A.md", "B.md"), opMove("B.md", "A.md"))
		if b.Changed || b.ChangesetID != nil || b.At != nil || !b.Results[0].Changed || !b.Results[1].Changed {
			t.Errorf("the batch = %+v, want no change, each op changed", b)
		}
		if now := len(structured(t, m.use(t, "log", map[string]any{}))["changesets"].([]any)); now != logs || tm.changesetsIn(t, nb) != sets {
			t.Errorf("the log has %d changesets, the notebook %d; want %d, as before", now, tm.changesetsIn(t, nb), logs)
		}
	})
	each("renamed and back, another page edited", []restPage{{"A", ""}, {"C", "c"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opMove("A.md", "B.md"), opMove("B.md", "A.md"), opEdit("C.md", "c", "c2"))
		in(t, b, nil, []string{ids["C"] + " 1 2 c2"})
		changes, _ := structured(t, m.use(t, "changeset", map[string]any{"id": *b.ChangesetID}))["changes"].([]any)
		if len(changes) != 1 || changes[0].(map[string]any)["kind"] == "reordered" ||
			changes[0].(map[string]any)["node"].(map[string]any)["id"] != ids["C"] {
			t.Errorf("the changeset's changes are %v, want C's edit alone", changes)
		}
	})
	each("renamed and back, edited between", []restPage{{"A", "a"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		b := m.batchDone(t, opMove("A.md", "B.md"), opEdit("B.md", "a", "a [[Z]]"), opMove("B.md", "A.md"))
		if !b.Changed || !b.Results[1].Changed {
			t.Errorf("the batch = %+v, want it and its edit changed", b)
		}
		in(t, b, nil, []string{ids["A"] + " 1 2 a [[Z]]"})
		tm.resolves(t, ids["A"], "")
	})
	each("a stale version, the same bytes", []restPage{{"A", "a"}, {"C", "c"}}, func(t *testing.T, ids map[string]string, m *mcpApp) {
		tm.send(t, contentWrite("alice", ids["A"], "b", 1, ""), http.StatusOK)
		b := m.batchDone(t, opWriteOn("A.md", "b", versionOf(ids["A"], 1)), opEdit("C.md", "c", "c2"))
		if r := b.Results[0]; r.Changed || r.Version == nil || *r.Version != versionOf(ids["A"], 2) {
			t.Errorf("ops[0] = %+v, want no change, A's version now, 2", r)
		}
		in(t, b, nil, []string{ids["C"] + " 1 2 c2"})
		if n := count(t, tm.pool, "SELECT count(*) FROM page_revisions WHERE node_id = $1", ids["A"]); n != 2 {
			t.Errorf("A has %d versions, want 2: its creation's and REST's write", n)
		}
	})
}
