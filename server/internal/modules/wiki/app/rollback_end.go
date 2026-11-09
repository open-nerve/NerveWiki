package app

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RollbackEndDeps are what an upgrade's rollback's end needs (M11/P1
// design 3.20): the notebooks' workspaces, which the mode's change names;
// the notebook module's upgrade's columns; the page module's part; the
// module's rows; the clock and the log.
type RollbackEndDeps struct {
	Notebooks Notebooks
	States    UpgradeStates
	Pages     RollbackPages
	Rows      UpgradeRows
	Clock     Clock
	Logger    Logger
}

// RollbackEnd ends an upgrade as its rollback's job ends (M11 design
// 4.10), in the job's end's transaction, which locked the notebook's row
// first: the transfer module's RollbackEnd. Its writer is the job's
// starter, ctx's actor.
type RollbackEnd struct{ d RollbackEndDeps }

// NewRollbackEnd returns the end.
func NewRollbackEnd(d RollbackEndDeps) *RollbackEnd { return &RollbackEnd{d: d} }

// End ends the upgrade upgradeID of the notebook when its rollback left
// nothing, and answers whether it left something: the notebook's upgrade
// cleared, its baseline deleted, its row rolled back, complete, and the
// mode's subscribers told; and committed, which logs an info line
// "upgrade rolled back", measured from when the rollback was abandoned or
// continued last, for its caller to call once the transaction committed
// (M11/P1 S3 review A-N1). Nothing, false and nil when the notebook's
// upgrade is no longer it, rolling back.
func (r *RollbackEnd) End(ctx context.Context, notebookID, upgradeID uuid.UUID) (bool, func(), error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return false, nil, err
	}
	state, ok, err := r.d.States.Lock(ctx, notebookID)
	if err != nil || !ok || state.Upgrade.ID != upgradeID || state.Upgrade.State != domain.UpgradeRollingBack {
		return false, nil, err
	}
	left, err := r.d.Pages.Left(ctx, upgradeID, notebookID)
	switch {
	case err != nil:
		return false, nil, err
	case left != (domain.RollbackLeft{}):
		return true, nil, nil
	}
	workspace, ok, err := r.d.Notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return false, nil, err
	case !ok:
		return false, nil, fmt.Errorf("wiki: the notebook %s locked has no workspace, a defect", notebookID)
	}
	at := r.d.Clock.Now()
	if err := r.d.States.Clear(ctx, notebookID, upgradeID, actor.UserID, at); err != nil {
		return false, nil, err
	}
	if err := r.d.Pages.DeleteBaseline(ctx, upgradeID); err != nil {
		return false, nil, err
	}
	since, ok, err := r.d.Rows.EndRollback(ctx, upgradeID, nil, actor.UserID, at)
	switch {
	case err != nil:
		return false, nil, err
	case !ok:
		return false, nil, fmt.Errorf("wiki: the upgrade %s rolling back on its notebook has no row rolling back, a defect", upgradeID)
	}
	if err := r.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: notebookID, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return false, nil, err
	}
	return false, func() {
		r.d.Logger.InfoContext(ctx, "upgrade rolled back", slog.String("workspace_id", workspace.String()),
			slog.String("notebook_id", notebookID.String()), slog.String("upgrade_id", upgradeID.String()),
			slog.String("user_id", actor.UserID.String()), slog.Int64("elapsed_ms", at.Sub(since).Milliseconds()))
	}, nil
}

// Stopped tells the mode's subscribers that the rollback of upgradeID
// stopped, when the notebook's upgrade is still it, rolling back: its
// change is one from standard to standard (M11 design 4.3).
func (r *RollbackEnd) Stopped(ctx context.Context, notebookID, upgradeID uuid.UUID) error {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return err
	}
	state, ok, err := r.d.States.Lock(ctx, notebookID)
	if err != nil || !ok || state.Upgrade.ID != upgradeID || state.Upgrade.State != domain.UpgradeRollingBack {
		return err
	}
	workspace, ok, err := r.d.Notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return err
	case !ok:
		return fmt.Errorf("wiki: the notebook %s locked has no workspace, a defect", notebookID)
	}
	return r.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: notebookID, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: r.d.Clock.Now()})
}
