package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CreateNotebookDeps are what CreateNotebook needs.
type CreateNotebookDeps struct {
	Workspaces Workspaces
	Notebooks  NotebookCreator
	Auth       shared.Authorizer
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
}

// CreateNotebook creates a notebook whose admin is the caller:
// POST /api/v0/workspaces/{slug}/notebooks (M3/P1 design 3.7).
type CreateNotebook struct {
	d CreateNotebookDeps
}

// NewCreateNotebook returns the use case.
func NewCreateNotebook(d CreateNotebookDeps) *CreateNotebook {
	return &CreateNotebook{d: d}
}

// Execute creates the notebook name, of access (none when nil), in the
// workspace of slug. It shares the workspace's row, so a change of the
// workspace's memberships, its deletion too, runs before or after it;
// then decides; then checks the values: a caller who cannot see the
// workspace gets 404, not 422. The notebook and its admin's membership
// are written at one time.
func (c *CreateNotebook) Execute(ctx context.Context, slug, name string, access *string) (View, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return View{}, err
	}
	workspaceID, ok, err := c.d.Workspaces.FindBySlug(ctx, slug)
	switch {
	case err != nil:
		return View{}, err
	case !ok:
		return View{}, domain.ErrWorkspaceNotFound
	}
	var out View
	err = c.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if ok, err := c.d.Workspaces.ShareByID(ctx, workspaceID); err != nil || !ok {
			return orNotFound(err, domain.ErrWorkspaceNotFound)
		}
		if _, err := authorize(ctx, c.d.Auth, actor, domain.ActionCreate, shared.Target{WorkspaceID: workspaceID},
			domain.ErrWorkspaceNotFound); err != nil {
			return err
		}
		draft, err := domain.CheckDraft(name, access)
		if err != nil {
			return err
		}
		now := c.d.Clock.Now()
		n := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: workspaceID, Name: draft.Name, Access: draft.Access, CreatedAt: now, UpdatedAt: now}
		admin := domain.Member{ID: uuid.NewV7(), NotebookID: n.ID, UserID: actor.UserID, Role: shared.NotebookAdmin, CreatedAt: now}
		if err := c.d.Notebooks.CreateNotebook(ctx, n, actor.UserID); err != nil {
			return err
		}
		if err := c.d.Notebooks.AddMember(ctx, admin, actor.UserID); err != nil {
			return err
		}
		out = View{Notebook: n, Role: admin.Role, MemberCount: 1}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	c.d.Logger.InfoContext(ctx, "notebook created", slog.String("workspace_id", workspaceID.String()),
		slog.String("notebook_id", out.Notebook.ID.String()), slog.String("user_id", actor.UserID.String()))
	return out, nil
}
