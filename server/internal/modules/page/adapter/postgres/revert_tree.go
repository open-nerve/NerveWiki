package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// ListRevertTree implements app.RevertTrees without changing the public node read.
func (s *Store) ListRevertTree(ctx context.Context, notebookID uuid.UUID) ([]domain.RevertTreeNode, error) {
	rows, err := s.queries(ctx).ListRevertTree(ctx, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list revert tree: %w", err)
	}
	out := make([]domain.RevertTreeNode, len(rows))
	for i, n := range rows {
		out[i] = domain.RevertTreeNode{ID: n.ID, ParentID: n.ParentID, Kind: domain.Kind(n.Kind),
			Name: n.Name, NameKey: n.NameKey, SortOrder: n.SortOrder}
	}
	return out, nil
}
