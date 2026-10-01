package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListWorkspaces lists the caller's workspaces: GET /api/v0/workspaces. It
// reads the caller's own memberships only, so no rule decides it.
type ListWorkspaces struct {
	store Store
}

// NewListWorkspaces returns the use case.
func NewListWorkspaces(store Store) *ListWorkspaces {
	return &ListWorkspaces{store: store}
}

// Execute returns the workspaces the caller is an active member of, with
// the caller's role in each, by name.
func (l *ListWorkspaces) Execute(ctx context.Context) ([]Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	return l.store.ListWorkspacesOf(ctx, actor.UserID)
}
