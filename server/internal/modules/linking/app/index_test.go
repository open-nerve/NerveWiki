package app_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// world is a notebook's tree and index, and the observer over them.
type world struct {
	t         *testing.T
	tree      *tree
	store     *store
	published *publisher
	index     app.Index
	notebook  uuid.UUID
	revision  int
}

func newWorld(t *testing.T, pages ...string) *world {
	w := &world{t: t, tree: newTree(), store: newStore(), published: &publisher{}, notebook: uuid.NewV7()}
	w.index = app.Index{Store: w.store, Pages: w.tree, Publisher: w.published}
	for _, p := range pages {
		w.tree.add(p)
	}
	return w
}

func (w *world) id(path string) uuid.UUID {
	id, ok := w.tree.names[path]
	if !ok {
		w.t.Fatalf("no page %s", path)
	}
	return id
}

func (w *world) place(id uuid.UUID) *domain.Place {
	n := w.tree.nodes[id]
	return &domain.Place{ParentID: n.parent, Name: n.name}
}

// write is the change of a content of the page at path with links to
// targets, the first at 0, the next at 10, and aliases.
func (w *world) write(path string, targets []string, aliases ...string) domain.Change {
	id := w.id(path)
	w.revision++
	f := domain.Facts{FrontmatterValid: true, Aliases: domain.AliasesOf(aliases)}
	for i, target := range targets {
		f.Links = append(f.Links, domain.Link{Kind: "wikilink", Target: target, Start: 10 * i, End: 10*i + len(target)})
	}
	return domain.Change{NodeID: id, Before: w.place(id), After: w.place(id), Revision: w.revision, Facts: f}
}

// create is the change that creates the page at path, its content empty.
func (w *world) create(path string) domain.Change {
	id := w.tree.add(path)
	w.revision++
	return domain.Change{NodeID: id, After: w.place(id), Revision: w.revision, Facts: domain.Facts{FrontmatterValid: true}}
}

func (w *world) rename(path, name string) domain.Change {
	id := w.id(path)
	before := w.place(id)
	w.tree.nodes[id].name = name
	w.repath(path, path[:strings.LastIndex(path, "/")+1]+name)
	return domain.Change{NodeID: id, Before: before, After: w.place(id)}
}

// repath has the pages at from, and under it, be at to.
func (w *world) repath(from, to string) {
	for p, id := range w.tree.names {
		if rest, ok := strings.CutPrefix(p, from); ok && (rest == "" || rest[0] == '/') {
			delete(w.tree.names, p)
			w.tree.names[to+rest] = id
		}
	}
}

// move is the changes that move the page at path under parent ("" for the
// root): its own, and those of the pages under it, which stay.
func (w *world) move(path, parent string) []domain.Change {
	id := w.id(path)
	before := w.place(id)
	w.tree.nodes[id].parent = nil
	to := w.tree.nodes[id].name
	if parent != "" {
		p := w.id(parent)
		w.tree.nodes[id].parent = &p
		to = parent + "/" + to
	}
	w.repath(path, to)
	out := []domain.Change{{NodeID: id, Before: before, After: w.place(id)}}
	sub, _ := w.tree.Subtree(context.Background(), w.notebook, id)
	for _, s := range sub[1:] {
		out = append(out, domain.Change{NodeID: s.ID, Before: w.place(s.ID), After: w.place(s.ID)})
	}
	return out
}

// remove is the changes that delete the page at path and those under it.
func (w *world) remove(path string) []domain.Change {
	sub, _ := w.tree.Subtree(context.Background(), w.notebook, w.id(path))
	var out []domain.Change
	for _, s := range sub {
		out = append(out, domain.Change{NodeID: s.ID, Before: w.place(s.ID)})
		w.tree.nodes[s.ID].gone = true
	}
	return out
}

// run has the observer follow a unit of changes.
func (w *world) run(changes ...domain.Change) {
	w.t.Helper()
	w.published.events = nil
	if err := w.index.PagesChanged(context.Background(), app.PagesChanged{WorkspaceID: uuid.NewV7(), NotebookID: w.notebook, Changes: changes}); err != nil {
		w.t.Fatal(err)
	}
}

// resolves checks where the link of the page at path at start resolves.
func (w *world) resolves(path string, start int, to string, ambiguous bool) {
	w.t.Helper()
	want := domain.Resolution{Ambiguous: ambiguous}
	if to != "" {
		want.ID = w.id(to)
	}
	if got := w.store.resolution(w.id(path), start); got != want {
		w.t.Errorf("the link of %s at %d resolves to %v, want %s (%v)", path, start, got, to, ambiguous)
	}
}

// publishedEvent checks the one links event of the last unit, by paths, of
// a unit that kept the aliases; none when pages and targets are both nil.
func (w *world) publishedEvent(pages, targets []string) {
	w.t.Helper()
	w.publishedOf(pages, targets, false)
}

// publishedAliases checks the one links event of the last unit, by paths,
// of a unit that changed the aliases (M8 Codex review R3).
func (w *world) publishedAliases(pages, targets []string) {
	w.t.Helper()
	w.publishedOf(pages, targets, true)
}

func (w *world) publishedOf(pages, targets []string, aliases bool) {
	w.t.Helper()
	if pages == nil && targets == nil && !aliases {
		if len(w.published.events) != 0 {
			w.t.Errorf("published %+v, want none", w.published.events)
		}
		return
	}
	if len(w.published.events) != 1 {
		w.t.Fatalf("published %d events, want 1", len(w.published.events))
	}
	e := w.published.events[0]
	if e.NotebookID != w.notebook {
		w.t.Errorf("the event is of notebook %s, want %s", e.NotebookID, w.notebook)
	}
	if e.Aliases != aliases {
		w.t.Errorf("the event's aliases = %v, want %v", e.Aliases, aliases)
	}
	if got, want := e.Pages, w.ids(pages); !reflect.DeepEqual(got, want) {
		w.t.Errorf("the event's pages = %v, want %v", got, pages)
	}
	if got, want := e.Targets, w.ids(targets); !reflect.DeepEqual(got, want) {
		w.t.Errorf("the event's targets = %v, want %v", got, targets)
	}
}

func (w *world) ids(paths []string) []uuid.UUID {
	out := []uuid.UUID{}
	for _, p := range paths {
		out = append(out, w.id(p))
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// A unit that writes no content of a page it leaves, and relocates no node,
// leaves the index alone, untouched and unlocked: a move among siblings,
// with the pages under it, a rename to the same key and length, a page
// created and deleted.
func TestAUnitWithNothingForTheIndexIsLeftAlone(t *testing.T) {
	w := newWorld(t, "A", "A/B", "src")
	w.run(w.write("src", []string{"B"}))
	w.store.locked = nil
	order := w.move("A", "")
	created := w.create("Gone")
	created.After = nil
	w.run(append(order, w.rename("A/B", "b"), created)...)
	if len(w.store.locked) != 0 || len(w.published.events) != 0 {
		t.Errorf("the unit locked %v and published %v", w.store.locked, w.published.events)
	}
}

// A page's content written: its links resolve, under the notebook's lock;
// the event lists the pages they resolve to, not the page.
func TestAContentsLinksResolve(t *testing.T) {
	w := newWorld(t, "A", "A/B", "src")
	w.run(w.write("src", []string{"B", "Missing", "A/B"}))
	w.resolves("src", 0, "A/B", false)
	w.resolves("src", 10, "", false)
	w.resolves("src", 20, "A/B", false)
	if !reflect.DeepEqual(w.store.locked, []uuid.UUID{w.notebook}) {
		t.Errorf("locked %v, want the notebook", w.store.locked)
	}
	w.publishedEvent([]string{}, []string{"A/B"})

	// Written again, the links it had go: the pages they resolved to are
	// targets too.
	w.run(w.write("src", []string{"A"}))
	w.resolves("src", 0, "A", false)
	w.publishedEvent([]string{}, []string{"A", "A/B"})

	// Links to nothing written, and nothing resolved before: no event.
	w.run(w.write("src", []string{"Missing"}))
	w.publishedEvent([]string{}, []string{"A"})
	w.run(w.write("src", []string{"Other missing"}))
	w.publishedEvent(nil, nil)
}

// A page created resolves the links to its name; its own resolve too.
func TestAPageCreatedTakesTheLinksToItsName(t *testing.T) {
	w := newWorld(t, "src")
	w.run(w.write("src", []string{"New"}))
	w.resolves("src", 0, "", false)
	w.run(w.create("New"))
	w.resolves("src", 0, "New", false)
	w.publishedEvent([]string{"src"}, []string{"New"})
}

// A page renamed: the links to it and to the pages under it, by their
// paths, resolve anew, and those to its new name; the event lists the
// pages they are written in and the pages they resolve to.
func TestAPageRenamedResolvesItsSubtreesLinksAnew(t *testing.T) {
	w := newWorld(t, "A", "A/B", "A/B/C", "src")
	w.run(w.write("src", []string{"A/B", "D/B", "D/B/C", "A/B/C", "D"}))
	w.resolves("src", 0, "A/B", false)
	w.resolves("src", 10, "", false)
	w.run(w.rename("A", "D"))
	w.resolves("src", 0, "", false)
	w.resolves("src", 10, "D/B", false)
	w.resolves("src", 20, "D/B/C", false)
	w.resolves("src", 30, "", false)
	w.resolves("src", 40, "D", false)
	w.publishedEvent([]string{"src"}, []string{"D", "D/B", "D/B/C"})
}

// A page moved: the links to it and under it resolve anew, and those
// written in them, from their new folder, which prefers its own subtree.
func TestAPageMovedResolvesItsLinksAndThoseToItAnew(t *testing.T) {
	w := newWorld(t, "A", "A/B", "A/B/src", "X", "X/sib", "sib", "P", "P/dup", "P/s2", "Q", "Q/dup")
	w.run(w.write("A/B/src", []string{"sib", "../sib"}))
	w.run(w.write("sib", []string{"X/B", "A/B"}))
	w.run(w.write("P/s2", []string{"dup"}))
	w.resolves("A/B/src", 0, "sib", false)
	w.resolves("A/B/src", 10, "", false)
	w.resolves("P/s2", 0, "P/dup", false)
	w.run(w.move("A/B", "X")...)
	w.resolves("X/B/src", 0, "sib", false)
	w.resolves("X/B/src", 10, "X/sib", false)
	w.resolves("sib", 0, "X/B", false)
	w.resolves("sib", 10, "", false)
	w.publishedEvent([]string{"X/B/src", "sib"}, []string{"X/sib", "X/B"})
	w.run(w.move("P/s2", "Q")...)
	w.resolves("Q/s2", 0, "Q/dup", false)
	w.publishedEvent([]string{"Q/s2"}, []string{"P/dup", "Q/dup"})
}

// Pages deleted: their rows go, the links to them resolve anew, and the
// pages their own links resolved to lose those backlinks.
func TestPagesDeletedLeaveTheIndex(t *testing.T) {
	w := newWorld(t, "A", "A/dup", "B", "B/dup", "src", "B/src2")
	w.run(w.write("src", []string{"dup"}))
	w.run(w.write("B/src2", []string{"src"}))
	w.run(w.write("B/dup", []string{"A"}))
	w.resolves("src", 0, "A/dup", true)
	w.run(w.remove("A")...)
	w.resolves("src", 0, "B/dup", false)
	w.resolves("B/dup", 0, "", false)
	w.publishedEvent([]string{"B/dup", "src"}, []string{"A", "A/dup", "B/dup"})
	w.run(w.remove("B")...)
	w.resolves("src", 0, "", false)
	if _, ok := w.store.facts[w.id("B/src2")]; ok {
		t.Error("a deleted page keeps its facts")
	}
	w.publishedEvent([]string{"src"}, []string{"B/dup", "src"})
}

// An alias written or dropped: the links to it resolve anew, as a name
// alone, after every page's name. Its event tells that the aliases
// changed, though no link of the index resolves otherwise.
func TestAnAliasWrittenOrDroppedResolvesTheLinksToIt(t *testing.T) {
	w := newWorld(t, "X", "Y", "src", "Named")
	w.run(w.write("src", []string{"nick", "Named", "Y/nick"}))
	w.run(w.write("X", nil, "Nick", "Named"))
	w.resolves("src", 0, "X", false)
	w.resolves("src", 10, "Named", false)
	w.resolves("src", 20, "", false)
	w.publishedAliases([]string{"src"}, []string{"X"})
	w.run(w.write("Y", nil, "nick"))
	w.resolves("src", 0, "X", true)
	w.publishedAliases([]string{}, []string{})
	w.run(w.write("X", nil))
	w.resolves("src", 0, "Y", false)
	w.publishedAliases([]string{"src"}, []string{"X", "Y"})
}

// A unit that changes the notebook's aliases says so in its links event,
// one of its own when no link the index holds resolves otherwise: a link
// of a page's version, which the index does not hold, may resolve by them
// (M8 Codex review R3). An alias added by a content or with its page, or
// dropped by a content or with its page, changes them; a content written
// that keeps its aliases' keys, its case alone changed or its links, does
// not, and publishes what its links change alone.
func TestAUnitThatChangesTheAliasesSaysSo(t *testing.T) {
	w := newWorld(t, "T", "Other")
	w.run(w.write("T", nil, "OldAlias"))
	w.publishedAliases([]string{}, []string{})
	w.run(w.write("T", []string{"Other"}, "OldAlias"))
	w.publishedEvent([]string{}, []string{"Other"})
	w.run(w.write("T", nil, "oldalias"))
	w.publishedEvent([]string{}, []string{"Other"})
	w.run(w.write("T", nil, "oldalias"))
	w.publishedEvent(nil, nil)
	w.run(w.write("T", nil))
	w.publishedAliases([]string{}, []string{})
	created := w.create("New")
	created.Facts.Aliases = domain.AliasesOf([]string{"Nick"})
	w.run(created)
	w.publishedAliases([]string{}, []string{})
	w.run(w.remove("New")...)
	w.publishedAliases([]string{}, []string{})
	w.run(w.remove("Other")...)
	w.publishedEvent(nil, nil)
}

// A link whose tie alone changes is set, and in no event.
func TestATieAloneIsNoEvent(t *testing.T) {
	w := newWorld(t, "A", "A/dup", "B", "B/dup", "src")
	w.run(w.write("src", []string{"dup"}))
	w.resolves("src", 0, "A/dup", true)
	w.run(w.remove("B")...)
	w.resolves("src", 0, "A/dup", false)
	w.publishedEvent(nil, nil)
}

// More than MaxEventPages pages in a set lists none: null in the event.
func TestAnEventWithTooManyPagesListsNone(t *testing.T) {
	w := newWorld(t)
	for i := range app.MaxEventPages + 1 {
		path := "src" + string(rune('a'+i))
		w.tree.add(path)
		w.run(w.write(path, []string{"New"}))
	}
	w.run(w.create("New"))
	e := w.published.events[0]
	if e.Pages != nil || !reflect.DeepEqual(e.Targets, []uuid.UUID{w.id("New")}) {
		t.Errorf("the event = %+v, want no pages and the one target", e)
	}
}

// Both sets past MaxEventPages are still an event, which lists none: a
// rename of a folder that many pages link into.
func TestAnEventWithTooManyPagesAndTargetsListsNone(t *testing.T) {
	w := newWorld(t, "A")
	for i := range app.MaxEventPages + 1 {
		name := string(rune('a' + i))
		w.tree.add("A/" + name)
		w.tree.add("src" + name)
		w.run(w.write("src"+name, []string{"A/" + name}))
	}
	w.run(w.rename("A", "Z"))
	if len(w.published.events) != 1 || w.published.events[0].Pages != nil || w.published.events[0].Targets != nil {
		t.Errorf("published %+v, want one event that lists none", w.published.events)
	}
}

// A content written moves no page: it reaches its own links, and the links
// by the aliases it drops or adds, not those to it or by the aliases it
// keeps.
func TestAContentWrittenReachesItsLinksAndTheAliasesItChanges(t *testing.T) {
	w := newWorld(t, "X", "src")
	w.run(w.write("src", []string{"X", "kept", "dropped", "added"}))
	w.run(w.write("X", []string{"Y"}, "kept", "dropped"))
	w.run(w.write("X", []string{"Z"}, "kept", "added"))
	if want := (domain.Reach{Keys: []string{"added", "dropped"}, Sources: []uuid.UUID{w.id("X")}}); !reflect.DeepEqual(w.store.reached, want) {
		t.Errorf("the write reached %+v, want %+v", w.store.reached, want)
	}
	w.resolves("src", 10, "X", false)
	w.resolves("src", 20, "", false)
	w.resolves("src", 30, "X", false)
	w.publishedAliases([]string{"src"}, []string{"X"})
}

// A link whose page the tree does not have is a defect: an error, as is a
// link the store does not find to set.
func TestALinkOfNoPageIsAnError(t *testing.T) {
	w := newWorld(t, "src", "B")
	w.run(w.write("src", []string{"B"}))
	w.tree.nodes[w.id("src")].gone = true
	err := w.index.PagesChanged(context.Background(), app.PagesChanged{NotebookID: w.notebook, Changes: []domain.Change{w.rename("B", "C")}})
	if err == nil {
		t.Error("a link of no page resolved")
	}
	w.tree.nodes[w.id("src")].gone = false
	w.store.missing = true
	if err := w.index.PagesChanged(context.Background(), app.PagesChanged{NotebookID: w.notebook, Changes: []domain.Change{w.rename("C", "D")}}); err == nil {
		t.Error("a link not found was set")
	}
}

// A page deleted with an alias takes its aliases' keys along: a tie of
// another page with that alias ends.
func TestAPageDeletedWithAnAliasEndsItsTie(t *testing.T) {
	w := newWorld(t, "Y", "X", "src")
	w.run(w.write("Y", nil, "nick"))
	w.run(w.write("X", nil, "nick"))
	w.run(w.write("src", []string{"nick"}))
	w.resolves("src", 0, "Y", true)
	w.run(w.remove("X")...)
	w.resolves("src", 0, "Y", false)
	w.publishedAliases([]string{}, []string{})
}

// A page with an alias in both of a target's forms is one candidate, no
// tie with itself.
func TestAPageAliasedInBothFormsIsOneCandidate(t *testing.T) {
	w := newWorld(t, "X", "src")
	w.run(w.write("X", nil, "Al", "Al.md"))
	w.run(w.write("src", []string{"Al.md"}))
	w.resolves("src", 0, "X", false)
}

// An alias a content no longer writes takes its key along: a tie of
// another page with that alias ends.
func TestAnAliasDroppedEndsItsTie(t *testing.T) {
	w := newWorld(t, "Y", "X", "src")
	w.run(w.write("Y", nil, "nick"))
	w.run(w.write("X", nil, "nick"))
	w.run(w.write("src", []string{"nick"}))
	w.resolves("src", 0, "Y", true)
	w.run(w.write("X", nil))
	w.resolves("src", 0, "Y", false)
	w.publishedAliases([]string{}, []string{})
}

// A target written with ".md" reaches the pages named so, by its second
// key: a page whose title ends with ".md".
func TestATargetWithMdReachesAPageNamedSo(t *testing.T) {
	w := newWorld(t, "Note.md", "src")
	w.run(w.write("src", []string{"Note.md"}))
	w.resolves("src", 0, "Note.md", false)
}

// An alias of a page the tree does not have is a defect: an error.
func TestAnAliasOfNoPageIsAnError(t *testing.T) {
	w := newWorld(t, "X", "src")
	w.run(w.write("X", nil, "nick"))
	w.tree.nodes[w.id("X")].gone = true
	err := w.index.PagesChanged(context.Background(), app.PagesChanged{NotebookID: w.notebook, Changes: []domain.Change{w.write("src", []string{"nick"})}})
	if err == nil {
		t.Error("a link resolved by the alias of no page")
	}
}

// A page with an alias moved: its path decides between the pages with
// that alias as between those with a name, so the links to the alias
// resolve anew, those to the other page's too.
func TestAPageWithAnAliasMovedResolvesTheLinksToItsAliasAnew(t *testing.T) {
	w := newWorld(t, "X", "Y", "P", "src")
	w.run(w.write("X", nil, "nick"))
	w.run(w.write("Y", nil, "nick"))
	w.run(w.write("src", []string{"nick"}))
	w.resolves("src", 0, "X", true)
	w.run(w.move("X", "P")...)
	w.resolves("src", 0, "Y", false)
	w.publishedEvent([]string{"src"}, []string{"P/X", "Y"})
	w.run(w.move("Y", "P")...)
	w.resolves("src", 0, "P/X", true)
	w.publishedEvent([]string{"src"}, []string{"P/X", "P/Y"})
}

// Each source resolves the same relative target from its own folder, whether
// sources are ordered, descending or reappear later. Zero is an ordinary source.
func TestResolutionCachesFollowEachSource(t *testing.T) {
	for _, tc := range []struct {
		name  string
		zero  bool
		order [4]int
	}{
		{"ordered", false, [4]int{0, 1, 2, 3}},
		{"descending", false, [4]int{2, 3, 0, 1}},
		{"reappearing", false, [4]int{0, 2, 1, 3}},
		{"zero_ordered", true, [4]int{0, 1, 2, 3}},
		{"zero_descending", true, [4]int{2, 3, 0, 1}},
		{"zero_reappearing", true, [4]int{0, 2, 1, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, "left", "left/x", "left/src", "right", "right/x", "right/src")
			if tc.zero {
				old := w.id("left/src") // A leaf: no child's parent needs changing.
				w.tree.nodes[uuid.UUID{}] = w.tree.nodes[old]
				delete(w.tree.nodes, old)
				w.tree.names["left/src"] = uuid.UUID{}
			}
			if w.id("left/src").Compare(w.id("right/src")) >= 0 {
				t.Fatal("fixture source IDs are not strictly ordered")
			}
			// Seed the existing fake index without resolving first. Both forms
			// have the same canonical target, with a different answer per source.
			for _, path := range []string{"left/src", "right/src"} {
				c := w.write(path, []string{"./x", "./X"})
				p := app.Page{ID: c.NodeID, NotebookID: w.notebook, Revision: c.Revision}
				if err := w.store.AddPage(t.Context(), p, c.Facts); err != nil {
					t.Fatal(err)
				}
			}
			seeded := slices.Clone(w.store.links)
			if len(seeded) != 4 {
				t.Fatalf("seeded %d links, want four", len(seeded))
			}
			for i, j := range tc.order {
				w.store.links[i] = seeded[j]
			}
			left, right := w.id("left/x"), w.id("right/x")
			if left == (uuid.UUID{}) || right == (uuid.UUID{}) || left == right {
				t.Fatal("expected two distinct nonempty relative targets")
			}
			wantIDs := [4]uuid.UUID{left, left, right, right}
			// Creating root x reaches every ./x and ./X through the original
			// Store.Links filter, which retains the chosen order. It must not
			// replace either folder's relative answer with the new root page.
			w.run(w.create("x"))
			if len(w.store.links) != 4 {
				t.Fatalf("index has %d links, want four", len(w.store.links))
			}
			for i, j := range tc.order {
				want := seeded[j]
				want.Resolution = domain.Resolution{ID: wantIDs[j]}
				if got := w.store.links[i]; got != want {
					t.Errorf("link %d = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

// A view can contain more links than an index keeps. New, repeated and
// invalid targets still have an answer after many distinct raw targets.
func TestAViewResolvesAllLinksAfterManyDistinctTargets(t *testing.T) {
	w := newWorld(t, "folder", "folder/src", "folder/x", "x", "tail")
	source := w.id("folder/src")
	x, tail := w.id("folder/x"), w.id("tail")
	if x == (uuid.UUID{}) || tail == (uuid.UUID{}) || x == tail {
		t.Fatal("expected distinct nonempty relative and tail targets")
	}
	// Fill a fixed 10,000 distinct raw targets, including the entry at
	// index zero and two invalid forms, before the mixed six-link tail.
	targets := make([]string, 10_000, 10_006)
	targets[0], targets[1], targets[2] = "./x", "", "a//b"
	for i := 3; i < 10_000; i++ {
		targets[i] = fmt.Sprintf("missing%05d", i)
	}
	targets = append(targets, "tail", "./x", "tail", "", "./x", "a//b")
	links := make([]app.Link, len(targets))
	for i, target := range targets {
		links[i] = app.Link{SourceID: source, Start: 10 * i, Target: target}
	}
	// No index rows were seeded: the real reading-view path resolves
	// every input anew, including those beyond a page's index limit.
	views := app.Views{Store: w.store, Pages: w.tree}
	got, err := views.Resolve(t.Context(), app.Page{ID: source, NotebookID: w.notebook, Revision: 1}, links)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]domain.Resolution, 10_006)
	want[0] = domain.Resolution{ID: x}
	want[10_000], want[10_002] = domain.Resolution{ID: tail}, domain.Resolution{ID: tail}
	want[10_001], want[10_004] = domain.Resolution{ID: x}, domain.Resolution{ID: x}
	if len(got) != len(want) {
		t.Fatalf("resolved %d starts, want all %d", len(got), len(want))
	}
	for i, expected := range want {
		start := 10 * i
		if actual, present := got[start]; !present || actual != expected {
			t.Errorf("start %d present=%t resolution=%+v, want %+v", start, present, actual, expected)
		}
	}
	if w.store.viewed != 1 || w.tree.reads != 2 {
		t.Errorf("viewed %d, read candidates and paths %d times, want 1 and 2", w.store.viewed, w.tree.reads)
	}
}
