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

// The invitations' part of the store (M2/P3).

// invitationOf is a row of the queries that read an invitation's columns.
func invitationOf(r gen.LockPendingInvitationRow) domain.Invitation {
	return domain.Invitation{ID: r.ID, WorkspaceID: r.WorkspaceID, Email: r.Email, Role: shared.WorkspaceRole(r.Role), CreatedAt: r.CreatedAt}
}

// CreateInvitation inserts inv, created by by at inv.CreatedAt:
// domain.ErrAlreadyInvited when its workspace has a pending invitation to
// its address.
func (s *Store) CreateInvitation(ctx context.Context, inv domain.Invitation, by uuid.UUID) error {
	err := s.queries(ctx).CreateInvitation(ctx, gen.CreateInvitationParams{
		ID: inv.ID, WorkspaceID: inv.WorkspaceID, Email: inv.Email, Role: string(inv.Role), By: by, CreatedAt: inv.CreatedAt,
	})
	switch {
	case uniqueViolation(err, "workspace_invitations_workspace_id_email_key"):
		return domain.ErrAlreadyInvited
	case err != nil:
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

// ListPendingInvitations returns workspaceID's pending invitations, newest
// first.
func (s *Store) ListPendingInvitations(ctx context.Context, workspaceID uuid.UUID) ([]domain.Invitation, error) {
	rows, err := s.queries(ctx).ListPendingInvitations(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	list := make([]domain.Invitation, len(rows))
	for i, r := range rows {
		list[i] = invitationOf(gen.LockPendingInvitationRow(r))
	}
	return list, nil
}

// FindPendingInvitation returns the pending invitation id of a workspace
// not deleted, unlocked; app.ErrNotFound when there is none.
func (s *Store) FindPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error) {
	row, err := s.queries(ctx).FindPendingInvitation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invitation{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("find invitation: %w", err)
	}
	return invitationOf(gen.LockPendingInvitationRow(row)), nil
}

// LockPendingInvitation returns the pending invitation id, locked FOR
// UPDATE until the transaction ends, which holds its workspace's lock;
// app.ErrNotFound when there is none, an acceptance or a deletion committed
// while it waited too.
func (s *Store) LockPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error) {
	row, err := s.queries(ctx).LockPendingInvitation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invitation{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Invitation{}, fmt.Errorf("lock invitation: %w", err)
	}
	return invitationOf(row), nil
}

// DeleteInvitation deletes the invitation id softly at now, by by.
func (s *Store) DeleteInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).DeleteInvitation(ctx, gen.DeleteInvitationParams{ID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}
	return nil
}

// AcceptInvitation marks the invitation id accepted, and deleted, at now,
// by by.
func (s *Store) AcceptInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error {
	if err := s.queries(ctx).AcceptInvitation(ctx, gen.AcceptInvitationParams{ID: id, By: by, Now: now}); err != nil {
		return fmt.Errorf("accept invitation: %w", err)
	}
	return nil
}

// DeleteInvitationsTo deletes the pending invitations of workspaceIDs to
// email softly at now, by by.
func (s *Store) DeleteInvitationsTo(ctx context.Context, workspaceIDs []uuid.UUID, email string, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).DeleteInvitationsTo(ctx, gen.DeleteInvitationsToParams{WorkspaceIds: workspaceIDs, Email: email, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("delete invitations to: %w", err)
	}
	return nil
}

// DeleteInvitationsOf deletes every pending invitation of workspaceID softly
// at now, by by.
func (s *Store) DeleteInvitationsOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error {
	err := s.queries(ctx).DeleteInvitationsOf(ctx, gen.DeleteInvitationsOfParams{WorkspaceID: workspaceID, By: by, Now: now})
	if err != nil {
		return fmt.Errorf("delete invitations of: %w", err)
	}
	return nil
}
