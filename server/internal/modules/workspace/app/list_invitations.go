package app

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ListInvitationsDeps are what ListInvitations needs.
type ListInvitationsDeps struct {
	Workspaces  WorkspaceFinder
	Invitations InvitationFinder
	Tokens      InvitationTokens
	Auth        shared.Authorizer
}

// ListInvitations lists a workspace's pending invitations:
// GET /api/v0/workspaces/{slug}/invitations (M2/P3 design 3.3).
type ListInvitations struct {
	d ListInvitationsDeps
}

// NewListInvitations returns the use case.
func NewListInvitations(d ListInvitationsDeps) *ListInvitations {
	return &ListInvitations{d: d}
}

// Execute returns the pending invitations of the workspace of slug, newest
// first, each with its token, computed again: tokens are not stored. A read
// takes no lock and opens no transaction (v0.1 design 8.3).
func (l *ListInvitations) Execute(ctx context.Context, slug string) ([]ListedInvitation, error) {
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
	if _, err := authorize(ctx, l.d.Auth, actor, domain.ActionListInvitations, w.ID, domain.ErrNotFound); err != nil {
		return nil, err
	}
	invitations, err := l.d.Invitations.ListPendingInvitations(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	list := make([]ListedInvitation, len(invitations))
	for i, inv := range invitations {
		list[i] = listed(l.d.Tokens, inv)
	}
	return list, nil
}
