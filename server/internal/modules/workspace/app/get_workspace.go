package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetWorkspace reads a workspace the caller is a member of:
// GET /api/v0/workspaces/{slug}.
type GetWorkspace struct {
	workspaces WorkspaceFinder
	auth       shared.Authorizer
}

// NewGetWorkspace returns the use case.
func NewGetWorkspace(workspaces WorkspaceFinder, auth shared.Authorizer) *GetWorkspace {
	return &GetWorkspace{workspaces: workspaces, auth: auth}
}

// Execute returns the workspace of slug, with the caller's role. A read
// takes no lock and opens no transaction (v0.1 design 8.3). A workspace
// that does not exist, is deleted, or that the caller cannot see is
// workspace.not_found alike, and so is a slug spelled as none is.
func (g *GetWorkspace) Execute(ctx context.Context, slug string) (Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Membership{}, err
	}
	if !domain.ValidSlug(slug) {
		return Membership{}, domain.ErrNotFound
	}
	w, err := g.workspaces.FindWorkspaceBySlug(ctx, slug)
	if err != nil {
		return Membership{}, found(err, domain.ErrNotFound)
	}
	grant, err := authorize(ctx, g.auth, actor, domain.ActionRead, w.ID, domain.ErrNotFound)
	if err != nil {
		return Membership{}, err
	}
	return Membership{Workspace: w, Role: grant.WorkspaceRole}, nil
}
