package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListMembersDeps are what ListMembers needs.
type ListMembersDeps struct {
	Notebooks NotebookFinder
	Members   MemberFinder
	Profiles  MemberProfiles
	Auth      shared.Authorizer
}

// ListMembers lists a notebook's members:
// GET /api/v0/notebooks/{notebook_id}/members (M3/P2 design 3.2).
type ListMembers struct {
	d ListMembersDeps
}

// NewListMembers returns the use case.
func NewListMembers(d ListMembersDeps) *ListMembers {
	return &ListMembers{d: d}
}

// Execute returns the active members of notebook id, by when they joined,
// with their profiles: the emails to the workspace's admins and members.
// Any role in the notebook lists them; a notebook that does not exist, is
// deleted, or that the caller has no role in is notebook.not_found. A read
// takes no lock and opens no transaction.
func (l *ListMembers) Execute(ctx context.Context, id uuid.UUID) ([]ListedMember, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	n, err := find(ctx, l.d.Notebooks, id)
	if err != nil {
		return nil, err
	}
	grant, err := authorize(ctx, l.d.Auth, actor, domain.ActionListMembers, shared.Target{WorkspaceID: n.WorkspaceID, NotebookID: n.ID},
		domain.ErrNotFound)
	if err != nil {
		return nil, err
	}
	members, err := l.d.Members.ListMembers(ctx, n.ID)
	if err != nil {
		return nil, err
	}
	return withProfiles(ctx, l.d.Profiles, members, seesEmails(grant.WorkspaceRole))
}
