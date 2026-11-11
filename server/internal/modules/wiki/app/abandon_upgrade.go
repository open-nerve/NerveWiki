package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AbandonUpgrade abandons a notebook's upgrade, or continues its stopped
// rollback: POST /api/v0/notebooks/{notebook_id}/upgrade/abandon (M11
// design 4.10, M11/P1 design 3.20).
type AbandonUpgrade struct{ d UpgradeDeps }

// NewAbandonUpgrade returns the use case.
func NewAbandonUpgrade(d UpgradeDeps) *AbandonUpgrade { return &AbandonUpgrade{d: d} }

// rollbackOutcome is what an abandonment's transaction leaves for its log
// line.
type rollbackOutcome struct {
	workspace uuid.UUID
	job       uuid.UUID
	view      UpgradeView
	continued bool
}

// Execute abandons notebook id's upgrade, upgrading or reviewing, or
// continues its rollback that stopped, in the order of M11/P1 design 3.20:
//
//  1. the notebook's workspace, unlocked (404);
//  2. one transaction: the workspace's row FOR SHARE, the notebook's FOR
//     NO KEY UPDATE, the decision of notebook.change_mode (404, 403), the
//     clock once;
//  3. no upgrade, or one in no state to roll back, wiki.upgrade_state;
//  4. upgrading or reviewing: rollBack, abandoned;
//  5. rolling back: its job queued or running transfer.busy; else a new
//     job (server_busy for a full queue), the module's row's job, and the
//     mode's subscribers told, the state unchanged;
//  6. after the commit, an info line, "upgrade abandoned" or "upgrade
//     rollback continued".
//
// It answers the upgrade's row as the transaction left it.
func (a *AbandonUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	workspace, ok, err := a.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	var out rollbackOutcome
	err = a.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		state, at, err := a.d.locked(ctx, actor, workspace, id)
		if err != nil {
			return err
		}
		up := state.Upgrade
		switch up.State {
		case domain.UpgradeUpgrading, domain.UpgradeReviewing:
			out.job, err = a.d.rollBack(ctx, actor, client, workspace, id, up.ID, domain.UpgradeAbandoned, at)
		case domain.UpgradeRollingBack:
			out.job, err = a.continueRollback(ctx, actor, client, workspace, id, up.ID, at)
			out.continued = true
		default:
			return domain.ErrUpgradeState
		}
		if err != nil {
			return err
		}
		out.workspace = workspace
		out.view, err = a.d.view(ctx, id, up.ID)
		return err
	})
	if err != nil {
		return UpgradeView{}, err
	}
	up := out.view.Upgrade
	msg := "upgrade abandoned"
	if out.continued {
		msg = "upgrade rollback continued"
	}
	a.d.Logger.InfoContext(ctx, msg, slog.String("workspace_id", out.workspace.String()), slog.String("notebook_id", id.String()),
		slog.String("upgrade_id", up.ID.String()), slog.String("user_id", actor.UserID.String()),
		slog.String("job_id", out.job.String()))
	return out.view, nil
}

// rollBack sets notebook id's upgrade upgradeID, upgrading or reviewing,
// rolling back for ending by the caller at at, in the transaction that
// locked the notebook's row: the notebook's state, the mode's subscribers
// told, the rollback's job started (server_busy for a full queue), then
// the module's row, rolling back with the job in one statement. It
// answers the job's id. The abandonment's and the rejection's.
func (d UpgradeDeps) rollBack(ctx context.Context, actor shared.Actor, client string, workspace, id, upgradeID uuid.UUID,
	ending domain.UpgradeEnding, at time.Time,
) (uuid.UUID, error) {
	if err := d.States.SetState(ctx, id, upgradeID, domain.UpgradeRollingBack, actor.UserID, at); err != nil {
		return uuid.UUID{}, err
	}
	if err := d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return uuid.UUID{}, err
	}
	job, err := d.Rollbacks.Start(ctx, id, upgradeID, client)
	if err != nil {
		return uuid.UUID{}, err
	}
	ok, err := d.Rows.RollBack(ctx, upgradeID, ending, job, actor.UserID, at)
	switch {
	case err != nil:
		return uuid.UUID{}, err
	case !ok:
		return uuid.UUID{}, fmt.Errorf("wiki: the upgrade %s under way on its notebook has no row to roll back, a defect", upgradeID)
	}
	return job, nil
}

// continueRollback starts the stopped rollback of notebook id's upgrade
// upgradeID again: transfer.busy while its job is queued or running; a new
// job, the module's row's, and the mode's subscribers told, from standard
// to standard as each step of the upgrade (M11 design 4.3). It answers the
// job's id.
func (a *AbandonUpgrade) continueRollback(ctx context.Context, actor shared.Actor, client string, workspace, id, upgradeID uuid.UUID,
	at time.Time,
) (uuid.UUID, error) {
	writing, err := a.d.Writing.Writing(ctx, id)
	switch {
	case err != nil:
		return uuid.UUID{}, err
	case writing:
		return uuid.UUID{}, domain.ErrRollbackWriting
	}
	job, err := a.d.Rollbacks.Start(ctx, id, upgradeID, client)
	if err != nil {
		return uuid.UUID{}, err
	}
	ok, err := a.d.Rows.ContinueRollback(ctx, upgradeID, job, actor.UserID, at)
	switch {
	case err != nil:
		return uuid.UUID{}, err
	case !ok:
		return uuid.UUID{}, fmt.Errorf("wiki: the upgrade %s rolling back on its notebook has no row rolling back, a defect", upgradeID)
	}
	return job, a.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at})
}

// view reads notebook id's upgrade upgradeID under way as its admins read
// it, in the transaction that wrote it: a notebook whose upgrade has no
// row under way is a defect.
func (d UpgradeDeps) view(ctx context.Context, id, upgradeID uuid.UUID) (UpgradeView, error) {
	up, ok, err := d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok || up.ID != upgradeID:
		return UpgradeView{}, fmt.Errorf("wiki: the notebook %s's upgrade %s has no row under way, a defect", id, upgradeID)
	}
	return d.named(ctx, up)
}

// named is up with the display names of who started and who submitted it.
func (d UpgradeDeps) named(ctx context.Context, up domain.Upgrade) (UpgradeView, error) {
	ids := []uuid.UUID{up.CreatedBy}
	if up.SubmittedBy != nil {
		ids = append(ids, *up.SubmittedBy)
	}
	names, err := d.Names.DisplayNames(ctx, ids)
	if err != nil {
		return UpgradeView{}, err
	}
	view := UpgradeView{Upgrade: up, StartedByName: names[up.CreatedBy]}
	if up.SubmittedBy != nil {
		view.SubmittedByName = names[*up.SubmittedBy]
	}
	return view, nil
}
