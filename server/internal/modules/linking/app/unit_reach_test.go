package app_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A single rename or move reads and resolves the links it reaches once
// (M12/P4/S7 design 3.2, 4.3): its participant leaves them in the unit's
// memo, its observer takes them. These run a unit as the page module does,
// over the fakes: the participant follows the step, then the observer the
// unit's changes, the step's merged by node with the pages the participant
// wrote; each case once with a memo and once without (B off), whose reads
// and results the first must have, or fewer reads alone where B is used.

// memo is a unit's memo as the page module's: what is stored under a key
// until taken or dropped; safe when nil, as the page module's is.
type memo struct {
	values map[any]any
}

func (m *memo) Store(key, value any) {
	if m == nil {
		return
	}
	if m.values == nil {
		m.values = map[any]any{}
	}
	m.values[key] = value
}

func (m *memo) Take(key any) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.values[key]
	delete(m.values, key)
	return v, ok
}

func (m *memo) Drop(key any) {
	if m != nil {
		delete(m.values, key)
	}
}

func (m *memo) len() int {
	if m == nil {
		return 0
	}
	return len(m.values)
}

// unitReads is what a unit read and what it did: the reaches of the
// links it read, the reads of candidates, paths and aliases, of the keys
// of pages' aliases and of subtrees (the observer's spread reads those
// two, code review B, M1); the resolutions it set and the events it
// published, by path; the pages its participant wrote.
type unitReads struct {
	reaches                []domain.Reach
	reached                []string // each reach, by path
	byKeys, paths, aliases int
	aliasKeys, subtrees    int
	set, events            []string
	writes                 int
}

// reads is u's reads without its results.
func (u unitReads) reads() string {
	return fmt.Sprintf("links %d, by keys %d, paths %d, aliases %d, alias keys %d, subtrees %d",
		len(u.reaches), u.byKeys, u.paths, u.aliases, u.aliasKeys, u.subtrees)
}

// unit runs m, a unit's single step, with mem, as the page module runs a
// unit: the participant, then the observer the unit's changes, observe of
// the changes merged (nil: those merged themselves).
func (w *rewriting) unit(m app.Moved, mem app.UnitMemo, observe func([]domain.Change) []domain.Change) (unitReads, error) {
	w.t.Helper()
	// Counted from here: the step was made before, the move's by Subtree.
	w.store.reaches, w.store.aliasReads, w.store.aliasKeys, w.store.resolved = nil, 0, 0, nil
	w.tree.byKeys, w.tree.paths, w.tree.subtrees = 0, 0, 0
	w.published.events = nil
	m.NotebookID, m.At, m.Memo = w.notebook, time.Now(), mem
	u := &appender{}
	if err := w.rewrite.Participate(context.Background(), m, u); err != nil {
		return unitReads{}, err
	}
	changes := slices.Clone(m.Changes)
	for _, x := range u.writes {
		w.revision++
		c := domain.Change{NodeID: x.PageID, Before: w.place(x.PageID), After: w.place(x.PageID), Revision: w.revision, Facts: w.facts(x.Content)}
		if i := slices.IndexFunc(changes, func(o domain.Change) bool { return o.NodeID == x.PageID }); i >= 0 {
			changes[i].Revision, changes[i].Facts = c.Revision, c.Facts
		} else {
			changes = append(changes, c)
		}
	}
	if observe != nil {
		changes = observe(changes)
	}
	err := w.index.PagesChanged(context.Background(), app.PagesChanged{WorkspaceID: uuid.NewV7(), NotebookID: w.notebook, Changes: changes, Memo: mem})
	out := unitReads{reaches: w.store.reaches, byKeys: w.tree.byKeys, paths: w.tree.paths, aliases: w.store.aliasReads,
		aliasKeys: w.store.aliasKeys, subtrees: w.tree.subtrees, writes: len(u.writes)}
	for _, r := range w.store.reaches {
		keys := slices.Clone(r.Keys)
		slices.Sort(keys)
		out.reached = append(out.reached, fmt.Sprintf("keys %v targets %v sources %v", keys, w.pathsOf(r.Targets), w.pathsOf(r.Sources)))
	}
	for _, set := range w.store.resolved {
		for _, l := range set {
			out.set = append(out.set, fmt.Sprintf("%s@%d->%s", w.pathOf(l.SourceID), l.Start, w.pathOf(l.Resolution.ID)))
		}
	}
	for _, e := range w.published.events {
		out.events = append(out.events, fmt.Sprintf("pages %v targets %v aliases %t", w.pathsOf(e.Pages), w.pathsOf(e.Targets), e.Aliases))
	}
	return out, err
}

// pathOf is id's path in the tree now, "" for none.
func (w *rewriting) pathOf(id uuid.UUID) string {
	if id == uuid.Nil() {
		return ""
	}
	for p, n := range w.tree.names {
		if n == id {
			return p
		}
	}
	return id.String()
}

func (w *rewriting) pathsOf(ids []uuid.UUID) []string {
	out := []string{}
	for _, id := range ids {
		out = append(out, w.pathOf(id))
	}
	slices.Sort(out)
	return out
}

// unitReachWorld is the world of the cases: a link that leads nowhere until
// X is under A ([[A/X]]), one that leads nowhere until C is Newname
// ([[Newname]]), and a value of a page's aliases that leads nowhere until X
// is under A; none of them is written again.
func unitReachWorld(t *testing.T) *rewriting {
	w := newRewriting(t, "A", "B", "B/X", "B/X/Y", "C", "src", "al", "far")
	w.write("src", "[[A/X]] [[Newname]] [[far]]\n")
	w.write("al", "---\naliases: [\"[[A/X]]\"]\n---\n")
	w.write("far", "[[Y]]\n")
	return w
}

func treeStep(changes []domain.Change) app.Moved {
	return app.Moved{UpdateLinks: true, Tree: true, Changes: changes}
}

// unitCase is a unit of one step over a world: its step, and the changes
// the observer gets of the merged (nil: those).
type unitCase struct {
	name    string
	world   func(t *testing.T) *rewriting
	step    func(w *rewriting) app.Moved
	observe func(w *rewriting, changes []domain.Change) []domain.Change
}

// both runs c with a memo and without one, on two worlds alike, and
// answers each's reads, failing on an error.
func (c unitCase) both(t *testing.T) (with, without unitReads, mem *memo) {
	t.Helper()
	for _, on := range []bool{true, false} {
		w := c.world(t)
		var m app.UnitMemo
		if on {
			mem = &memo{}
			m = mem
		}
		var observe func([]domain.Change) []domain.Change
		if c.observe != nil {
			observe = func(changes []domain.Change) []domain.Change { return c.observe(w, changes) }
		}
		got, err := w.unit(c.step(w), m, observe)
		if err != nil {
			t.Fatalf("%s (memo %t): %v", c.name, on, err)
		}
		if on {
			with = got
		} else {
			without = got
		}
	}
	return with, without, mem
}

// A single move and a single rename of a tree unit that write no page read
// the links they reach once and resolve them once, and set and publish what
// they did with two reads (M12/P4/S7 design 4.3 item 1): links that lead
// somewhere only after them, a value of the aliases among them. The
// observer skips its spread too: the keys of the aliases are read once,
// and a rename's subtree once (code review B, M1).
func TestASingleRenameOrMoveReadsItsLinksOnce(t *testing.T) {
	for _, c := range []struct {
		unitCase
		with, without string
	}{
		{unitCase{name: "move", world: unitReachWorld, step: func(w *rewriting) app.Moved { return treeStep(w.move("B/X", "A")) }},
			"links 1, by keys 1, paths 1, aliases 1, alias keys 1, subtrees 0",
			"links 2, by keys 2, paths 2, aliases 2, alias keys 2, subtrees 0"},
		{unitCase{name: "rename", world: unitReachWorld, step: func(w *rewriting) app.Moved {
			return treeStep([]domain.Change{w.rename("C", "Newname")})
		}},
			"links 1, by keys 1, paths 1, aliases 1, alias keys 1, subtrees 1",
			"links 2, by keys 2, paths 2, aliases 2, alias keys 2, subtrees 2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			with, without, mem := c.both(t)
			if with.writes != 0 || without.writes != 0 {
				t.Fatalf("the participant wrote %d and %d pages: not the shape", with.writes, without.writes)
			}
			if got := with.reads(); got != c.with {
				t.Errorf("with the memo: %s; want %s", got, c.with)
			}
			if got := without.reads(); got != c.without {
				t.Errorf("without the memo: %s; want %s", got, c.without)
			}
			if len(with.set) == 0 || len(with.events) != 1 || strings.Contains(with.events[0], "[]") ||
				fmt.Sprint(with.set, with.events) != fmt.Sprint(without.set, without.events) {
				t.Errorf("with the memo set %v and published %v; without it %v and %v", with.set, with.events, without.set, without.events)
			}
			if mem.len() != 0 {
				t.Errorf("the memo holds %d after the unit", mem.len())
			}
		})
	}
}

// Wherever a condition of B fails, a unit reads and does what it does
// without the memo (M12/P4/S7 design 4.3 item 2): not a tree unit; a
// batch's step, of moves and renames alone or with a page its content ops
// wrote, a node it deleted, one it froze or one it created after a move;
// a change of a title's case alone; a participant that writes a page,
// another or the one it moves; changes the observer gets that are not the
// participant's (one more, one fewer, in another order, a name, a
// parent, a parent in the last of them, a place before or a version that
// differs); a deletion; a step without update_links; and no
// memo at all, a nil one in the interface.
func TestEveryConditionOfASingleReadHolds(t *testing.T) {
	move := func(w *rewriting) []domain.Change { return w.move("B/X", "A") }
	other := func(w *rewriting, changes []domain.Change) []domain.Change {
		return append(changes, domain.Change{NodeID: w.id("C"), Before: w.place(w.id("C")), After: w.place(w.id("C"))})
	}
	rewritten := func(t *testing.T) *rewriting {
		w := unitReachWorld(t)
		w.write("far", "[[B/X/Y]]\n")
		return w
	}
	ownRewritten := func(t *testing.T) *rewriting {
		w := unitReachWorld(t)
		w.write("B/X", "[[B/X/Y]]\n")
		return w
	}
	cases := []unitCase{
		{name: "not a tree unit", step: func(w *rewriting) app.Moved { m := treeStep(move(w)); m.Tree = false; return m }},
		{name: "a batch of moves alone", step: func(w *rewriting) app.Moved { m := treeStep(move(w)); m.Batch = true; return m }},
		{name: "a batch with a page it wrote", step: func(w *rewriting) app.Moved {
			m := treeStep(move(w))
			m.Batch = true
			facts := w.wrote("far", "[[Y]]\n")
			m.Written = map[uuid.UUID]app.WrittenPage{w.id("far"): {Before: w.before("[[Y]]\n", "A/X/Y"), Facts: facts}}
			return m
		}},
		{name: "a batch with a page it deleted", world: func(t *testing.T) *rewriting {
			w := unitReachWorld(t)
			w.tree.add("G")
			w.write("G", "[[A/X]]\n")
			return w
		}, step: func(w *rewriting) app.Moved {
			m := treeStep(move(w))
			m.Batch = true
			gone := w.remove("G")
			m.Changes = append(m.Changes, gone...)
			m.Gone = []uuid.UUID{w.id("G")}
			return m
		}},
		{name: "a batch with a node it froze", step: func(w *rewriting) app.Moved {
			m := treeStep(move(w))
			m.Batch, m.Frozen = true, []uuid.UUID{w.id("C")}
			return m
		}},
		{name: "a batch with a node it created after a move", step: func(w *rewriting) app.Moved {
			m := treeStep(move(w))
			z := w.tree.add("A/X/Z")
			m.Batch, m.CreatedAfter = true, map[uuid.UUID][]uuid.UUID{w.id("A/X"): {z}}
			m.Changes = append(m.Changes, domain.Change{NodeID: z, After: w.place(z)})
			return m
		}},
		{name: "a title's case alone", step: func(w *rewriting) app.Moved { return treeStep([]domain.Change{w.rename("C", "c")}) }},
		{name: "the participant writes a page", world: rewritten, step: func(w *rewriting) app.Moved { return treeStep(move(w)) }},
		{name: "the participant writes the page it moves", world: ownRewritten, step: func(w *rewriting) app.Moved { return treeStep(move(w)) }},
		{name: "the observer gets one change more", step: func(w *rewriting) app.Moved { return treeStep(move(w)) }, observe: other},
		{name: "the observer gets one change fewer", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(_ *rewriting, changes []domain.Change) []domain.Change { return changes[:len(changes)-1] }},
		{name: "the observer gets the changes in another order", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(_ *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes)
				slices.Reverse(out)
				return out
			}},
		{name: "the observer gets another name", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(_ *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes)
				after := *out[0].After
				after.Name = "Elsewhere"
				out[0].After = &after
				return out
			}},
		{name: "the observer gets another parent", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(w *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes)
				after, c := *out[0].After, w.id("C") // not B, where it was: that would move nothing
				after.ParentID = &c
				out[0].After = &after
				return out
			}},
		{name: "the observer gets another parent in its last change", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(w *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes) // the moved page's and its child Y's
				last, c := *out[len(out)-1].After, w.id("C")
				last.ParentID = &c
				out[len(out)-1].After = &last
				return out
			}},
		{name: "the observer gets another place before", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(_ *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes)
				before := *out[0].Before
				before.Name = "Elsewhere"
				out[0].Before = &before
				return out
			}},
		{name: "the observer gets another version", step: func(w *rewriting) app.Moved { return treeStep(move(w)) },
			observe: func(w *rewriting, changes []domain.Change) []domain.Change {
				out := slices.Clone(changes)
				w.revision++
				out[0].Revision, out[0].Facts = w.revision, w.facts("")
				return out
			}},
		{name: "a deletion", step: func(w *rewriting) app.Moved { return treeStep(w.remove("C")) }},
		{name: "without update_links", step: func(w *rewriting) app.Moved { m := treeStep(move(w)); m.UpdateLinks = false; return m }},
	}
	for _, c := range cases {
		if c.world == nil {
			c.world = unitReachWorld
		}
		t.Run(c.name, func(t *testing.T) {
			with, without, _ := c.both(t)
			if c.name == "the participant writes a page" || c.name == "the participant writes the page it moves" {
				if with.writes == 0 {
					t.Fatal("the participant wrote no page: not the shape")
				}
			}
			if with.reads() != without.reads() || fmt.Sprint(with.reached) != fmt.Sprint(without.reached) {
				t.Errorf("with the memo: %s; without it: %s", with.reads(), without.reads())
			}
			if fmt.Sprint(with.set, with.events) != fmt.Sprint(without.set, without.events) {
				t.Errorf("with the memo set %v and published %v; without it %v and %v", with.set, with.events, without.set, without.events)
			}
		})
	}
	t.Run("a nil memo", func(t *testing.T) {
		w := unitReachWorld(t)
		var none *memo
		got, err := w.unit(treeStep(move(w)), none, nil)
		if err != nil || got.reads() != "links 2, by keys 2, paths 2, aliases 2, alias keys 2, subtrees 0" {
			t.Fatalf("with a nil memo: %s, %v", got.reads(), err)
		}
	})
}

// The memo holds the links no longer than they are of use (M12/P4/S7
// design 4.3 item 3): a participant that writes a page drops them, an
// observer takes them, whether or not it uses them.
func TestTheMemoHoldsTheLinksNoLonger(t *testing.T) {
	t.Run("a participant that writes", func(t *testing.T) {
		w := unitReachWorld(t)
		w.write("far", "[[B/X/Y]]\n")
		mem := &memo{}
		m := treeStep(w.move("B/X", "A"))
		m.NotebookID, m.At, m.Memo = w.notebook, time.Now(), mem
		u := &appender{}
		if err := w.rewrite.Participate(context.Background(), m, u); err != nil || len(u.writes) == 0 {
			t.Fatalf("the participant: %d writes, %v", len(u.writes), err)
		}
		if mem.len() != 0 {
			t.Errorf("the memo holds %d after a participant that writes", mem.len())
		}
	})
	t.Run("an observer that does not use them", func(t *testing.T) {
		w := unitReachWorld(t)
		mem := &memo{}
		changes := w.move("B/X", "A")
		m := treeStep(changes)
		m.NotebookID, m.At, m.Memo = w.notebook, time.Now(), mem
		if err := w.rewrite.Participate(context.Background(), m, &appender{}); err != nil || mem.len() != 1 {
			t.Fatalf("the participant left %d, %v", mem.len(), err)
		}
		w.store.reaches = nil
		err := w.index.PagesChanged(context.Background(), app.PagesChanged{NotebookID: w.notebook, Changes: changes[:1], Memo: mem})
		if err != nil || mem.len() != 0 || len(w.store.reaches) != 1 {
			t.Errorf("the observer of other changes: %d reads, the memo holds %d, %v", len(w.store.reaches), mem.len(), err)
		}
	})
}

// Each unit has its own memo (M12/P4/S7 design 4.3 item 4): a unit whose
// participant left its links and then rolled back, its observer never run,
// leaves nothing to the next, whose participant leaves none (no
// update_links), and whose observer reads the links itself.
func TestTheMemoIsTheUnits(t *testing.T) {
	w := unitReachWorld(t)
	first := &memo{}
	m := treeStep(w.move("B/X", "A"))
	m.NotebookID, m.At, m.Memo = w.notebook, time.Now(), first
	if err := w.rewrite.Participate(context.Background(), m, &appender{}); err != nil || first.len() != 1 {
		t.Fatalf("the first participant left %d, %v", first.len(), err)
	}
	w.move("A/X", "B") // the first unit rolls back
	m = treeStep(w.move("B/X", "A"))
	m.UpdateLinks = false
	got, err := w.unit(m, &memo{}, nil)
	if err != nil || len(got.reaches) != 1 {
		t.Fatalf("the second unit read the links %d times, %v; want once, by its observer", len(got.reaches), err)
	}
}

// Without B the participant and the observer read the links of the same
// reach (M12/P4/S7 design 4.3 item 5; decision 667): what B takes for
// granted, on each shape above and a rename whose key stays and whose
// length does not (Straße to STRASSE).
func TestTheParticipantAndTheObserverReachAlike(t *testing.T) {
	for _, c := range []unitCase{
		{name: "move", world: unitReachWorld, step: func(w *rewriting) app.Moved { return treeStep(w.move("B/X", "A")) }},
		{name: "rename", world: unitReachWorld, step: func(w *rewriting) app.Moved { return treeStep([]domain.Change{w.rename("C", "Newname")}) }},
		{name: "Straße to STRASSE", world: func(t *testing.T) *rewriting {
			// No link to Straße itself: its participant would write it again
			// by the title as now written (domain.Rewrites).
			w := newRewriting(t, "Straße", "Straße/sub", "src")
			w.write("src", "[[STRASSE/sub]] [[Strasse/missing]] [[sub]]\n")
			return w
		}, step: func(w *rewriting) app.Moved { return treeStep([]domain.Change{w.rename("Straße", "STRASSE")}) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := c.world(t)
			got, err := w.unit(c.step(w), nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.writes != 0 || len(got.reaches) != 2 || !sameReach(got.reaches[0], got.reaches[1]) {
				t.Fatalf("the participant wrote %d pages; it and the observer reached %v", got.writes, got.reached)
			}
		})
	}
}

// sameReach tells whether two reaches are alike, each array as a set.
func sameReach(a, b domain.Reach) bool {
	norm := func(r domain.Reach) string {
		keys, targets, sources := slices.Clone(r.Keys), slices.Clone(r.Targets), slices.Clone(r.Sources)
		slices.Sort(keys)
		slices.SortFunc(targets, uuid.UUID.Compare)
		slices.SortFunc(sources, uuid.UUID.Compare)
		return strings.Join(keys, ",") + fmt.Sprint(targets, sources)
	}
	return norm(a) == norm(b)
}
