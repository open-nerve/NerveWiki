package app_test

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// move runs moveNode as alice from the web.
func (f *fixture) move(id uuid.UUID, to app.Destination) (domain.Node, error) {
	return app.NewMoveNode(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, to, domain.ClientWeb)
}

// A move finds the node, then runs a unit of its notebook that finds it
// again under the lock. The node's change is its item; its descendants go
// with it, in the guard's step and the event, their places unchanged, and
// are no items.
func TestMoveNodeMovesTheSubtreeUnderTheLock(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionMove)
	g, o := &guard{recorder: f.rec}, &observer{recorder: f.rec}
	f.guards, f.observers = []app.WriteGuard{g}, []app.PageObserver{o}
	p, q := f.page("P", nil, 0), f.page("Q", nil, 1)
	n := f.page("N", &p.ID, 0)
	child := f.page("Child", &n.ID, 0)
	f.page("Older", &q.ID, 3)
	got, err := f.move(n.ID, app.Destination{ParentID: &q.ID})
	if err != nil {
		t.Fatal(err)
	}
	if *got.ParentID != q.ID || got.SortOrder != 4 || got.UpdatedBy != f.alice || !got.UpdatedAt.Equal(now()) {
		t.Errorf("moved = %+v, want under Q at 4 by alice at %v", got, now())
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize node.move in tx", "FindNodeIn in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) {
		t.Errorf("calls = %v, want them to begin %v", f.rec.calls, want)
	}
	item := f.store.items[n.ID]
	if *item.Change.Before.ParentID != p.ID || *item.Change.After.ParentID != q.ID || item.Change.After.SortOrder != 4 {
		t.Errorf("item = %+v, want N from under P to under Q at 4", item.Change)
	}
	if _, ok := f.store.items[child.ID]; ok || len(f.store.items) != 1 {
		t.Errorf("items = %v, want N's alone", f.store.items)
	}
	stay := child.State()
	for what, changes := range map[string][]domain.Change{"the guard's step": g.steps[0].Changes, "the event": o.events[0].Changes} {
		if len(changes) != 2 || changes[0].NodeID != n.ID || changes[1].NodeID != child.ID || *changes[1].Before != stay ||
			*changes[1].After != stay {
			t.Errorf("%s = %+v, want N's move, then Child in its place", what, changes)
		}
	}
	if g.steps[0].Operation != domain.OpMove || len(o.events) != 1 {
		t.Errorf("the guard saw %s, the observers %d events; want a move, one event", g.steps[0].Operation, len(o.events))
	}
	if !strings.Contains(f.logs.String(), "node moved") || strings.Contains(f.logs.String(), `"N"`) {
		t.Errorf("log %q, want the move without a title", f.logs)
	}
}

// A move places the node among its new siblings as a new page goes, the
// node itself left out of its old place; renumbering keeps their order.
// The pages are under a page, which each request names by an id of its
// own: the same parent is the same id, not the same pointer.
func TestMoveNodePlacesItAmongItsSiblings(t *testing.T) {
	type pages struct{ p, a, b, c, x domain.Node }
	under := func(n domain.Node) *uuid.UUID {
		id := n.ID
		return &id
	}
	for _, tt := range []struct {
		name  string
		to    func(f *fixture, s pages) (domain.Node, app.Destination)
		order []string // P's children after the move
	}{
		{"first", func(f *fixture, s pages) (domain.Node, app.Destination) {
			return s.c, app.Destination{ParentID: under(s.p), Position: app.First()}
		}, []string{"C", "A", "B"}},
		{"after a sibling", func(f *fixture, s pages) (domain.Node, app.Destination) {
			return s.c, app.Destination{ParentID: under(s.p), Position: app.After(s.a.ID)}
		}, []string{"A", "C", "B"}},
		{"last from the first place", func(f *fixture, s pages) (domain.Node, app.Destination) {
			return s.a, app.Destination{ParentID: under(s.p)}
		}, []string{"B", "C", "A"}},
		{"from the last place between two with no gap", func(f *fixture, s pages) (domain.Node, app.Destination) {
			f.store.nodes[s.b.ID] = withOrder(s.b, s.a.SortOrder+5e-10)
			return s.c, app.Destination{ParentID: under(s.p), Position: app.After(s.a.ID)}
		}, []string{"A", "C", "B"}},
		{"from the first place between two with no gap", func(f *fixture, s pages) (domain.Node, app.Destination) {
			f.store.nodes[s.c.ID] = withOrder(s.c, s.b.SortOrder+5e-10)
			return s.a, app.Destination{ParentID: under(s.p), Position: app.After(s.b.ID)}
		}, []string{"B", "A", "C"}},
		{"in from another parent", func(f *fixture, s pages) (domain.Node, app.Destination) {
			return s.x, app.Destination{ParentID: under(s.p), Position: app.After(s.a.ID)}
		}, []string{"A", "X", "B", "C"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionMove)
			var s pages
			s.p, s.x = f.page("P", nil, 0), f.page("X", nil, 1)
			s.a, s.b, s.c = f.page("A", &s.p.ID, 0), f.page("B", &s.p.ID, 1), f.page("C", &s.p.ID, 2)
			n, to := tt.to(f, s)
			if _, err := f.move(n.ID, to); err != nil {
				t.Fatal(err)
			}
			if got := childNames(f, &s.p.ID); !slices.Equal(got, tt.order) {
				t.Errorf("P holds %v, want %v", got, tt.order)
			}
		})
	}
}

func withOrder(n domain.Node, order float64) domain.Node {
	n.SortOrder = order
	return n
}

// childNames are the names of the children of parent in eng, in order.
func childNames(f *fixture, parent *uuid.UUID) []string {
	var children []domain.Node
	for _, n := range f.store.nodes {
		if domain.SameParent(n.ParentID, parent) {
			children = append(children, n)
		}
	}
	slices.SortFunc(children, func(a, b domain.Node) int { return cmp.Compare(a.SortOrder, b.SortOrder) })
	out := make([]string, len(children))
	for i, n := range children {
		out[i] = n.Name
	}
	return out
}

// A move to where the node is writes nothing: no changeset, no event, no
// log; it answers the node. The parent is named by an id of its own.
func TestMoveNodeToWhereItIs(t *testing.T) {
	for _, tt := range []struct {
		name string
		to   func(a, b, c domain.Node) (domain.Node, app.Position)
	}{
		{"last, the last", func(a, b, c domain.Node) (domain.Node, app.Position) { return c, app.Position{} }},
		{"first, the first", func(a, b, c domain.Node) (domain.Node, app.Position) { return a, app.First() }},
		{"after the one it follows", func(a, b, c domain.Node) (domain.Node, app.Position) { return b, app.After(a.ID) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionMove)
			o := &observer{recorder: f.rec}
			f.observers = []app.PageObserver{o}
			p := f.page("P", nil, 0)
			n, at := tt.to(f.page("A", &p.ID, 0), f.page("B", &p.ID, 1), f.page("C", &p.ID, 2))
			parent := p.ID
			got, err := f.move(n.ID, app.Destination{ParentID: &parent, Position: at})
			if err != nil || got != n || f.called("MoveNode in tx") || len(f.store.changesets) != 0 || len(o.events) != 0 ||
				f.logs.Len() != 0 {
				t.Errorf("moving %s to its place = %+v, %v; moved %v, %d changesets, %d events, log %q; want nothing", n.Name, got, err,
					f.called("MoveNode in tx"), len(f.store.changesets), len(o.events), f.logs)
			}
		})
	}
}

// A subtree may reach the tenth level, not the eleventh: its height counts,
// not how many pages it holds.
func TestMoveNodeRefusesASubtreeTooDeep(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionMove)
	var parent *uuid.UUID
	var levels []domain.Node
	for i := range domain.MaxDepth - 2 {
		n := f.page("Level "+string(rune('A'+i)), parent, 0)
		levels = append(levels, n)
		parent = &n.ID
	}
	top := f.page("Top", nil, 1)
	middle := f.page("Middle", &top.ID, 0)
	f.page("Side", &top.ID, 1)
	f.page("Bottom", &middle.ID, 0)
	if _, err := f.move(top.ID, app.Destination{ParentID: &levels[len(levels)-1].ID}); codeOf(err) != "page.too_deep" {
		t.Errorf("three levels under the eighth = %v, want page.too_deep", err)
	}
	if _, err := f.move(top.ID, app.Destination{ParentID: &levels[len(levels)-2].ID}); err != nil {
		t.Errorf("three levels of four pages under the seventh = %v, want them moved, the deepest at the tenth", err)
	}
}

func TestMoveNodeAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	type nodes struct{ n, child, grandchild, other, twin domain.Node }
	// chain adds count pages under parent in f, each under the one before,
	// and returns the last.
	chain := func(f *fixture, parent domain.Node, count int) domain.Node {
		for i := range count {
			id := parent.ID
			parent = f.page(fmt.Sprintf("%s %d", parent.Name, i), &id, 0)
		}
		return parent
	}
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, s nodes) (uuid.UUID, app.Destination)
		want  string
	}{
		{"an unknown node", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			return uuid.NewV7(), app.Destination{}
		}, "page.not_found"},
		{"a notebook deleted while it waited", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			f.notebooks.gone[f.eng] = true
			return s.n.ID, app.Destination{}
		}, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			return s.n.ID, app.Destination{}
		}, "page.not_found"},
		{"a node deleted while it waited", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			f.notebooks.waited = func() { delete(f.store.nodes, s.n.ID) }
			return s.n.ID, app.Destination{ParentID: &s.other.ID}
		}, "page.not_found"},
		{"a reader", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.auth.forbidden[domain.ActionMove] = true
			return s.n.ID, app.Destination{}
		}, "forbidden"},
		{"a parent that does not exist", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			missing := uuid.NewV7()
			return s.n.ID, app.Destination{ParentID: &missing}
		}, "validation_failed parent_id"},
		{"a sibling to follow of another parent", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			return s.n.ID, app.Destination{ParentID: &s.other.ID, Position: app.After(s.child.ID)}
		}, "validation_failed after_id"},
		{"itself to follow", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			return s.n.ID, app.Destination{Position: app.After(s.n.ID)}
		}, "validation_failed after_id"},
		{"the values before the cycle", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			return s.n.ID, app.Destination{ParentID: &s.child.ID, Position: app.After(uuid.NewV7())}
		}, "validation_failed after_id"},
		{"under itself", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			return s.n.ID, app.Destination{ParentID: &s.n.ID}
		}, "page.cycle"},
		{"under its grandchild, before the title", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			f.page("n", &s.grandchild.ID, 0)
			return s.n.ID, app.Destination{ParentID: &s.grandchild.ID}
		}, "page.cycle"},
		{"under its deepest descendant, before the depth", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			deepest := chain(f, s.grandchild, 3)
			return s.n.ID, app.Destination{ParentID: &deepest.ID}
		}, "page.cycle"},
		{"a new sibling's title, before the depth", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			eighth := chain(f, s.other, 7)
			f.page("n", &eighth.ID, 0)
			return s.n.ID, app.Destination{ParentID: &eighth.ID}
		}, "page.title_taken"},
		{"a new sibling's title", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return s.n.ID, app.Destination{ParentID: &s.twin.ID}
		}, "page.title_taken"},
		{"the guard last", func(f *fixture, s nodes) (uuid.UUID, app.Destination) {
			f.grant(domain.ActionMove)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return s.n.ID, app.Destination{ParentID: &s.other.ID}
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			var s nodes
			s.n = f.page("N", nil, 0)
			s.child = f.page("Child", &s.n.ID, 0)
			s.grandchild = f.page("Grandchild", &s.child.ID, 0)
			s.other, s.twin = f.page("Other", nil, 1), f.page("Twin", nil, 2)
			f.page("n", &s.twin.ID, 0)
			id, to := tt.setup(f, s)
			before := f.store.nodes[s.n.ID]
			_, err := f.move(id, to)
			if got := codeOf(err); got != tt.want {
				t.Errorf("moveNode = %q, want %q", got, tt.want)
			}
			if after, ok := f.store.nodes[s.n.ID]; ok && after != before || f.logs.Len() != 0 || f.called("MoveNode in tx") {
				t.Errorf("a refused move left %+v, logged %q; want N where it was, nothing", after, f.logs)
			}
		})
	}
}
