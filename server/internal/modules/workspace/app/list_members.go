package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListMembersDeps are what ListMembers needs.
type ListMembersDeps struct {
	Workspaces WorkspaceFinder
	Members    MemberFinder
	Profiles   MemberProfiles
	Auth       shared.Authorizer
}

// ListMembers lists a workspace's members:
// GET /api/v0/workspaces/{slug}/members (M2/P2 design 3.6).
type ListMembers struct {
	d ListMembersDeps
}

// NewListMembers returns the use case.
func NewListMembers(d ListMembersDeps) *ListMembers {
	return &ListMembers{d: d}
}

// Execute returns the active members of the workspace of slug, by when
// they joined, each with the account's profile. A read takes no lock and
// opens no transaction (v0.1 design 8.3).
func (l *ListMembers) Execute(ctx context.Context, slug string) ([]ListedMember, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if !domain.ValidSlug(slug) {
		return nil, domain.ErrNotFound
	}
	w, err := l.d.Workspaces.FindWorkspaceBySlug(ctx, slug)
	if err != nil {
		return nil, found(err, domain.ErrNotFound)
	}
	grant, err := authorize(ctx, l.d.Auth, actor, domain.ActionListMembers, w.ID, domain.ErrNotFound)
	if err != nil {
		return nil, err
	}
	members, err := l.d.Members.ListActiveMembers(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	return withProfiles(ctx, l.d.Profiles, members, grant.WorkspaceRole != shared.WorkspaceGuest)
}
