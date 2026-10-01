package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The member operations' reads and writes (M3/P2 design 3.7).

// ListMembers implements app.MemberFinder.
func (s *Store) ListMembers(ctx context.Context, notebookID uuid.UUID) ([]domain.Member, error) {
	rows, err := s.queries(ctx).ListMembers(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list notebook members: %w", err)
	}
	members := make([]domain.Member, len(rows))
	for i, r := range rows {
		members[i] = memberOf(gen.FindActiveMemberRow(r))
	}
	return members, nil
}

// FindActiveMember implements app.MemberFinder.
func (s *Store) FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error) {
	row, err := s.queries(ctx).FindActiveMember(ctx, id)
	if err != nil {
		return domain.Member{}, notFound("find notebook member", err)
	}
	return memberOf(row), nil
}

// FindMemberOf implements app.MemberWriter.
func (s *Store) FindMemberOf(ctx context.Context, notebookID, userID uuid.UUID) (domain.Member, error) {
	row, err := s.queries(ctx).FindMemberOf(ctx, gen.FindMemberOfParams{NotebookID: notebookID, UserID: userID})
	if err != nil {
		return domain.Member{}, notFound("find notebook member of", err)
	}
	return memberOf(gen.FindActiveMemberRow(row)), nil
}

// memberOf is a row of the queries that read a membership's columns: they
// read the same, so their rows convert to this one.
func memberOf(r gen.FindActiveMemberRow) domain.Member {
	return domain.Member{
		ID: r.ID, NotebookID: r.NotebookID, UserID: r.UserID, Role: shared.NotebookRole(r.Role),
		CreatedAt: r.CreatedAt, EndedAt: r.EndedAt,
	}
}

// CountAdmins implements app.MemberWriter.
func (s *Store) CountAdmins(ctx context.Context, notebookID uuid.UUID) (int, error) {
	n, err := s.queries(ctx).CountAdmins(ctx, notebookID)
	if err != nil {
		return 0, fmt.Errorf("count notebook admins: %w", err)
	}
	return int(n), nil
}

// UpdateMemberRole implements app.MemberWriter.
func (s *Store) UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).UpdateMemberRole(ctx, gen.UpdateMemberRoleParams{ID: id, Role: string(role), By: by, Now: now}); err != nil {
		return fmt.Errorf("update notebook member role: %w", err)
	}
	return nil
}

// EndMember implements app.MemberWriter.
func (s *Store) EndMember(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).EndMember(ctx, gen.EndMemberParams{ID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("end notebook member: %w", err)
	}
	return nil
}

// RestoreMember implements app.MemberWriter.
func (s *Store) RestoreMember(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).RestoreMember(ctx, gen.RestoreMemberParams{ID: id, Role: string(role), By: by, Now: now}); err != nil {
		return fmt.Errorf("restore notebook member: %w", err)
	}
	return nil
}
