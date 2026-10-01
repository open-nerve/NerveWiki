package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// ListOwnerless implements app.OwnerlessFinder.
func (s *Store) ListOwnerless(ctx context.Context, workspaceID uuid.UUID) ([]app.OwnerlessListed, error) {
	rows, err := s.queries(ctx).ListOwnerless(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list ownerless notebooks: %w", err)
	}
	listed := make([]app.OwnerlessListed, len(rows))
	for i, r := range rows {
		n := notebookOf(gen.FindNotebookRow{
			ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, WorkspaceAccess: r.WorkspaceAccess, CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt, OwnerlessSince: r.OwnerlessSince, FormerOwnerID: r.FormerOwnerID,
		})
		listed[i] = app.OwnerlessListed{Notebook: n, MemberCount: int(r.MemberCount)}
	}
	return listed, nil
}

// ClearOwnerless implements app.OwnerlessWriter.
func (s *Store) ClearOwnerless(ctx context.Context, notebookIDs []uuid.UUID) error {
	if err := s.queries(ctx).ClearOwnerless(ctx, notebookIDs); err != nil {
		return fmt.Errorf("clear notebooks ownerless: %w", err)
	}
	return nil
}
