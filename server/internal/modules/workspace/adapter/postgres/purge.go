package postgresadapter

import (
	"context"
	"fmt"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres/gen"
)

// The purge's statements (M2/P4 design 3.4): each deletes up to batch rows
// deleted before before, skipping those another transaction holds, and
// returns how many it deleted.

// PurgeInvitations purges the invitations.
func (s *Store) PurgeInvitations(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeInvitations(ctx, gen.PurgeInvitationsParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge invitations: %w", err)
	}
	return int(n), nil
}

// PurgeMembers purges the members, those of the deleted workspaces.
func (s *Store) PurgeMembers(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeMembers(ctx, gen.PurgeMembersParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge members: %w", err)
	}
	return int(n), nil
}

// PurgeWorkspaces purges the workspaces.
func (s *Store) PurgeWorkspaces(ctx context.Context, before time.Time, batch int) (int, error) {
	n, err := s.queries(ctx).PurgeWorkspaces(ctx, gen.PurgeWorkspacesParams{Before: before, Batch: int32(batch)})
	if err != nil {
		return 0, fmt.Errorf("purge workspaces: %w", err)
	}
	return int(n), nil
}
