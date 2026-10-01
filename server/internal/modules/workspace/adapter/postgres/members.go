package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The memberships' half of the Store: the access module's fact, the member
// use cases' reads and writes, and rule two's view (M2/P4 design 3.1).

// memberOf is a row of the queries that read a member's columns.
func memberOf(r gen.FindActiveMemberRow) domain.Member {
	return domain.Member{ID: r.ID, WorkspaceID: r.WorkspaceID, UserID: r.UserID, Role: shared.WorkspaceRole(r.Role), CreatedAt: r.CreatedAt}
}

// AddMember implements app.WorkspaceCreator.
func (s *Store) AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error {
	err := s.queries(ctx).AddMember(ctx, gen.AddMemberParams{
		ID: m.ID, WorkspaceID: m.WorkspaceID, UserID: m.UserID, Role: string(m.Role), By: by, CreatedAt: m.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// RoleOf returns userID's role in workspaceID, and whether userID is an
// active member of it, in the transaction ctx carries: the access module's
// fact of the workspace level.
func (s *Store) RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	role, err := s.queries(ctx).RoleOf(ctx, gen.RoleOfParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("role of: %w", err)
	}
	return shared.WorkspaceRole(role), true, nil
}

// FindActiveMember implements app.MemberFinder.
func (s *Store) FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error) {
	row, err := s.queries(ctx).FindActiveMember(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("find member: %w", err)
	}
	return memberOf(row), nil
}

// FindMembership implements app.MemberFinder: ended memberships too,
// deleted ones not.
func (s *Store) FindMembership(ctx context.Context, workspaceID, userID uuid.UUID) (domain.Member, error) {
	row, err := s.queries(ctx).FindMembership(ctx, gen.FindMembershipParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Member{}, fmt.Errorf("find membership: %w", err)
	}
	return domain.Member{ID: row.ID, WorkspaceID: row.WorkspaceID, UserID: row.UserID, Role: shared.WorkspaceRole(row.Role),
		CreatedAt: row.CreatedAt, EndedAt: row.EndedAt}, nil
}

// ListActiveMembers implements app.MemberFinder.
func (s *Store) ListActiveMembers(ctx context.Context, workspaceID uuid.UUID) ([]domain.Member, error) {
	rows, err := s.queries(ctx).ListActiveMembers(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	list := make([]domain.Member, len(rows))
	for i, r := range rows {
		list[i] = memberOf(gen.FindActiveMemberRow(r))
	}
	return list, nil
}

// CountActiveAdmins implements app.MemberFinder.
func (s *Store) CountActiveAdmins(ctx context.Context, workspaceID uuid.UUID) (int, error) {
	n, err := s.queries(ctx).CountActiveAdmins(ctx, workspaceID)
	if err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return int(n), nil
}

// UpdateMemberRole implements app.MemberUpdater.
func (s *Store) UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).UpdateMemberRole(ctx, gen.UpdateMemberRoleParams{ID: id, Role: string(role), By: by, Now: now})
	if err != nil {
		return fmt.Errorf("update member role: %w", err)
	}
	return nil
}

// RestoreMember makes the ended membership id active again with role,
// updated by by at now; when it was first created stays.
func (s *Store) RestoreMember(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).RestoreMember(ctx, gen.RestoreMemberParams{ID: id, Role: string(role), By: by, Now: now})
	if err != nil {
		return fmt.Errorf("restore member: %w", err)
	}
	return nil
}

// EndMemberships implements app.MemberUpdater.
func (s *Store) EndMemberships(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).EndMemberships(ctx, gen.EndMembershipsParams{UserID: userID, WorkspaceIds: workspaceIDs, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("end memberships: %w", err)
	}
	return nil
}

// DeleteMembersOf implements app.MemberUpdater.
func (s *Store) DeleteMembersOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).DeleteMembersOf(ctx, gen.DeleteMembersOfParams{WorkspaceID: workspaceID, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("delete members: %w", err)
	}
	return nil
}

// ListStandings implements app.StandingLister.
func (s *Store) ListStandings(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID) ([]domain.Standing, error) {
	rows, err := s.queries(ctx).ListStandings(ctx, gen.ListStandingsParams{UserID: userID, WorkspaceIds: workspaceIDs})
	if err != nil {
		return nil, fmt.Errorf("list standings: %w", err)
	}
	list := make([]domain.Standing, len(rows))
	for i, r := range rows {
		list[i] = domain.Standing{WorkspaceID: r.WorkspaceID, Slug: r.Slug, Role: shared.WorkspaceRole(r.Role),
			Admins: int(r.Admins), Members: int(r.Members)}
	}
	return list, nil
}
