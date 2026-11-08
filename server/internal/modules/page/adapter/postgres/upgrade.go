package postgresadapter

import (
	"context"
	"fmt"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
)

// Protected implements app.UpgradeNodes: those of ids that have a row of
// the upgrade upgradeID, its baseline's or its protected source root's, in
// the transaction ctx carries.
func (s *Store) Protected(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := s.queries(ctx).UpgradeProtected(ctx, gen.UpgradeProtectedParams{UpgradeID: upgradeID, NodeIds: ids})
	if err != nil {
		return nil, fmt.Errorf("read an upgrade's protected nodes: %w", err)
	}
	protected := make(map[uuid.UUID]bool, len(rows))
	for _, id := range rows {
		protected[id] = true
	}
	return protected, nil
}

// DeleteNotebooksUpgradeNodes implements app.NotebookPages: the deleted
// notebooks' upgrades' rows go, in the deletion's transaction.
func (s *Store) DeleteNotebooksUpgradeNodes(ctx context.Context, ids []uuid.UUID) error {
	if err := s.queries(ctx).DeleteUpgradeNodesOfNotebooks(ctx, ids); err != nil {
		return fmt.Errorf("delete the notebooks' upgrade rows: %w", err)
	}
	return nil
}
