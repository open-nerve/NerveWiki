package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// accountWorkspaces are the caller's workspaces, in the order given.
type accountWorkspaces []app.AccountWorkspace

func (a accountWorkspaces) WorkspacesOf(context.Context, uuid.UUID) ([]app.AccountWorkspace, error) {
	return slices.Clone(a), nil
}

// listers are each workspace's notebooks, by its id; each call is
// recorded, "<workspace> reached" when the access reaches the caller.
type listers struct {
	notebooks map[uuid.UUID][]app.Listed
	names     map[uuid.UUID]string
	calls     []string
}

func (l *listers) ListNotebooks(_ context.Context, workspaceID, _ uuid.UUID, reached bool) ([]app.Listed, error) {
	call := l.names[workspaceID]
	if reached {
		call += " reached"
	}
	l.calls = append(l.calls, call)
	return l.notebooks[workspaceID], nil
}

// The caller's notebooks are each workspace's that it sees, the
// workspaces by slug, each notebook with the role ListNotebooks gives it
// there: the access reaches an admin and a member, not a guest (M9/P1
// design 3.17.2).
func TestListAccountNotebooksListsEachWorkspaces(t *testing.T) {
	zeta := app.AccountWorkspace{ID: uuid.NewV7(), Slug: "zeta", Name: "Acme", Role: shared.WorkspaceMember}
	acme := app.AccountWorkspace{ID: uuid.NewV7(), Slug: "acme", Name: "Zeta", Role: shared.WorkspaceGuest}
	open := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: zeta.ID, Name: "Open", Access: shared.AccessEditor}
	private := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: zeta.ID, Name: "Private", Access: shared.AccessNone}
	guestOf := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: acme.ID, Name: "Handbook", Access: shared.AccessEditor}
	l := &listers{
		notebooks: map[uuid.UUID][]app.Listed{
			zeta.ID: {{Notebook: open, Explicit: shared.NotebookReader}, {Notebook: private, Explicit: shared.NotebookAdmin}},
			acme.ID: {{Notebook: guestOf, Explicit: shared.NotebookReader}},
		},
		names: map[uuid.UUID]string{zeta.ID: "zeta", acme.ID: "acme"},
	}
	got, err := app.NewListAccountNotebooks(accountWorkspaces{zeta, acme}, l).Execute(as(uuid.NewV7()))
	want := app.AccountNotebooks{Notebooks: []app.AccountNotebook{
		{Notebook: guestOf, Role: shared.NotebookReader, Workspace: acme},
		{Notebook: open, Role: shared.NotebookEditor, Workspace: zeta},
		{Notebook: private, Role: shared.NotebookAdmin, Workspace: zeta},
	}}
	if err != nil || got.Truncated || !slices.Equal(got.Notebooks, want.Notebooks) {
		t.Errorf("Execute() = %+v, %v; want %+v", got, err, want)
	}
	if want := []string{"acme", "zeta reached"}; !slices.Equal(l.calls, want) {
		t.Errorf("the lists = %q, want %q", l.calls, want)
	}
	var se *shared.Error
	if _, err := app.NewListAccountNotebooks(accountWorkspaces{zeta}, l).Execute(context.Background()); !errors.As(err, &se) || se.Code != "unauthorized" {
		t.Errorf("Execute() without an actor = %v, want unauthorized", err)
	}
}

// The list stops past MaxAccountNotebooks: 1,000 notebooks and truncated,
// the workspaces after it unread; exactly 1,000 are not truncated.
func TestListAccountNotebooksStopsAtItsBound(t *testing.T) {
	var spaces accountWorkspaces
	l := &listers{notebooks: map[uuid.UUID][]app.Listed{}, names: map[uuid.UUID]string{}}
	for i, n := range []int{600, 400, 1, 5} {
		ws := app.AccountWorkspace{ID: uuid.NewV7(), Slug: fmt.Sprintf("w%d", i), Role: shared.WorkspaceAdmin}
		spaces = append(spaces, ws)
		l.names[ws.ID] = ws.Slug
		for j := range n {
			l.notebooks[ws.ID] = append(l.notebooks[ws.ID], app.Listed{Notebook: domain.Notebook{ID: uuid.NewV7(), Name: fmt.Sprint(j)}})
		}
	}
	got, err := app.NewListAccountNotebooks(spaces, l).Execute(as(uuid.NewV7()))
	if err != nil || len(got.Notebooks) != app.MaxAccountNotebooks || !got.Truncated {
		t.Errorf("1,006 notebooks: %d, truncated %v, %v; want 1,000 truncated", len(got.Notebooks), got.Truncated, err)
	}
	if want := []string{"w0 reached", "w1 reached", "w2 reached"}; !slices.Equal(l.calls, want) {
		t.Errorf("the lists = %q, want %q: none after the bound", l.calls, want)
	}
	got, err = app.NewListAccountNotebooks(spaces[:2], l).Execute(as(uuid.NewV7()))
	if err != nil || len(got.Notebooks) != app.MaxAccountNotebooks || got.Truncated {
		t.Errorf("1,000 notebooks: %d, truncated %v, %v; want all, not truncated", len(got.Notebooks), got.Truncated, err)
	}
}
