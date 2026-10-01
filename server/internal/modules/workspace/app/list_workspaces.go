package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListWorkspaces lists the caller's workspaces: GET /api/v0/workspaces. It
// reads the caller's own memberships only, so no rule decides it.
type ListWorkspaces struct {
	memberships MembershipLister
}

// NewListWorkspaces returns the use case.
func NewListWorkspaces(memberships MembershipLister) *ListWorkspaces {
	return &ListWorkspaces{memberships: memberships}
}

// Execute returns the workspaces the caller is an active member of, with
// the caller's role in each, by name, case-insensitively.
func (l *ListWorkspaces) Execute(ctx context.Context) ([]Membership, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	return l.memberships.ListWorkspacesOf(ctx, actor.UserID)
}
