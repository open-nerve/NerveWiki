package app

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// CreateNotebookDeps are what CreateNotebook needs: the seeders are the
// creation's registrants (Seeder), which the module's New refuses to be
// none.
type CreateNotebookDeps struct {
	Workspaces  Workspaces
	Notebooks   NotebookCreator
	Seeders     []Seeder
	Subscribers []VisibilitySubscriber
	Auth        shared.Authorizer
	Tx          shared.TxManager
	Clock       Clock
	Logger      *slog.Logger
}

// CreateNotebook creates a notebook whose admin is the caller:
// POST /api/v0/workspaces/{slug}/notebooks (M3/P1 design 3.7), an LLM Wiki
// with its template (M10 design 4.4).
type CreateNotebook struct {
	d CreateNotebookDeps
}

// NewCreateNotebook returns the use case.
func NewCreateNotebook(d CreateNotebookDeps) *CreateNotebook {
	return &CreateNotebook{d: d}
}

// NewNotebook is a creation's request: the name, and the access, mode and
// template language when given; Client is where the request came from,
// which a template's changeset records (M10 design 4.4).
type NewNotebook struct {
	Name                 string
	Access, Mode, Locale *string
	Client               string
}

// Execute creates the notebook r names in the workspace of slug, its
// caller its admin, and, for an LLM Wiki, its template, in one
// transaction. It shares the workspace's row, so a change of the
// workspace's memberships, its deletion too, runs before or after it;
// then decides; then checks the values: a caller who cannot see the
// workspace gets 404, not 422. The clock is read once: the notebook, its
// admin's membership and the template's changeset and pages have one
// time. An LLM Wiki's source root's id is decided before its row, which
// holds it from its first insert; the seeders write the template after
// the admin, whom their unit's decision reads, and before the
// visibility's subscribers are told of the caller, and of the workspace's
// admins and members when the notebook is open to them. Exactly one
// seeder must answer an LLM Wiki's source root, none a standard
// notebook's: anything else is a defect, and nothing is written.
func (c *CreateNotebook) Execute(ctx context.Context, slug string, r NewNotebook) (View, error) {
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
		draft, err := domain.CheckDraft(r.Name, r.Access, r.Mode, r.Locale)
		if err != nil {
			return err
		}
		now := c.d.Clock.Now()
		n := domain.Notebook{ID: uuid.NewV7(), WorkspaceID: workspaceID, Name: draft.Name, Access: draft.Access, CreatedAt: now, UpdatedAt: now,
			Mode: draft.Mode}
		if n.Mode == domain.ModeLLMWiki {
			n.SourceRootID = uuid.NewV7()
		}
		admin := domain.Member{ID: uuid.NewV7(), NotebookID: n.ID, UserID: actor.UserID, Role: shared.NotebookAdmin, CreatedAt: now}
		if err := c.d.Notebooks.CreateNotebook(ctx, n, actor.UserID); err != nil {
			return err
		}
		if err := c.d.Notebooks.AddMember(ctx, admin, actor.UserID); err != nil {
			return err
		}
		roots, err := seed(ctx, c.d.Seeders, NotebookCreation{NotebookID: n.ID, WorkspaceID: workspaceID, Mode: n.Mode,
			SourceRootID: n.SourceRootID, Locale: draft.Locale, By: actor.UserID, Client: r.Client, At: now})
		if err != nil {
			return err
		}
		if !seededItsRoot(n, roots) {
			return fmt.Errorf("notebook: the seeders answered source roots %v for notebook %s of mode %s; want exactly its own",
				roots, n.ID, n.Mode)
		}
		v := VisibilityChange{WorkspaceID: workspaceID, UserIDs: []uuid.UUID{actor.UserID}, Reached: n.Access != shared.AccessNone, At: now}
		if err := publishVisibility(ctx, c.d.Subscribers, v); err != nil {
			return err
		}
		out = View{Notebook: n, Role: admin.Role, MemberCount: 1}
		return nil
	})
	if err != nil {
		return View{}, err
	}
	c.d.Logger.InfoContext(ctx, "notebook created", slog.String("workspace_id", workspaceID.String()),
		slog.String("notebook_id", out.Notebook.ID.String()), slog.String("user_id", actor.UserID.String()),
		slog.String("mode", string(out.Notebook.Mode)))
	return out, nil
}

// seededItsRoot reports whether roots, the seeders' answers, are n's: its
// source root alone for an LLM Wiki, none for a standard notebook.
func seededItsRoot(n domain.Notebook, roots []uuid.UUID) bool {
	if n.Mode == domain.ModeLLMWiki {
		return len(roots) == 1 && roots[0] == n.SourceRootID
	}
	return len(roots) == 0
}
