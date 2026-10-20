package app_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A batch's deferred rewrite (M9/P3 design 3.7.2): the pages it wrote are
// followed from their contents, their own changes and where their links
// led at their last content op; the rest from the index.

// facts is content's facts as the index keeps them.
func (w *rewriting) facts(content string) domain.Facts {
	w.t.Helper()
	facts, err := markdownadapter.PageFacts(w.md.Parse([]byte(content)).Facts())
	if err != nil {
		w.t.Fatal(err)
	}
	return facts
}

// wrote has the page at path hold content, which a batch wrote after its
// index: its revision the next, its index as it was. It answers the facts
// the batch's parse of the final content kept.
func (w *rewriting) wrote(path, content string) domain.Facts {
	w.t.Helper()
	w.revision++
	w.contents[w.id(path)] = page{content, w.revision}
	return w.facts(content)
}

// before is where the links of content led at a page's last content op,
// in their order, to the pages at targets ("" for none), at their paths
// now: as the plan resolves them.
func (w *rewriting) before(content string, targets ...string) app.LinksBefore {
	w.t.Helper()
	out := app.LinksBefore{Leads: map[int]domain.Resolution{}, Targets: map[uuid.UUID][]domain.Step{}}
	for i, l := range w.facts(content).Links {
		if targets[i] == "" {
			out.Leads[l.Start] = domain.Resolution{}
			continue
		}
		id := w.id(targets[i])
		out.Leads[l.Start] = domain.Resolution{ID: id}
		out.Targets[id] = w.tree.path(id)
	}
	return out
}

// batch has the rewrite follow m, a batch's unit, rewriting links.
func (w *rewriting) batch(m app.Moved) (*appender, error) {
	u := &appender{}
	m.NotebookID, m.At, m.UpdateLinks = w.notebook, time.Now(), true
	return u, w.rewrite.Participate(context.Background(), m, u)
}

// renamed is the change of the node at path renamed from one name to
// another under its parent, the tree left as it is.
func (w *rewriting) renamed(path, from, to string) domain.Change {
	id := w.id(path)
	place := w.place(id)
	return domain.Change{NodeID: id, Before: &domain.Place{ParentID: place.ParentID, Name: from},
		After: &domain.Place{ParentID: place.ParentID, Name: to}}
}

// A page the batch wrote follows its own changes, those after its last
// content op, from where its links led then (M9/P3 design 3.7.2 (a)-(e)):
// "rename A to B, edit P to [[B]], rename B to C" writes [[C]], as three
// calls would, not [[C|B]]; whatever its index, and on its current
// revision. A page the batch did not write follows the merged changes from
// the index.
func TestABatchFollowsAPageItWroteFromItsOwnChanges(t *testing.T) {
	w := newRewriting(t, "A", "P", "Q")
	w.write("Q", "[[A]]\n")
	before := w.before("[[B]]\n", "A")
	before.Targets[w.id("A")] = []domain.Step{{ID: w.id("A"), Key: "b", Name: "B"}} // A was named B then
	facts := w.wrote("P", "[[B]]\n")
	merged := w.rename("A", "C")
	u, err := w.batch(app.Moved{Changes: []domain.Change{merged}, Written: map[uuid.UUID]app.WrittenPage{
		w.id("P"): {Before: before, Changes: []domain.Change{w.renamed("C", "B", "C")}, Facts: facts},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"P": "[[C]]\n", "Q": "[[C]]\n"})
}

// "rename A to B, edit P to [[B]], rename B to A" leaves the merged
// changes nothing to move, while P's own write it back to [[A]] (M9/P3
// review A-I3); a node an op after P's last moved without update_links is
// not among P's changes, which leave it.
func TestABatchFollowsAPageItWroteWhenTheMergedChangesMoveNothing(t *testing.T) {
	w := newRewriting(t, "A", "P")
	before := w.before("[[B]]\n", "A")
	before.Targets[w.id("A")] = []domain.Step{{ID: w.id("A"), Key: "b", Name: "B"}}
	facts := w.wrote("P", "[[B]]\n")
	same := w.renamed("A", "A", "A")
	u, err := w.batch(app.Moved{Changes: []domain.Change{same}, Written: map[uuid.UUID]app.WrittenPage{
		w.id("P"): {Before: before, Changes: []domain.Change{w.renamed("A", "B", "A")}, Facts: facts},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"P": "[[A]]\n"})
	u, err = w.batch(app.Moved{Changes: []domain.Change{same}, Written: map[uuid.UUID]app.WrittenPage{
		w.id("P"): {Before: before, Facts: facts},
	}})
	if err != nil || len(u.writes) != 0 {
		t.Errorf("without its own changes: %d writes, %v", len(u.writes), err)
	}
}

// parses is the rewrite's parser, recording each content it parses.
type parses struct {
	app.RewriteParser
	contents []string
}

func (p *parses) ParseNow(ctx context.Context, content string) (app.Parsed, error) {
	p.contents = append(p.contents, content)
	return p.RewriteParser.ParseNow(ctx, content)
}

// A page the batch wrote has its links from the facts its final content's
// parse kept: the rewrite parses what it writes alone, never that content
// again (M9/P3 review A2-M2).
func TestABatchTakesTheLinksOfAPageItWroteFromItsFacts(t *testing.T) {
	w := newRewriting(t, "A", "P")
	parser := &parses{RewriteParser: w.rewrite.Parser}
	w.rewrite.Parser = parser
	before := w.before("see [[A]]\n", "A")
	facts := w.wrote("P", "see [[A]]\n")
	u, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("A", "B")}, Written: map[uuid.UUID]app.WrittenPage{
		w.id("P"): {Before: before, Changes: []domain.Change{w.renamed("B", "A", "B")}, Facts: facts},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"P": "see [[B]]\n"})
	if !slices.Equal(parser.contents, []string{"see [[B]]\n"}) {
		t.Errorf("parsed %q, want what it wrote alone", parser.contents)
	}
}

// counted is the tree and the index as a batch's rewrite reads them,
// counting the subtrees it reads and the aliases' keys it asks.
type counted struct {
	app.Pages
	subtrees map[uuid.UUID]int
}

func (c *counted) Subtree(ctx context.Context, notebookID, id uuid.UUID) ([]domain.Step, error) {
	c.subtrees[id]++
	return c.Pages.Subtree(ctx, notebookID, id)
}

type countedStore struct {
	app.Store
	asked [][]uuid.UUID
}

func (c *countedStore) AliasKeys(ctx context.Context, ids []uuid.UUID) ([]string, error) {
	c.asked = append(c.asked, slices.Clone(ids))
	return c.Store.AliasKeys(ctx, ids)
}

// A page the batch wrote reaches, through its own changes, the nodes under
// those they move (M9/P3 review A3-I1): F moved alone, [[Top/F/X]] follows
// X; two pages that link into F read F's subtree once for the batch, and
// its aliases' keys once; a node under F renamed without update_links before the
// page's last content op is in the subtree read, its link followed.
func TestABatchReachesTheNodesUnderThoseAPageItWroteMoves(t *testing.T) {
	w := newRewriting(t, "Top", "Top/F", "Top/F/X", "Top/F/X2", "Top/F/X2/Y", "Other", "W1", "W2")
	pages := &counted{Pages: w.tree, subtrees: map[uuid.UUID]int{}}
	store := &countedStore{Store: w.store}
	w.rewrite.Pages, w.rewrite.Store = pages, store
	b1, b2 := w.before("[[Top/F/X]]\n", "Top/F/X"), w.before("[[Top/F/X2/Y]]\n", "Top/F/X2/Y")
	f1, f2 := w.wrote("W1", "[[Top/F/X]]\n"), w.wrote("W2", "[[Top/F/X2/Y]]\n")
	f, x := w.id("Top/F"), w.id("Top/F/X")
	changes := w.move("Top/F", "Other")
	own := []domain.Change{changes[0]}
	u, err := w.batch(app.Moved{Changes: changes, Written: map[uuid.UUID]app.WrittenPage{
		w.id("W1"): {Before: b1, Changes: own, Facts: f1},
		w.id("W2"): {Before: b2, Changes: own, Facts: f2},
	}})
	if err != nil {
		t.Fatal(err)
	}
	// As a move alone writes them: by their names, which lead there alone.
	w.written(u, map[string]string{"W1": "[[X]]\n", "W2": "[[Y]]\n"})
	if pages.subtrees[f] != 1 {
		t.Errorf("F's subtree read %d times, want once for the batch", pages.subtrees[f])
	}
	asked := 0
	for _, ids := range store.asked {
		if slices.Contains(ids, x) {
			asked++
		}
	}
	// Once for the pages the batch did not write, spread over the merged
	// changes, which list F's nodes; once for the two it wrote.
	if asked != 2 {
		t.Errorf("the aliases' keys of F's subtree asked %d times, want twice", asked)
	}
}

// treeReads is the tree as a rewrite reads it, counting its reads of paths
// and of keys.
type treeReads struct {
	app.Pages
	paths, keys int
}

func (r *treeReads) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	r.paths++
	return r.Pages.Paths(ctx, notebookID, ids)
}

func (r *treeReads) ByKeys(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error) {
	r.keys++
	return r.Pages.ByKeys(ctx, notebookID, keys)
}

// The pages a batch wrote are written on one read of the tree (M9/P3
// review B-M3): of the paths and of the keys, once for them all, past the
// reads each page's links resolve with, not once a page.
func TestABatchWritesThePagesItWroteOnOneTree(t *testing.T) {
	w := newRewriting(t, "Top", "Top/F", "Top/F/X0", "Top/F/X1", "Top/F/X2", "G", "W0", "W1", "W2")
	reads := &treeReads{Pages: w.tree}
	w.rewrite.Pages = reads
	written := map[uuid.UUID]app.WrittenPage{}
	for i := range 3 {
		link := fmt.Sprintf("[[Top/F/X%d]]\n", i)
		before := w.before(link, fmt.Sprint("Top/F/X", i))
		written[w.id(fmt.Sprint("W", i))] = app.WrittenPage{Before: before, Facts: w.wrote(fmt.Sprint("W", i), link)}
	}
	changes := w.move("Top/F", "G")
	for id, p := range written {
		p.Changes = changes[:1]
		written[id] = p
	}
	u, err := w.batch(app.Moved{Changes: changes, Written: written})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"W0": "[[X0]]\n", "W1": "[[X1]]\n", "W2": "[[X2]]\n"})
	if reads.paths != 3+1 || reads.keys != 3+1 {
		t.Errorf("the tree's paths read %d times, its keys %d; want 3 for the pages' links and 1 for the tree, each", reads.paths,
			reads.keys)
	}
}

// The links that led to a node an op of the batch renamed or moved with
// update_links false are left as they are, in every page, whatever its
// ancestors' changes (M9 design 4.11; M9/P3 review B-M1): Q, a page the
// batch did not write, P, one it wrote, and P's content in the plan,
// whether F is renamed (d1) or moved (d3) with update_links, X under it
// renamed without. Without the frozen nodes, F's change reaches X's links:
// the plan's rule, ownLinks, spreads F's subtree in both; the index's,
// reached, only a renamed F's.
func TestABatchLeavesTheLinksToTheNodesItFroze(t *testing.T) {
	for _, tt := range []struct {
		name, path string
		pages      []string
		change     func(w *rewriting) []domain.Change
		unfrozen   map[string]string
	}{
		{"d1 F renamed", "F/X", []string{"F", "F/X", "P", "Q"}, func(w *rewriting) []domain.Change {
			changes := []domain.Change{w.rename("F", "G")}
			w.rename("G/X", "Y")
			return changes
		}, map[string]string{"P": "[[Y]]\n", "Q": "[[Y|F/X]]\n"}},
		{"d3 F moved", "Top/F/X", []string{"Top", "Top/F", "Top/F/X", "G", "P", "Q"}, func(w *rewriting) []domain.Change {
			changes := w.move("Top/F", "G")[:1] // X's entry the unit drops: it froze X
			w.rename("G/F/X", "Y")
			return changes
		}, map[string]string{"P": "[[Y]]\n"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newRewriting(t, tt.pages...)
			link := "[[" + tt.path + "]]\n"
			w.write("Q", link)
			before, x, q := w.before(link, tt.path), w.id(tt.path), w.id("Q")
			revision := w.contents[q].revision
			facts := w.wrote("P", link)
			changes := tt.change(w)
			links := app.BatchLinks{Store: w.store, Logger: w.rewrite.Logger, MaxContent: 1 << 10}
			parse := func(_ context.Context, content string) (domain.Facts, error) { return w.facts(content), nil }
			on := app.LinksOn{NotebookID: w.notebook, PageID: w.id("P"), Content: link, Tree: w.tree, Parse: parse}
			for _, frozen := range [][]uuid.UUID{{x}, nil} {
				u, err := w.batch(app.Moved{Changes: changes, Frozen: frozen, Written: map[uuid.UUID]app.WrittenPage{
					w.id("P"): {Before: before, Changes: changes, Facts: facts, Frozen: frozen},
				}})
				if err != nil {
					t.Fatal(err)
				}
				planned, err := links.Rewrite(context.Background(), app.LinksMoved{LinksOn: on, Before: before, Changes: changes,
					Frozen: frozen})
				if err != nil {
					t.Fatal(err)
				}
				reached, err := links.Reached(context.Background(), w.notebook, q, revision, changes, frozen, nil, w.tree, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if frozen == nil {
					w.written(u, tt.unfrozen)
					if planned != tt.unfrozen["P"] || !reached {
						t.Errorf("X not frozen: the plan wrote P %q, reached Q %t; want P %q, Q reached", planned, reached,
							tt.unfrozen["P"])
					}
					continue
				}
				w.written(u, map[string]string{})
				if planned != link || reached {
					t.Errorf("X frozen: the plan wrote P %q, reached Q %t; want it as it was, Q not reached", planned, reached)
				}
			}
		})
	}
}

// The nodes a batch created after a node's last rename or move are out of
// the subtree its rewrite spreads (M9 design 4.11: a creation only
// resolves again; M9/P3 review B1-M1): N, created under H after H is
// renamed to H2, or under F after F is moved under G, takes [[N]] from
// A/B/C/D/N, by the shorter path; the index's Q (K3b), the written P (K2),
// the plan's rewrite of Q (K1r) and its reach leave it. Without them, the
// subtree, read as the batch left it, reaches N's key, and [[N]] is written
// back to A/B/C/D/N. They are left out of the subtree of the node they are
// keyed by alone: N, created under K/S2 after S's rename and before K's,
// is in K's, as K's rename reached it.
func TestABatchLeavesOutTheNodesItCreatedAfterAMove(t *testing.T) {
	type world struct {
		w       *rewriting
		changes []domain.Change
		after   map[uuid.UUID][]uuid.UUID
	}
	setUp := func(t *testing.T, how string) world {
		w := newRewriting(t, "A", "A/B", "A/B/C", "A/B/C/D", "A/B/C/D/N", "Top", "Top/F", "Top/F/X", "G", "H", "H/Y", "K", "K/S", "P",
			"Q")
		w.write("Q", "[[N]] q\n")
		switch how {
		case "renamed":
			h := w.id("H")
			changes := []domain.Change{w.rename("H", "H2")}
			return world{w, changes, map[uuid.UUID][]uuid.UUID{h: {w.tree.add("H2/N")}}}
		case "moved":
			f := w.id("Top/F")
			changes := w.move("Top/F", "G")[:1] // a written page's own: F's alone
			return world{w, changes, map[uuid.UUID][]uuid.UUID{f: {w.tree.add("G/F/N")}}}
		}
		s := w.id("K/S")
		changes := []domain.Change{w.rename("K/S", "S2")}
		n := w.tree.add("K/S2/N")
		return world{w, append(changes, w.rename("K", "K2")), map[uuid.UUID][]uuid.UUID{s: {n}}}
	}
	for _, given := range []bool{true, false} {
		t.Run(fmt.Sprint("given ", given), func(t *testing.T) {
			after := func(x world) map[uuid.UUID][]uuid.UUID {
				if given {
					return x.after
				}
				return nil
			}
			rewrote := func(page, to string) map[string]string {
				if given {
					return map[string]string{}
				}
				return map[string]string{page: to}
			}
			// The index's Q, H renamed (K3b): reached spreads renamed nodes
			// alone.
			x := setUp(t, "renamed")
			u, err := x.w.batch(app.Moved{Changes: x.changes, CreatedAfter: after(x)})
			if err != nil {
				t.Fatal(err)
			}
			x.w.written(u, rewrote("Q", "[[A/B/C/D/N]] q\n"))
			// The written P, F moved after its edit (K2).
			x = setUp(t, "moved")
			before := x.w.before("p [[N]]\n", "A/B/C/D/N")
			facts := x.w.wrote("P", "p [[N]]\n")
			u, err = x.w.batch(app.Moved{Changes: x.changes, CreatedAfter: after(x), Written: map[uuid.UUID]app.WrittenPage{
				x.w.id("P"): {Before: before, Changes: x.changes, Facts: facts, CreatedAfter: after(x)},
			}})
			if err != nil {
				t.Fatal(err)
			}
			x.w.written(u, rewrote("P", "p [[A/B/C/D/N]]\n"))
			// The plan's Q, edited after the move and the creation (K1r), and
			// its reach for a write after them.
			x = setUp(t, "moved")
			q := x.w.id("Q")
			links := app.BatchLinks{Store: x.w.store, Logger: x.w.rewrite.Logger, MaxContent: 1 << 10}
			parse := func(_ context.Context, content string) (domain.Facts, error) { return x.w.facts(content), nil }
			on := app.LinksOn{NotebookID: x.w.notebook, PageID: q, Content: "[[N]] q\n", Tree: x.w.tree, Parse: parse}
			planned, err := links.Rewrite(context.Background(), app.LinksMoved{LinksOn: on, Before: x.w.before("[[N]] q\n", "A/B/C/D/N"),
				Changes: x.changes, CreatedAfter: after(x)})
			if err != nil {
				t.Fatal(err)
			}
			reached, err := links.Reached(context.Background(), x.w.notebook, q, x.w.contents[q].revision, x.changes, nil, after(x),
				x.w.tree, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := map[bool]string{true: "[[N]] q\n", false: "[[A/B/C/D/N]] q\n"}[given]; planned != want || reached == given {
				t.Errorf("the plan wrote Q %q, reached it %t; want %q, reached %t", planned, reached, want, !given)
			}
			// N under K/S2, created after S's rename, before K's: K's
			// subtree holds it, for P and for the index's Q.
			x = setUp(t, "nested")
			before = x.w.before("p [[N]]\n", "A/B/C/D/N")
			facts = x.w.wrote("P", "p [[N]]\n")
			u, err = x.w.batch(app.Moved{Changes: x.changes, CreatedAfter: after(x), Written: map[uuid.UUID]app.WrittenPage{
				x.w.id("P"): {Before: before, Changes: x.changes, Facts: facts, CreatedAfter: after(x)},
			}})
			if err != nil {
				t.Fatal(err)
			}
			x.w.written(u, map[string]string{"P": "p [[A/B/C/D/N]]\n", "Q": "[[A/B/C/D/N]] q\n"})
		})
	}
	// X created, then renamed to Y: its own change is followed, [[Y]] kept
	// at G/Y (3.14.3's 17).
	w := newRewriting(t, "G", "G/Y", "Q")
	w.write("Q", "[[Y]]\n")
	w.tree.add("X")
	u, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("X", "Y")}})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{"Q": "[[G/Y]]\n"})
}

// The plan's rewrites on one tree read the subtrees of the nodes their
// changes move once, kept in the reads their caller gives (M9/P3 review
// B-N2), and spread them once, the reach kept with them (review A's N1):
// two content ops after a folder's move, one read of its subtree and one
// ask of the aliases' keys of its nodes and of the moved one; without the
// reads, one each.
func TestBatchLinksKeepTheirReadsOfATree(t *testing.T) {
	w := newRewriting(t, "Top", "Top/F", "Top/F/X", "G", "P", "Q")
	before := w.before("[[Top/F/X]]\n", "Top/F/X")
	f := w.id("Top/F")
	changes := w.move("Top/F", "G")[:1]
	pages := &counted{Pages: w.tree, subtrees: map[uuid.UUID]int{}}
	store := &countedStore{Store: w.store}
	links := app.BatchLinks{Store: store, Logger: w.rewrite.Logger, MaxContent: 1 << 10}
	parse := func(_ context.Context, content string) (domain.Facts, error) { return w.facts(content), nil }
	rewrite := func(page string, reads *any) string {
		got, err := links.Rewrite(context.Background(), app.LinksMoved{LinksOn: app.LinksOn{NotebookID: w.notebook, PageID: w.id(page),
			Content: "[[Top/F/X]]\n", Tree: pages, Parse: parse}, Before: before, Changes: changes, Reads: reads})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	var reads any
	if p, q := rewrite("P", &reads), rewrite("Q", &reads); p != "[[X]]\n" || q != p || pages.subtrees[f] != 1 || len(store.asked) != 2 {
		t.Errorf("two rewrites on one tree = %q and %q, F's subtree read %d times, the aliases' keys asked %d times; want [[X]] both, "+
			"once, twice", p, q, pages.subtrees[f], len(store.asked))
	}
	rewrite("P", nil)
	rewrite("Q", nil)
	if pages.subtrees[f] != 3 {
		t.Errorf("two rewrites without reads read F's subtree %d times in all, want once each", pages.subtrees[f]-1)
	}
}

// A page of no links spreads nothing for the changes before it (M9/P3
// review A's N1: the plan spread a renamed folder's subtree for each
// content op of a page with none): its reach by the index, its rewrite in
// the plan and in the participant read no subtree, after a folder's move.
func TestBatchLinksSpreadNothingForAPageOfNoLinks(t *testing.T) {
	w := newRewriting(t, "Top", "Top/F", "Top/F/X", "G", "P", "W")
	w.write("P", "p\n")
	f, p := w.id("Top/F"), w.id("P")
	changes := w.move("Top/F", "G")
	pages := &counted{Pages: w.tree, subtrees: map[uuid.UUID]int{}}
	links := app.BatchLinks{Store: w.store, Logger: w.rewrite.Logger, MaxContent: 1 << 10}
	parse := func(_ context.Context, content string) (domain.Facts, error) { return w.facts(content), nil }
	reached, err := links.Reached(context.Background(), w.notebook, p, w.contents[p].revision, changes[:1], nil, nil, pages, nil, nil)
	if err != nil || reached {
		t.Errorf("P's reach = %t, %v; want none", reached, err)
	}
	got, err := links.Rewrite(context.Background(), app.LinksMoved{LinksOn: app.LinksOn{NotebookID: w.notebook, PageID: p,
		Content: "p2\n", Tree: pages, Parse: parse}, Before: w.before("p2\n"), Changes: changes[:1]})
	if err != nil || got != "p2\n" {
		t.Errorf("P's rewrite = %q, %v; want it as it is", got, err)
	}
	w.rewrite.Pages = pages
	facts := w.wrote("W", "w\n")
	u, err := w.batch(app.Moved{Changes: changes, Written: map[uuid.UUID]app.WrittenPage{
		w.id("W"): {Before: w.before("w\n"), Changes: changes[:1], Facts: facts},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.written(u, map[string]string{})
	if pages.subtrees[f] != 0 {
		t.Errorf("F's subtree read %d times for pages of no links, want none", pages.subtrees[f])
	}
}

// The pages a batch deleted (M9/P3 review A-I2, A-M13): their rows are not
// followed, their aliases are none; a page gone that the batch did not
// delete is the defect it is; a link whose page it led to the batch
// deleted is left, at info, one whose page another deleted, at warn, and
// none at error.
func TestABatchLeavesThePagesItDeleted(t *testing.T) {
	t.Run("a page deleted that links", func(t *testing.T) {
		w := newRewriting(t, "A", "G", "S")
		w.write("G", "[[A]]\n")
		w.write("S", "[[A]]\n")
		w.remove("G")
		u, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("A", "B")}, Gone: []uuid.UUID{w.id("G")}})
		if err != nil {
			t.Fatal(err)
		}
		w.written(u, map[string]string{"S": "[[B]]\n"})
	})
	t.Run("a page deleted with an alias, its name taken", func(t *testing.T) {
		setUp := func() *rewriting {
			w := newRewriting(t, "P", "Plan", "S", "T")
			w.write("P", "---\naliases: [Roadmap]\n---\n")
			w.write("S", "[[Plan]]\n")
			w.write("T", "[[Roadmap]]\n") // to P, by its alias
			w.remove("P")
			return w
		}
		w := setUp()
		u, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("Plan", "Roadmap")}, Gone: []uuid.UUID{w.id("P")}})
		if err != nil {
			t.Fatal(err)
		}
		w.written(u, map[string]string{"S": "[[Roadmap]]\n"})
		w = setUp()
		if _, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("Plan", "Roadmap")}}); err == nil ||
			!strings.Contains(err.Error(), "are not pages of") {
			t.Errorf("a page gone the batch did not delete: %v, want the defect", err)
		}
	})
	t.Run("a link whose page is gone", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			gone  bool
			level string
		}{{"deleted by the batch", true, "INFO"}, {"by another", false, "WARN"}} {
			w := newRewriting(t, "T", "F", "P")
			before := w.before("[[T]]\n", "T")
			facts := w.wrote("P", "[[T]]\n")
			t1 := w.id("T")
			w.remove("T")
			changes := w.move("P", "F")
			m := app.Moved{Changes: changes, Written: map[uuid.UUID]app.WrittenPage{w.id("F/P"): {Before: before, Changes: changes, Facts: facts}}}
			if tt.gone {
				m.Gone = []uuid.UUID{t1}
			}
			u, err := w.batch(m)
			if err != nil || len(u.writes) != 0 {
				t.Errorf("%s: %d writes, %v", tt.name, len(u.writes), err)
			}
			logs := w.logs.String()
			if !strings.Contains(logs, "level="+tt.level+` msg="a link is not rewritten: where it led is gone"`) || strings.Contains(logs, "level=ERROR") {
				t.Errorf("%s: logged %s", tt.name, logs)
			}
		}
	})
}

// A batch's rewrite counts the pages it would write against MaxPages
// before it reads the tree they are written on, a lock or a content
// (M9/P3 design 3.7.2; review B-I1: a refusal paid for the tree first):
// past it, ErrTooManyRewrites with the count, a page being edited among
// them or not; 0 bounds nothing.
func TestABatchCountsItsRewritesFirst(t *testing.T) {
	setUp := func() *rewriting {
		w := newRewriting(t, "A", "Q1", "Q2")
		w.write("Q1", "[[A]]\n")
		w.write("Q2", "[[A]]\n")
		return w
	}
	w := setUp()
	parser := &parses{RewriteParser: w.rewrite.Parser}
	w.rewrite.Parser = parser
	tree := &askedPaths{Pages: w.tree}
	w.rewrite.Pages = tree
	w.locks.held[w.id("Q1")] = shared.LockHolder{PageID: w.id("Q1"), UserID: uuid.NewV7(), DisplayName: "Ada"}
	a := w.id("A")
	_, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("A", "B")}, MaxPages: 1, WrittenWeight: 2})
	var many *app.TooManyRewrites
	if !errors.As(err, &many) || !errors.Is(err, app.ErrTooManyRewrites) || many.Pages != 2 || many.Max != 1 || many.Weight != 0 {
		t.Fatalf("past the bound: %v, want TooManyRewrites of 2 at most 1, no page written to weigh", err)
	}
	if w.locks.reads != 0 || len(parser.contents) != 0 {
		t.Errorf("the locks read %d times and %d contents parsed before the count", w.locks.reads, len(parser.contents))
	}
	// The tree's read of the links' targets, A, comes after the count; the
	// reads before it resolve the links, by their pages.
	for _, ids := range tree.asked {
		if slices.Contains(ids, a) {
			t.Errorf("the paths of %v were read before the count", ids)
		}
	}
	for _, max := range []int{0, 2} {
		w := setUp()
		u, err := w.batch(app.Moved{Changes: []domain.Change{w.rename("A", "B")}, MaxPages: max})
		if err != nil {
			t.Fatalf("MaxPages %d: %v", max, err)
		}
		w.written(u, map[string]string{"Q1": "[[B]]\n", "Q2": "[[B]]\n"})
	}
}

// A page the batch wrote counts WrittenWeight against MaxPages (M9/P3
// review B-M3: its rewrite costs more than an index page's), when its own
// changes meet one of its links, and not at all when they meet none
// (review B1-M2): with Q1 and Q2 of the index, W written, its own change
// reaching its link, and V written, its own change reaching none of its,
// a weight of 2 counts 4, past a bound of 3, the refusal saying the
// weight; a bound of 4 writes the three, V left; a weight of 0 counts W
// once.
func TestABatchCountsAPageItWroteAsItsWeight(t *testing.T) {
	batch := func(max, weight int) (*rewriting, *appender, error) {
		w := newRewriting(t, "A", "Q1", "Q2", "W", "V", "Z")
		w.write("Q1", "[[A]]\n")
		w.write("Q2", "[[A]]\n")
		before, vBefore := w.before("w [[A]]\n", "A"), w.before("v [[Z]]\n", "Z")
		facts, vFacts := w.wrote("W", "w [[A]]\n"), w.wrote("V", "v [[Z]]\n")
		merged := w.rename("A", "B")
		own := []domain.Change{w.renamed("B", "A", "B")}
		u, err := w.batch(app.Moved{Changes: []domain.Change{merged}, MaxPages: max, WrittenWeight: weight,
			Written: map[uuid.UUID]app.WrittenPage{w.id("W"): {Before: before, Changes: own, Facts: facts},
				w.id("V"): {Before: vBefore, Changes: own, Facts: vFacts}}})
		return w, u, err
	}
	_, _, err := batch(3, 2)
	var many *app.TooManyRewrites
	if !errors.As(err, &many) || many.Pages != 4 || many.Max != 3 || many.Weight != 2 {
		t.Fatalf("a weight of 2, at most 3: %v %+v, want TooManyRewrites of 4 at most 3, weighing 2", err, many)
	}
	for _, c := range []struct{ max, weight int }{{4, 2}, {3, 0}} {
		w, u, err := batch(c.max, c.weight)
		if err != nil {
			t.Fatalf("a weight of %d, at most %d: %v", c.weight, c.max, err)
		}
		w.written(u, map[string]string{"Q1": "[[B]]\n", "Q2": "[[B]]\n", "W": "w [[B]]\n"})
	}
}

// A batch's rewrite counts the links it would write against MaxLinks,
// before it reads the tree, a lock or a content, as its pages (M9/P3
// review B1-M3: the unit's time follows the links): Q1's two of the index
// and the one of W, written, that its own change meets, 3; at a bound of 3
// the three are written, past one of 2 the refusal says 3; 0 bounds
// nothing.
func TestABatchCountsTheLinksItWouldWrite(t *testing.T) {
	batch := func(max int) (*rewriting, *parses, *appender, error) {
		w := newRewriting(t, "A", "Q1", "W")
		w.write("Q1", "[[A]] and [[A]]\n")
		parser := &parses{RewriteParser: w.rewrite.Parser}
		w.rewrite.Parser = parser
		before := w.before("w [[A]]\n", "A")
		facts := w.wrote("W", "w [[A]]\n")
		merged := w.rename("A", "B")
		u, err := w.batch(app.Moved{Changes: []domain.Change{merged}, MaxLinks: max, MaxPages: 10, WrittenWeight: 2,
			Written: map[uuid.UUID]app.WrittenPage{w.id("W"): {Before: before, Changes: []domain.Change{w.renamed("B", "A", "B")},
				Facts: facts}}})
		return w, parser, u, err
	}
	w, parser, _, err := batch(2)
	var many *app.TooManyRewrites
	if !errors.As(err, &many) || !errors.Is(err, app.ErrTooManyRewrites) || many.Links != 3 || many.Max != 2 || many.Pages != 0 {
		t.Fatalf("past the links' bound: %v %+v, want TooManyRewrites of 3 links at most 2", err, many)
	}
	if w.locks.reads != 0 || len(parser.contents) != 0 {
		t.Errorf("the locks read %d times and %d contents parsed before the count", w.locks.reads, len(parser.contents))
	}
	for _, max := range []int{3, 0} {
		w, _, u, err := batch(max)
		if err != nil {
			t.Fatalf("at most %d links: %v", max, err)
		}
		w.written(u, map[string]string{"Q1": "[[B]] and [[B]]\n", "W": "w [[B]]\n"})
	}
}

// askedPaths is Pages that records the ids each read of paths asks for.
type askedPaths struct {
	app.Pages
	asked [][]uuid.UUID
}

func (a *askedPaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) ([]domain.Node, error) {
	a.asked = append(a.asked, slices.Clone(ids))
	return a.Pages.Paths(ctx, notebookID, ids)
}

// A batch's rewrite writes the pages it wrote again first (M9/P3 design
// 3.7.2 (a)): their read backs, of their final contents, are the largest
// it parses, taken while the budget holds none of the shares the facts of
// its writes keep to the unit's end. On a budget of 44 KiB, W of 40 KiB is
// written again, then the index's two pages of 30 KiB; after them, whose
// facts keep some 3 KiB each, W's read back would be server_busy.
func TestABatchRewritesThePagesItWroteFirst(t *testing.T) {
	w := newRewriting(t, "A", "W", "R1", "R2")
	filler := strings.Repeat("r", 30<<10) + "\n"
	w.write("R1", "[[A]]\n"+filler)
	w.write("R2", "[[A]]\n"+filler)
	w.rewrite.Parser = markdownadapter.NewParser(w.md, markdown.NewBudget(44<<10, time.Hour, slog.New(slog.DiscardHandler)))
	w.rewrite.MaxContent = 64 << 10
	content := "[[A]] " + strings.Repeat("w", 40<<10) + "\n"
	before := w.before(content, "A")
	facts := w.wrote("W", content)
	renamed := w.rename("A", "B")
	u, err := w.batch(app.Moved{Changes: []domain.Change{renamed}, Written: map[uuid.UUID]app.WrittenPage{
		w.id("W"): {Before: before, Changes: []domain.Change{renamed}, Facts: facts},
	}})
	if err != nil {
		t.Fatalf("the batch's rewrite: %v, want W, then the index's pages, within the budget", err)
	}
	w.written(u, map[string]string{"W": "[[B]] " + strings.Repeat("w", 40<<10) + "\n", "R1": "[[B]]\n" + filler, "R2": "[[B]]\n" + filler})
	if first := u.writes[0].PageID; first != w.id("W") {
		t.Errorf("the first page written again is %s, want W", first)
	}
}

// The plan's links resolve and rewrite by the participant's rules, on the
// tree they are given (M9/P3 design 3.7.4): the bytes a page the batch
// wrote is written again with; the aliases of the pages the batch deleted
// by then none.
func TestBatchLinksRewriteAsTheParticipant(t *testing.T) {
	w := newRewriting(t, "A", "P", "R")
	w.write("R", "---\naliases: [B]\n---\n")
	links := app.BatchLinks{Store: w.store, Logger: w.rewrite.Logger, MaxContent: 1 << 10}
	parse := func(_ context.Context, content string) (domain.Facts, error) { return w.facts(content), nil }
	on := app.LinksOn{NotebookID: w.notebook, PageID: w.id("P"), Content: "see [[A]] and [[B]]\n", Tree: w.tree, Parse: parse}
	before, err := links.Resolve(context.Background(), on)
	if err != nil {
		t.Fatal(err)
	}
	if got := before.Leads; len(got) != 2 || got[6].ID != w.id("A") || got[16].ID != w.id("R") {
		t.Errorf("Resolve = %+v", got)
	}
	w.remove("R")
	on.Gone = []uuid.UUID{w.id("R")}
	if before, err = links.Resolve(context.Background(), on); err != nil || before.Leads[16].ID != (uuid.UUID{}) {
		t.Errorf("with R deleted: %+v, %v; want [[B]] to lead nowhere", before.Leads, err)
	}
	changes := []domain.Change{w.rename("A", "C")}
	got, err := links.Rewrite(context.Background(), app.LinksMoved{LinksOn: on, Before: before, Changes: changes})
	if err != nil {
		t.Fatal(err)
	}
	facts := w.wrote("P", on.Content)
	u, err := w.batch(app.Moved{Changes: changes, Gone: on.Gone, Written: map[uuid.UUID]app.WrittenPage{
		w.id("P"): {Before: before, Changes: changes, Facts: facts},
	}})
	if err != nil || len(u.writes) != 1 || u.writes[0].Content != got || got != "see [[C]] and [[B]]\n" {
		t.Errorf("the plan wrote %q, the participant %+v (%v)", got, u.writes, err)
	}
}
