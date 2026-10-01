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

// The workspace membership end's and restore's reads and writes (M3/P3
// design 3.2).

// LockHoldings implements app.Holdings.
func (s *Store) LockHoldings(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID) ([]domain.Holding, error) {
	rows, err := s.queries(ctx).LockHoldings(ctx, gen.LockHoldingsParams{UserID: userID, WorkspaceIds: workspaceIDs})
	if err != nil {
		return nil, fmt.Errorf("lock the notebooks of a member: %w", err)
	}
	holdings := make([]domain.Holding, len(rows))
	for i, r := range rows {
		holdings[i] = domain.Holding{
			NotebookID: r.ID, WorkspaceID: r.WorkspaceID, Role: shared.NotebookRole(r.Role), Admins: int(r.Admins), Members: int(r.Members),
		}
	}
	return holdings, nil
}

// EndMembershipsOf implements app.Holdings.
func (s *Store) EndMembershipsOf(ctx context.Context, userID uuid.UUID, notebookIDs []uuid.UUID, by uuid.UUID, at time.Time) error {
	err := s.queries(ctx).EndMembershipsOf(ctx, gen.EndMembershipsOfParams{UserID: userID, NotebookIds: notebookIDs, By: by, Now: at})
	if err != nil {
		return fmt.Errorf("end the notebook memberships of a member: %w", err)
	}
	return nil
}

// SetOwnerless implements app.Holdings.
func (s *Store) SetOwnerless(ctx context.Context, notebookIDs []uuid.UUID, formerOwner uuid.UUID, at time.Time) error {
	if err := s.queries(ctx).SetOwnerless(ctx, gen.SetOwnerlessParams{Ids: notebookIDs, FormerOwnerID: &formerOwner, Since: at}); err != nil {
		return fmt.Errorf("set notebooks ownerless: %w", err)
	}
	return nil
}

// LockOwnerlessOf implements app.Returner.
func (s *Store) LockOwnerlessOf(ctx context.Context, workspaceID, userID uuid.UUID) ([]domain.Notebook, error) {
	rows, err := s.queries(ctx).LockOwnerlessOf(ctx, gen.LockOwnerlessOfParams{WorkspaceID: workspaceID, FormerOwnerID: &userID})
	if err != nil {
		return nil, fmt.Errorf("lock the ownerless notebooks of a former owner: %w", err)
	}
	notebooks := make([]domain.Notebook, len(rows))
	for i, r := range rows {
		notebooks[i] = notebookOf(gen.FindNotebookRow(r))
	}
	return notebooks, nil
}

// ReturnNotebooks implements app.Returner. Each notebook has the former
// owner's ended membership: it became ownerless when that ended, and the
// owner could not come back to it outside the workspace.
func (s *Store) ReturnNotebooks(ctx context.Context, notebookIDs []uuid.UUID, userID, by uuid.UUID, at time.Time) error {
	q := s.queries(ctx)
	n, err := q.RestoreAdmins(ctx, gen.RestoreAdminsParams{UserID: userID, NotebookIds: notebookIDs, By: by, Now: at})
	switch {
	case err != nil:
		return fmt.Errorf("restore the former owner's memberships: %w", err)
	case int(n) != len(notebookIDs):
		return fmt.Errorf("restore the former owner's memberships: %d of %d notebooks have one", n, len(notebookIDs))
	}
	if err := q.ClearOwnerless(ctx, notebookIDs); err != nil {
		return fmt.Errorf("clear notebooks ownerless: %w", err)
	}
	return nil
}
