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

// The fake repository's P2 ports: locks, workspace writes and memberships.

func (f *fakeStore) lock(ctx context.Context, call string, match func(domain.Workspace) bool) (domain.Workspace, error) {
	f.record(ctx, call)
	if f.onLock != nil {
		f.onLock()
	}
	for _, w := range f.workspaces {
		if match(w) {
			return w, nil
		}
	}
	return domain.Workspace{}, app.ErrNotFound
}

func (f *fakeStore) LockWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error) {
	return f.lock(ctx, "LockWorkspaceBySlug "+slug, func(w domain.Workspace) bool { return w.Slug == slug })
}

func (f *fakeStore) LockWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error) {
	return f.lock(ctx, "LockWorkspaceByID", func(w domain.Workspace) bool { return w.ID == id })
}

func (f *fakeStore) RenameWorkspace(ctx context.Context, _ uuid.UUID, name string, by uuid.UUID, now time.Time) error {
	f.record(ctx, "RenameWorkspace "+name+" by "+by.String()+" at "+now.Format(time.RFC3339))
	return nil
}

func (f *fakeStore) DeleteWorkspace(ctx context.Context, _, by uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteWorkspace by "+by.String()+" at "+now.Format(time.RFC3339))
	return nil
}

func (f *fakeStore) FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error) {
	f.record(ctx, "FindActiveMember")
	m, ok := f.active[id]
	if !ok {
		return domain.Member{}, app.ErrNotFound
	}
	return m, nil
}

func (f *fakeStore) ListActiveMembers(ctx context.Context, workspaceID uuid.UUID) ([]domain.Member, error) {
	f.record(ctx, "ListActiveMembers")
	var list []domain.Member
	for _, m := range f.active {
		if m.WorkspaceID == workspaceID {
			list = append(list, m)
		}
	}
	slices.SortFunc(list, func(a, b domain.Member) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID.String(), b.ID.String()))
	})
	return list, nil
}

func (f *fakeStore) CountActiveAdmins(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	f.record(ctx, "CountActiveAdmins")
	n := 0
	for m := range maps.Values(f.active) {
		if m.WorkspaceID == workspaceID && m.Role == shared.WorkspaceAdmin {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	f.record(ctx, "UpdateMemberRole "+string(role)+" by "+by.String()+" at "+now.Format(time.RFC3339))
	m := f.active[id]
	m.Role = role
	f.active[id] = m
	return nil
}

func (f *fakeStore) EndMemberships(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID, by uuid.UUID, now time.Time) error {
	f.record(ctx, "EndMemberships by "+by.String()+" at "+now.Format(time.RFC3339))
	maps.DeleteFunc(f.active, func(_ uuid.UUID, m domain.Member) bool {
		return m.UserID == userID && slices.Contains(workspaceIDs, m.WorkspaceID)
	})
	return nil
}

func (f *fakeStore) DeleteMembersOf(ctx context.Context, _, by uuid.UUID, now time.Time) error {
	f.record(ctx, "DeleteMembersOf by "+by.String()+" at "+now.Format(time.RFC3339))
	return nil
}

// fakeProfiles answers from profiles, and records the accounts asked for.
type fakeProfiles struct {
	profiles map[uuid.UUID]app.Profile
	asked    [][]uuid.UUID
}

func (f *fakeProfiles) MemberProfiles(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]app.Profile, error) {
	f.asked = append(f.asked, ids)
	found := map[uuid.UUID]app.Profile{}
	for _, id := range ids {
		if p, ok := f.profiles[id]; ok {
			found[id] = p
		}
	}
	return found, nil
}

// fakeVetoer refuses with err, records what it saw in the store's calls
// and in seen.
type fakeVetoer struct {
	store *fakeStore
	err   error
	seen  []app.MembershipEnd
}

func (f *fakeVetoer) VetoMembershipEnd(ctx context.Context, e app.MembershipEnd) error {
	f.store.record(ctx, "VetoMembershipEnd")
	f.seen = append(f.seen, e)
	return f.err
}

// fakeSubscriber fails with err; it follows a membership end and a
// deletion, recording each in the store's calls and in its own lists.
type fakeSubscriber struct {
	store   *fakeStore
	err     error
	ended   []app.MembershipEnd
	deleted []app.WorkspaceDeletion
}

func (f *fakeSubscriber) MembershipEnded(ctx context.Context, e app.MembershipEnd) error {
	f.store.record(ctx, "MembershipEnded")
	f.ended = append(f.ended, e)
	return f.err
}

func (f *fakeSubscriber) WorkspaceDeleted(ctx context.Context, d app.WorkspaceDeletion) error {
	f.store.record(ctx, "WorkspaceDeleted")
	f.deleted = append(f.deleted, d)
	return f.err
}
