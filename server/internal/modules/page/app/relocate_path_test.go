package app_test

import (
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// movePath runs move in notebook as alice from an agent.
func (f *fixture) movePath(notebook uuid.UUID, p app.PathMove) (app.Relocated, error) {
	return app.NewRelocatePath(f.writer(), f.store, f.notebooks, f.auth, f.logger()).Execute(f.asAlice(), notebook, p, agent)
}

// moveTree is the tree the moves start from: pages A and B; under A the
// page x, with its child x1, and the attachments flow.png and img.png;
// under B the page y and the attachment pic.png.
type moveTree struct {
	*fixture
	a, b, x, x1, flow, img, y, pic domain.Node
}

func newMoveTree() moveTree {
	t := moveTree{fixture: newFixture()}
	t.a, t.b = t.page("A", nil, 0), t.page("B", nil, 1)
	t.x = t.page("x", &t.a.ID, 0)
	t.x1 = t.page("x1", &t.x.ID, 0)
	t.flow, t.img = t.asset("flow.png", &t.a.ID, 1), t.asset("img.png", &t.a.ID, 2)
	t.y, t.pic = t.page("y", &t.b.ID, 0), t.asset("pic.png", &t.b.ID, 1)
	return t
}

// move answers in its order (M9/P2 design 3.8.3): both paths' 422 and
// their kinds' before the notebook; the notebook, the node, then the
// decision: node.rename for a to under from's parent, node.move for one
// under another, each its own 403.
func TestRelocatePathAnswersInOrder(t *testing.T) {
	f := newMoveTree()
	unseen := uuid.NewV7()
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		p        app.PathMove
		want     string
	}{
		{"an empty segment in from", unseen, app.PathMove{From: "A//x.md", To: "../y.md"}, "validation_failed from"},
		{"a .. in to", unseen, app.PathMove{From: "A/x.md", To: "../y.md"}, "validation_failed to"},
		{"a page to an attachment's path", unseen, app.PathMove{From: "A/x.md", To: "A/x.png"}, "validation_failed to"},
		{"an attachment to a page's path", unseen, app.PathMove{From: "A/flow.png", To: "A/flow.md"}, "validation_failed to"},
		{"a notebook not seen", unseen, app.PathMove{From: "Missing.md", To: "B/x.md"}, "notebook.not_found"},
		{"the zero id", uuid.UUID{}, app.PathMove{From: "A/x.md", To: "B/x.md"}, "notebook.not_found"},
	} {
		f.rec.calls = nil
		if _, err := f.movePath(tt.notebook, tt.p); codeOf(err) != tt.want || f.called("RootByKey") {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.grant(domain.ActionRead)
	f.auth.forbidden[domain.ActionRename], f.auth.forbidden[domain.ActionMove] = true, true
	for _, tt := range []struct {
		name string
		p    app.PathMove
		want string
	}{
		{"a reader's move of a path that names nothing", app.PathMove{From: "A/missing.md", To: "B/x.md"}, "page.not_found"},
		{"a reader's rename", app.PathMove{From: "A/x.md", To: "a/z.md"}, "forbidden node.rename"},
		{"a reader's move", app.PathMove{From: "A/x.md", To: "B/x.md"}, "forbidden node.move"},
	} {
		f.rec.calls = nil
		_, err := f.movePath(f.eng, tt.p)
		decided := ""
		for _, c := range f.rec.calls {
			if action, ok := strings.CutPrefix(c, "Authorize node."); ok {
				decided = " node." + action
			}
		}
		if codeOf(err)+decided != tt.want || f.called("LockNotebook in tx") {
			t.Errorf("%s: %v deciding %q after %v, want %s", tt.name, err, decided, f.rec.calls, tt.want)
		}
	}
	delete(f.auth.forbidden, domain.ActionRename)
	f.grant(domain.ActionRename)
	if _, err := f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "A/z.md", UpdateLinks: true}); err != nil {
		t.Errorf("a rename by who may rename = %v", err)
	}
	if _, err := f.movePath(f.eng, app.PathMove{From: "A/z.md", To: "B/z.md", UpdateLinks: true}); codeOf(err) != "forbidden" {
		t.Errorf("a move by who may only rename = %v, want forbidden", err)
	}
	if _, err := app.NewRelocatePath(f.writer(), f.store, f.notebooks, f.auth, f.logger()).
		Execute(t.Context(), f.eng, app.PathMove{From: "A/z.md", To: "A/w.md"}, agent); codeOf(err) != "unauthorized" {
		t.Errorf("no actor: %v", err)
	}
}

// move puts the node at to, its whole new path, in one step: renamed
// under its parent, moved under another, or both; a case alone changed
// under one parent or under another; an attachment named with another
// extension. The answer is to's parent's names and the node's; the line
// logged says which it was (M9/P2 design 3.8.3).
func TestRelocatePathMovesAndRenames(t *testing.T) {
	for _, tt := range []struct {
		name, from, to string
		want, logged   string
	}{
		{"a rename", "A/x.md", "a/z.md", "A/z.md", "node renamed"},
		{"a move", "A/x.md", "b/x.md", "B/x.md", "node moved"},
		{"both", "A/x.md", "B/z.md", "B/z.md", "node moved"},
		{"the case alone", "a/x.md", "A/X.md", "A/X.md", "node renamed"},
		{"the case alone under another parent", "A/x.md", "b/X.md", "B/X.md", "node moved"},
		{"to the root", "A/x.md", "z.md", "z.md", "node moved"},
		{"an attachment's extension", "A/flow.png", "A/flow.jpg", "A/flow.jpg", "node renamed"},
		{"an attachment under another page", "A/flow.png", "B/flow.png", "B/flow.png", "node moved"},
	} {
		f := newMoveTree()
		f.grant(domain.ActionRead, domain.ActionRename, domain.ActionMove)
		got, err := f.movePath(f.eng, app.PathMove{From: tt.from, To: tt.to, UpdateLinks: true})
		node := f.x
		if strings.HasPrefix(tt.from, "A/flow") {
			node = f.flow
		}
		switch {
		case err != nil:
			t.Errorf("%s: %v", tt.name, err)
		case got.Path != tt.want || got.Node.ID != node.ID || got.Outcome.ChangesetID == (uuid.UUID{}):
			t.Errorf("%s: move = %+v, want %s at %s", tt.name, got, node.Name, tt.want)
		case f.store.nodes[node.ID].Name != got.Node.Name || len(f.writes()) != 1:
			t.Errorf("%s: the store holds %+v after %v, want one write", tt.name, f.store.nodes[node.ID], f.writes())
		case !strings.Contains(f.logs.String(), `msg="`+tt.logged+`"`) || !strings.Contains(f.logs.String(), "update_links=true"):
			t.Errorf("%s: logs = %s, want %s", tt.name, f.logs, tt.logged)
		case f.store.nodes[f.x1.ID].ParentID == nil || *f.store.nodes[f.x1.ID].ParentID != f.x.ID:
			t.Errorf("%s: x1 is under %v, want it under x still", tt.name, f.store.nodes[f.x1.ID].ParentID)
		}
	}
	f := newMoveTree()
	f.grant(domain.ActionRead, domain.ActionRename)
	got, err := f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "a/x.md", UpdateLinks: true})
	if err != nil || got.Path != "A/x.md" || got.Outcome.ChangesetID != (uuid.UUID{}) || len(f.writes()) != 0 || f.logs.Len() != 0 {
		t.Errorf("a move to where the node is = %+v, %v after %v; want it as it is, nothing written", got, err, f.rec.calls)
	}
}

// move refuses in its unit, to's parent first: the first segment of it
// that fails decides, an attachment's 422 on to, a missing one's 404 at
// the parent's path with the hint that a move creates no parent, unless
// a missing one ends with .md: write's 422 on to, the parent written as a
// page's path (M9/P2 S1 review B-N2); then Relocate's 422 on to and its
// 409s, a name taken answered at to with the holder's kind (M9/P2 design
// 3.8.3).
func TestRelocatePathRefuses(t *testing.T) {
	f := newMoveTree()
	f.grant(domain.ActionRead, domain.ActionRename, domain.ActionMove)
	chain(f.fixture, 9) // La/Lb/…/Li, at the ninth level
	for _, tt := range []struct {
		name, from, to string
		want           string
	}{
		{"an attachment without its extension", "A/flow.png", "A/flow", "validation_failed to not_allowed"},
		{"a parent missing", "A/x.md", "C/x.md", "page.not_found C.md to"},
		{"a parent missing under one there", "A/x.md", "B/missing/x.md", "page.not_found B/missing.md to"},
		{"a parent written as a page's path", "A/x.md", "B.md/x.md", "validation_failed to not_allowed"},
		{"a parent missing under one there, written so", "A/x.md", "B/missing.MD/x.md", "validation_failed to not_allowed"},
		{"an attachment, then a missing segment", "A/x.md", "A/img.png/missing/z.md", "validation_failed to not_allowed"},
		{"a missing segment, then an attachment", "A/x.md", "A/missing/img.png/z.md", "page.not_found A/missing/img.png.md to"},
		{"an attachment as the parent", "A/x.md", "A/img.png/z.md", "validation_failed to not_allowed"},
		{"a page's name taken by a page", "A/x.md", "B/Y.md", "page.title_taken B/Y.md page"},
		{"a page's name taken by an attachment", "A/x.md", "B/pic.png.md", "page.title_taken B/pic.png.md attachment"},
		{"an attachment's name taken", "A/flow.png", "B/PIC.png", "page.title_taken B/PIC.png attachment"},
		{"under its own subtree", "A/x.md", "A/x/x1/x.md", "page.cycle"},
		{"too deep there", "A/x.md", "La/Lb/Lc/Ld/Le/Lf/Lg/Lh/Li/x.md", "page.too_deep"},
	} {
		f.rec.calls = nil
		_, err := f.movePath(f.eng, app.PathMove{From: tt.from, To: tt.to, UpdateLinks: true})
		got := fieldProblem(err)
		var missing *domain.PathNotFound
		var taken *domain.PathTaken
		switch {
		case errors.As(err, &missing):
			got += " " + missing.Path + " " + missing.ParentOf
			if missing.Hint != domain.MoveParentHint {
				t.Errorf("%s: hint %q", tt.name, missing.Hint)
			}
		case errors.As(err, &taken):
			got += " " + taken.Path + map[bool]string{false: " page", true: " attachment"}[taken.Asset]
		}
		if got != tt.want || len(f.writes()) != 0 || !f.called("LockNotebook in tx") {
			t.Errorf("%s: move = %q after %v, want %q in the unit", tt.name, got, f.rec.calls, tt.want)
		}
	}
	if !strings.Contains(fieldMessage(f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "A/img.png/missing/z.md"})),
		"segment 2 is an attachment, which holds no nodes") {
		t.Error("the attachment's 422 does not name its segment")
	}
	if got := fieldMessage(f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "B.md/x.md"})); got !=
		"segment 1 ends with .md: an ancestor is written by its title, without .md" {
		t.Errorf("the 422 of a parent written as a page's path says %q, want write's", got)
	}
	// A page titled B.md is found by its title: no missing segment, under
	// it the first missing one is 404 as any other.
	titled := f.page("B.md", nil, 2)
	_, err := f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "B.md/missing/x.md"})
	if missing := (*domain.PathNotFound)(nil); !errors.As(err, &missing) || missing.Path != "B.md/missing.md" {
		t.Errorf("a move under a missing page under the page titled B.md = %v, want page.not_found at B.md/missing.md", err)
	}
	if got, err := f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "B.md/x.md"}); err != nil || *got.Node.ParentID != titled.ID {
		t.Errorf("a move under the page titled B.md = %+v, %v; want x under it", got, err)
	}
}

// fieldMessage is the message of err's first field.
func fieldMessage(_ app.Relocated, err error) string {
	var e *shared.Error
	if !errors.As(err, &e) || len(e.Fields) == 0 {
		return codeOf(err)
	}
	return e.Fields[0].Message
}

// update_links is the unit's option, which the participants see, with the
// agent's client (M9/P2 design 3.8.3 step 6.4).
func TestRelocatePathGivesItsLinksOption(t *testing.T) {
	for _, links := range []bool{true, false} {
		f := newMoveTree()
		f.grant(domain.ActionRead, domain.ActionMove)
		p := &participant{recorder: f.rec}
		f.partakers = []app.Participant{p}
		if _, err := f.movePath(f.eng, app.PathMove{From: "A/x.md", To: "B/z.md", UpdateLinks: links}); err != nil ||
			len(p.steps) != 1 || p.steps[0].Options.UpdateLinks != links || p.steps[0].Client != agent ||
			p.steps[0].Operation != domain.OpMove {
			t.Errorf("a move with update_links %v = %v, the participant saw %+v", links, err, p.steps)
		}
	}
}
