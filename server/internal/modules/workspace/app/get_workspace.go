package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetWorkspace reads a workspace the caller is a member of:
// GET /api/v0/workspaces/{slug}.
type GetWorkspace struct {
	store Store
	auth  shared.Authorizer
}

// NewGetWorkspace returns the use case.
func NewGetWorkspace(store Store, auth shared.Authorizer) *GetWorkspace {
	return &GetWorkspace{store: store, auth: auth}
}

// Execute returns the workspace of slug, with the caller's role. A read
// takes no lock and opens no transaction (v0.1 design 8.3). A workspace
// that does not exist, is deleted, or that the caller cannot see is
// workspace.not_found alike.
func (g *GetWorkspace) Execute(ctx context.Context, slug string) (Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Membership{}, err
	}
	w, err := g.store.FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, ErrNotFound) {
		return Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	grant, err := g.auth.Authorize(ctx, actor, domain.ActionRead, shared.Target{WorkspaceID: w.ID})
	if errors.Is(err, shared.ErrNotVisible) {
		return Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	return Membership{Workspace: w, Role: grant.WorkspaceRole}, nil
}
