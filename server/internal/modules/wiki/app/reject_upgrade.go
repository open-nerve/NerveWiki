package app

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// RejectUpgrade rejects a notebook's upgrade in review: POST
// /api/v0/notebooks/{notebook_id}/upgrade/reject (M11 design 4.10, M11/P3
// design 3.7).
type RejectUpgrade struct{ d UpgradeDeps }

// NewRejectUpgrade returns the use case.
func NewRejectUpgrade(d UpgradeDeps) *RejectUpgrade { return &RejectUpgrade{d: d} }

// Execute rejects notebook id's upgrade, in review, with its report or
// not, in the order of M11/P3 design 3.7:
//
//  1. the notebook's workspace, unlocked (404);
//  2. one transaction, the request's: the workspace's row FOR SHARE, the
//     notebook's FOR NO KEY UPDATE, the decision of notebook.change_mode
//     (404, 403), the clock once;
//  3. no upgrade in review, upgrading too, wiki.upgrade_state;
//  4. rollBack, rejected (server_busy for a full queue); the row as
//     written;
//  5. after the commit, an info line "upgrade rejected".
//
// It answers the upgrade's row as the transaction left it, rolling back
// with its job. A submission still checked then stores nothing (its
// second transaction reads it rolling back).
func (r *RejectUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	client, err := shared.ClientOf(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	workspace, ok, err := r.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	var out rollbackOutcome
	err = r.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		state, at, err := r.d.locked(ctx, actor, workspace, id)
		if err != nil {
			return err
		}
		up := state.Upgrade
		if up.State != domain.UpgradeReviewing {
			return domain.ErrUpgradeState
		}
		if out.job, err = r.d.rollBack(ctx, actor, client, workspace, id, up.ID, domain.UpgradeRejected, at); err != nil {
			return err
		}
		out.view, err = r.d.view(ctx, id, up.ID)
		return err
	})
	if err != nil {
		return UpgradeView{}, err
	}
	up := out.view.Upgrade
	var submission string
	if up.SubmissionID != nil {
		submission = up.SubmissionID.String()
	}
	r.d.Logger.InfoContext(ctx, "upgrade rejected", slog.String("workspace_id", workspace.String()), slog.String("notebook_id", id.String()),
		slog.String("upgrade_id", up.ID.String()), slog.String("submission_id", submission), slog.String("user_id", actor.UserID.String()),
		slog.String("job_id", out.job.String()))
	return out.view, nil
}
