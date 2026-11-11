package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ApproveUpgrade approves a notebook's upgrade in review: POST
// /api/v0/notebooks/{notebook_id}/upgrade/approve (M11 design 4.9, M11/P3
// design 3.6).
type ApproveUpgrade struct{ d UpgradeDeps }

// NewApproveUpgrade returns the use case.
func NewApproveUpgrade(d UpgradeDeps) *ApproveUpgrade { return &ApproveUpgrade{d: d} }

// approveOutcome is what an approval's transaction leaves for its line.
type approveOutcome struct {
	view       UpgradeView
	submission uuid.UUID
	hashed     int
}

// gatesRefused is what an approval whose gates did not pass leaves for its
// error line: the upgrade and the gates' keys.
type gatesRefused struct {
	upgrade uuid.UUID
	failed  []string
}

// Execute approves notebook id's upgrade, in the order of M11/P3 design
// 3.6:
//
//  1. the notebook's workspace, unlocked (404);
//  2. one transaction, on a deadline of its own: the workspace's row FOR
//     SHARE, the notebook's FOR NO KEY UPDATE, the decision of
//     notebook.change_mode (404, 403), the clock once;
//  3. no upgrade in review wiki.upgrade_state;
//  4. its row, a defect unless the notebook's;
//  5. no report, its submission still checked or never finished,
//     wiki.upgrade_state; a report that did not pass a defect;
//  6. gates 1 to 3 judged again: not passed wiki.upgrade_not_ready, the
//     upgrade still in review;
//  7. the answer's row, named, read before the writes;
//  8. the notebook an LLM Wiki whose source root is the upgrade's, its
//     upgrade ended; the source hashes rebuilt; the baseline deleted; the
//     module's row approved (none a defect); the mode's subscribers told;
//  9. a deadline of the approval's own, at any step, is server_busy with a
//     warn line; gates not passed an error line, after the rollback
//     (decision 293); after the commit an info line.
//
// It answers the upgrade's row as the transaction left it, approved.
func (a *ApproveUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
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
	started := time.Now()
	ctxApprove, cancel := context.WithTimeout(ctx, a.d.ApproveTimeout)
	defer cancel()
	var out approveOutcome
	var refused gatesRefused
	err = a.d.Tx.WithinTx(ctxApprove, func(ctx context.Context) error {
		var err error
		out, err = a.approve(ctx, actor, workspace, id, &refused)
		return err
	})
	elapsed := time.Since(started)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctxApprove.Err() != nil && ctx.Err() == nil {
			a.d.Logger.WarnContext(ctx, "upgrade approval timed out", slog.String("workspace_id", workspace.String()),
				slog.String("notebook_id", id.String()), slog.String("user_id", actor.UserID.String()),
				slog.Int64("elapsed_ms", elapsed.Milliseconds()), slog.Int64("timeout_ms", a.d.ApproveTimeout.Milliseconds()))
			return UpgradeView{}, shared.ServerBusy(time.Second)
		}
		// Only a defect reaches it, the upgrade frozen in review (decision
		// 293); a rollback that failed after it is that failure's answer.
		if errors.Is(err, domain.ErrUpgradeNotReady) {
			a.d.Logger.ErrorContext(ctx, "upgrade gates failed at approval", slog.String("notebook_id", id.String()),
				slog.String("upgrade_id", refused.upgrade.String()), slog.Any("failed_gates", refused.failed))
		}
		return UpgradeView{}, err
	}
	up := out.view.Upgrade
	a.d.Logger.InfoContext(ctx, "upgrade approved", slog.String("workspace_id", workspace.String()),
		slog.String("notebook_id", id.String()), slog.String("upgrade_id", up.ID.String()),
		slog.String("submission_id", out.submission.String()), slog.String("user_id", actor.UserID.String()),
		slog.Int("sources_hashed", out.hashed), slog.Int64("elapsed_ms", elapsed.Milliseconds()))
	return out.view, nil
}

// approve is the approval's transaction, steps 2 to 8; refused receives
// what its error line names when the gates do not pass.
func (a *ApproveUpgrade) approve(ctx context.Context, actor shared.Actor, workspace, id uuid.UUID, refused *gatesRefused,
) (approveOutcome, error) {
	state, at, err := a.d.locked(ctx, actor, workspace, id)
	if err != nil {
		return approveOutcome{}, err
	}
	if state.Upgrade.State != domain.UpgradeReviewing {
		return approveOutcome{}, domain.ErrUpgradeState
	}
	up, ok, err := a.d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return approveOutcome{}, err
	case !ok || up.ID != state.Upgrade.ID || up.State != state.Upgrade.State:
		return approveOutcome{}, fmt.Errorf("wiki: the notebook %s's upgrade %s has no row in its state %s, a defect", id, state.Upgrade.ID,
			state.Upgrade.State)
	}
	review, err := domain.ReviewOf(up, at, a.d.SubmitTimeout)
	switch {
	case err != nil:
		return approveOutcome{}, err
	case review != domain.ReviewReported:
		return approveOutcome{}, domain.ErrUpgradeState
	case !up.Report.Passed || up.SubmissionID == nil:
		// Only a check that passed keeps its report (M11/P3 design 2.1
		// item 5).
		return approveOutcome{}, fmt.Errorf("wiki: the upgrade %s's report kept did not pass, or has no submission, a defect", up.ID)
	}
	roots, err := a.d.Gates.Roots(ctx, id, up.SourceRootID)
	if err != nil {
		return approveOutcome{}, err
	}
	nodes, err := a.d.Pages.RootNodes(ctx, id, a.d.ReservedKeys)
	if err != nil {
		return approveOutcome{}, err
	}
	reserved := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		reserved[i] = n.ID
	}
	if failed := domain.JudgeRoots(roots, up.SourceRootID, reserved).Failed(); len(failed) > 0 {
		refused.upgrade, refused.failed = up.ID, make([]string, len(failed))
		for i, key := range failed {
			refused.failed[i] = string(key)
		}
		return approveOutcome{}, domain.ApprovalNotReady(failed)
	}
	// The row as the approval leaves it: Current reads it no more.
	view, err := a.d.named(ctx, up)
	if err != nil {
		return approveOutcome{}, err
	}
	if err := a.d.States.Approve(ctx, id, up.ID, up.SourceRootID, actor.UserID, at); err != nil {
		return approveOutcome{}, err
	}
	hashed, err := a.d.SourceHashes.Rebuild(ctx, id, up.SourceRootID)
	if err != nil {
		return approveOutcome{}, err
	}
	if err := a.d.Rollback.DeleteBaseline(ctx, up.ID); err != nil {
		return approveOutcome{}, err
	}
	ok, err = a.d.Rows.Approve(ctx, up.ID, *up.SubmissionID, actor.UserID, at)
	switch {
	case err != nil:
		return approveOutcome{}, err
	case !ok:
		return approveOutcome{}, fmt.Errorf("wiki: the upgrade %s in review has no row to approve, a defect", up.ID)
	}
	if err := a.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeLLMWiki, By: actor.UserID, At: at}); err != nil {
		return approveOutcome{}, err
	}
	view.Upgrade.State = domain.UpgradeApproved
	return approveOutcome{view: view, submission: *up.SubmissionID, hashed: hashed}, nil
}
