package app

import (
	"context"
	"slices"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxAccountNotebooks bounds the notebooks ListAccountNotebooks reads: the
// tool's answer holds fewer, and the use case need not hold more of an
// account that sees many (M9/P1 design 3.17.2).
const MaxAccountNotebooks = 1000

// ListAccountNotebooks lists the notebooks the caller sees in each
// workspace it is a member of (M9 design 4.6): by the rule ListNotebooks
// filters by, unlocked. Only the caller's own memberships and notebooks are
// read, so no rule of the table decides it (v0.1 design 13.1 item 3).
type ListAccountNotebooks struct {
	workspaces AccountWorkspaces
	notebooks  NotebookLister
}

// NewListAccountNotebooks returns the use case.
func NewListAccountNotebooks(workspaces AccountWorkspaces, notebooks NotebookLister) *ListAccountNotebooks {
	return &ListAccountNotebooks{workspaces: workspaces, notebooks: notebooks}
}

// AccountNotebooks are the caller's notebooks, at most
// MaxAccountNotebooks: Truncated when more are there.
type AccountNotebooks struct {
	Notebooks []AccountNotebook
	Truncated bool
}

// AccountNotebook is a notebook the caller sees, its role in it, and its
// workspace.
type AccountNotebook struct {
	Notebook  domain.Notebook
	Role      shared.NotebookRole
	Workspace AccountWorkspace
}

// Execute lists the caller's workspaces, by slug, and in each the
// notebooks it sees there, by name, each with its role there, as
// ListNotebooks lists and decides them (shared.ReachedByAccess,
// shared.EffectiveNotebookRole); it stops past MaxAccountNotebooks, the
// workspaces left unread.
func (l *ListAccountNotebooks) Execute(ctx context.Context) (AccountNotebooks, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return AccountNotebooks{}, err
	}
	workspaces, err := l.workspaces.WorkspacesOf(ctx, actor.UserID)
	if err != nil {
		return AccountNotebooks{}, err
	}
	slices.SortFunc(workspaces, func(a, b AccountWorkspace) int { return strings.Compare(a.Slug, b.Slug) })
	var out AccountNotebooks
	for _, ws := range workspaces {
		listed, err := l.notebooks.ListNotebooks(ctx, ws.ID, actor.UserID, shared.ReachedByAccess(ws.Role))
		if err != nil {
			return AccountNotebooks{}, err
		}
		for _, n := range listed {
			if len(out.Notebooks) == MaxAccountNotebooks {
				out.Truncated = true
				return out, nil
			}
			out.Notebooks = append(out.Notebooks, AccountNotebook{
				Notebook: n.Notebook, Role: shared.EffectiveNotebookRole(n.Explicit, n.Notebook.Access, ws.Role), Workspace: ws,
			})
		}
	}
	return out, nil
}
