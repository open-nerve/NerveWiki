package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpdateNotebookDeps are what UpdateNotebook needs.
type UpdateNotebookDeps struct {
	Workspaces Workspaces
	Finder     NotebookFinder
	Notebooks  NotebookWriter
	Auth       shared.Authorizer
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
}

// UpdateNotebook changes a notebook's name or workspace access:
// PATCH /api/v0/notebooks/{notebook_id} (M3/P1 design 3.7).
type UpdateNotebook struct {
	d UpdateNotebookDeps
	m manager
}

// NewUpdateNotebook returns the use case.
func NewUpdateNotebook(d UpdateNotebookDeps) *UpdateNotebook {
	return &UpdateNotebook{d: d, m: manager{workspaces: d.Workspaces, notebooks: d.Notebooks, auth: d.Auth}}
}

// Execute gives notebook id the fields that are not nil, and returns it
// with the caller's role. Under the locks and the decision it checks the
// values: a caller who cannot see the notebook gets 404, not 422. Values
// that change nothing write nothing.
func (u *UpdateNotebook) Execute(ctx context.Context, id uuid.UUID, name, access *string) (View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return View{}, err
	}
	n, err := find(ctx, u.d.Finder, id)
	if err != nil {
		return View{}, err
	}
	var (
		out      View
		modified bool
	)
	err = u.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		n, grant, err := u.m.lock(ctx, actor, domain.ActionUpdate, n)
		if err != nil {
			return err
		}
		change, err := domain.CheckChange(name, access)
		if err != nil {
			return err
		}
		n, modified = n.Apply(change)
		if modified {
			n.UpdatedAt = u.d.Clock.Now()
			if err := u.d.Notebooks.UpdateNotebook(ctx, n, actor.UserID); err != nil {
				return err
			}
		}
		count, err := u.d.Notebooks.CountMembers(ctx, n.ID)
		if err != nil {
			return err
		}
		out = View{Notebook: n, Role: grant.NotebookRole, MemberCount: count}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	if modified {
		u.d.Logger.InfoContext(ctx, "notebook updated", slog.String("workspace_id", n.WorkspaceID.String()),
			slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()))
	}
	return out, nil
}
