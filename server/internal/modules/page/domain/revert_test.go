package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// world is a notebook as a revert reads it after its changeset: its
// nodes, alive or in the trash, their contents and versions, and the
// changeset's items and versions.
type world struct {
	t        *testing.T
	nodes    map[uuid.UUID]domain.NodeNow
	contents map[uuid.UUID]domain.ContentNow
	hashes   map[domain.VersionRef][]byte
	items    []domain.ChangesetItem
	versions []domain.ChangesetVersion
	at       time.Time
}

const retention = 30 * 24 * time.Hour

func newWorld(t *testing.T) *world {
	return &world{t: t, nodes: map[uuid.UUID]domain.NodeNow{}, contents: map[uuid.UUID]domain.ContentNow{},
		hashes: map[domain.VersionRef][]byte{}, at: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
}

func hash(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

// page adds a page alive under parent at order, its content text at
// revision, which its version holds.
func (w *world) page(name string, parent *uuid.UUID, order float64, revision int, text string) uuid.UUID {
	id := w.node(domain.KindPage, name, parent, order)
	w.contents[id] = domain.ContentNow{Revision: revision, Hash: hash(text)}
	w.hashes[domain.VersionRef{NodeID: id, Revision: revision}] = hash(text)
	return id
}

// asset adds an attachment alive under parent at order.
func (w *world) asset(name string, parent *uuid.UUID, order float64) uuid.UUID {
	return w.node(domain.KindAsset, name, parent, order)
}

func (w *world) node(kind domain.Kind, name string, parent *uuid.UUID, order float64) uuid.UUID {
	id := uuid.NewV7()
	w.nodes[id] = domain.NodeNow{Node: domain.Node{ID: id, ParentID: parent, Kind: kind, Name: name, NameKey: shared.TitleKey(name),
		SortOrder: order}}
	return id
}

// trash deletes the subtree of root at at, a trash item: its contents go
// with it.
func (w *world) trash(root uuid.UUID, at time.Time) {
	pages, assets := 0, 0
	for _, id := range w.subtree(root) {
		n := w.nodes[id]
		n.DeletedAt = &at
		if n.Kind == domain.KindPage {
			pages++
			delete(w.contents, id)
		} else {
			assets++
		}
		w.nodes[id] = n
	}
	n := w.nodes[root]
	n.TrashRoot, n.TrashPages, n.TrashAssets = true, pages, assets
	w.nodes[root] = n
}

// subtree is the node id and the nodes under it, alive or deleted with
// it, level by level, each level by order.
func (w *world) subtree(id uuid.UUID) []uuid.UUID {
	out := []uuid.UUID{id}
	for at := 0; at < len(out); at++ {
		var level []domain.NodeNow
		for _, n := range w.nodes {
			if n.ParentID != nil && *n.ParentID == out[at] && (n.DeletedAt == nil) == (w.nodes[id].DeletedAt == nil) {
				level = append(level, n)
			}
		}
		slices.SortFunc(level, func(a, b domain.NodeNow) int { return a.ID.Compare(b.ID) })
		for _, n := range level {
			out = append(out, n.ID)
		}
	}
	return out
}

// state is the world as PlanRevert reads it: the groups RevertGroups
// names, as TrashGroup reads them, and the tree when the changeset has
// items.
func (w *world) state() domain.RevertState {
	s := domain.RevertState{Items: w.items, Versions: w.versions, Nodes: map[uuid.UUID]domain.NodeNow{}, Contents: w.contents,
		Hashes: w.hashes, Groups: map[uuid.UUID][]domain.TrashMember{}, At: w.at, Retention: retention}
	touched := map[uuid.UUID]bool{}
	for _, it := range w.items {
		touched[it.NodeID] = true
	}
	for _, v := range w.versions {
		touched[v.NodeID] = true
	}
	for id := range touched {
		if n, ok := w.nodes[id]; ok {
			s.Nodes[id] = n
		}
	}
	for root := range domain.RevertGroups(w.items, s.Nodes) {
		levels := map[uuid.UUID]int{root: 1}
		for _, id := range w.subtree(root) {
			n := w.nodes[id]
			if id != root {
				levels[id] = levels[*n.ParentID] + 1
			}
			m := domain.TrashMember{Node: n.Node, Level: levels[id]}
			if n.Kind == domain.KindPage {
				m.Revision = w.trashRevision(id)
			}
			s.Groups[root] = append(s.Groups[root], m)
		}
	}
	if len(w.items) > 0 {
		for _, n := range w.nodes {
			if n.DeletedAt == nil {
				s.Tree = append(s.Tree, n.Node)
			}
		}
	}
	return s
}

// trashRevision is a deleted page's content's revision: the newest
// version's.
func (w *world) trashRevision(id uuid.UUID) int {
	r := 0
	for ref := range w.hashes {
		if ref.NodeID == id {
			r = max(r, ref.Revision)
		}
	}
	return r
}

// created records the changeset's creation of the node id, as it is now,
// and of a page its version.
func (w *world) created(id uuid.UUID) {
	n := w.nodes[id]
	it := domain.ChangesetItem{NodeID: id, After: new(n.State())}
	if n.Kind == domain.KindPage {
		c := w.contents[id]
		it.Revision = c.Revision
		w.versions = append(w.versions, domain.ChangesetVersion{NodeID: id, Revision: c.Revision, Hash: c.Hash})
	}
	w.items = append(w.items, it)
}

// restored records the changeset's restore of the node id, as it is now.
func (w *world) restored(id uuid.UUID) {
	n := w.nodes[id]
	w.items = append(w.items, domain.ChangesetItem{NodeID: id, After: new(n.State()), Restored: true, Revision: w.contents[id].Revision})
}

// deleted records the changeset's deletion of the node id, from where it
// is in the trash, a page at the revision of its content there.
func (w *world) deleted(id uuid.UUID) {
	n := w.nodes[id]
	it := domain.ChangesetItem{NodeID: id, Before: new(n.State())}
	if n.Kind == domain.KindPage {
		it.Revision = w.trashRevision(id)
	}
	w.items = append(w.items, it)
}

// placed records the changeset's move of the node id from before to
// where it is now.
func (w *world) placed(id uuid.UUID, before domain.TreeState) {
	w.items = append(w.items, domain.ChangesetItem{NodeID: id, Before: &before, After: new(w.nodes[id].State())})
}

// wrote records the changeset's write of the page id from its version
// base, of text, to its content now.
func (w *world) wrote(id uuid.UUID, base int, text string) {
	w.hashes[domain.VersionRef{NodeID: id, Revision: base}] = hash(text)
	c := w.contents[id]
	w.versions = append(w.versions, domain.ChangesetVersion{NodeID: id, Base: &base, Revision: c.Revision, Hash: c.Hash})
}

// revive brings the nodes ids back alive where they are, no root any
// more: a restore of their trash item after the changeset.
func (w *world) revive(ids ...uuid.UUID) {
	for _, id := range ids {
		n := w.nodes[id]
		n.DeletedAt, n.TrashRoot, n.DeletedByRevert = nil, false, false
		w.nodes[id] = n
	}
}

// byRevert marks the trash item of the root id as deleted by the revert of
// the restore of the item the changeset made.
func (w *world) byRevert(id uuid.UUID) {
	n := w.nodes[id]
	n.DeletedByRevert = true
	w.nodes[id] = n
}

// rename, move and set change the node id after the changeset.
func (w *world) rename(id uuid.UUID, name string) {
	n := w.nodes[id]
	n.Name, n.NameKey = name, shared.TitleKey(name)
	w.nodes[id] = n
}

func (w *world) move(id uuid.UUID, parent *uuid.UUID, order float64) {
	n := w.nodes[id]
	n.ParentID, n.SortOrder = parent, order
	w.nodes[id] = n
}

// ops are the plan's steps, each its kind and node's name.
func (w *world) ops(plan domain.RevertPlan) []string {
	out := make([]string, len(plan.Ops))
	for i, op := range plan.Ops {
		out[i] = string(op.Kind) + " " + w.nodes[op.NodeID].Name
		switch {
		case op.Move && op.Rename:
			out[i] += " moved and renamed"
		case op.Move:
			out[i] += " moved"
		case op.Rename:
			out[i] += " renamed"
		}
	}
	return out
}

// conflicts are the plan's, each its reason, name and holder's name.
func (w *world) conflicts(plan domain.RevertPlan) []string {
	out := make([]string, len(plan.Conflicts))
	for i, c := range plan.Conflicts {
		out[i] = fmt.Sprintf("%s %s", c.Reason, c.Name)
		if c.HolderID != nil {
			out[i] += " held by " + w.nodes[*c.HolderID].Name
		}
	}
	return out
}

func (w *world) plan() domain.RevertPlan {
	w.t.Helper()
	plan, err := domain.PlanRevert(w.state())
	if err != nil {
		w.t.Fatalf("PlanRevert() = %v", err)
	}
	return plan
}

// unordered is the ops of a revert PlanRevert answers ErrRevertUnordered.
const unordered = "ErrRevertUnordered"

// A revert reverses each kind of change in one round: a creation is
// deleted, from its highest node; a deletion restored, each of its trash
// items whole, where it was; a restore deleted again; a rename, a move, a
// reorder put back; a written page's version before it written back. A
// page written, then deleted, a batch's, is unordered until S2 (M9/P3
// design section 0, 3.21.1).
func TestPlanRevertReversesEachChange(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		build func(w *world)
		want  []string
	}{
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
		}, []string{"restore P"}},
		{"a deletion of a subtree with an attachment", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			q, a := w.page("Q", &p, 0, 4, "q"), w.asset("a.png", &p, 1)
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(q)
			w.deleted(a)
		}, []string{"restore P"}},
		{"a restore", func(w *world) {
			p := w.page("P", nil, 0, 2, "p")
			w.restored(p)
			w.restored(w.asset("a.png", &p, 0))
		}, []string{"delete P"}},
		{"a rename", func(w *world) {
			p := w.page("New", nil, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "Old"})
		}, []string{"place New renamed"}},
		{"a move", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 3})
		}, []string{"place P moved"}},
		{"a move of an attachment and its rename", func(w *world) {
			f, g := w.page("F", nil, 0, 1, "f"), w.page("G", nil, 1, 1, "g")
			a := w.asset("b.png", &g, 0)
			w.placed(a, domain.TreeState{ParentID: &f, Name: "a.png"})
		}, []string{"place b.png moved and renamed"}},
		{"a reorder", func(w *world) {
			p := w.page("P", nil, 2, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 1})
		}, []string{"place P moved"}},
		{"a content's write", func(w *world) {
			p := w.page("P", nil, 0, 2, "two")
			w.wrote(p, 1, "one")
		}, []string{"content P"}},
		{"a version's restore", func(w *world) {
			p := w.page("P", nil, 0, 7, "one")
			w.hashes[domain.VersionRef{NodeID: p, Revision: 7}] = hash("one")
			w.wrote(p, 6, "six")
		}, []string{"content P"}},
		{"a revert of a deletion: a restore of the item", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			q := w.page("Q", &p, 0, 2, "q")
			w.restored(p)
			w.restored(q)
		}, []string{"delete P"}},
		{"a page written, then deleted (M9's shape): unordered, which the round restores at the content it was deleted at", func(w *world) {
			p := w.page("P", nil, 0, 2, "two")
			w.wrote(p, 1, "one")
			w.trash(p, deletedAt)
			w.deleted(p)
		}, []string{unordered}},
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
		}, []string{"restore P (2)"}},
		{"a move back to exactly MaxDepth levels, an attachment under its deepest page", func(w *world) {
			var parent *uuid.UUID
			for i := range domain.MaxDepth - 2 {
				parent = new(w.page(fmt.Sprintf("D%d", i+1), parent, 0, 1, "d"))
			}
			x := w.page("X", nil, 1, 1, "x")
			w.asset("y.png", new(w.page("Y", &x, 0, 1, "y")), 0)
			w.placed(x, domain.TreeState{ParentID: parent, Name: "X"})
		}, []string{"place X moved"}},
		{"an import's revert: its trash items", func(w *world) {
			a, b := w.page("A", nil, 0, 1, "a"), w.page("B", nil, 1, 1, "b")
			a1 := w.asset("a1.png", &a, 0)
			w.trash(a, deletedAt)
			w.trash(b, deletedAt)
			w.deleted(a)
			w.deleted(a1)
			w.deleted(b)
		}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			tt.build(w)
			if slices.Equal(tt.want, []string{unordered}) {
				if _, err := domain.PlanRevert(w.state()); !errors.Is(err, domain.ErrRevertUnordered) {
					t.Errorf("PlanRevert() = %v, want ErrRevertUnordered", err)
				}
				return
			}
			plan := w.plan()
			if plan.Count != 0 || len(plan.Conflicts) != 0 || len(plan.Missing) != 0 {
				t.Fatalf("conflicts %v, %d missing; want none", w.conflicts(plan), len(plan.Missing))
			}
			want := tt.want
			if want == nil {
				// Both of the import's items, by their roots' ids.
				var tops []domain.ChangesetItem
				for _, it := range w.items {
					if it.Before.ParentID == nil {
						tops = append(tops, it)
					}
				}
				slices.SortFunc(tops, func(a, b domain.ChangesetItem) int { return a.NodeID.Compare(b.NodeID) })
				for _, it := range tops {
					want = append(want, "restore "+it.Before.Name)
				}
			}
			if got := w.ops(plan); !slices.Equal(got, want) {
				t.Errorf("ops = %q, want %q", got, want)
			}
		})
	}
}

// A restore's step brings the trash item back whole, at its time, each
// node where the changeset deleted it, its root's parent and order as
// they were; a place's step names where the node goes back; a content's
// the version written back and its hash.
func TestPlanRevertsStepsSayWhereEachGoes(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	w := newWorld(t)
	f := w.page("F", nil, 0, 1, "f")
	p := w.page("P", &f, 5, 2, "p")
	a := w.asset("a.png", &p, 1)
	w.trash(p, deletedAt)
	w.deleted(p)
	w.deleted(a)
	plan := w.plan()
	if len(plan.Ops) != 1 {
		t.Fatalf("ops = %q", w.ops(plan))
	}
	op := plan.Ops[0]
	if op.NodeID != p || !op.DeletedAt.Equal(deletedAt) || !op.To.Same(domain.TreeState{ParentID: &f, Name: "P", SortOrder: 5}) ||
		len(op.Group) != 2 || op.Group[0].Node.ID != p || op.Group[1].Node.ID != a || op.Group[0].Revision != 2 ||
		!op.States[1].Same(domain.TreeState{ParentID: &p, Name: "a.png", SortOrder: 1}) || !op.States[0].Same(op.To) {
		t.Errorf("the restore = %+v", op)
	}

	w = newWorld(t)
	q := w.page("Q", nil, 0, 3, "three")
	w.wrote(q, 2, "two")
	plan = w.plan()
	if len(plan.Ops) != 1 || plan.Ops[0].Base != 2 || string(plan.Ops[0].Hash) != string(hash("two")) {
		t.Errorf("the content = %+v, want version 2 written back", plan.Ops)
	}

	// A page whose content is its base's already is not written: a session
	// that wrote the bytes it found back changed nothing.
	w = newWorld(t)
	q = w.page("Q", nil, 0, 3, "same")
	w.wrote(q, 2, "same")
	if plan := w.plan(); len(plan.Ops) != 0 || plan.Count != 0 {
		t.Errorf("a write of the same bytes back = %q, %v; want nothing to do", w.ops(plan), w.conflicts(plan))
	}
}

// Each conflict, of the first reason a node meets, named as S* names it,
// or as it is now where S* has none.
func TestPlanRevertConflicts(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		build func(w *world)
		want  []string
	}{
		{"a rename renamed again", func(w *world) {
			p := w.page("New", nil, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "Old"})
			w.rename(p, "Newer")
		}, []string{"changed Old"}},
		{"a move reordered since", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "P", SortOrder: 3})
			w.move(p, &f, 2)
		}, []string{"changed P"}},
		{"a creation renamed since, named as it is", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.created(p)
			w.rename(p, "Q")
		}, []string{"changed Q"}},
		{"a creation written since", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.created(p)
			w.contents[p] = domain.ContentNow{Revision: 2, Hash: hash("written")}
		}, []string{"changed P"}},
		{"a creation deleted since", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.created(p)
			w.trash(p, deletedAt)
		}, []string{"changed P"}},
		{"a write written again", func(w *world) {
			p := w.page("P", nil, 0, 2, "two")
			w.wrote(p, 1, "one")
			w.contents[p] = domain.ContentNow{Revision: 3, Hash: hash("three")}
		}, []string{"changed P"}},
		{"a restore of a page written since", func(w *world) {
			p := w.page("P", nil, 0, 2, "p")
			w.restored(p)
			w.contents[p] = domain.ContentNow{Revision: 3, Hash: hash("three")}
		}, []string{"changed P"}},
		{"a deletion restored since", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			q := w.page("Q", &p, 0, 1, "q")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(q)
			for _, id := range []uuid.UUID{p, q} {
				n := w.nodes[id]
				n.DeletedAt, n.TrashRoot = nil, false
				w.nodes[id] = n
			}
		}, []string{"changed P", "changed Q"}},
		{"a deletion whose page in the trash is at another content", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.hashes[domain.VersionRef{NodeID: p, Revision: 2}] = hash("restored, written and deleted again")
		}, []string{"changed P"}},
		{"a deletion of a page before M8/P3: no revision", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.items[0].Revision = 0
		}, []string{"changed P"}},
		{"a deletion restored and deleted with its parent since: no root, its group unread", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			n := w.nodes[p]
			n.DeletedAt, n.TrashRoot = nil, false
			w.nodes[p] = n
			w.trash(f, w.at)
		}, []string{"changed P"}},
		{"a deletion restored, filled and deleted again: its counts differ", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			n := w.nodes[p]
			n.DeletedAt, n.TrashRoot = nil, false
			w.nodes[p] = n
			w.page("X", &p, 0, 1, "x")
			w.trash(p, w.at)
		}, []string{"changed P"}},
		{"a deletion restored, its top renamed and moved, its member renamed, deleted again", func(w *world) {
			f, g := w.page("F", nil, 0, 1, "f"), w.page("G", nil, 1, 1, "g")
			x := w.page("X", &f, 0, 1, "x")
			z := w.page("Z", &x, 0, 1, "z")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.deleted(z)
			w.revive(x, z)
			w.rename(z, "Z2")
			w.rename(x, "Y")
			w.move(x, &g, 5)
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed X", "changed Z"}},
		{"a deletion restored, its top renamed, deleted again", func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.revive(x)
			w.rename(x, "Y")
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed X"}},
		{"a deletion restored, its top reordered, deleted again", func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.revive(x)
			w.move(x, nil, 4)
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed X"}},
		{"a deletion restored, its member renamed, deleted again where it was", func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			z := w.page("Z", &x, 0, 1, "z")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.deleted(z)
			w.revive(x, z)
			w.rename(z, "Z2")
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed Z"}},
		{"a deletion restored, its member moved within the group, deleted again", func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			q := w.page("Q", &x, 0, 1, "q")
			r := w.page("R", &q, 0, 1, "r")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.deleted(q)
			w.deleted(r)
			w.revive(x, q, r)
			w.move(r, &x, 1)
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed R"}},
		{"a deletion restored, its member reordered, deleted again", func(w *world) {
			x := w.page("X", nil, 0, 1, "x")
			q := w.page("Q", &x, 0, 1, "q")
			w.trash(x, deletedAt)
			w.deleted(x)
			w.deleted(q)
			w.revive(x, q)
			w.move(q, &x, 3)
			w.trash(x, deletedAt.Add(time.Hour))
		}, []string{"changed Q"}},
		{"a deletion restored, a member with its child moved to the root, the top deleted again: the grandchild alive", func(w *world) {
			top := w.page("T", nil, 0, 1, "t")
			x := w.page("X", &top, 0, 1, "x")
			n := w.page("N", &x, 0, 1, "n")
			w.trash(top, deletedAt)
			w.deleted(top)
			w.deleted(x)
			w.deleted(n)
			w.revive(top, x, n)
			w.move(x, nil, 9)
			w.trash(top, deletedAt.Add(time.Hour))
		}, []string{"changed T", "changed X", "changed N"}},
		{"a deletion past its time", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.trash(p, w.at.Add(domain.TrashMargin-retention))
			w.deleted(p)
		}, []string{"expired P"}},
		{"a move whose parent before it is deleted since", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", nil, 1, 1, "p")
			w.placed(p, domain.TreeState{ParentID: &f, Name: "P"})
			w.trash(f, deletedAt)
		}, []string{"parent_gone P"}},
		{"a deletion whose parent is deleted since", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", &f, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.trash(f, w.at)
		}, []string{"parent_gone P"}},
		{"a creation another put a page under since", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			w.created(f)
			w.page("G", &f, 0, 1, "g")
		}, []string{"children_added G"}},
		{"a rename whose name another took since", func(w *world) {
			p := w.page("New", nil, 0, 1, "p")
			w.placed(p, domain.TreeState{Name: "Old"})
			w.page("OLD", nil, 1, 1, "x")
		}, []string{"title_taken Old held by OLD"}},
		{"a deletion whose name another took since", func(w *world) {
			p := w.page("P", nil, 0, 1, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.page("p", nil, 1, 1, "x")
		}, []string{"title_taken P held by p"}},
		{"a move whose page another moved under it since", func(w *world) {
			f := w.page("F", nil, 0, 1, "f")
			p := w.page("P", nil, 1, 1, "p")
			w.placed(p, domain.TreeState{ParentID: &f, Name: "P"})
			w.move(f, &p, 0)
		}, []string{"cycle P"}},
		{"a deletion whose parent went deeper since, past MaxDepth for its subtree", func(w *world) {
			var parent *uuid.UUID
			for i := range domain.MaxDepth - 2 {
				parent = new(w.page(fmt.Sprintf("D%d", i+1), parent, 0, 1, "d"))
			}
			top := w.page("Top", nil, 1, 1, "t")
			p := w.page("P", parent, 0, 1, "p")
			q := w.page("Q", &p, 0, 1, "q")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.deleted(q)
			for id, n := range w.nodes {
				if n.Name == "D1" {
					w.move(id, &top, 0)
				}
			}
		}, []string{"too_deep P"}},
		{"a move whose subtree grew since past MaxDepth where it was", func(w *world) {
			var parent *uuid.UUID
			for i := range domain.MaxDepth - 2 {
				parent = new(w.page(fmt.Sprintf("D%d", i+1), parent, 0, 1, "d"))
			}
			p := w.page("P", nil, 1, 1, "p")
			q := w.page("Q", &p, 0, 1, "q")
			w.placed(p, domain.TreeState{ParentID: parent, Name: "P"})
			w.page("R", &q, 0, 1, "r")
		}, []string{"too_deep P"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			tt.build(w)
			plan := w.plan()
			if got := w.conflicts(plan); !slices.Equal(got, tt.want) || plan.Count != len(tt.want) || len(plan.Ops) != 0 {
				t.Errorf("conflicts = %q (%d), ops %q; want %q", got, plan.Count, w.ops(plan), tt.want)
			}
		})
	}
}

// A deletion whose trash item's group the reads did not find, gone
// between the read of its root and of its group, is changed: a group is
// never restored unread.
func TestPlanRevertOfAGroupUnreadIsChanged(t *testing.T) {
	w := newWorld(t)
	p := w.page("P", nil, 0, 1, "p")
	w.trash(p, time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC))
	w.deleted(p)
	s := w.state()
	delete(s.Groups, p)
	plan, err := domain.PlanRevert(s)
	if got := w.conflicts(plan); err != nil || !slices.Equal(got, []string{"changed P"}) || len(plan.Ops) != 0 {
		t.Errorf("PlanRevert(P's group unread) = %q, %q, %v; want P changed", got, w.ops(plan), err)
	}
}

// An attachment the changeset deleted has no content, so no revision to
// compare: its deletion is restored. A page in the trash at another
// revision of the same bytes is no conflict: a revert's write back moved
// its revision on, not its content.
func TestPlanRevertComparesContentsByHash(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	w := newWorld(t)
	a := w.asset("a.png", nil, 0)
	w.trash(a, deletedAt)
	w.deleted(a)
	if plan := w.plan(); plan.Count != 0 || !slices.Equal(w.ops(plan), []string{"restore a.png"}) {
		t.Errorf("an attachment's deletion = %q, %q", w.conflicts(plan), w.ops(plan))
	}

	w = newWorld(t)
	p := w.page("P", nil, 0, 3, "three")
	w.trash(p, deletedAt)
	w.deleted(p)
	w.hashes[domain.VersionRef{NodeID: p, Revision: 5}] = hash("three")
	if plan := w.plan(); plan.Count != 0 || !slices.Equal(w.ops(plan), []string{"restore P"}) {
		t.Errorf("a page in the trash at revision 5 of revision 3's bytes = %q, %q", w.conflicts(plan), w.ops(plan))
	}
}

// A version whose hash the purge took is missing, no conflict, and the
// plan has no steps: a page written back whose base is gone, a page
// restored whose version is, a deleted page whose content is.
func TestPlanRevertMissesWhatThePurgeTook(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		build func(w *world) uuid.UUID
	}{
		{"a write whose base is gone", func(w *world) uuid.UUID {
			p := w.page("P", nil, 0, 2, "two")
			w.wrote(p, 1, "one")
			delete(w.hashes, domain.VersionRef{NodeID: p, Revision: 1})
			return p
		}},
		{"a restore whose version is gone", func(w *world) uuid.UUID {
			p := w.page("P", nil, 0, 2, "p")
			w.restored(p)
			delete(w.hashes, domain.VersionRef{NodeID: p, Revision: 2})
			return p
		}},
		{"a deletion whose version in the trash is gone", func(w *world) uuid.UUID {
			p := w.page("P", nil, 0, 2, "p")
			w.trash(p, deletedAt)
			w.deleted(p)
			w.hashes[domain.VersionRef{NodeID: p, Revision: 3}] = hash("p")
			w.items[0].Revision = 2
			delete(w.hashes, domain.VersionRef{NodeID: p, Revision: 2})
			return p
		}},
	} {
		w := newWorld(t)
		p := tt.build(w)
		if plan := w.plan(); !slices.Equal(plan.Missing, []uuid.UUID{p}) || plan.Count != 0 || len(plan.Ops) != 0 {
			t.Errorf("%s: missing %v, conflicts %q, ops %q; want P missing alone", tt.name, plan.Missing, w.conflicts(plan), w.ops(plan))
		}
	}
}

// The conflicts come in their reasons' order, then by node id; all are
// counted, the first MaxConflicts named.
func TestPlanRevertsConflictsInTheirOrder(t *testing.T) {
	for _, n := range []int{domain.MaxConflicts, domain.MaxConflicts + 1} {
		w := newWorld(t)
		f := w.page("F", nil, 0, 1, "f")
		w.created(f)
		w.page("Added", &f, 0, 1, "x")
		var renamed []uuid.UUID
		for i := range n - 1 {
			p := w.page(fmt.Sprintf("P%d", i), nil, float64(i+1), 1, "p")
			w.placed(p, domain.TreeState{Name: fmt.Sprintf("Old%d", i), SortOrder: float64(i + 1)})
			w.rename(p, fmt.Sprintf("Q%d", i))
			renamed = append(renamed, p)
		}
		plan := w.plan()
		if plan.Count != n || len(plan.Conflicts) != min(n, domain.MaxConflicts) {
			t.Fatalf("%d conflicts: counted %d, named %d", n, plan.Count, len(plan.Conflicts))
		}
		for i, c := range plan.Conflicts[:n-1] {
			if c.Reason != domain.ConflictChanged || c.NodeID != renamed[i] {
				t.Errorf("conflict %d = %+v, want the changed in id order first", i, c)
			}
		}
		if n == domain.MaxConflicts && plan.Conflicts[n-1].Reason != domain.ConflictChildrenAdded {
			t.Errorf("the last of %d = %+v, want children_added after the changed", n, plan.Conflicts[n-1])
		}
	}
	// One of each reason, in their order whatever the nodes' ids.
	w := newWorld(t)
	var parent *uuid.UUID
	for i := range domain.MaxDepth - 2 {
		parent = new(w.page(fmt.Sprintf("D%d", i+1), parent, 0, 1, "d"))
	}
	deep := w.page("Deep", nil, 1, 1, "t")
	w.placed(deep, domain.TreeState{ParentID: parent, Name: "Deep"})
	w.page("V", new(w.page("U", &deep, 0, 1, "u")), 0, 1, "v")
	h := w.page("H", nil, 2, 1, "h")
	cycled := w.page("Cycled", nil, 3, 1, "p")
	w.placed(cycled, domain.TreeState{ParentID: &h, Name: "Cycled"})
	w.move(h, &cycled, 0)
	taken := w.page("New", nil, 4, 1, "n")
	w.placed(taken, domain.TreeState{Name: "Old", SortOrder: 4})
	w.page("OLD", nil, 5, 1, "x")
	f := w.page("F", nil, 6, 1, "f")
	w.created(f)
	w.page("Added", &f, 0, 1, "k")
	g := w.page("G", nil, 7, 1, "g")
	moved := w.page("Moved", nil, 8, 1, "m")
	w.placed(moved, domain.TreeState{ParentID: &g, Name: "Moved"})
	w.trash(g, w.at)
	old := w.page("Expired", nil, 9, 1, "e")
	w.trash(old, w.at.Add(-retention))
	w.deleted(old)
	changed := w.page("Changed", nil, 10, 1, "c")
	w.placed(changed, domain.TreeState{Name: "Was", SortOrder: 10})
	w.rename(changed, "Since")
	want := []string{"changed Was", "expired Expired", "parent_gone Moved", "children_added Added", "title_taken Old held by OLD",
		"cycle Cycled", "too_deep Deep"}
	if got := w.conflicts(w.plan()); !slices.Equal(got, want) {
		t.Errorf("conflicts = %q, want %q", got, want)
	}
}

// The round goes in its fixed order (M8/P3 design 3.5), each kind of step
// needing the ones before it: Y moved into the root as A, where X, named
// A, was deleted; N created as D where D was deleted; P's content
// written. Y goes back first, freeing A; N goes to the trash, freeing D;
// then X and D come back; then P's version is written back. Four of the
// five steps count against MaxRevertSteps.
func TestPlanRevertsRoundGoesInItsOrder(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	w := newWorld(t)
	f := w.page("F", nil, 0, 1, "f")
	x, d := w.page("A", nil, 1, 1, "x"), w.page("D", nil, 2, 1, "d")
	for _, id := range []uuid.UUID{x, d} {
		w.trash(id, deletedAt)
		w.deleted(id)
	}
	w.placed(w.page("A", nil, 3, 1, "y"), domain.TreeState{ParentID: &f, Name: "Y"})
	w.created(w.page("D", nil, 4, 1, "n"))
	w.wrote(w.page("P", nil, 5, 2, "two"), 1, "one")
	want := []string{"place A moved and renamed", "delete D", "restore A", "restore D", "content P"}
	plan := w.plan()
	if got := w.ops(plan); !slices.Equal(got, want) {
		t.Errorf("ops = %q, want %q", got, want)
	}
	// MaxRevertSteps bounds the places, deletions and restores, a content's
	// write not among them.
	if n := plan.Steps(); n != 4 {
		t.Errorf("Steps() = %d, want 4", n)
	}
}

// A changeset of two nodes trading names, or parents, which only M9's
// batch makes, reverts with a step aside: the first fallback renames
// aside the node holding the name the other goes back to, then both are
// placed (M9/P3 design 3.21.1), no aside name left.
func TestPlanRevertOfASwapGoesAside(t *testing.T) {
	w := newWorld(t)
	a, b := w.page("B", nil, 0, 1, "a"), w.page("A", nil, 1, 1, "b")
	w.placed(a, domain.TreeState{Name: "A"})
	w.placed(b, domain.TreeState{Name: "B", SortOrder: 1})
	plan := w.plan()
	want := []string{"aside A renamed", "place B renamed", "place A renamed"}
	if got := w.ops(plan); !slices.Equal(got, want) {
		t.Errorf("ops of a swap = %q, want %q", got, want)
	}
	if aside := plan.Ops[0]; aside.NodeID != b || aside.To.ParentID != nil || aside.To.Name != "A ~"+hex.EncodeToString(b[12:]) {
		t.Errorf("the step aside = %+v, want B's node renamed A ~<its last 8 digits> where it is", aside)
	}
	if last := plan.Ops[2]; last.NodeID != b || last.To.Name != "B" || plan.Ops[1].NodeID != a || plan.Ops[1].To.Name != "A" {
		t.Errorf("the places = %+v, %+v", plan.Ops[1], last)
	}

	// Two pages of one name trading parents: the first move meets the other.
	w = newWorld(t)
	f, g := w.page("F", nil, 0, 1, "f"), w.page("G", nil, 1, 1, "g")
	x, y := w.page("N", &g, 0, 1, "x"), w.page("N", &f, 0, 1, "y")
	w.placed(x, domain.TreeState{ParentID: &f, Name: "N"})
	w.placed(y, domain.TreeState{ParentID: &g, Name: "N"})
	plan = w.plan()
	want = []string{"aside N renamed", "place N moved", "place N moved and renamed"}
	if got := w.ops(plan); !slices.Equal(got, want) {
		t.Errorf("ops of a swap of parents = %q, want %q", got, want)
	}
	if ids := []uuid.UUID{plan.Ops[0].NodeID, plan.Ops[1].NodeID, plan.Ops[2].NodeID}; !slices.Equal(ids, []uuid.UUID{y, x, y}) ||
		!domain.SameParent(plan.Ops[0].To.ParentID, &f) || !domain.SameParent(plan.Ops[2].To.ParentID, &g) || plan.Ops[2].To.Name != "N" {
		t.Errorf("steps of a swap of parents = %+v", plan.Ops)
	}
	for i, op := range plan.Ops {
		if op.Seq != i {
			t.Errorf("step %d's Seq = %d", i, op.Seq)
		}
	}
}

// An MCP move that renames, Drafts/Plan v2 to Final/Plan, is one item of a
// new parent and a new name; its revert is one place of both, its last key
// alone checked (M9/P2 design 3.5.4; review B, B-I1): a move back first
// with the name it has meets a Plan of Drafts, a rename first a Plan v2 of
// Final, which a valid state before the changeset may hold. A Plan v2
// created in Drafts since holds the name it goes back to: a conflict, not
// an unordered round.
func TestARevertOfAMoveThatRenamesIsOnePlace(t *testing.T) {
	for _, tt := range []struct {
		name      string
		beside    func(w *world, drafts, final uuid.UUID)
		ops       []string
		conflicts []string
	}{
		{"a Plan in Drafts", func(w *world, drafts, _ uuid.UUID) { w.page("Plan", &drafts, 0, 1, "other plan") },
			[]string{"place Plan moved and renamed"}, nil},
		{"nothing in the way", func(*world, uuid.UUID, uuid.UUID) {}, []string{"place Plan moved and renamed"}, nil},
		{"a Plan v2 in Final", func(w *world, _, final uuid.UUID) { w.page("Plan v2", &final, 0, 1, "final's") },
			[]string{"place Plan moved and renamed"}, nil},
		{"a Plan v2 made in Drafts since", func(w *world, drafts, _ uuid.UUID) { w.page("Plan v2", &drafts, 2, 1, "new") },
			nil, []string{"title_taken Plan v2 held by Plan v2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			drafts, final := w.page("Drafts", nil, 0, 1, "d"), w.page("Final", nil, 1, 1, "f")
			x := w.page("Plan", &final, 1, 1, "x")
			tt.beside(w, drafts, final)
			w.placed(x, domain.TreeState{ParentID: &drafts, Name: "Plan v2", SortOrder: 1})
			plan, err := domain.PlanRevert(w.state())
			if err != nil {
				t.Fatalf("PlanRevert = %v, want a plan", err)
			}
			if got := w.ops(plan); !slices.Equal(got, tt.ops) {
				t.Errorf("ops = %q, want %q", got, tt.ops)
			}
			if got := w.conflicts(plan); !slices.Equal(got, tt.conflicts) || plan.Count != len(tt.conflicts) {
				t.Errorf("conflicts = %q (%d), want %q", got, plan.Count, tt.conflicts)
			}
		})
	}
}

// The trash items a revert reads are its deletions' tops that are still
// roots of as many pages and attachments; the versions it compares are a
// restored page's, a deleted page's at its item's revision and its
// group's, and the bases it writes back.
func TestRevertGroupsAndRefs(t *testing.T) {
	deletedAt := time.Date(2026, 10, 14, 11, 0, 0, 0, time.UTC)
	w := newWorld(t)
	p := w.page("P", nil, 0, 3, "p")
	a := w.asset("a.png", &p, 0)
	w.trash(p, deletedAt)
	w.deleted(p)
	w.deleted(a)
	q := w.page("Q", nil, 1, 1, "q")
	w.trash(q, deletedAt)
	w.deleted(q)
	n := w.nodes[q]
	n.TrashPages = 2
	w.nodes[q] = n
	r := w.page("R", nil, 2, 2, "r")
	w.restored(r)
	s := w.page("S", nil, 3, 5, "s")
	w.wrote(s, 4, "four")
	u := w.page("U", nil, 4, 1, "u")
	w.trash(u, deletedAt)
	w.deleted(u)
	n = w.nodes[u]
	n.TrashAssets = 1
	w.nodes[u] = n
	// P in the trash at revision 6 of revision 3's bytes: both compared.
	w.hashes[domain.VersionRef{NodeID: p, Revision: 6}] = hash("p")
	state := w.state()
	if groups := domain.RevertGroups(w.items, state.Nodes); len(groups) != 1 || !groups[p].Equal(deletedAt) {
		t.Errorf("RevertGroups = %v, want P's alone: Q's root holds another page, U's an attachment", groups)
	}
	got := domain.RevertRefs(w.items, w.versions, state.Groups)
	want := []domain.VersionRef{{NodeID: p, Revision: 3}, {NodeID: p, Revision: 6}, {NodeID: q, Revision: 1}, {NodeID: r, Revision: 2},
		{NodeID: s, Revision: 4}, {NodeID: u, Revision: 1}}
	if !slices.Equal(got, want) {
		t.Errorf("RevertRefs = %v, want %v", got, want)
	}
}
