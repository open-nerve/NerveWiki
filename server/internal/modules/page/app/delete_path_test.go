package app_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// deletePath runs delete in notebook as alice from an agent.
func (f *fixture) deletePath(notebook uuid.UUID, p app.PathDelete) (app.Deleted, error) {
	return app.NewDeletePath(f.writer(), f.store, f.notebooks, f.auth, f.logger()).Execute(f.asAlice(), notebook, p, agent)
}

// delete answers in its order (M9/P2 design 3.8.4): the path's 422; the
// notebook, the node, then node.delete's 403.
func TestDeletePathAnswersInOrder(t *testing.T) {
	f := newFixture()
	f.page("Notes", nil, 0)
	unseen := uuid.NewV7()
	for _, tt := range []struct {
		name     string
		notebook uuid.UUID
		path     string
		want     string
	}{
		{"a path ending with /", unseen, "Notes/", "validation_failed path"},
		{"a notebook not seen", unseen, "Missing.md", "notebook.not_found"},
		{"the zero id", uuid.UUID{}, "Notes.md", "notebook.not_found"},
	} {
		if _, err := f.deletePath(tt.notebook, app.PathDelete{Path: tt.path}); codeOf(err) != tt.want || f.called("RootByKey") {
			t.Errorf("%s: %v after %v, want %s", tt.name, err, f.rec.calls, tt.want)
		}
	}
	f.grant(domain.ActionRead)
	f.auth.forbidden[domain.ActionDelete] = true
	for path, want := range map[string]string{"Missing.md": "page.not_found", "Notes.md": "forbidden"} {
		f.rec.calls = nil
		if _, err := f.deletePath(f.eng, app.PathDelete{Path: path}); codeOf(err) != want || f.called("LockNotebook in tx") {
			t.Errorf("a reader's delete of %s = %v after %v, want %s", path, err, f.rec.calls, want)
		}
	}
	if _, err := app.NewDeletePath(f.writer(), f.store, f.notebooks, f.auth, f.logger()).
		Execute(t.Context(), f.eng, app.PathDelete{Path: "Notes.md"}, agent); codeOf(err) != "unauthorized" {
		t.Errorf("no actor: %v", err)
	}
}

// delete puts a page or an attachment in the trash with what is under it,
// counted in the unit, itself included; a page with nodes under it only
// with recursive, the 422 counting them. The guard sees the agent's
// client (M9/P2 design 3.8.4, 3.6).
func TestDeletePathPutsTheNodeInTheTrash(t *testing.T) {
	for _, tt := range []struct {
		name, path            string
		recursive             bool
		want                  string
		pages, assets, logged int
	}{
		{"a page", "Guide/Child/Leaf.md", false, "", 1, 0, 1},
		{"an attachment", "Guide/img.png", false, "", 0, 1, 1},
		{"a page with children", "guide.md", false, "validation_failed recursive", 2, 2, 0},
		{"a page with children, recursive", "Guide.md", true, "", 3, 2, 5},
	} {
		f := newFixture()
		f.grant(domain.ActionRead, domain.ActionDelete)
		g := &guard{recorder: f.rec}
		f.guards = []app.WriteGuard{g}
		guide := f.page("Guide", nil, 0)
		f.asset("img.png", &guide.ID, 0)
		child := f.page("Child", &guide.ID, 1)
		f.asset("file.pdf", &child.ID, 0)
		f.page("Leaf", &child.ID, 1)
		got, err := f.deletePath(f.eng, app.PathDelete{Path: tt.path, Recursive: tt.recursive})
		var r *domain.Recursive
		switch {
		case codeOf(err) != tt.want:
			t.Errorf("%s: delete = %v, want %q", tt.name, err, tt.want)
		case err != nil && (!errors.As(err, &r) || r.Pages != tt.pages || r.Assets != tt.assets || len(f.store.deletions) != 0 ||
			!f.called("Subtree in tx")):
			t.Errorf("%s: delete = %+v after %v, want %d pages and %d attachments under it, counted in the unit",
				tt.name, r, f.rec.calls, tt.pages, tt.assets)
		case err == nil && (got.Pages != tt.pages || got.Assets != tt.assets || got.Outcome.ChangesetID == (uuid.UUID{}) ||
			len(f.store.deletions) != 1 || f.store.deletions[0].Root != got.Node.ID):
			t.Errorf("%s: delete = %+v, want %d pages and %d attachments in the trash", tt.name, got, tt.pages, tt.assets)
		case err == nil && (len(g.steps) != 1 || g.steps[0].Client != agent || g.steps[0].Operation != domain.OpDelete):
			t.Errorf("%s: the guard saw %+v, want the agent's deletion", tt.name, g.steps)
		case err == nil && !strings.Contains(f.logs.String(), `msg="node deleted"`):
			t.Errorf("%s: logs = %s", tt.name, f.logs)
		case err == nil && !strings.Contains(f.logs.String(), fmt.Sprintf("nodes=%d pages=%d assets=%d", tt.logged, tt.pages, tt.assets)):
			t.Errorf("%s: logs = %s, want nodes=%d", tt.name, f.logs, tt.logged)
		case err == nil && got.Path != tt.path:
			t.Errorf("%s: delete answered %s", tt.name, got.Path)
		}
	}
}
