package app_test

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The fake repository's P4 ports: a deactivation's lock and rule two's
// view, read from workspaces and active.

func (f *fakeStore) workspaceByID(id uuid.UUID) (domain.Workspace, bool) {
	for _, w := range f.workspaces {
		if w.ID == id {
			return w, true
		}
	}
	return domain.Workspace{}, false
}

func (f *fakeStore) LockWorkspacesOf(ctx context.Context, userID uuid.UUID) ([]domain.Workspace, error) {
	f.record(ctx, "LockWorkspacesOf")
	var locked []domain.Workspace
	for _, m := range f.active {
		if w, ok := f.workspaceByID(m.WorkspaceID); ok && m.UserID == userID {
			locked = append(locked, w)
		}
	}
	if f.onLock != nil {
		f.onLock()
	}
	slices.SortFunc(locked, func(a, b domain.Workspace) int { return a.ID.Compare(b.ID) })
	return locked, nil
}

func (f *fakeStore) ListStandings(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID) ([]domain.Standing, error) {
	f.record(ctx, "ListStandings")
	var standings []domain.Standing
	for _, id := range workspaceIDs {
		w, _ := f.workspaceByID(id)
		s := domain.Standing{WorkspaceID: id, Slug: w.Slug}
		mine := false
		for _, m := range f.active {
			if m.WorkspaceID != id {
				continue
			}
			s.Members++
			if m.Role == shared.WorkspaceAdmin {
				s.Admins++
			}
			if m.UserID == userID {
				s.Role, mine = m.Role, true
			}
		}
		if mine {
			standings = append(standings, s)
		}
	}
	return standings, nil
}
