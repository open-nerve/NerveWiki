package app_test

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The fake repository's P3 ports: the share locks, the memberships ended
// and restored, the invitations.

func (f *fakeStore) FindWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	f.record(ctx, "FindWorkspaceByID")
	for _, w := range f.workspaces {
		if w.ID == id {
			return w, nil
		}
	}
	return domain.Workspace{}, app.ErrNotFound
}

func (f *fakeStore) ShareWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	return f.lock(ctx, "ShareWorkspaceBySlug "+slug, func(w domain.Workspace) bool { return w.Slug == slug })
}

func (f *fakeStore) ShareWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	return f.lock(ctx, "ShareWorkspaceByID", func(w domain.Workspace) bool { return w.ID == id })
}

func (f *fakeStore) FindMembership(ctx context.Context, workspaceID, userID uuid.UUID) (domain.Member, error) {
	f.record(ctx, "FindMembership")
	for _, m := range f.active {
		if m.WorkspaceID == workspaceID && m.UserID == userID {
			return m, nil
		}
	}
	for _, m := range f.ended {
		if m.WorkspaceID == workspaceID && m.UserID == userID {
			if m.EndedAt == nil { // put there by the test, not by EndMemberships
				ended := now().Add(-time.Hour)
				m.EndedAt = &ended
			}
			return m, nil
		}
	}
	return domain.Member{}, app.ErrNotFound
}

func (f *fakeStore) RestoreMember(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	f.record(ctx, "RestoreMember "+string(role)+" by "+by.String()+" at "+now.Format(time.RFC3339Nano))
	m := f.ended[id]
	delete(f.ended, id)
	m.Role, m.EndedAt = role, nil
	f.active[id] = m
	return nil
}

func (f *fakeStore) FindPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error) {
	f.record(ctx, "FindPendingInvitation")
	inv, ok := f.invitations[id]
	if !ok {
		return domain.Invitation{}, app.ErrNotFound
	}
	return inv, nil
}

func (f *fakeStore) ListPendingInvitations(ctx context.Context, workspaceID uuid.UUID) ([]domain.Invitation, error) {
	f.record(ctx, "ListPendingInvitations")
	var list []domain.Invitation
	for inv := range maps.Values(f.invitations) {
		if inv.WorkspaceID == workspaceID {
			list = append(list, inv)
		}
	}
	slices.SortFunc(list, func(a, b domain.Invitation) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID.String(), a.ID.String()))
	})
	return list, nil
}

func (f *fakeStore) LockPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error) {
	f.record(ctx, "LockPendingInvitation")
	inv, ok := f.invitations[id]
	if !ok {
		return domain.Invitation{}, app.ErrNotFound
	}
	return inv, nil
}

func (f *fakeStore) CreateInvitation(ctx context.Context, inv domain.Invitation, by uuid.UUID) error {
	f.record(ctx, "CreateInvitation "+inv.Email+" "+string(inv.Role)+" by "+by.String()+" at "+inv.CreatedAt.Format(time.RFC3339Nano))
	if f.createInvitationErr != nil {
		return f.createInvitationErr
	}
	f.invitations[inv.ID] = inv
	return nil
}

func (f *fakeStore) DeleteInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteInvitation by "+by.String()+" at "+now.Format(time.RFC3339Nano))
	delete(f.invitations, id)
	return nil
}

func (f *fakeStore) AcceptInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	f.record(ctx, "AcceptInvitation by "+by.String()+" at "+now.Format(time.RFC3339Nano))
	delete(f.invitations, id)
	return nil
}

func (f *fakeStore) DeleteInvitationsTo(ctx context.Context, workspaceIDs []uuid.UUID, email string, by uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteInvitationsTo "+email+" by "+by.String()+" at "+now.Format(time.RFC3339Nano))
	maps.DeleteFunc(f.invitations, func(_ uuid.UUID, inv domain.Invitation) bool {
		return inv.Email == email && slices.Contains(workspaceIDs, inv.WorkspaceID)
	})
	return nil
}

func (f *fakeStore) DeleteInvitationsOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteInvitationsOf by "+by.String()+" at "+now.Format(time.RFC3339Nano))
	maps.DeleteFunc(f.invitations, func(_ uuid.UUID, inv domain.Invitation) bool { return inv.WorkspaceID == workspaceID })
	return nil
}

// fakeTokens's token of an invitation is "token of" its id; it records each
// check in the store's calls, so a test sees it come before every read.
type fakeTokens struct{ store *fakeStore }

func (fakeTokens) Token(id uuid.UUID) string { return "token of " + id.String() }

func (f fakeTokens) Valid(id uuid.UUID, token string) bool {
	f.store.calls = append(f.store.calls, "Valid")
	return token == f.Token(id)
}

// fakeAccountFinder finds the accounts of ids by their address, and records
// the lookups in the store's calls.
type fakeAccountFinder struct {
	store *fakeStore
	ids   map[string]uuid.UUID
}

func (f fakeAccountFinder) AccountIDByEmail(ctx context.Context, email string) (uuid.UUID, bool, error) {
	f.store.record(ctx, "AccountIDByEmail "+email)
	id, ok := f.ids[email]
	return id, ok, nil
}
