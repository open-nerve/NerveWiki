package app_test

import (
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The tree reads without a transaction: its notebook's workspace, the
// decision, the nodes, each parent before its children.
func TestListNodesReadsTheTreeInOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	b := f.page("B", nil, 1)
	a := f.page("A", nil, 0)
	f.page("B2", &b.ID, 1)
	f.page("B1", &b.ID, 0)
	f.page("A1", &a.ID, 0)
	nodes, err := app.NewListNodes(f.notebooks, f.store, f.auth).Execute(f.asAlice(), f.eng)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if !slices.Equal(names, []string{"A", "A1", "B", "B1", "B2"}) {
		t.Errorf("tree = %v, want A, A1, B, B1, B2", names)
	}
	if !slices.Equal(f.rec.calls, []string{"WorkspaceOf", "Authorize node.list", "ListNodes"}) {
		t.Errorf("calls = %v, want the workspace, the decision, the nodes, outside a transaction", f.rec.calls)
	}
}

func TestListNodesOfANotebookNotSeen(t *testing.T) {
	f := newFixture()
	for _, id := range []uuid.UUID{uuid.NewV7(), f.eng} {
		if _, err := app.NewListNodes(f.notebooks, f.store, f.auth).Execute(f.asAlice(), id); codeOf(err) != "notebook.not_found" {
			t.Errorf("listNodes = %v, want notebook.not_found", err)
		}
	}
}

// A page reads without a transaction: its node, its notebook's workspace,
// the decision, its ancestors from the root and its content's meta.
func TestGetPageReadsThePageAndItsAncestors(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	a := f.page("A", nil, 0)
	b := f.page("B", &a.ID, 0)
	c := f.page("C", &b.ID, 0)
	v, err := app.NewGetPage(f.notebooks, f.store, f.auth).Execute(f.asAlice(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Node.ID != c.ID || !slices.Equal(v.Ancestors, []domain.Ancestor{{ID: a.ID, Name: "A"}, {ID: b.ID, Name: "B"}}) ||
		v.Content.Revision != 1 || v.Content.UpdatedBy != f.alice {
		t.Errorf("page = %+v, want C under A and B, revision 1 by alice", v)
	}
	if !slices.Equal(f.rec.calls, []string{"FindNode", "WorkspaceOf", "Authorize page.read", "Ancestors", "ContentMeta"}) {
		t.Errorf("calls = %v, want the node, the workspace, the decision, then the rest, outside a transaction", f.rec.calls)
	}
	if f.auth.targets[0] != (shared.Target{WorkspaceID: f.acme, NotebookID: f.eng}) {
		t.Errorf("decided on %+v, want acme's eng", f.auth.targets[0])
	}
}

func TestGetPageOfAPageNotSeen(t *testing.T) {
	f := newFixture()
	n := f.page("A", nil, 0)
	asset := f.page("photo.png", nil, 1)
	asset.Kind = domain.KindAsset
	f.store.nodes[asset.ID] = asset
	f.grant(domain.ActionRead)
	other := uuid.NewV7()
	moved := f.page("Elsewhere", nil, 2)
	moved.NotebookID = other
	f.store.nodes[moved.ID] = moved
	for _, id := range []uuid.UUID{uuid.NewV7(), asset.ID, moved.ID} {
		if _, err := app.NewGetPage(f.notebooks, f.store, f.auth).Execute(f.asAlice(), id); codeOf(err) != "page.not_found" {
			t.Errorf("getPage = %v, want page.not_found", err)
		}
	}
	delete(f.auth.grants, domain.ActionRead)
	if _, err := app.NewGetPage(f.notebooks, f.store, f.auth).Execute(f.asAlice(), n.ID); codeOf(err) != "page.not_found" {
		t.Errorf("getPage of a notebook not seen = %v, want page.not_found", err)
	}
}

// A page's reading view reads as the page does, without a transaction:
// its node, its notebook's workspace, the decision, then its content and
// version, parsed and rendered for this page.
func TestGetPageViewRendersThePagesContent(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	n := f.page("A", nil, 0)
	c := f.store.contents[n.ID]
	c.Content, c.Revision = "# Hello", 3
	f.store.contents[n.ID] = c
	md := &fakeMarkdown{recorder: f.rec}
	v, err := app.NewGetPageView(f.notebooks, f.store, f.auth, md, f.budget).Execute(f.asAlice(), n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v != (app.ReadingView{HTML: "<p># Hello</p>", Revision: 3}) {
		t.Errorf("view = %+v, want the content rendered at revision 3", v)
	}
	if !slices.Equal(f.rec.calls, []string{"FindNode", "WorkspaceOf", "Authorize page.read", "PageContent", "Take 7", "Parse", "Render",
		"Release 7"}) {
		t.Errorf("calls = %v, want the node, the workspace, the decision, the content, then its parse and rendering within the"+
			" budget, outside a transaction", f.rec.calls)
	}
	if !slices.Equal(md.pages, []app.PageRef{{NotebookID: f.eng, PageID: n.ID}}) {
		t.Errorf("rendered for %v, want eng's page", md.pages)
	}
}

func TestGetPageViewOfAPageNotSeen(t *testing.T) {
	f := newFixture()
	n := f.page("A", nil, 0)
	asset := f.page("photo.png", nil, 1)
	asset.Kind = domain.KindAsset
	f.store.nodes[asset.ID] = asset
	contentless := f.page("B", nil, 2)
	delete(f.store.contents, contentless.ID)
	other := f.page("Elsewhere", nil, 3)
	other.NotebookID = uuid.NewV7()
	f.store.nodes[other.ID] = other
	f.grant(domain.ActionRead)
	view := app.NewGetPageView(f.notebooks, f.store, f.auth, &fakeMarkdown{recorder: f.rec}, f.budget)
	for _, id := range []uuid.UUID{uuid.NewV7(), asset.ID, contentless.ID, other.ID} {
		if _, err := view.Execute(f.asAlice(), id); codeOf(err) != "page.not_found" {
			t.Errorf("getPageView = %v, want page.not_found", err)
		}
	}
	delete(f.auth.grants, domain.ActionRead)
	if _, err := view.Execute(f.asAlice(), n.ID); codeOf(err) != "page.not_found" {
		t.Errorf("getPageView of a notebook not seen = %v, want page.not_found", err)
	}
	if f.called("Render") {
		t.Error("rendered a page not seen")
	}
}

func TestGetPageViewReturnsRendersError(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionRead)
	n := f.page("A", nil, 0)
	f.writeAs(n.ID, "# A")
	down := errors.New("down")
	_, err := app.NewGetPageView(f.notebooks, f.store, f.auth, &fakeMarkdown{recorder: f.rec, err: down}, f.budget).Execute(f.asAlice(), n.ID)
	if !errors.Is(err, down) || f.budget.held != 0 {
		t.Errorf("getPageView = %v, %d bytes of the budget held; want %v, the budget released", err, f.budget.held, down)
	}
}
