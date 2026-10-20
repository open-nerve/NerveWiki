package domain_test

import (
	"cmp"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// m8Sample is a changeset of one kind of operation, as M8's units write
// them, and the steps M8's round planned for its revert, each spelt by
// describe.
type m8Sample struct {
	name  string
	build func(w *world)
	want  []string
}

// m8Samples are the changesets of M8's revert tests (revert_test.go), one
// kind of operation each: an import's, an edit session's, a rename's, a
// move's, a deletion's, a restore's, a revert's, an MCP move's that
// renames. Their steps are those M8's round planned, taken from it before
// schedule replaced it (M9/P3 design 3.21.1).
func m8Samples() []m8Sample {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	mcpMove := func(beside func(w *world, drafts, final uuid.UUID)) func(w *world) {
		return func(w *world) {
			drafts, final := w.page("Drafts", nil, 0, 1, "d"), w.page("Final", nil, 1, 1, "f")
			x := w.page("Plan", &final, 1, 1, "x")
			beside(w, drafts, final)
			w.placed(x, domain.TreeState{ParentID: &drafts, Name: "Plan v2", SortOrder: 1})
		}
	}
	return []m8Sample{
		{"a page's creation", func(w *world) {
			w.created(w.page("P", nil, 0, 1, "p"))
		}, []string{"delete P"}},
		{"an attachment's creation", func(w *world) {
			w.created(w.asset("a.png", new(w.page("P", nil, 0, 1, "p")), 0))
		}, []string{"delete a.png"}},
		{"an import's creation, a folder and its pages", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			w.created(f)
			w.created(w.page("Q", &f, 0, 1, "q"))
			w.created(w.asset("q.png", &f, 1))
		}, []string{"delete F"}},
		{"a deletion of a page", func(w *world) {
			p := w.page("P", nil, 2, 3, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
		}, []string{"restore P to /:P@2 group [P/1/r3] states [/:P@2] at 11:00:00"}},
		{"a deletion of a subtree with an attachment", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			q, a := w.page("Q", &p, 0, 4, "q"), w.asset("a.png", &p, 1)
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(q)
			w.deleted(a)
		}, []string{"restore P to /:P@0 group [P/1/r1 Q/2/r4 a.png/2/r0] states [/:P@0 P:Q@0 P:a.png@1] at 11:00:00"}},
		{"a deletion under a folder, with an attachment", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 5, 2, "p")
			a := w.asset("a.png", &p, 1)
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(a)
		}, []string{"restore P to F:P@5 group [P/1/r2 a.png/2/r0] states [F:P@5 P:a.png@1] at 11:00:00"}},
		{"a restore", func(w *world) {
			p := w.page("P", nil, 0, 2, "p")
			w.restored(p)
			w.restored(w.asset("a.png", &p, 0))
		}, []string{"delete P"}},
		{"a rename", func(w *world) {
			p := w.page("New", nil, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "Old"})
		}, []string{"place New to /:Old@0 rename"}},
		{"a move", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 3})
		}, []string{"place P to /:P@3 move"}},
		{"a move of an attachment and its rename", func(w *world) {
			f, g := w.page("F", nil, 0, 1, "f"), w.page("G", nil, 1, 1, "g")
			a := w.asset("b.png", &g, 0)
			w.placed(a, domain.TreeState{ParentID: &f, Name: "a.png"})
		}, []string{"place b.png to F:a.png@0 move rename"}},
		{"a reorder", func(w *world) {
			p := w.page("P", nil, 2, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 1})
		}, []string{"place P to /:P@1 move"}},
		{"a content's write", func(w *world) {
			p := w.page("P", nil, 0, 3, "three")
			w.wrote(p, 2, "two")
		}, []string{"content P base 2 hash 3fc4ccfe"}},
		{"a write of the bytes it found", func(w *world) {
			q := w.page("Q", nil, 0, 3, "same")
			w.wrote(q, 2, "same")
		}, nil},
		{"a version's restore", func(w *world) {
			p := w.page("P", nil, 0, 7, "one")
			w.hashes[domain.VersionRef{NodeID: p, Revision: 7}] = hash("one")
			w.wrote(p, 6, "six")
		}, []string{"content P base 6 hash 44778d82"}},
		{"a revert of a deletion: a restore of the item", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			q := w.page("Q", &p, 0, 2, "q")
			w.restored(p)
			w.restored(q)
		}, []string{"delete P"}},
		{"a deletion whose item a restore put at the root, numbered, and the restore's revert deleted there", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 3, 1, "p")
			q := w.page("Q", &p, 0, 1, "q")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(q)
			w.revive(p, q)
			w.move(p, nil, 9)
			w.rename(p, "P (2)")
			w.trash(p, deletedAt.Add(time.Hour))
			w.byRevert(p)
		}, []string{"restore P (2) to F:P@3 group [P (2)/1/r1 Q/2/r1] states [F:P@3 P (2):Q@0] at 12:00:00"}},
		{"a move back to exactly MaxDepth levels, an attachment under its deepest page", func(w *world) {
			var parent *uuid.UUID
			for i := range domain.MaxDepth - 2 {
				parent = new(w.page(fmt.Sprintf("D%d", i+1), parent, 0, 1, "d"))
			}
			x := w.page("X", nil, 1, 1, "x")
			w.asset("y.png", new(w.page("Y", &x, 0, 1, "y")), 0)
			w.placed(x, domain.TreeState{ParentID: parent, Name: "X"})
		}, []string{"place X to D8:X@0 move"}},
		{"an import's revert: its trash items", func(w *world) {
			a, b := w.page("A", nil, 0, 1, "a"), w.page("B", nil, 1, 1, "b")
			a1 := w.asset("a1.png", &a, 0)
			w.trash(a, deletedAt)
			w.trash(b, deletedAt)
			w.deleted(a)
			w.deleted(a1)
			w.deleted(b)
		}, []string{"restore A to /:A@0 group [A/1/r1 a1.png/2/r0] states [/:A@0 A:a1.png@0] at 11:00:00", "restore B to /:B@1 group [B/1/r1] states [/:B@1] at 11:00:00"}},
		{"a round of each kind of step", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			x, d := w.page("A", nil, 1, 1, "x"), w.page("D", nil, 2, 1, "d")
			for _, id := range []uuid.UUID{x, d} {
				w.trash(id, deletedAt)
				w.deleted(id)
			}
			w.placed(w.page("A", nil, 3, 1, "y"), domain.TreeState{ParentID: &f, Name: "Y"})
			w.created(w.page("D", nil, 4, 1, "n"))
			w.wrote(w.page("P", nil, 5, 2, "two"), 1, "one")
		}, []string{"place A to F:Y@0 move rename", "delete D", "restore A to /:A@1 group [A/1/r1] states [/:A@1] at 11:00:00", "restore D to /:D@2 group [D/1/r1] states [/:D@2] at 11:00:00", "content P base 1 hash 7692c3ad"}},
		{"an MCP move that renames, a Plan in Drafts", mcpMove(func(w *world, drafts, _ uuid.UUID) {
			w.page("Plan", &drafts, 0, 1, "other plan")
		}), []string{"place Plan to Drafts:Plan v2@1 move rename"}},
		{"an MCP move that renames, nothing in the way", mcpMove(func(*world, uuid.UUID, uuid.UUID) {}), []string{"place Plan to Drafts:Plan v2@1 move rename"}},
		{"an MCP move that renames, a Plan v2 in Final", mcpMove(func(w *world, _, final uuid.UUID) {
			w.page("Plan v2", &final, 0, 1, "final's")
		}), []string{"place Plan to Drafts:Plan v2@1 move rename"}},
	}
}

// describe is a step as m8Samples spell it: its kind and node, where it
// puts it, whether it moves or renames, a restore's group, states and
// time, a content's base and hash.
func (w *world) describe(op domain.RevertOp) string {
	name := func(id *uuid.UUID) string {
		if id == nil {
			return "/"
		}
		return w.nodes[*id].Name
	}
	at := func(s domain.TreeState) string { return fmt.Sprintf("%s:%s@%g", name(s.ParentID), s.Name, s.SortOrder) }
	out := fmt.Sprintf("%s %s", op.Kind, w.nodes[op.NodeID].Name)
	switch op.Kind {
	case domain.RevertPlace, domain.RevertRestore:
		out += " to " + at(op.To)
	}
	if op.Move {
		out += " move"
	}
	if op.Rename {
		out += " rename"
	}
	if op.Kind == domain.RevertRestore {
		var g, s []string
		for i, m := range op.Group {
			g = append(g, fmt.Sprintf("%s/%d/r%d", m.Node.Name, m.Level, m.Revision))
			s = append(s, at(op.States[i]))
		}
		out += fmt.Sprintf(" group [%s] states [%s] at %s", strings.Join(g, " "), strings.Join(s, " "), op.DeletedAt.Format(time.TimeOnly))
	}
	if op.Kind == domain.RevertContent {
		out += fmt.Sprintf(" base %d hash %s", op.Base, hex.EncodeToString(op.Hash[:4]))
	}
	return out
}

// M8's changesets, one kind of operation each, are scheduled in one
// round, as M8's round planned them: the same steps in the same order,
// each where it was, numbered in turn (M9/P3 design 3.16.3, 3.21.1).
func TestScheduleIsTheRoundForM8sChangesets(t *testing.T) {
	for _, tt := range m8Samples() {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			tt.build(w)
			plan := w.plan()
			if plan.Count != 0 || len(plan.Missing) != 0 {
				t.Fatalf("conflicts %q, missing %v", w.conflicts(plan), plan.Missing)
			}
			var got []string
			for i, op := range plan.Ops {
				got = append(got, w.describe(op))
				if op.Seq != i {
					t.Errorf("step %d's Seq = %d", i, op.Seq)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("steps = %q,\nwant %q", got, tt.want)
			}
		})
	}
}

// The shapes of 4.12 a batch's changeset takes, each reverted to S* by
// the schedule, in rounds and with steps aside (M9/P3 design 3.21.1).
func TestScheduleRevertsEachBatchShape(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name     string
		maxDepth int
		build    func(w *world)
		want     []string
	}{
		{"names swapped at the root: the first fallback", 0, func(w *world) {
			a, b := w.page("B", nil, 0, 1, "a"), w.page("A", nil, 1, 1, "b")
			w.placed(a, domain.TreeState{Name: "A"})
			w.placed(b, domain.TreeState{Name: "B", SortOrder: 1})
		}, []string{"aside A renamed", "place B renamed", "place A renamed"}},
		{"names turned round three pages", 0, func(w *world) {
			x, y, z := w.page("B", nil, 0, 1, "x"), w.page("C", nil, 1, 1, "y"), w.page("A", nil, 2, 1, "z")
			w.placed(x, domain.TreeState{Name: "A"})
			w.placed(y, domain.TreeState{Name: "B", SortOrder: 1})
			w.placed(z, domain.TreeState{Name: "C", SortOrder: 2})
		}, []string{"aside A renamed", "place B renamed", "place C renamed", "place A renamed"}},
		{"a rename, then a page made of the old name", 0, func(w *world) {
			p := w.page("Old", nil, 0, 1, "plan")
			w.placed(p, domain.TreeState{Name: "Plan"})
			w.created(w.page("Plan", nil, 1, 1, "new plan"))
		}, []string{"delete Plan", "place Old renamed"}},
		{"a deletion, then a page made of its name", 0, func(w *world) {
			x := w.page("X", nil, 0, 2, "x")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.created(w.page("X", nil, 1, 1, "new x"))
		}, []string{"delete X", "restore X"}},
		{"a move into a folder made", 0, func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			f := w.page("F", nil, 1, 1, "f")
			w.created(f)
			w.move(x, &f, 0)
			w.placed(x, domain.TreeState{Name: "X"})
		}, []string{"place X moved", "delete F"}},
		{"a move and rename, a sibling holding the name it has between", 0, func(w *world) {
			f, g := w.page("F", nil, 0, 1, "f"), w.page("G", nil, 1, 1, "g")
			w.page("A", &f, 1, 1, "a of f")
			x := w.page("A", &g, 0, 1, "x")
			w.placed(x, domain.TreeState{ParentID: &f, Name: "B"})
		}, []string{"place A moved and renamed"}},
		{"the review's counterexample: out of G, G deleted and made again, one moved back in", 0, func(w *world) {
			g := w.page("G", nil, 0, 1, "g")
			x, y := w.page("x", &g, 0, 1, "x"), w.page("y", &g, 1, 1, "y")
			w.move(x, nil, 1)
			w.move(y, nil, 2)
			w.trash(g, deletedAt)
			w.deleted(g)
			newG := w.page("G", nil, 3, 1, "new g")
			w.created(newG)
			w.move(y, &newG, 0)
			w.placed(x, domain.TreeState{ParentID: &g, Name: "x"})
			w.placed(y, domain.TreeState{ParentID: &g, Name: "y", SortOrder: 1})
		}, []string{"aside y moved and renamed", "delete G", "restore G", "place x moved", "place y moved and renamed"}},
		{"a place that fits once its parent's line moves up", 3, func(w *world) {
			// X, two levels, goes back under P, which goes back to the root
			// from under Q: X tried first, by its id, waits a round.
			x := w.page("X", nil, 0, 1, "x")
			w.page("Y", &x, 0, 1, "y")
			q := w.page("Q", nil, 1, 1, "q")
			p := w.page("P", &q, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 2})
			w.placed(x, domain.TreeState{ParentID: &p, Name: "X"})
		}, []string{"place P moved", "place X moved"}},
		{"a restore that fits once its parent's line moves up", 3, func(w *world) {
			// X and Z go back under B, at the root once it trades places
			// with A under Q, which a step aside lets it.
			q, p := w.page("Q", nil, 0, 1, "q"), w.page("B", nil, 1, 1, "p")
			x := w.page("X", &p, 0, 1, "x")
			w.page("Z", &x, 0, 1, "z")
			h := w.page("A", &q, 0, 1, "h")
			w.trash(x, deletedAt)
			w.deleted(x)
			for _, id := range w.subtree(x)[1:] {
				w.deleted(id)
			}
			w.move(p, &q, 0)
			w.rename(p, "A")
			w.move(h, nil, 1)
			w.rename(h, "B")
			w.placed(p, domain.TreeState{Name: "B", SortOrder: 1})
			w.placed(h, domain.TreeState{ParentID: &q, Name: "A"})
		}, []string{"aside B renamed", "place A moved and renamed", "place B moved and renamed", "restore X"}},
		{"a deletion of a page at MaxDepth with an attachment under it: restored, its levels the page's", 2, func(w *world) {
			// Q/P, P at MaxDepth, P/a.png a level below (fix round 1, A-M2).
			q := w.page("Q", nil, 0, 1, "q")
			p := w.page("P", &q, 0, 1, "p")
			w.asset("a.png", &p, 0)
			w.trash(p, deletedAt)
			for _, id := range w.subtree(p) {
				w.deleted(id)
			}
		}, []string{"restore P"}},
		{"a content written, then the page deleted: restored, then written back on its revision in the trash", 0, func(w *world) {
			x := w.page("X", nil, 0, 3, "written")
			w.wrote(x, 2, "before")
			w.trash(x, deletedAt)
			w.deleted(x)
		}, []string{"restore X", "content X"}},
		{"a revert's revert: written back, then deleted", 0, func(w *world) {
			x := w.page("X", nil, 0, 3, "written back by the revert")
			w.restored(x)
			w.wrote(x, 2, "deleted by the batch")
		}, []string{"content X", "delete X"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			tt.build(w)
			plan, err := domain.PlanRevertWithin(w.state(), cmp.Or(tt.maxDepth, domain.MaxDepth))
			if err != nil {
				t.Fatalf("PlanRevert = %v", err)
			}
			if plan.Count != 0 || len(plan.Missing) != 0 {
				t.Fatalf("conflicts %q, missing %v", w.conflicts(plan), plan.Missing)
			}
			if got := w.ops(plan); !slices.Equal(got, tt.want) {
				t.Errorf("steps = %q, want %q", got, tt.want)
			}
			// A write back is on the revision the page has as it comes to it:
			// alive, its content's; restored, its revision in the trash.
			for _, op := range plan.Ops {
				if op.Kind == domain.RevertContent && (op.Base != 2 || op.Current != 3) {
					t.Errorf("the write back = %+v, want version 2 on revision 3", op)
				}
			}
		})
	}
}

// The second fallback picks the node that frees the deletion it waits on
// (M9/P3 design 3.16.3): two nodes go back under G, which the batch
// deleted and made again, the lesser by id, L, from a folder, the other,
// H, from under the new G. Stuck, the second moves H aside to the root;
// the third alone, by id, would move L first, a step for nothing.
func TestTheSecondFallbackPicksTheNodeItFrees(t *testing.T) {
	w := newWorld(t)
	g := w.page("G", nil, 0, 1, "g")
	l, h := w.page("l", &g, 0, 1, "l"), w.page("h", &g, 1, 1, "h")
	f := w.page("F", nil, 1, 1, "f")
	w.move(l, &f, 0)
	w.move(h, nil, 2)
	w.trash(g, time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC))
	w.deleted(g)
	newG := w.page("G", nil, 3, 1, "new g")
	w.created(newG)
	w.move(h, &newG, 0)
	w.placed(l, domain.TreeState{ParentID: &g, Name: "l"})
	w.placed(h, domain.TreeState{ParentID: &g, Name: "h", SortOrder: 1})
	want := []string{"aside h moved and renamed", "delete G", "restore G", "place l moved", "place h moved and renamed"}
	if got := w.ops(w.plan()); !slices.Equal(got, want) {
		t.Errorf("steps = %q, want %q", got, want)
	}
}

// The second fallback takes its candidates by id, the least first (M9/P3
// design 3.16.3; fix round 1, C-N1): five nodes go back under G, which
// the batch deleted and made again, each from under the new G, made in
// turn. Stuck, it moves them aside to the root one a round, in their ids'
// order, whatever order the tree holds them in; the plan is the same each
// time it is made.
func TestTheSecondFallbackTakesTheLeastFirst(t *testing.T) {
	for range 20 {
		w := newWorld(t)
		g := w.page("G", nil, 0, 1, "g")
		var hs []uuid.UUID
		for i := range 5 {
			hs = append(hs, w.page(fmt.Sprint("h", i), &g, float64(i), 1, fmt.Sprint("h", i)))
		}
		for i, h := range hs {
			w.move(h, nil, float64(1+i))
		}
		w.trash(g, time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC))
		w.deleted(g)
		newG := w.page("G", nil, 9, 1, "new g")
		w.created(newG)
		var want, places []string
		for i, h := range hs {
			w.move(h, &newG, float64(4-i))
			w.placed(h, domain.TreeState{ParentID: &g, Name: fmt.Sprint("h", i), SortOrder: float64(i)})
			want = append(want, fmt.Sprintf("aside h%d moved and renamed", i))
			places = append(places, fmt.Sprintf("place h%d moved and renamed", i))
		}
		want = append(append(want, "delete G", "restore G"), places...)
		if got := w.ops(w.plan()); !slices.Equal(got, want) {
			t.Fatalf("steps = %q, want %q", got, want)
		}
	}
}

// A schedule's rounds can be more than its steps pending as it begins,
// plus one, which the bound's 2N, a step aside a node each fallback,
// leaves room for (T2; M9/P3 S2 fix round 1, A-M1). Reviewer A's shape
// (its generator "cycles", MaxDepth 3, seed 400): S* is /b (A), A/b (B),
// B/x.png (the attachment C); the batch left /b (B), /tmp.png (C),
// B/x.png (the page A) and A/x.png (a new attachment D). D is deleted;
// C waits for its key, which A holds, A for B's, and B for A, under B:
// two steps aside and four places, in six rounds, of four steps pending.
// Without D, five rounds of three, its bound with no 2N (N, 3) at four.
func TestAScheduleTakesMoreRoundsThanItsSteps(t *testing.T) {
	for _, tt := range []struct {
		name            string
		withD           bool
		want            []string
		rounds, pending int
	}{
		{"reviewer A's shape", true, []string{"delete x.png", "aside x.png renamed", "place tmp.png moved and renamed",
			"aside b renamed", "place x.png moved and renamed", "place b moved and renamed"}, 6, 4},
		{"the same without D", false, []string{"aside x.png renamed", "place tmp.png moved and renamed", "aside b renamed",
			"place x.png moved and renamed", "place b moved and renamed"}, 5, 3},
	} {
		w := newWorld(t)
		c := w.asset("tmp.png", nil, 1)
		b := w.page("b", nil, 0, 1, "b")
		a := w.page("x.png", &b, 0, 1, "a")
		if tt.withD {
			w.created(w.asset("x.png", &a, 0))
		}
		w.placed(c, domain.TreeState{ParentID: &b, Name: "x.png"})
		w.placed(b, domain.TreeState{ParentID: &a, Name: "b"})
		w.placed(a, domain.TreeState{Name: "b"})
		ops, rounds, pending, err := domain.ScheduleRounds(w.state(), 3)
		if err != nil {
			t.Errorf("%s: schedule = %v, want it planned", tt.name, err)
			continue
		}
		if got := w.ops(domain.RevertPlan{Ops: ops}); !slices.Equal(got, tt.want) {
			t.Errorf("%s: steps = %q, want %q", tt.name, got, tt.want)
		}
		if rounds != tt.rounds || pending != tt.pending || rounds <= pending+1 {
			t.Errorf("%s: %d rounds of %d steps pending, want %d of %d, more than the steps plus one", tt.name, rounds, pending,
				tt.rounds, tt.pending)
		}
	}
}

// The versions a revert writes back are every one of the changeset on a
// base (M9/P3 design 3.18.1): a page it wrote, renamed or not, or wrote
// then deleted; not a page it created.
func TestRevertWritesAreEveryVersionOnABase(t *testing.T) {
	w := newWorld(t)
	created := w.page("Created", nil, 0, 1, "c")
	w.created(created)
	deleted := w.page("Deleted", nil, 1, 3, "d")
	w.wrote(deleted, 2, "d before")
	w.trash(deleted, w.at.Add(-time.Hour))
	w.deleted(deleted)
	renamed := w.page("Renamed", nil, 2, 5, "r")
	w.placed(renamed, domain.TreeState{Name: "Was", SortOrder: 2})
	w.wrote(renamed, 4, "r before")
	written := w.page("Written", nil, 3, 7, "w")
	w.wrote(written, 6, "w before")
	var got []uuid.UUID
	for _, v := range domain.RevertWrites(w.items, w.versions) {
		got = append(got, v.NodeID)
	}
	if want := []uuid.UUID{deleted, renamed, written}; !slices.Equal(got, want) {
		t.Errorf("RevertWrites = %v, want the deleted, renamed and written pages' %v", got, want)
	}
}

// MaxRevertSteps bounds a plan's places, steps aside of both kinds,
// deletions and restores; a content's write is no step of it (M9/P3
// design 3.19).
func TestStepsCountTheStepsAside(t *testing.T) {
	plan := domain.RevertPlan{Ops: []domain.RevertOp{{Kind: domain.RevertAside, Rename: true},
		{Kind: domain.RevertAside, Move: true, Rename: true}, {Kind: domain.RevertDelete}, {Kind: domain.RevertRestore},
		{Kind: domain.RevertPlace, Move: true}, {Kind: domain.RevertPlace, Rename: true}, {Kind: domain.RevertContent}}}
	if n := plan.Steps(); n != 6 {
		t.Errorf("Steps() = %d, want 6: both steps aside, a deletion, a restore and two places", n)
	}

	// The review's counterexample, planned: a step aside, a deletion, a
	// restore and two places.
	w := newWorld(t)
	g := w.page("G", nil, 0, 1, "g")
	x, y := w.page("x", &g, 0, 1, "x"), w.page("y", &g, 1, 1, "y")
	w.move(x, nil, 1)
	w.move(y, nil, 2)
	w.trash(g, time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC))
	w.deleted(g)
	newG := w.page("G", nil, 3, 1, "new g")
	w.created(newG)
	w.move(y, &newG, 0)
	w.placed(x, domain.TreeState{ParentID: &g, Name: "x"})
	w.placed(y, domain.TreeState{ParentID: &g, Name: "y", SortOrder: 1})
	w.wrote(w.page("P", nil, 4, 2, "two"), 1, "one")
	if plan := w.plan(); plan.Steps() != 5 || len(plan.Ops) != 6 {
		t.Errorf("the counterexample's steps = %q, Steps() %d; want 5 of 6, the write back not counted", w.ops(plan), plan.Steps())
	}
}
