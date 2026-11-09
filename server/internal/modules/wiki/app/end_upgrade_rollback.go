package app

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// EndUpgradeRollback ends a notebook's upgrade's stopped rollback, keeping
// the notebook as it is: POST
// /api/v0/notebooks/{notebook_id}/upgrade/end-rollback (M11 design 4.10,
// M11/P1 design 3.20).
type EndUpgradeRollback struct{ d UpgradeDeps }

// NewEndUpgradeRollback returns the use case.
func NewEndUpgradeRollback(d UpgradeDeps) *EndUpgradeRollback { return &EndUpgradeRollback{d: d} }

// Execute ends the stopped rollback of notebook id's upgrade, in the order
// of M11/P1 design 3.20:
//
//  1. the notebook's workspace, unlocked (404);
//  2. one transaction: the workspace's row FOR SHARE, the notebook's FOR
//     NO KEY UPDATE, the decision of notebook.change_mode (404, 403), the
//     clock once;
//  3. no upgrade rolling back wiki.upgrade_state;
//  4. its job queued or running transfer.busy;
//  5. what the rollback left, read now; the notebook's upgrade cleared,
//     its baseline deleted, the module's row rolled back, complete when
//     nothing was left, else what was left (decision 230); the mode's
//     subscribers told;
//  6. after the commit, an info line "upgrade rollback ended".
//
// It answers the upgrade's row, rolled back.
func (e *EndUpgradeRollback) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	workspace, ok, err := e.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	var view UpgradeView
	var left domain.RollbackLeft
	err = e.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		state, at, err := e.d.locked(ctx, actor, workspace, id)
		if err != nil {
			return err
		}
		if state.Upgrade.State != domain.UpgradeRollingBack {
			return domain.ErrUpgradeState
		}
		upgradeID := state.Upgrade.ID
		writing, err := e.d.Writing.Writing(ctx, id)
		switch {
		case err != nil:
			return err
		case writing:
			return domain.ErrRollbackWriting
		}
		if view, err = e.d.view(ctx, id, upgradeID); err != nil {
			return err
		}
		if left, err = e.d.Rollback.Left(ctx, upgradeID, id); err != nil {
			return err
		}
		if err := e.d.States.Clear(ctx, id, upgradeID, actor.UserID, at); err != nil {
			return err
		}
		if err := e.d.Rollback.DeleteBaseline(ctx, upgradeID); err != nil {
			return err
		}
		var short *domain.RollbackLeft
		if left != (domain.RollbackLeft{}) {
			short = &left
		}
		_, ok, err := e.d.Rows.EndRollback(ctx, upgradeID, short, actor.UserID, at)
		switch {
		case err != nil:
			return err
		case !ok:
			return fmt.Errorf("wiki: the upgrade %s rolling back on its notebook has no row rolling back, a defect", upgradeID)
		}
		complete := short == nil
		view.Upgrade.State, view.Upgrade.RollbackComplete, view.Upgrade.RollbackLeft = domain.UpgradeRolledBack, &complete, short
		return e.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
			To: domain.ModeStandard, By: actor.UserID, At: at})
	})
	if err != nil {
		return UpgradeView{}, err
	}
	e.d.Logger.InfoContext(ctx, "upgrade rollback ended", slog.String("workspace_id", workspace.String()),
		slog.String("notebook_id", id.String()), slog.String("upgrade_id", view.Upgrade.ID.String()),
		slog.String("user_id", actor.UserID.String()), slog.Int("nodes", left.Nodes), slog.Int("pages", left.Pages),
		slog.Int("created", left.Created), slog.Bool("complete", *view.Upgrade.RollbackComplete))
	return view, nil
}
