package postgresadapter

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
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

// CaptureBaseline writes the upgrade's baseline of the notebook, a row for
// each node alive, and answers its nodes and pages (M11 design 4.4 step
// 11).
func (s *Store) CaptureBaseline(ctx context.Context, upgradeID, notebookID uuid.UUID) (app.Baseline, error) {
	r, err := s.queries(ctx).CaptureBaseline(ctx, gen.CaptureBaselineParams{UpgradeID: upgradeID, NotebookID: notebookID})
	if err != nil {
		return app.Baseline{}, fmt.Errorf("capture an upgrade's baseline: %w", err)
	}
	return app.Baseline{Nodes: int(r.Nodes), Pages: int(r.Pages)}, nil
}

// ProtectSourceRoot keeps the source root rootID the upgrade's start
// created from a delete: a row not of the baseline (M11 design 4.4 step
// 12).
func (s *Store) ProtectSourceRoot(ctx context.Context, upgradeID, notebookID, rootID uuid.UUID) error {
	err := s.queries(ctx).ProtectSourceRoot(ctx, gen.ProtectSourceRootParams{UpgradeID: upgradeID, NodeID: rootID, NotebookID: notebookID})
	if err != nil {
		return fmt.Errorf("protect an upgrade's source root: %w", err)
	}
	return nil
}

// SetBaselineWanted sets the wanted count of each baseline page of ids to
// wanted's, both in one order (M11 design 4.4 step 13); an id not of a
// baseline page changes no row.
func (s *Store) SetBaselineWanted(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID, wanted []int32) error {
	err := s.queries(ctx).SetBaselineWanted(ctx, gen.SetBaselineWantedParams{UpgradeID: upgradeID, NodeIds: ids, Wanted: wanted})
	if err != nil {
		return fmt.Errorf("set an upgrade's baseline wanted: %w", err)
	}
	return nil
}

// AnalyzeUpgradeNodes analyzes the upgrades' rows (M11 design 4.4 step
// 16).
func (s *Store) AnalyzeUpgradeNodes(ctx context.Context) error {
	if err := s.queries(ctx).AnalyzeUpgradeNodes(ctx); err != nil {
		return fmt.Errorf("analyze the upgrades' rows: %w", err)
	}
	return nil
}

// RootNodesByKey are the nodes alive at the notebook's root whose title
// keys are among keys (M11 design 4.4 step 8).
func (s *Store) RootNodesByKey(ctx context.Context, notebookID uuid.UUID, keys []string) ([]app.KeyedRootNode, error) {
	rows, err := s.queries(ctx).RootNodesByKey(ctx, gen.RootNodesByKeyParams{NotebookID: notebookID, Keys: keys})
	if err != nil {
		return nil, fmt.Errorf("read the notebook's root by key: %w", err)
	}
	out := make([]app.KeyedRootNode, len(rows))
	for i, r := range rows {
		out[i] = app.KeyedRootNode{ID: r.ID, Kind: domain.Kind(r.Kind), Key: r.NameKey}
	}
	return out, nil
}

// DeleteExpiredSessionsOfNotebook deletes the notebook's edit sessions
// expired at at, tombstones among them, and answers how many (M11 design
// 4.4 step 7).
func (s *Store) DeleteExpiredSessionsOfNotebook(ctx context.Context, notebookID uuid.UUID, at time.Time) (int64, error) {
	n, err := s.queries(ctx).DeleteExpiredSessionsOfNotebook(ctx, gen.DeleteExpiredSessionsOfNotebookParams{NotebookID: notebookID, Now: at})
	if err != nil {
		return 0, fmt.Errorf("delete the notebook's expired edit sessions: %w", err)
	}
	return n, nil
}

// LiveSessionsOfNotebook are the first 100 sessions alive in the notebook
// at at, by page, and how many there are (M11 design 4.4 step 7).
func (s *Store) LiveSessionsOfNotebook(ctx context.Context, notebookID uuid.UUID, at time.Time) ([]app.SessionHolder, int, error) {
	rows, err := s.queries(ctx).LiveSessionsOfNotebook(ctx, gen.LiveSessionsOfNotebookParams{NotebookID: notebookID, Now: at})
	if err != nil {
		return nil, 0, fmt.Errorf("read the notebook's live edit sessions: %w", err)
	}
	if len(rows) == 0 {
		return nil, 0, nil
	}
	out := make([]app.SessionHolder, len(rows))
	for i, r := range rows {
		out[i] = app.SessionHolder{PageID: r.NodeID, UserID: r.UserID}
	}
	return out, int(rows[0].Total), nil
}
