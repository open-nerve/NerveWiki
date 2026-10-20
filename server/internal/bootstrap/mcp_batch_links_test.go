package bootstrap

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The links of a batch through serve (M9/P3 design 3.14.3, 3.14.4): two
// notebooks alike, made by the same writes through REST, one changed by a
// batch, the other by the same ops as separate calls, in their order.

// pageState is a page as these tests compare it: its content, and where
// its links lead, in their order, by the paths of the nodes they resolve
// to ("-" for none).
type pageState struct{ content, leads string }

// linkedState is the notebook nb's pages alive, by path.
func (tm acmeTeam) linkedState(t *testing.T, nb string) map[string]pageState {
	t.Helper()
	paths, out := map[string]string{}, map[string]pageState{}
	rows, err := tm.pool.Query(t.Context(), `WITH RECURSIVE p AS (
			SELECT id, kind, name::text AS path FROM nodes WHERE notebook_id = $1 AND parent_id IS NULL AND deleted_at IS NULL
			UNION ALL
			SELECT n.id, n.kind, p.path || '/' || n.name FROM nodes n JOIN p ON n.parent_id = p.id WHERE n.deleted_at IS NULL)
		SELECT p.id::text, p.path || CASE WHEN p.kind = 'page' THEN '.md' ELSE '' END, coalesce(c.content, '')
		FROM p LEFT JOIN page_contents c ON c.node_id = p.id AND c.deleted_at IS NULL`, nb)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, path, content string
		if err := rows.Scan(&id, &path, &content); err != nil {
			t.Fatal(err)
		}
		paths[id], out[path] = path, pageState{content: content}
	}
	if rows.Close(); rows.Err() != nil {
		t.Fatal(rows.Err())
	}
	leads := map[string][]string{}
	for _, row := range queryStrings(t, tm.pool, `SELECT source_id::text || ' ' || coalesce(resolved_id::text, '-') FROM page_links
		WHERE notebook_id = $1 ORDER BY source_id, range_start`, nb) {
		source, resolved, _ := strings.Cut(row, " ")
		if p, ok := paths[resolved]; ok {
			resolved = p
		}
		leads[paths[source]] = append(leads[paths[source]], resolved)
	}
	for path, l := range leads {
		s := out[path]
		s.leads = strings.Join(l, ", ")
		out[path] = s
	}
	return out
}

// shown is a notebook's state, a page a line, by path.
func shown(state map[string]pageState) string {
	var b strings.Builder
	for _, path := range slices.Sorted(maps.Keys(state)) {
		fmt.Fprintf(&b, "  %s: %q → %s\n", path, state[path].content, state[path].leads)
	}
	return b.String()
}

// twins opens two notebooks alike for the case i, each made of pages
// through REST: one for the batch, one for the separate calls.
func (tm acmeTeam) twins(t *testing.T, i int, pages []restPage) (batch, apart string) {
	t.Helper()
	batch, apart = tm.openNotebook(t, "alice", fmt.Sprint("Batch ", i)), tm.openNotebook(t, "alice", fmt.Sprint("Apart ", i))
	tm.seed(t, batch, pages...)
	tm.seed(t, apart, pages...)
	return batch, apart
}

// titled is the pages of B1-M1's cases (M9/P3 review), and more: A/B/C/N,
// which a link [[N]] leads to, and the folder Top/F of X to move under G,
// or, renamed, the folder F of X to rename. A page N created under G/F or
// G is nearer the root, and takes [[N]] by its shorter path.
func titled(renamed bool, more ...restPage) []restPage {
	pages := []restPage{{"A", ""}, {"A/B", ""}, {"A/B/C", ""}, {"A/B/C/N", ""}, {"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""}}
	if renamed {
		pages = append(pages[:4], restPage{"F", ""}, restPage{"F/X", ""})
	}
	return append(pages, more...)
}

// The links of a batch are those of separate calls (M9/P3 design 3.14.3):
// for each case, each page's bytes and where each of its links leads are
// the same in both notebooks, and as the case says; the batch is one
// changeset where the case says so; nothing is logged at ERROR. Its cases:
// (1) content, then a move, and (2) a move, then content whose old_string
// holds the link rewritten; (3) a rename, content and a rename; (4) nested
// renames in both orders; (5) a page moved out of a folder deleted after,
// its Markdown link's text; (6) a page linking to one renamed, deleted;
// (7) a page of an alias deleted, another renamed to it, and then an edit
// of the linker; (8) a target deleted, its key taken; (9) a page created,
// linked, renamed and moved; (10) one created with a link, its target
// moved; (11) a rename and back with content between; (12) a move out and
// back; (13) a page created, renamed and back; (14) a link of a written
// page taken by a creation after, and so when a rename after has the
// page followed, its other links not written back; (15) two renames in
// one unit; (16) two recases; (17) a creation renamed, taking a link of a
// page not written, and (18) of a written one; (19) a written page's link
// into a folder moved after; (20) a page created under a folder moved
// before, which takes a link of its title, the linker edited after it,
// its old_string with the link or without, or created where nothing
// moved, and (21) edited before the move: the creation resolves the link
// again, none is written back where it led (M9 design 4.11; M9/P3 review
// B1-M1, its K1r, K1r2, K2 and the control K0); and when the page is
// created before the move, which carries it, its title's link not taken
// by the creation (the paths equally long, the older page's id the less),
// the move's rewrite keeps it; (22) such a page under a folder renamed,
// and the rename with update_links false (K3b and the control K3c).
func TestTheLinksOfABatchAreThoseOfSeparateCalls(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	const aliased = "---\naliases: [Roadmap]\n---\n"
	for i, tt := range []struct {
		name  string
		pages []restPage
		ops   []map[string]any
		want  map[string]string // the contents some pages hold after
		one   bool              // the batch is one changeset
	}{
		{"1 content, then a move", []restPage{{"A", ""}, {"P", "p"}},
			[]map[string]any{opEdit("P.md", "p", "p [[A]]"), opMove("A.md", "B.md")}, map[string]string{"P.md": "p [[B]]"}, true},
		{"2 a move, then content of the link rewritten", []restPage{{"A", ""}, {"P", "[[A]] x"}},
			[]map[string]any{opMove("A.md", "B.md"), opEdit("P.md", "[[B]] x", "[[B]] y")}, map[string]string{"P.md": "[[B]] y"}, true},
		{"3 a rename, content and a rename", []restPage{{"A", ""}, {"P", "[[A]] x"}},
			[]map[string]any{opMove("A.md", "B.md"), opEdit("P.md", "x", "y"), opMove("B.md", "C.md")}, map[string]string{"P.md": "[[C]] y"}, true},
		{"4 a folder renamed, then its page", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[[F/X]] [[X]]"}},
			[]map[string]any{opMove("F.md", "G.md"), opMove("G/X.md", "G/Y.md")}, nil, false},
		{"4 a page renamed, then its folder", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[[F/X]] [[X]]"}},
			[]map[string]any{opMove("F/X.md", "F/Y.md"), opMove("F.md", "G.md")}, nil, false},
		{"5 a page moved out of a folder, deleted after", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[F/X](F/X.md)"}},
			[]map[string]any{opMove("F/X.md", "X.md"), opDelete("F.md")}, map[string]string{"Q.md": "[X](X.md)"}, false},
		{"6 a page linking to one renamed, deleted", []restPage{{"A", ""}, {"P", "[[A]]"}, {"Q", "[[A]]"}},
			[]map[string]any{opMove("A.md", "B.md"), opDelete("P.md")}, map[string]string{"Q.md": "[[B]]"}, false},
		{"7 a page of an alias deleted, another renamed to it", []restPage{{"P", aliased}, {"Plan", ""}, {"Q", "[[Roadmap]] [[Plan]]"}},
			[]map[string]any{opDelete("P.md"), opMove("Plan.md", "Roadmap.md")}, nil, false},
		{"7 and the linker edited after", []restPage{{"P", aliased}, {"Plan", ""}, {"Q", "[[Roadmap]] [[Plan]]"}},
			[]map[string]any{opDelete("P.md"), opMove("Plan.md", "Roadmap.md"), opEdit("Q.md", "[[Roadmap]] [[Roadmap]]",
				"[[Roadmap]] [[Roadmap]] more")}, map[string]string{"Q.md": "[[Roadmap]] [[Roadmap]] more"}, false},
		{"8 a target deleted, its key taken", []restPage{{"T", ""}, {"A", ""}, {"Q", "[[T]]"}},
			[]map[string]any{opDelete("T.md"), opMove("A.md", "T.md")}, map[string]string{"Q.md": "[[T]]"}, false},
		{"9 a page created, linked, renamed and moved", []restPage{{"F", ""}, {"P", "p"}},
			[]map[string]any{opWrite("X.md", "x"), opEdit("P.md", "p", "p [[X]]"), opMove("X.md", "F/Y.md")}, map[string]string{"P.md": "p [[Y]]"}, true},
		{"10 a page created with a link, its target moved", []restPage{{"A", ""}},
			[]map[string]any{opWrite("N.md", "[[A]]"), opMove("A.md", "B.md")}, map[string]string{"N.md": "[[B]]"}, true},
		{"11 a rename and back, content between", []restPage{{"A", ""}, {"P", "p"}},
			[]map[string]any{opMove("A.md", "B.md"), opEdit("P.md", "p", "p [[B]]"), opMove("B.md", "A.md")}, map[string]string{"P.md": "p [[A]]"}, true},
		{"12 a move out and back, content between", []restPage{{"Archive", ""}, {"P", ""}, {"Q", "q"}},
			[]map[string]any{opMove("P.md", "Archive/P.md"), opEdit("Q.md", "q", "q [[Archive/P]]"), opMove("Archive/P.md", "P.md")},
			map[string]string{"Q.md": "q [[P]]"}, false},
		{"13 a page created, renamed and back", []restPage{{"P", "p"}},
			[]map[string]any{opWrite("X.md", "x"), opMove("X.md", "Y.md"), opEdit("P.md", "p", "p [[Y]]"), opMove("Y.md", "X.md")},
			map[string]string{"P.md": "p [[X]]"}, false},
		{"14 a written page's link taken by a creation after", []restPage{{"F", ""}, {"F/X", ""}},
			[]map[string]any{opWrite("W.md", "[[X]]"), opWrite("X.md", "x")}, map[string]string{"W.md": "[[X]]"}, false},
		{"14 and the written page followed for a rename after", []restPage{{"F", ""}, {"F/X", ""}, {"A", ""}},
			[]map[string]any{opWrite("W.md", "[[X]] [[A]]"), opWrite("X.md", "x"), opMove("A.md", "B.md")},
			map[string]string{"W.md": "[[X]] [[B]]"}, false},
		{"15 two renames", []restPage{{"A", ""}, {"C", ""}, {"Q", "[[A]] [[C]]"}},
			[]map[string]any{opMove("A.md", "B.md"), opMove("C.md", "D.md")}, map[string]string{"Q.md": "[[B]] [[D]]"}, false},
		{"16 two recases", []restPage{{"Notes", ""}, {"Plan", ""}, {"Q", "[[Notes]] [[Plan]]"}},
			[]map[string]any{opMove("Notes.md", "notes.md"), opMove("Plan.md", "plan.md")}, map[string]string{"Q.md": "[[notes]] [[plan]]"}, false},
		{"17 a creation renamed, taking a link", []restPage{{"G", ""}, {"G/Y", ""}, {"Q", "[[Y]]"}},
			[]map[string]any{opWrite("X.md", "x"), opMove("X.md", "Y.md")}, map[string]string{"Q.md": "[[G/Y]]"}, false},
		{"18 a creation renamed, taking a written page's link", []restPage{{"G", ""}, {"G/Y", ""}, {"W", "w"}},
			[]map[string]any{opEdit("W.md", "w", "w [[Y]]"), opWrite("X.md", "x"), opMove("X.md", "Y.md")},
			map[string]string{"W.md": "w [[G/Y]]"}, false},
		{"19 a written page's link into a folder moved after", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""}, {"P", "p"}},
			[]map[string]any{opEdit("P.md", "p", "[[Top/F/X]] more"), opMove("Top/F.md", "G/F.md")}, map[string]string{"P.md": "[[X]] more"}, false},
		{"20 a page created under a folder moved, its title's linker edited after (K1r)", titled(false, restPage{"R", "[[N]] r"}, restPage{"Q", "[[N]] q"}),
			[]map[string]any{opMove("Top/F.md", "G/F.md"), opWrite("G/F/N.md", "n"), opEdit("R.md", "[[N]] r", "[[N]] r2")},
			map[string]string{"R.md": "[[N]] r2", "Q.md": "[[N]] q"}, true},
		{"20 and the edit's old_string without the link (K1r2)", titled(false, restPage{"R", "[[N]] r"}),
			[]map[string]any{opMove("Top/F.md", "G/F.md"), opWrite("G/F/N.md", "n"), opEdit("R.md", "r", "r2")},
			map[string]string{"R.md": "[[N]] r2"}, true},
		{"20 created where nothing moved (K0)", titled(false, restPage{"H", ""}, restPage{"R", "[[N]] r"}),
			[]map[string]any{opMove("Top/F.md", "G/F.md"), opWrite("H/N.md", "n"), opEdit("R.md", "[[N]] r", "[[N]] r2")},
			map[string]string{"R.md": "[[N]] r2"}, true},
		{"21 the linker edited before the move (K2)", titled(false, restPage{"P", "p"}),
			[]map[string]any{opEdit("P.md", "p", "p [[N]]"), opMove("Top/F.md", "G/F.md"), opWrite("G/F/N.md", "n")},
			map[string]string{"P.md": "p [[N]]"}, true},
		{"21 created before the move, which carries it", titled(false, restPage{"Q", "[[N]] q"}),
			[]map[string]any{opWrite("Top/F/N.md", "n"), opMove("Top/F.md", "G/F.md")}, map[string]string{"Q.md": "[[A/B/C/N]] q"}, true},
		{"22 a page created under a folder renamed, a linker not written (K3b)", titled(true, restPage{"Q", "[[N]] q"}),
			[]map[string]any{opMove("F.md", "G.md"), opWrite("G/N.md", "n")}, map[string]string{"Q.md": "[[N]] q"}, true},
		{"22 the rename with update_links false (K3c)", titled(true, restPage{"Q", "[[N]] q"}),
			[]map[string]any{opWith(opMove("F.md", "G.md"), "update_links", false), opWrite("G/N.md", "n")},
			map[string]string{"Q.md": "[[N]] q"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			batch, apart := tm.twins(t, i, tt.pages)
			sets := tm.changesetsIn(t, batch)
			agent.in(batch).batchDone(t, tt.ops...)
			agent.in(apart).separately(t, tt.ops...)
			b, a := tm.linkedState(t, batch), tm.linkedState(t, apart)
			if !maps.Equal(b, a) {
				t.Errorf("the batch left\n%sseparate calls\n%s", shown(b), shown(a))
			}
			for path, want := range tt.want {
				if got := b[path].content; got != want {
					t.Errorf("%s after the batch is %q, want %q", path, got, want)
				}
			}
			if now := tm.changesetsIn(t, batch); tt.one && now != sets+1 {
				t.Errorf("the batch made %d changesets, want one", now-sets)
			}
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
}

// The ways a batch differs from separate calls, each as M9 design 4.11's
// rule says (M9/P3 design 3.14.4): links follow the batch's renames and
// moves, merged node by node, from where they led before it, or, in a page
// it writes, at its latest write before them, on the tree then with the
// aliases from before the batch; what other ops did between to where they
// lead is not seen, nor an alias the batch changed. A page not written:
// (1a) a creation before a move, (1b) a dangling link that a rename takes
// before another rename, (1c) a link taken and freed again, (a) a deletion
// that turns a link to another page, (b) an alias taken away; (1a2) a
// creation under a folder before its rename, which the rename reaches as
// separate calls' does, the link the creation took written back where it
// led before the batch (M9/P3 review B1-M1: one created after the rename
// is not reached, 3.14.3's 22). A written
// page: (e1) a creation after its write, (e2) an alias the batch changed,
// (e3) a name merged away. (d) update_links false on a node, its ancestor
// renamed or moved with update_links: the links that led to the node left
// as they are, in every page, whatever its ancestors' changes (M9/P3
// review B-M1), dangling where separate calls write them first, the same
// where the node's op comes first (d1r, d3r); the node's own op with
// update_links, its ancestor's without, followed (d2, d4); the plan's
// rewrite leaving them too, an edit's old_string and a write's bytes
// judged on the links left (d6, d6w). (c) A page written after the plan
// rewrote it. (2) The lock's code.
func TestTheWaysABatchDiffersFromSeparateCalls(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	const aliased = "---\naliases: [Foo]\n---\n"
	cases := []struct {
		name         string
		pages        []restPage
		ops          []map[string]any
		path         string    // the page that differs
		batch, apart pageState // it after the batch, after the calls
	}{
		{"1a a creation before a move", []restPage{{"F", ""}, {"F/X", ""}, {"G", ""}, {"Q", "[[X]]"}},
			[]map[string]any{opWrite("X.md", "x"), opMove("F/X.md", "G/X.md")}, "Q.md", pageState{"[[G/X]]", "G/X.md"}, pageState{"[[X]]", "X.md"}},
		{"1a2 a creation under a folder before its rename", titled(true, restPage{"Q", "[[N]] q"}),
			[]map[string]any{opWrite("F/N.md", "n"), opMove("F.md", "G.md")}, "Q.md", pageState{"[[A/B/C/N]] q", "A/B/C/N.md"},
			pageState{"[[N]] q", "G/N.md"}},
		{"1b a dangling link a rename takes, renamed again", []restPage{{"A", ""}, {"Q", "[[B]]"}},
			[]map[string]any{opMove("A.md", "B.md"), opMove("B.md", "C.md")}, "Q.md", pageState{"[[B]]", "-"}, pageState{"[[C]]", "C.md"}},
		{"1c a link taken and freed", []restPage{{"A", ""}, {"F", ""}, {"F/X", ""}, {"Q", "[[X]]"}},
			[]map[string]any{opMove("A.md", "X.md"), opMove("X.md", "Z.md")}, "Q.md", pageState{"[[X]]", "F/X.md"}, pageState{"[[F/X]]", "F/X.md"}},
		{"a a deletion turns a link", []restPage{{"F", ""}, {"F/X", ""}, {"G", ""}, {"G/H", ""}, {"G/H/X", ""}, {"Q", "[[X]]"}},
			[]map[string]any{opDelete("F/X.md"), opMove("G/H/X.md", "G/H/Y.md")}, "Q.md", pageState{"[[X]]", "-"}, pageState{"[[Y]]", "G/H/Y.md"}},
		{"b an alias taken away", []restPage{{"P", aliased}, {"A", ""}, {"Q", "[[Foo]]"}},
			[]map[string]any{opEdit("P.md", "aliases: [Foo]", "aliases: []"), opMove("A.md", "Foo.md")}, "Q.md",
			pageState{"[[P|Foo]]", "P.md"}, pageState{"[[Foo]]", "Foo.md"}},
		{"e1 a creation after a write", []restPage{{"F", ""}, {"F/X", ""}, {"P", "p"}},
			[]map[string]any{opEdit("P.md", "p", "p [[X]]"), opWrite("X.md", "x"), opMove("F/X.md", "F/Y.md")}, "P.md",
			pageState{"p [[Y]]", "F/Y.md"}, pageState{"p [[X]]", "X.md"}},
		{"e2 an alias the batch changed", []restPage{{"P", aliased}, {"A", ""}, {"Q", "q"}},
			[]map[string]any{opEdit("P.md", "aliases: [Foo]", "aliases: []"), opEdit("Q.md", "q", "q [[Foo]]"), opMove("A.md", "Foo.md")}, "Q.md",
			pageState{"q [[P|Foo]]", "P.md"}, pageState{"q [[Foo]]", "Foo.md"}},
		{"e3 a name merged away", []restPage{{"A", ""}, {"P", "p"}},
			[]map[string]any{opEdit("P.md", "p", "p [[B]]"), opMove("A.md", "B.md"), opMove("B.md", "C.md")}, "P.md",
			pageState{"p [[B]]", "-"}, pageState{"p [[C]]", "C.md"}},
		{"d1 a folder renamed, its page renamed without update_links", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[[F/X]]"}},
			[]map[string]any{opMove("F.md", "G.md"), frozen(opMove("G/X.md", "G/Y.md"))}, "Q.md", pageState{"[[F/X]]", "-"},
			pageState{"[[X]]", "-"}},
		{"d1r its page renamed without update_links, then the folder", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[[F/X]]"}},
			[]map[string]any{frozen(opMove("F/X.md", "F/Y.md")), opMove("F.md", "G.md")}, "Q.md", pageState{"[[F/X]]", "-"},
			pageState{"[[F/X]]", "-"}},
		{"d2 a folder renamed without update_links, its page renamed", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[[F/X]]"}},
			[]map[string]any{frozen(opMove("F.md", "G.md")), opMove("G/X.md", "G/Y.md")}, "Q.md", pageState{"[[Y]]", "G/Y.md"},
			pageState{"[[F/X]]", "-"}},
		{"d3 a folder moved, its page renamed without update_links", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""},
			{"Q", "[[Top/F/X]]"}}, []map[string]any{opMove("Top/F.md", "G/F.md"), frozen(opMove("G/F/X.md", "G/F/Y.md"))}, "Q.md",
			pageState{"[[Top/F/X]]", "-"}, pageState{"[[X]]", "-"}},
		{"d3w the same, of a page the batch wrote", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""}, {"P", "p"}},
			[]map[string]any{opEdit("P.md", "p", "p [[Top/F/X]]"), opMove("Top/F.md", "G/F.md"), frozen(opMove("G/F/X.md", "G/F/Y.md"))},
			"P.md", pageState{"p [[Top/F/X]]", "-"}, pageState{"p [[X]]", "-"}},
		{"d3r its page renamed without update_links, then the folder moved", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""},
			{"G", ""}, {"Q", "[[Top/F/X]]"}}, []map[string]any{frozen(opMove("Top/F/X.md", "Top/F/Y.md")), opMove("Top/F.md", "G/F.md")},
			"Q.md", pageState{"[[Top/F/X]]", "-"}, pageState{"[[Top/F/X]]", "-"}},
		{"d4 a folder moved without update_links, its page renamed", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""},
			{"Q", "[[Top/F/X]]"}}, []map[string]any{frozen(opMove("Top/F.md", "G/F.md")), opMove("G/F/X.md", "G/F/Y.md")}, "Q.md",
			pageState{"[[Y]]", "G/F/Y.md"}, pageState{"[[Top/F/X]]", "-"}},
	}
	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			batch, apart := tm.twins(t, i, tt.pages)
			agent.in(batch).batchDone(t, tt.ops...)
			agent.in(apart).separately(t, tt.ops...)
			if b, a := tm.linkedState(t, batch)[tt.path], tm.linkedState(t, apart)[tt.path]; b != tt.batch || a != tt.apart {
				t.Errorf("%s after the batch = %+v, after the calls = %+v; want %+v and %+v", tt.path, b, a, tt.batch, tt.apart)
			}
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
	t.Run("c a page written after the plan rewrote it", func(t *testing.T) {
		pages := []restPage{{"F", ""}, {"F/X", ""}, {"G", ""}, {"Q", "[[X]]"}}
		batch, apart := tm.twins(t, len(cases), pages)
		ops := []map[string]any{opWrite("X.md", "x"), opMove("F/X.md", "G/X.md"), opEdit("Q.md", "[[X]]", "[[X]] q")}
		was := tm.notebookState(t, batch)
		if r := refusalOf(t, agent.in(batch).batch(t, ops...)); r.Code != "validation_failed" || refusedAt(r) != 2 ||
			tm.notebookState(t, batch) != was {
			t.Errorf("the batch = %+v, want 422 at ops[2], whose old_string the plan rewrote, nothing written", r)
		}
		agent.in(apart).separately(t, ops...)
		if q := tm.linkedState(t, apart)["Q.md"]; q != (pageState{"[[X]] q", "X.md"}) {
			t.Errorf("Q after the calls = %+v, want its edit, leading to the new X", q)
		}
		checkPages(t, tm.pool)
		noErrorLogged(t, logs)
	})
	// The plan's rewrite leaves the links to a node frozen after an
	// ancestor's move as the participant does (M9/P3 review B1-N2, B's d6
	// and d6w): Q's [[Top/F/X]], X renamed without update_links after F's
	// move, matched by an edit's old_string, and judged the same by a
	// base-less write of its bytes; separate calls rewrote it at F's move,
	// the edit's old_string not there, the write's bytes not the page's.
	for i, third := range []map[string]any{opEdit("Q.md", "[[Top/F/X]] q", "[[Top/F/X]] y"), opWrite("Q.md", "[[Top/F/X]] q")} {
		name := map[int]string{0: "d6 an edit of a linker to a node frozen after its folder's move",
			1: "d6w a base-less write of the linker's bytes"}[i]
		t.Run(name, func(t *testing.T) {
			pages := []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""}, {"G", ""}, {"Q", "[[Top/F/X]] q"}}
			batch, apart := tm.twins(t, len(cases)+2+i, pages)
			ops := []map[string]any{opMove("Top/F.md", "G/F.md"), frozen(opMove("G/F/X.md", "G/F/Y.md")), third}
			agent.in(batch).batchDone(t, ops...)
			want := map[int]string{0: "[[Top/F/X]] y", 1: "[[Top/F/X]] q"}[i]
			if q := tm.linkedState(t, batch)["Q.md"]; q != (pageState{want, "-"}) {
				t.Errorf("Q after the batch = %+v, want %q, its link left", q, want)
			}
			m := agent.in(apart)
			m.separately(t, ops[:2]...)
			args := maps.Clone(third)
			tool, _ := args["op"].(string)
			delete(args, "op")
			code := map[int]string{0: "validation_failed", 1: "page.title_taken"}[i]
			if r := refusalOf(t, m.use(t, tool, args)); r.Code != code {
				t.Errorf("the %s alone after the moves = %+v, want %s", tool, r, code)
			}
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
	t.Run("2 the lock's code", func(t *testing.T) {
		batch, apart := tm.twins(t, len(cases)+1, []restPage{{"A", ""}, {"Q", "[[A]]"}})
		for _, nb := range []string{batch, apart} {
			tm.openSession(t, "bob", queryStrings(t, tm.pool, "SELECT id::text FROM nodes WHERE notebook_id = $1 AND name = 'Q'", nb)[0])
		}
		r := refusalOf(t, agent.in(batch).batch(t, opMove("A.md", "B.md"), opEdit("Q.md", "[[B]]", "[[B]] q")))
		if r.Code != "page.locked" || refusedAt(r) != 1 || r.Lock == nil || r.Lock.DisplayName != "bob" {
			t.Errorf("the batch = %+v, want page.locked by bob at ops[1], the lock's precheck", r)
		}
		if r := refusalOf(t, agent.in(apart).use(t, "move", map[string]any{"from": "A.md", "to": "B.md"})); r.Code != "linking.pages_locked" {
			t.Errorf("the move alone = %+v, want linking.pages_locked", r)
		}
		checkPages(t, tm.pool)
		noErrorLogged(t, logs)
	})
}

// frozen is op with update_links false.
func frozen(op map[string]any) map[string]any { return opWith(op, "update_links", false) }

// A write after a rename or a move that reaches its page is judged on the
// page as they leave it, as separate calls judge it (M9/P3 review A-I1):
// (1a) the bytes a move leaves, without a token, unchanged; (1c) an edit,
// a move and the bytes they leave; (1e) a stale token, the page changed
// and changed back on the web, and the bytes a move leaves, unchanged;
// (1b) the bytes from before the move, without a token: the page's 409 at
// the write. Each in two notebooks alike, the batch and the same ops apart:
// each op's changed and the pages and their links the same, or the same
// refusal at the same op.
func TestTheWritesOfABatchAreJudgedAsSeparateCallsJudgeThem(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	for i, tt := range []struct {
		name string
		web  bool // P changed and changed back on the web first
		ops  func(p string) []map[string]any
		code string // the refusal of the last op, "" for none
	}{
		{"1a a move, then the bytes it leaves", false, func(string) []map[string]any {
			return []map[string]any{opMove("A.md", "B.md"), opWrite("P.md", "[[B]] p")}
		}, ""},
		{"1c an edit, a move, then the bytes they leave", false, func(string) []map[string]any {
			return []map[string]any{opEdit("P.md", "p", "q"), opMove("A.md", "B.md"), opWrite("P.md", "[[B]] q")}
		}, ""},
		{"1e a stale token, the bytes a move leaves", true, func(p string) []map[string]any {
			return []map[string]any{opMove("A.md", "B.md"), opWriteOn("P.md", "[[B]] p", versionOf(p, 1))}
		}, ""},
		{"1b a move, then the bytes from before it", false, func(string) []map[string]any {
			return []map[string]any{opMove("A.md", "B.md"), opWrite("P.md", "[[A]] p")}
		}, "page.title_taken"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var nbs [2]string
			var ops [2][]map[string]any
			for k, which := range []string{"Batch", "Apart"} {
				nbs[k] = tm.openNotebook(t, "alice", fmt.Sprint(which, " ", i))
				ids := tm.seed(t, nbs[k], restPage{"A", ""}, restPage{"P", "[[A]] p"})
				if tt.web {
					tm.send(t, contentWrite("alice", ids["P"], "[[A]] x", 1, ""), http.StatusOK)
					tm.send(t, contentWrite("alice", ids["P"], "[[A]] p", 2, ""), http.StatusOK)
				}
				ops[k] = tt.ops(ids["P"])
			}
			a := agent.in(nbs[0]).batch(t, ops[0]...)
			changed, at, code := agent.in(nbs[1]).apart(t, ops[1]...)
			last := len(ops[0]) - 1
			if tt.code != "" {
				r := refusalOf(t, a)
				if r.Code != tt.code || refusedAt(r) != last || code != tt.code || at != last {
					t.Errorf("the batch = %s at op %d, apart %s at op %d; want %s at op %d both", r.Code, refusedAt(r), code, at, tt.code,
						last)
				}
				return
			}
			b := answerOf[mcpBatched](t, a)
			if code != "" {
				t.Fatalf("apart, ops[%d] answered %s", at, code)
			}
			for k, r := range b.Results {
				if r.Changed != changed[k] {
					t.Errorf("ops[%d] in the batch: changed %v, apart %v", k, r.Changed, changed[k])
				}
			}
			if bs, as := tm.linkedState(t, nbs[0]), tm.linkedState(t, nbs[1]); !maps.Equal(bs, as) {
				t.Errorf("the batch left\n%sseparate calls\n%s", shown(bs), shown(as))
			}
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
}

// apart makes each op of ops by its tool alone, in their order, until one
// is refused: each op's changed, and the op refused with its code, -1 and
// "" for none.
func (m *mcpApp) apart(t *testing.T, ops ...map[string]any) (changed []bool, at int, code string) {
	t.Helper()
	for i, op := range ops {
		args := maps.Clone(op)
		tool, _ := args["op"].(string)
		delete(args, "op")
		a := m.use(t, tool, args)
		if a.result(t)["isError"] == true {
			return changed, i, refusalOf(t, a).Code
		}
		var answer struct {
			Changed bool `json:"changed"`
		}
		structuredOf(t, a, &answer)
		changed = append(changed, answer.Changed)
	}
	return changed, -1, ""
}
