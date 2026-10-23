package bootstrap

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
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
// and the rename with update_links false (K3b and the control K3c); (23)
// a node followed under a node renamed, or moved under another, without
// update_links after its folder's move or rename: its Markdown link's
// text, its path before, written as M6 writes it (M9/P3 review B1-N1: its
// d5 and d5r; d5k); (24) a page created under a folder between two of its
// renames or moves, whose title's link it takes by neither (the paths
// equally long, the older page's id the less), the second carrying it to
// a shorter path: the second's rewrite writes the link back where it led,
// on a page the batch did not write (K4r: renames, the index's spread) and
// on one it wrote before (K4tw) (M9/P3 fix check 2, B2-N2: the window of a
// node is from its last rename or move, not its first); (25) a link of a
// page written, ambiguous before a rename of its folder or of the other
// and ambiguous still after it, left as it is, and so when the page is
// written again between two renames: the composition root carries the
// lead's ambiguity both ways (M9/P3 design review A-M3; M9 closeout A-F1).
func TestTheLinksOfABatchAreThoseOfSeparateCalls(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	const aliased = "---\naliases: [Roadmap]\n---\n"
	notes := []restPage{{"A", ""}, {"A/Notes", ""}, {"B", ""}, {"B/Notes", ""}, {"P", "p"}, {"Q", "[[Notes]] q"}}
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
		{"23 a node followed under one frozen, a Markdown link by its old path (d5)", []restPage{{"Top", ""}, {"Top/F", ""},
			{"Top/F/X", ""}, {"Top/F/X/D", ""}, {"G", ""}, {"Q", "[[Top/F/X/D]] [Top/F/X/D](Top/F/X/D.md)"}},
			[]map[string]any{opMove("Top/F.md", "G/F.md"), opWith(opMove("G/F/X.md", "G/F/Y.md"), "update_links", false)},
			map[string]string{"Q.md": "[[D]] [D](D.md)"}, true},
		{"23 the folder renamed (d5r)", []restPage{{"F", ""}, {"F/X", ""}, {"F/X/D", ""}, {"Q", "[F/X/D](F/X/D.md)"}},
			[]map[string]any{opMove("F.md", "G.md"), opWith(opMove("G/X.md", "G/Y.md"), "update_links", false)},
			map[string]string{"Q.md": "[D](D.md)"}, true},
		{"23 the frozen node moved under another (d5k)", []restPage{{"Top", ""}, {"Top/F", ""}, {"Top/F/X", ""},
			{"Top/F/X/D", ""}, {"Top/F/K", ""}, {"G", ""}, {"Q", "[Top/F/X/D](Top/F/X/D.md)"}},
			[]map[string]any{opMove("Top/F.md", "G/F.md"), opWith(opMove("G/F/X.md", "G/F/K/X.md"), "update_links", false)},
			map[string]string{"Q.md": "[D](D.md)"}, true},
		{"24 created between two renames of its folder, a linker not written (K4r)", []restPage{{"A", ""}, {"A/B", ""},
			{"A/B/C", ""}, {"A/B/C/D", ""}, {"A/B/C/D/N", ""}, {"Top", ""}, {"Top/FFF", ""}, {"Top/FFF/X", ""}, {"Q", "[[N]] q"}},
			[]map[string]any{opMove("Top/FFF.md", "Top/EEE.md"), opWrite("Top/EEE/N.md", "n"), opMove("Top/EEE.md", "Top/E.md")},
			map[string]string{"Q.md": "[[A/B/C/D/N]] q"}, true},
		{"24 created between two moves of its folder, the linker written first (K4tw)", titled(false, restPage{"P", "p"}),
			[]map[string]any{opEdit("P.md", "p", "p [[N]]"), opMove("Top/F.md", "Top/E.md"), opWrite("Top/E/N.md", "n"),
				opMove("Top/E.md", "G/E.md")},
			map[string]string{"P.md": "p [[A/B/C/N]]"}, true},
		{"25 an ambiguous link written, then its folder renamed", notes,
			[]map[string]any{opEdit("P.md", "p", "p [[Notes]]"), opMove("A.md", "C.md")},
			map[string]string{"P.md": "p [[Notes]]", "Q.md": "[[Notes]] q"}, true},
		{"25 written, then the other folder renamed", notes,
			[]map[string]any{opEdit("P.md", "p", "p [[Notes]]"), opMove("B.md", "D.md")},
			map[string]string{"P.md": "p [[Notes]]", "Q.md": "[[Notes]] q"}, true},
		{"25 written, renamed, written again", notes,
			[]map[string]any{opEdit("P.md", "p", "p [[Notes]]"), opMove("A.md", "C.md"), opEdit("P.md", "p [[Notes]]", "p [[Notes]] more"),
				opMove("B.md", "D.md")},
			map[string]string{"P.md": "p [[Notes]] more", "Q.md": "[[Notes]] q"}, true},
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

// The links of a page whose content ops leave it the bytes it held before
// the batch are those of separate calls (Codex review R1, D1): its last
// content op decides them, whatever its bytes, as it decides a page's the
// batch writes. Each case in two notebooks alike, the batch and the same
// ops apart: each op's changed, the pages alive and where their links lead,
// those in the trash, and both indexes their rebuilds. (1) A move, then an
// edit back to the bytes before, of a wikilink and (2) of a Markdown link;
// (3) a move, then a write of the bytes before on the token from before the
// batch, which separate calls give the token the move left (a write of them
// without one is page.title_taken in both:
// TestTheWritesOfABatchAreJudgedAsSeparateCallsJudgeThem's 1b); (4) two
// edits that cancel each other, after a creation that takes the page's
// link and before a move of where it led, and (5) a write of the bytes the
// page holds there, which changes nothing (M9/P3 review W3): the link
// resolved anew by the creation, as separate calls' index has it, not the
// index's from before the batch; (6) a move after the edit back that
// reaches the page's other link, (7) the same with update_links false, and
// (8) a move with update_links false before the move and the edit back;
// (9) the page then deleted, another's link rewritten; (10) a link with
// display text; (11) a link to an alias beside it.
func TestTheLinksOfAPageABatchPutsBackAreThoseOfSeparateCalls(t *testing.T) {
	tm, logs := newAcmeTeamLogged(t)
	agent := tm.agent(t, "alice", "")
	same := func(ops ...map[string]any) func(map[string]string, bool) []map[string]any {
		return func(map[string]string, bool) []map[string]any { return ops }
	}
	const aliased = "---\naliases: [Roadmap]\n---\n"
	created := []restPage{{"F", ""}, {"F/X", ""}, {"G", ""}, {"P", "[[X]] p"}}
	for i, tt := range []struct {
		name  string
		pages []restPage
		ops   func(ids map[string]string, apart bool) []map[string]any
		want  map[string]string // the contents some pages hold after
	}{
		{"1 a move, then an edit back", []restPage{{"A", ""}, {"P", "[[A]] x"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]")), map[string]string{"P.md": "[[A]] x"}},
		{"2 of a Markdown link", []restPage{{"A", ""}, {"P", "[A](A.md) x"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[B](B.md)", "[A](A.md)")), map[string]string{"P.md": "[A](A.md) x"}},
		{"3 a move, then a write back on the token from before", []restPage{{"A", ""}, {"P", "[[A]] x"}},
			func(ids map[string]string, apart bool) []map[string]any {
				read := 1 // the batch's base_version refers to the notebook before it
				if apart {
					read = 2 // the move's rewrite wrote P
				}
				return []map[string]any{opMove("A.md", "B.md"), opWriteOn("P.md", "[[A]] x", versionOf(ids["P"], read))}
			}, map[string]string{"P.md": "[[A]] x"}},
		{"4 two edits that cancel, between a creation and a move", created,
			same(opWrite("X.md", "x"), opEdit("P.md", "p", "q"), opEdit("P.md", "q", "p"), opMove("F/X.md", "G/X.md")),
			map[string]string{"P.md": "[[X]] p"}},
		{"5 a write of the bytes there, between a creation and a move", created,
			same(opWrite("X.md", "x"), opWrite("P.md", "[[X]] p"), opMove("F/X.md", "G/X.md")), map[string]string{"P.md": "[[X]] p"}},
		{"6 a move after the edit back, of the page's other link", []restPage{{"A", ""}, {"C", ""}, {"P", "[[A]] [[C]] x"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]"), opMove("C.md", "D.md")),
			map[string]string{"P.md": "[[A]] [[D]] x"}},
		{"7 that move with update_links false", []restPage{{"A", ""}, {"C", ""}, {"P", "[[A]] [[C]] x"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]"), frozen(opMove("C.md", "D.md"))),
			map[string]string{"P.md": "[[A]] [[C]] x"}},
		{"8 a move with update_links false first", []restPage{{"A", ""}, {"C", ""}, {"P", "[[A]] [[C]] x"}},
			same(frozen(opMove("C.md", "D.md")), opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]")),
			map[string]string{"P.md": "[[A]] [[C]] x"}},
		{"9 the page edited back, then deleted", []restPage{{"A", ""}, {"P", "[[A]] x"}, {"Q", "[[A]] q"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]"), opDelete("P.md")), map[string]string{"Q.md": "[[B]] q"}},
		{"10 a link with display text", []restPage{{"A", ""}, {"P", "[[A|see]] x"}},
			same(opMove("A.md", "B.md"), opEdit("P.md", "[[B|see]]", "[[A|see]]")), map[string]string{"P.md": "[[A|see]] x"}},
		{"11 a link to an alias beside it", []restPage{{"A", ""}, {"R", aliased}, {"P", "[[A]] [[Roadmap]] x"}},
			same(opMove("A.md", "B.md"), opMove("R.md", "S.md"), opEdit("P.md", "[[B]]", "[[A]]")),
			map[string]string{"P.md": "[[A]] [[Roadmap]] x"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			batch, apart := tm.openNotebook(t, "alice", fmt.Sprint("Back ", i)), tm.openNotebook(t, "alice", fmt.Sprint("Back apart ", i))
			got := agent.in(batch).batchDone(t, tt.ops(tm.seed(t, batch, tt.pages...), false)...)
			ops := tt.ops(tm.seed(t, apart, tt.pages...), true)
			changed, at, code := agent.in(apart).apart(t, ops...)
			if code != "" {
				t.Fatalf("apart, ops[%d] answered %s", at, code)
			}
			for k, r := range got.Results {
				if ops[k]["op"] != "delete" && r.Changed != changed[k] { // delete alone answers no changed
					t.Errorf("ops[%d] in the batch: changed %v, apart %v", k, r.Changed, changed[k])
				}
			}
			b, a := tm.linkedState(t, batch), tm.linkedState(t, apart)
			if !maps.Equal(b, a) {
				t.Errorf("the batch left\n%sseparate calls\n%s", shown(b), shown(a))
			}
			for path, want := range tt.want {
				if got := b[path].content; got != want {
					t.Errorf("%s after the batch is %q, want %q", path, got, want)
				}
			}
			if b, a := tm.trashed(t, batch), tm.trashed(t, apart); !slices.Equal(b, a) {
				t.Errorf("the trash after the batch = %q, after separate calls = %q", b, a)
			}
			tm.checkRebuilt(t, batch)
			tm.checkRebuilt(t, apart)
			checkPages(t, tm.pool)
			noErrorLogged(t, logs)
		})
	}
}

// The rewrite of a batch counts a page its content ops put back as one
// page against its bound, as the page of the index it is in separate
// calls, not as a page it wrote (Codex review R1): P, its link edited
// back after A's rename and then reached by C's, with Q1 and Q2 of the
// index, three pages within a bound of 3, rewritten; P edited once more,
// written, counts WrittenRewriteWeight, page.batch_too_large at the first
// rename saying so, nothing written. The app's bound is set as the
// composition root gives it, which carries what the batch tells of P to
// the rewrite.
func TestTheRewriteOfABatchCountsAPagePutBackAsOne(t *testing.T) {
	r := startReadsAppWith(t, publicOrigin, wiring{page: func(d *page.Deps) { d.MaxBatchRewrites = 3 }})
	for _, p := range []restPage{{"A", ""}, {"C", ""}, {"P", "[[A]] [[C]] p"}, {"Q1", "[[C]] q1"}, {"Q2", "[[C]] q2"}} {
		r.createPage(t, "", p.path, p.content)
	}
	state := func() []string {
		return queryStrings(t, r.pool, `SELECT n.name || ': ' || c.content || ' ' || c.revision FROM nodes n
			JOIN page_contents c ON c.node_id = n.id AND c.deleted_at IS NULL WHERE n.notebook_id = $1 AND n.deleted_at IS NULL
			ORDER BY n.name`, r.notebook)
	}
	was := state()
	past := fmt.Sprintf("4 pages whose links the batch's renames and moves would rewrite, each page the batch wrote counting %d, "+
		"at most 3", domain.WrittenRewriteWeight)
	a := r.batch(t, opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]"), opEdit("P.md", "p", "p2"), opMove("C.md", "D.md"))
	if refused := refusalOf(t, a); refused.Code != "page.batch_too_large" || refusedAt(refused) != 0 ||
		!strings.Contains(refused.Message, past) || !slices.Equal(state(), was) {
		t.Errorf("the batch writing P = %+v, want page.batch_too_large at ops[0] saying %q, nothing written", refused, past)
	}
	r.batchDone(t, opMove("A.md", "B.md"), opEdit("P.md", "[[B]]", "[[A]]"), opMove("C.md", "D.md"))
	if got, want := state(), []string{"B:  1", "D:  1", "P: [[A]] [[D]] p 2", "Q1: [[D]] q1 2", "Q2: [[D]] q2 2"}; !slices.Equal(got, want) {
		t.Errorf("after the batch putting P back: %q, want %q", got, want)
	}
	checkPages(t, r.pool)
}

// trashed is the notebook nb's pages in the trash, each "name: content".
func (tm acmeTeam) trashed(t *testing.T, nb string) []string {
	t.Helper()
	return queryStrings(t, tm.pool, `SELECT n.name || ': ' || c.content FROM nodes n JOIN page_contents c ON c.node_id = n.id
		WHERE n.notebook_id = $1 AND n.deleted_at IS NOT NULL ORDER BY 1`, nb)
}

// The ways a batch differs from separate calls, each as M9 design 4.11's
// rule says (M9/P3 design 3.14.4): links follow the batch's renames and
// moves, merged node by node, from where they led before it, or, in a page
// it writes, at its latest write before them, on the tree then with the
// aliases from before the batch; what other ops did between to where they
// lead is not seen, nor an alias the batch changed. A page not written: (1a)
// a creation before a move, (1b) a dangling link that a rename takes before
// another rename, (1c) a link taken and freed again, (a) a deletion that
// turns a link to another page, (b) an alias taken away; (1a2) a creation
// under a folder before its rename, which the rename reaches as separate
// calls' does, the link the creation took written back where it led before
// the batch (M9/P3 review B1-M1: one created after the rename is not
// reached, 3.14.3's 22). A written page: (e1) a creation after its write,
// (e2) an alias the batch changed, (e3) a name merged away. (d) update_links
// false on a node, its ancestor renamed or moved with update_links: the
// links that led to the node left as they are, in every page, whatever its
// ancestors' changes (M9/P3 review B-M1), dangling where separate calls
// write them first, the same where the node's op comes first (d1r, d3r); the
// node's own op with update_links, its ancestor's without, followed (d2,
// d4), a Markdown link's text by its path before, the frozen ancestor's name
// in it, as M6 writes it (d2m; M9/P3 review B1-N1); the plan's rewrite
// leaving them too, an edit's old_string and a write's bytes judged on the
// links left (d6, d6w). (c) A page written after the plan rewrote it. (2)
// The lock's code.
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
		{"d2m d2 by a Markdown link's path, its text the path before", []restPage{{"F", ""}, {"F/X", ""}, {"Q", "[F/X](F/X.md)"}},
			[]map[string]any{frozen(opMove("F.md", "G.md")), opMove("G/X.md", "G/Y.md")}, "Q.md", pageState{"[Y](Y.md)", "G/Y.md"},
			pageState{"[F/X](F/X.md)", "-"}},
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
