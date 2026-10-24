package app_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// fakeOverview is a notebook's overview: its counts, its last change and
// its root's nodes, each read a call.
type fakeOverview struct {
	*recorder
	pages, attachments int64
	last               *time.Time
	root               []domain.Node
}

func (f *fakeOverview) NotebookCounts(ctx context.Context, _ uuid.UUID) (int64, int64, error) {
	f.record(ctx, "NotebookCounts")
	return f.pages, f.attachments, nil
}

func (f *fakeOverview) NotebookLastChange(ctx context.Context, _ uuid.UUID) (*time.Time, error) {
	f.record(ctx, "NotebookLastChange")
	return f.last, nil
}

func (f *fakeOverview) FirstChildren(ctx context.Context, _ uuid.UUID, n int) ([]domain.Node, error) {
	f.record(ctx, fmt.Sprintf("FirstChildren %d", n))
	return f.root[:min(n, len(f.root))], nil
}

// opener is the use case on f's fakes and overview.
func (f *fixture) opener(o *fakeOverview) *app.OpenNotebook {
	return app.NewOpenNotebook(app.NewNotebookGate(f.notebooks, f.auth), f.notebooks, o, f.instructionsOf())
}

// open_notebook decides the notebook first, as the notebook entry's gate
// does, then reads its mode, its counts, its last change, the root's first
// nodes and its instructions, unlocked (M9/P1 design 3.17.3; M10/P1 design
// 3.12): a notebook the caller does not see, or the zero id, is
// notebook.not_found before any of them is read; one deleted after the
// decision is notebook.not_found at its mode.
func TestOpenNotebookAnswersInOrder(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	f.notebooks.states = map[uuid.UUID]domain.NotebookState{f.eng: {Mode: domain.ModeLLMWiki, SourceRootID: uuid.NewV7()}}
	agents := f.page("AGENTS", nil, 0)
	f.write(agents, "# Rules", 1)
	logo := f.asset("logo.png", nil, 1)
	last := now()
	o := &fakeOverview{recorder: f.rec, pages: 3, attachments: 1, last: &last, root: []domain.Node{agents, logo}}
	f.rec.calls = nil
	got, err := f.opener(o).Execute(f.asAlice(), f.eng)
	want := app.Opened{Mode: domain.ModeLLMWiki, Agents: domain.Agents{Body: "# Rules", Title: "AGENTS"}, HasAgents: true, Pages: 3,
		Attachments: 1, LastChange: &last,
		Root: []app.RootNode{{Path: "AGENTS.md", Kind: domain.KindPage}, {Path: "logo.png", Kind: domain.KindAsset}}}
	if err != nil || got.Mode != want.Mode || got.Agents != want.Agents || !got.HasAgents || got.Pages != 3 || got.Attachments != 1 ||
		got.LastChange != &last || !slices.Equal(got.Root, want.Root) || got.MoreRoot {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	calls := []string{"WorkspaceOf", "Authorize node.list", "StateOf", "NotebookCounts", "NotebookLastChange", "FirstChildren 101",
		"AgentsPage", "PageContent", "FrontmatterEnd"}
	if !slices.Equal(f.rec.calls, calls) {
		t.Errorf("calls = %q, want %q", f.rec.calls, calls)
	}
	for _, id := range []uuid.UUID{uuid.NewV7(), {}} {
		f.rec.calls = nil
		if _, err := f.opener(o).Execute(f.asAlice(), id); codeOf(err) != "notebook.not_found" ||
			slices.ContainsFunc(f.rec.calls, func(c string) bool { return c != "WorkspaceOf" }) {
			t.Errorf("open %s = %v after %q, want notebook.not_found, nothing of it read", id, err, f.rec.calls)
		}
	}
	f.auth.grants[domain.ActionList] = false
	f.rec.calls = nil
	if _, err := f.opener(o).Execute(f.asAlice(), f.eng); codeOf(err) != "notebook.not_found" ||
		!slices.Equal(f.rec.calls, []string{"WorkspaceOf", "Authorize node.list"}) {
		t.Errorf("open a notebook not seen = %v after %q, want notebook.not_found after the decision alone", err, f.rec.calls)
	}
	f.auth.grants[domain.ActionList] = true
	f.notebooks.gone[f.eng] = true
	f.rec.calls = nil
	if _, err := f.opener(o).Execute(f.asAlice(), f.eng); codeOf(err) != "notebook.not_found" ||
		!slices.Equal(f.rec.calls, []string{"WorkspaceOf", "Authorize node.list", "StateOf"}) {
		t.Errorf("open a notebook deleted after the decision = %v after %q, want notebook.not_found at its mode", err, f.rec.calls)
	}
}

// The overview lists the root's first 100 nodes, more following when
// there are; a notebook never written in has no last change; one without
// an AGENTS page has the built-in instructions, which the adapter writes.
func TestOpenNotebookOfTheRootsFirstNodes(t *testing.T) {
	f := newFixture()
	f.grant(domain.ActionList)
	var root []domain.Node
	for i := range domain.MaxOverviewRoot + 1 {
		root = append(root, f.page(fmt.Sprintf("P%03d", i), nil, float64(i)))
	}
	o := &fakeOverview{recorder: f.rec, root: root}
	got, err := f.opener(o).Execute(f.asAlice(), f.eng)
	if err != nil || len(got.Root) != domain.MaxOverviewRoot || !got.MoreRoot || got.Root[99].Path != "P099.md" || got.LastChange != nil ||
		got.HasAgents {
		t.Errorf("101 at the root: %d listed, more %v, last %v, agents %v, %v", len(got.Root), got.MoreRoot, got.LastChange, got.HasAgents, err)
	}
	o.root = root[:domain.MaxOverviewRoot]
	if got, err := f.opener(o).Execute(f.asAlice(), f.eng); err != nil || len(got.Root) != domain.MaxOverviewRoot || got.MoreRoot {
		t.Errorf("100 at the root: %d listed, more %v, %v; want all", len(got.Root), got.MoreRoot, err)
	}
}
