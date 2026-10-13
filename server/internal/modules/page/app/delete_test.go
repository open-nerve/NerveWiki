package app_test

import (
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// remove runs deleteNode as alice from the web.
func (f *fixture) remove(id uuid.UUID) error {
	return app.NewDeleteNode(f.writer(), f.store, f.logger()).Execute(f.asAlice(), id, domain.ClientWeb)
}

// A deletion finds the node, then runs a unit of its notebook that reads
// the subtree under the lock and deletes it whole. Every node is in the
// guard's step, the participants' and the event with no after, and is an
// item; the siblings stay where they are.
func TestDeleteNodeDeletesTheSubtreeUnderTheLock(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionDelete)
	g, o, p := &guard{recorder: f.rec}, &observer{recorder: f.rec}, &participant{recorder: f.rec}
	f.guards, f.observers, f.partakers = []app.WriteGuard{g}, []app.PageObserver{o}, []app.Participant{p}
	n := f.page("N", nil, 0)
	child := f.page("Child", &n.ID, 0)
	grandchild := f.page("Grandchild", &child.ID, 0)
	sibling := f.page("Sibling", nil, 1)
	if err := f.remove(n.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{"FindNode", "WorkspaceOf", "ShareWorkspace in tx", "LockNotebook in tx", "Authorize node.delete in tx", "Subtree in tx"}
	if !slices.Equal(f.rec.calls[:len(want)], want) {
		t.Errorf("calls = %v, want them to begin %v", f.rec.calls, want)
	}
	if len(f.store.nodes) != 1 || f.store.nodes[sibling.ID] != sibling {
		t.Errorf("nodes left = %v, want the sibling as it was", f.store.nodes)
	}
	ids := []uuid.UUID{n.ID, child.ID, grandchild.ID}
	if len(p.steps) != 1 || p.steps[0].Operation != domain.OpDelete {
		t.Fatalf("the participant followed %+v, want the deletion", p.steps)
	}
	for what, changes := range map[string][]domain.Change{
		"the guard's step": g.steps[0].Changes, "the participant's": p.steps[0].Changes, "the event": o.events[0].Changes,
	} {
		var got []uuid.UUID
		for _, c := range changes {
			if c.After == nil && c.Before != nil {
				got = append(got, c.NodeID)
			}
		}
		if !slices.Equal(got, ids) {
			t.Errorf("%s deletes %v, want N, Child, Grandchild", what, got)
		}
	}
	for _, id := range ids {
		if it, ok := f.store.items[id]; !ok || it.Change.After != nil {
			t.Errorf("item of %s = %+v, want a deletion", id, it)
		}
	}
	if g.steps[0].Operation != domain.OpDelete || len(o.events) != 1 {
		t.Errorf("the guard saw %s, the observers %d events; want a deletion, one event", g.steps[0].Operation, len(o.events))
	}
	logs := f.logs.String()
	if !strings.Contains(logs, "node deleted") || !strings.Contains(logs, "nodes=3") || !strings.Contains(logs, n.ID.String()) ||
		strings.Contains(logs, "Child") {
		t.Errorf("log %q, want the deletion of N's three nodes without a title", logs)
	}
}

// A deletion's root becomes a trash item: DeleteNodes gets it first among
// the subtree's ids, with the subtree's pages, attachments and height,
// and the step's items are one write (M8/P2 design 3.3).
func TestDeleteNodeMarksTheRootWithItsCounts(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionDelete)
	n := f.page("N", nil, 0)
	child := f.page("Child", &n.ID, 0)
	grandchild := f.page("Grandchild", &child.ID, 0)
	photo := f.asset("photo.png", &grandchild.ID, 0)
	scan := f.asset("scan.pdf", &n.ID, 1)
	f.page("Sibling", nil, 1)
	if err := f.remove(n.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.store.deletions) != 1 {
		t.Fatalf("deletions = %+v, want one", f.store.deletions)
	}
	d := f.store.deletions[0]
	if d.Root != n.ID || d.Pages != 3 || d.Assets != 2 || d.Height != 3 || d.By != f.alice || !d.At.Equal(now()) ||
		len(d.IDs) != 5 || d.IDs[0] != n.ID || !slices.Contains(d.IDs, photo.ID) || !slices.Contains(d.IDs, scan.ID) {
		t.Errorf("deletion = %+v; want N's, first of its five nodes, 3 pages, 2 attachments, 3 levels, by alice at %s", d, now())
	}
	if n := slices.Index(f.rec.calls, "RecordItems 5 in tx"); n < 0 || slices.ContainsFunc(f.rec.calls[n+1:], func(c string) bool {
		return strings.HasPrefix(c, "RecordItems")
	}) {
		t.Errorf("calls = %v, want the five items in one write", f.rec.calls)
	}
}

func TestDeleteNodeAnswersItsCodesInOrder(t *testing.T) {
	refusal := shared.NewError(shared.KindConflict, "page.locked", "Locked.")
	for _, tt := range []struct {
		name  string
		setup func(f *fixture, n domain.Node) uuid.UUID
		want  string
	}{
		{"an unknown node", func(f *fixture, n domain.Node) uuid.UUID { return uuid.NewV7() }, "page.not_found"},
		{"a notebook deleted while it waited", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionDelete)
			f.notebooks.gone[f.eng] = true
			return n.ID
		}, "page.not_found"},
		{"a notebook the caller cannot see", func(f *fixture, n domain.Node) uuid.UUID { return n.ID }, "page.not_found"},
		{"a node deleted while it waited", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionDelete)
			f.notebooks.waited = func() { delete(f.store.nodes, n.ID) }
			return n.ID
		}, "page.not_found"},
		{"a reader", func(f *fixture, n domain.Node) uuid.UUID {
			f.auth.forbidden[domain.ActionDelete] = true
			return n.ID
		}, "forbidden"},
		{"the guard", func(f *fixture, n domain.Node) uuid.UUID {
			f.grant(domain.ActionDelete)
			f.guards = []app.WriteGuard{&guard{recorder: f.rec, err: refusal}}
			return n.ID
		}, "page.locked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			n := f.page("N", nil, 0)
			f.page("Child", &n.ID, 0)
			err := f.remove(tt.setup(f, n))
			if got := codeOf(err); got != tt.want {
				t.Errorf("deleteNode = %q, want %q", got, tt.want)
			}
			if f.called("DeleteNodes in tx") || f.logs.Len() != 0 {
				t.Errorf("a refused deletion left %d nodes, deleted %v, logged %q; want both, nothing", len(f.store.nodes),
					f.called("DeleteNodes in tx"), f.logs)
			}
		})
	}
}
