package app_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// relocate runs Relocate of the node id to p in a unit of eng, as alice
// from an agent.
func (f *fixture) relocate(id uuid.UUID, p app.Placement) (domain.Node, error) {
	var got domain.Node
	_, err := f.run(f.asAlice(), domain.MCPClient("x"), func(ctx context.Context, u *app.Unit) error {
		var err error
		got, err = u.Relocate(ctx, id, p)
		return err
	})
	return got, err
}

// writes are the tree's writes the store was given, in order.
func (f *fixture) writes() []string {
	var out []string
	for _, c := range f.rec.calls {
		if call, ok := strings.CutSuffix(c, " in tx"); ok && slices.Contains([]string{"RelocateNode", "MoveNode", "RenameNode"}, call) {
			out = append(out, call)
		}
	}
	return out
}

// Under another parent, Relocate is one step of OpMove, the guard's and
// the participants', written by one RelocateNode (M9/P2 design 3.5.1): the
// node's change carries its new parent and its new name, last among its
// new siblings, and is the one item; its descendants go with it, in their
// places.
func TestRelocateMovesAndRenamesInOneStep(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	g, p := &guard{recorder: f.rec}, &participant{recorder: f.rec}
	f.guards, f.partakers = []app.WriteGuard{g}, []app.Participant{p}
	drafts, final := f.page("Drafts", nil, 0), f.page("Final", nil, 1)
	x := f.page("Plan v2", &drafts.ID, 0)
	child := f.page("Child", &x.ID, 0)
	f.page("Older", &final.ID, 3)
	got, err := f.relocate(x.ID, app.Placement{ParentID: &final.ID, Name: "Plan", Field: "to"})
	if err != nil {
		t.Fatal(err)
	}
	if *got.ParentID != final.ID || got.Name != "Plan" || got.NameKey != "plan" || got.SortOrder != 4 || got.UpdatedBy != f.alice ||
		f.store.nodes[x.ID] != got {
		t.Errorf("relocated = %+v, want Plan under Final at 4, by alice, stored", got)
	}
	if w := f.writes(); !slices.Equal(w, []string{"RelocateNode"}) {
		t.Errorf("writes = %v, want one RelocateNode", w)
	}
	if len(g.steps) != 1 || len(p.steps) != 1 || g.steps[0].Operation != domain.OpMove || p.steps[0].Operation != domain.OpMove {
		t.Fatalf("the guard saw %d steps, the participant %d; want one OpMove each", len(g.steps), len(p.steps))
	}
	c := g.steps[0].Changes
	stay := child.State()
	if len(c) != 2 || c[0].NodeID != x.ID || *c[0].Before.ParentID != drafts.ID || c[0].Before.Name != "Plan v2" ||
		*c[0].After.ParentID != final.ID || c[0].After.Name != "Plan" || c[1].NodeID != child.ID || *c[1].Before != stay ||
		*c[1].After != stay {
		t.Errorf("the step's changes = %+v, want X from Drafts as Plan v2 to Final as Plan, then Child in its place", c)
	}
	if _, ok := f.store.items[child.ID]; ok || len(f.store.items) != 1 {
		t.Errorf("items = %v, want X's alone", f.store.items)
	}
	if got := childNames(f, &final.ID); !slices.Equal(got, []string{"Older", "Plan"}) {
		t.Errorf("Final holds %v, want Plan last", got)
	}
}

// Under its parent, Relocate is a rename, OpRename written by
// RelocateNode, its place kept; a name that differs in case alone is
// written without reading the siblings, its key its own; the name it has
// writes nothing and takes no step.
func TestRelocateUnderItsParentRenames(t *testing.T) {
	for _, tt := range []struct {
		name, to string
		writes   []string
		children bool // the siblings read
	}{
		{"another name", "Final", []string{"RelocateNode"}, true},
		{"the case alone", "PLAN", []string{"RelocateNode"}, false},
		{"its name", "Plan", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			g := &guard{recorder: f.rec}
			f.guards = []app.WriteGuard{g}
			parent := f.page("Parent", nil, 0)
			f.page("First", &parent.ID, 0)
			n := f.page("Plan", &parent.ID, 1)
			f.page("Last", &parent.ID, 2)
			got, err := f.relocate(n.ID, app.Placement{ParentID: &parent.ID, Name: tt.to, Field: "to"})
			if err != nil || got.Name != tt.to || got.SortOrder != 1 || *got.ParentID != parent.ID {
				t.Fatalf("Relocate = %+v, %v; want %s in its place", got, err, tt.to)
			}
			if w := f.writes(); !slices.Equal(w, tt.writes) {
				t.Errorf("writes = %v, want %v", w, tt.writes)
			}
			if read := f.called("Children in tx"); read != tt.children {
				t.Errorf("the siblings read: %v, want %v", read, tt.children)
			}
			switch {
			case tt.writes == nil && len(g.steps) != 0:
				t.Errorf("the guard saw %d steps, want none", len(g.steps))
			case tt.writes != nil && (len(g.steps) != 1 || g.steps[0].Operation != domain.OpRename):
				t.Errorf("the guard saw %+v, want one OpRename", g.steps)
			}
			if got := childNames(f, &parent.ID); !slices.Equal(got, []string{"First", tt.to, "Last"}) {
				t.Errorf("Parent holds %v, want the order kept", got)
			}
		})
	}
}

// Relocate's refusals, in Move's order of the 409s (M9/P2 design 3.5.1):
// 404 for a node not in the notebook; 422 on the placement's field for a
// name against its kind's rules, an attachment's extension kept; 422 on
// parent_id for a parent that is no page; 409 for a parent in the node's
// subtree, a name a new sibling holds, a page's or an attachment's, its
// kind in *domain.PathTaken from the siblings read (no key looked up), a
// key a new sibling holds in another case, and a subtree too deep there,
// an attachment no level.
func TestRelocateRefuses(t *testing.T) {
	for _, tt := range []struct {
		name string
		do   func(f *fixture) (uuid.UUID, app.Placement)
		want string
	}{
		{"a node not in the notebook", func(f *fixture) (uuid.UUID, app.Placement) {
			return uuid.NewV7(), app.Placement{Name: "X", Field: "to"}
		}, "page.not_found"},
		{"a title against the rules", func(f *fixture) (uuid.UUID, app.Placement) {
			return f.page("Plan", nil, 0).ID, app.Placement{Name: "a/b", Field: "to"}
		}, "validation_failed to invalid_format"},
		{"an attachment without its extension", func(f *fixture) (uuid.UUID, app.Placement) {
			return f.asset("x.png", nil, 0).ID, app.Placement{Name: "x", Field: "to"}
		}, "validation_failed to not_allowed"},
		{"a parent that is an attachment", func(f *fixture) (uuid.UUID, app.Placement) {
			img := f.asset("img.png", nil, 0)
			return f.page("Plan", nil, 1).ID, app.Placement{ParentID: &img.ID, Name: "Plan", Field: "to"}
		}, "validation_failed parent_id not_allowed"},
		{"a parent in its subtree", func(f *fixture) (uuid.UUID, app.Placement) {
			n := f.page("Plan", nil, 0)
			child := f.page("Child", &n.ID, 0)
			return n.ID, app.Placement{ParentID: &child.ID, Name: "Plan", Field: "to"}
		}, "page.cycle"},
		{"a name a page holds there", func(f *fixture) (uuid.UUID, app.Placement) {
			b := f.page("B", nil, 0)
			f.page("x", &b.ID, 0)
			return f.page("Plan", nil, 1).ID, app.Placement{ParentID: &b.ID, Name: "x", Field: "to"}
		}, "taken page"},
		{"a name an attachment holds there", func(f *fixture) (uuid.UUID, app.Placement) {
			b := f.page("B", nil, 0)
			f.asset("x.png", &b.ID, 0)
			return f.page("Plan", nil, 1).ID, app.Placement{ParentID: &b.ID, Name: "x.png", Field: "to"}
		}, "taken attachment"},
		{"a key a new sibling holds in another case", func(f *fixture) (uuid.UUID, app.Placement) {
			a, b := f.page("A", nil, 0), f.page("B", nil, 1)
			f.page("x", &b.ID, 0)
			return f.page("x", &a.ID, 0).ID, app.Placement{ParentID: &b.ID, Name: "X", Field: "to"}
		}, "taken page"},
		{"a name a sibling holds under its parent", func(f *fixture) (uuid.UUID, app.Placement) {
			f.asset("Plan.png", nil, 0)
			return f.page("Draft", nil, 1).ID, app.Placement{Name: "plan.PNG", Field: "to"}
		}, "taken attachment"},
		{"a subtree too deep there", func(f *fixture) (uuid.UUID, app.Placement) {
			parent := chain(f, 9)
			n := f.page("Plan", nil, 1)
			f.page("Child", &n.ID, 0)
			return n.ID, app.Placement{ParentID: &parent.ID, Name: "Plan", Field: "to"}
		}, "page.too_deep"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.grant(domain.ActionRename)
			id, p := tt.do(f)
			_, err := f.relocate(id, p)
			var taken *domain.PathTaken
			got := fieldProblem(err)
			if errors.As(err, &taken) && errors.Is(err, domain.ErrTitleTaken) {
				got = map[bool]string{false: "taken page", true: "taken attachment"}[taken.Asset]
			}
			if got != tt.want {
				t.Errorf("Relocate = %v (%s), want %s", err, got, tt.want)
			}
			if w := f.writes(); len(w) != 0 || f.called("RootByKey in tx") || f.called("ChildByKey in tx") {
				t.Errorf("writes %v, calls %v; want nothing written, no key looked up", w, f.rec.calls)
			}
		})
	}
}

// A subtree whose deepest pages land at MaxDepth moves, its attachments
// below them no level of pages (M7/P2 design 3.3).
func TestRelocateCountsNoAttachmentALevel(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRename)
	parent := chain(f, 9)
	n := f.page("Plan", nil, 1)
	f.asset("img.png", &n.ID, 0)
	if got, err := f.relocate(n.ID, app.Placement{ParentID: &parent.ID, Name: "Plan", Field: "to"}); err != nil ||
		*got.ParentID != parent.ID {
		t.Errorf("Relocate to the tenth level = %+v, %v; want it there with its attachment", got, err)
	}
}

// fieldProblem is err's problem code, with its first field's name and
// code when it has fields; its text otherwise.
func fieldProblem(err error) string {
	var e *shared.Error
	switch {
	case errors.As(err, &e) && len(e.Fields) > 0:
		return e.Code + " " + e.Fields[0].Field + " " + e.Fields[0].Code
	case errors.As(err, &e):
		return e.Code
	case err == nil:
		return ""
	}
	return err.Error()
}

// chain is n pages of eng, each under the one before: the last, at depth
// n.
func chain(f *fixture, n int) domain.Node {
	var parent *uuid.UUID
	var last domain.Node
	for i := range n {
		last = f.page("L"+string(rune('a'+i)), parent, 0)
		id := last.ID // the page holds its parent's pointer
		parent = &id
	}
	return last
}
