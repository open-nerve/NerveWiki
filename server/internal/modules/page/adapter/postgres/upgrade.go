package postgresadapter

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

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

// BaselineContent implements app.UpgradeNodes: the content now of the
// page id when it is of the upgrade's baseline, false when it is not, in
// the transaction ctx carries (M11/P2 design 3.3).
func (s *Store) BaselineContent(ctx context.Context, upgradeID, id uuid.UUID) (string, bool, error) {
	content, err := s.queries(ctx).UpgradeBaselineContent(ctx, gen.UpgradeBaselineContentParams{UpgradeID: upgradeID, NodeID: id})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("read an upgrade's baseline page's content: %w", err)
	}
	return content, true, nil
}

// AcceptUpgradeContent implements app.UpgradeLedger: after is the hash the
// guardrails accepted of the baseline page nodeID, over before when it is
// not nil (M11/P2 design 3.5); a baseline page whose accepted hash is not
// before is a defect's error, nothing written.
func (s *Store) AcceptUpgradeContent(ctx context.Context, upgradeID, nodeID uuid.UUID, before, after []byte) error {
	r, err := s.queries(ctx).AcceptUpgradeContent(ctx, gen.AcceptUpgradeContentParams{UpgradeID: upgradeID, NodeID: nodeID,
		After: after, Before: before})
	switch {
	case err != nil:
		return fmt.Errorf("write an upgrade's ledger: %w", err)
	case r.Baseline == 1 && r.Accepted == 0:
		return fmt.Errorf("page: page %s of the upgrade's baseline holds content its guardrails never accepted", nodeID)
	}
	return nil
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

// RollbackFreeForest implements app.RollbackReads: the new nodes alive in
// the notebook that hold no baseline node of the upgrade, with their
// parents (M11/P1 design 3.18).
func (s *Store) RollbackFreeForest(ctx context.Context, upgradeID, notebookID uuid.UUID) ([]domain.FreeNode, error) {
	rows, err := s.queries(ctx).RollbackFreeForest(ctx, gen.RollbackFreeForestParams{UpgradeID: upgradeID, NotebookID: notebookID})
	if err != nil {
		return nil, fmt.Errorf("read a rollback's free nodes: %w", err)
	}
	out := make([]domain.FreeNode, len(rows))
	for i, r := range rows {
		out[i] = domain.FreeNode{ID: r.ID, ParentID: r.ParentID}
	}
	return out, nil
}

// RollbackHolders implements app.RollbackReads: the new nodes alive
// holding the upgrade's baseline nodes alive, as they are now.
func (s *Store) RollbackHolders(ctx context.Context, upgradeID uuid.UUID) ([]domain.Node, error) {
	rows, err := s.queries(ctx).RollbackHolders(ctx, upgradeID)
	if err != nil {
		return nil, fmt.Errorf("read a rollback's holders: %w", err)
	}
	out := make([]domain.Node, len(rows))
	for i, r := range rows {
		out[i] = nodeOf(gen.FindNodeRow(r))
	}
	return out, nil
}

// RollbackNow implements app.RollbackReads: the upgrade's baseline nodes
// alive and its pages' contents, as they are now.
func (s *Store) RollbackNow(ctx context.Context, upgradeID uuid.UUID) (map[uuid.UUID]domain.NodeNow, map[uuid.UUID]domain.ContentNow,
	error,
) {
	nodes, err := s.queries(ctx).RollbackNodesNow(ctx, upgradeID)
	if err != nil {
		return nil, nil, fmt.Errorf("read a baseline's nodes now: %w", err)
	}
	now := make(map[uuid.UUID]domain.NodeNow, len(nodes))
	for _, r := range nodes {
		now[r.ID] = domain.NodeNow{Node: nodeOf(gen.FindNodeRow(r))}
	}
	contents, err := s.queries(ctx).RollbackContentsNow(ctx, upgradeID)
	if err != nil {
		return nil, nil, fmt.Errorf("read a baseline's contents now: %w", err)
	}
	out := make(map[uuid.UUID]domain.ContentNow, len(contents))
	for _, r := range contents {
		out[r.NodeID] = domain.ContentNow{Revision: int(r.Revision), Hash: r.ContentHash, Bytes: int(r.ByteSize)}
	}
	return now, out, nil
}

// RollbackBaseline implements app.RollbackReads: the upgrade's baseline
// rows, every one.
func (s *Store) RollbackBaseline(ctx context.Context, upgradeID uuid.UUID) ([]domain.BaselineNode, error) {
	rows, err := s.queries(ctx).RollbackBaseline(ctx, upgradeID)
	if err != nil {
		return nil, fmt.Errorf("read an upgrade's baseline: %w", err)
	}
	out := make([]domain.BaselineNode, len(rows))
	for i, r := range rows {
		b := domain.BaselineNode{NodeID: r.NodeID, Kind: domain.Kind(r.Kind), ParentID: r.ParentID, Hash: r.ContentHash}
		if r.Name == nil || r.SortOrder == nil {
			return nil, fmt.Errorf("the baseline row of node %s without its place", r.NodeID)
		}
		b.Name, b.SortOrder = *r.Name, *r.SortOrder
		if r.Revision != nil && r.ByteSize != nil {
			b.Revision, b.Bytes = int(*r.Revision), int(*r.ByteSize)
		}
		out[i] = b
	}
	return out, nil
}

// RollbackLeft implements app.RollbackReads: what the rollback of the
// upgrade has left to do in the notebook (M11 decision 226).
func (s *Store) RollbackLeft(ctx context.Context, upgradeID, notebookID uuid.UUID) (domain.RollbackLeft, error) {
	r, err := s.queries(ctx).RollbackLeft(ctx, gen.RollbackLeftParams{UpgradeID: upgradeID, NotebookID: notebookID})
	if err != nil {
		return domain.RollbackLeft{}, fmt.Errorf("read what a rollback has left: %w", err)
	}
	return domain.RollbackLeft{Nodes: int(r.Nodes), Pages: int(r.Pages), Created: int(r.Created)}, nil
}

// DeleteUpgradeNodes deletes the upgrade's rows, as it ends (M11 design
// 4.10), in the transaction ctx carries, and answers how many.
func (s *Store) DeleteUpgradeNodes(ctx context.Context, upgradeID uuid.UUID) (int64, error) {
	n, err := s.queries(ctx).DeleteUpgradeNodes(ctx, upgradeID)
	if err != nil {
		return 0, fmt.Errorf("delete an upgrade's rows: %w", err)
	}
	return n, nil
}

// UpgradeGateBaseline is the upgrade's baseline rows, each node's kind and
// parent as it started, a page's wanted count then (M11/P2 design 3.9), in
// the transaction ctx carries.
func (s *Store) UpgradeGateBaseline(ctx context.Context, upgradeID uuid.UUID) ([]app.BaselineNode, error) {
	rows, err := s.queries(ctx).UpgradeGateBaseline(ctx, upgradeID)
	if err != nil {
		return nil, fmt.Errorf("read an upgrade's baseline: %w", err)
	}
	out := make([]app.BaselineNode, len(rows))
	for i, r := range rows {
		out[i] = app.BaselineNode{ID: r.NodeID, ParentID: r.ParentID, Kind: domain.Kind(r.Kind)}
		if r.Wanted != nil {
			out[i].Wanted = int(*r.Wanted)
		}
	}
	return out, nil
}

// UpgradePreserved is gate 7 of the upgrade's check (M11/P2 design 3.9),
// in one pass, in the transaction ctx carries.
func (s *Store) UpgradePreserved(ctx context.Context, upgradeID uuid.UUID) (app.Preserved, error) {
	r, err := s.queries(ctx).UpgradePreserved(ctx, upgradeID)
	if err != nil {
		return app.Preserved{}, fmt.Errorf("read what an upgrade kept: %w", err)
	}
	return app.Preserved{Nodes: int(r.Nodes), NodesKept: int(r.NodesKept), Pages: int(r.Pages), PagesKept: int(r.PagesKept),
		NotKept: r.NotKept}, nil
}

// UpgradeChangesets is how many changesets the upgrade wrote (M11/P2
// design 3.9), in the transaction ctx carries.
func (s *Store) UpgradeChangesets(ctx context.Context, upgradeID uuid.UUID) (int, error) {
	n, err := s.queries(ctx).UpgradeChangesets(ctx, &upgradeID)
	if err != nil {
		return 0, fmt.Errorf("count an upgrade's changesets: %w", err)
	}
	return int(n), nil
}
