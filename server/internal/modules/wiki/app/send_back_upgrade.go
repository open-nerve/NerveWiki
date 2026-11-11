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

// SendBackUpgrade sends a notebook's upgrade in review back to upgrading:
// POST /api/v0/notebooks/{notebook_id}/upgrade/send-back (M11 design 4.8,
// M11/P3 design 3.8).
type SendBackUpgrade struct{ d UpgradeDeps }

// NewSendBackUpgrade returns the use case.
func NewSendBackUpgrade(d UpgradeDeps) *SendBackUpgrade { return &SendBackUpgrade{d: d} }

// sentBack is what a send-back's transaction leaves for its line: the
// row as written, and the submission it cleared, as read before.
type sentBack struct {
	view       UpgradeView
	submission uuid.UUID
	reported   bool
}

// Execute sends notebook id's upgrade back, in the order of M11/P3 design
// 3.8:
//
//  1. the notebook's workspace, unlocked (404);
//  2. one transaction, the request's: the workspace's row FOR SHARE, the
//     notebook's FOR NO KEY UPDATE, the decision of notebook.change_mode
//     (404, 403), the clock once, truncated to the microsecond the table
//     keeps (Go and the SQL judge the same moment);
//  3. no upgrade in review wiki.upgrade_state;
//  4. its row, a defect unless the notebook's;
//  5. without a report, its submission still checked: wiki.upgrade_state,
//     "still being checked";
//  6. the notebook upgrading again; the module's row upgrading, its
//     submission and report cleared (none a defect); the mode's
//     subscribers told; the row as written;
//  7. after the commit, an info line "upgrade sent back".
//
// It answers the upgrade's row as the transaction left it, upgrading.
func (s *SendBackUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return UpgradeView{}, err
	}
	workspace, ok, err := s.d.Notebooks.WorkspaceOf(ctx, id)
	switch {
	case err != nil:
		return UpgradeView{}, err
	case !ok:
		return UpgradeView{}, domain.ErrNotebookNotFound
	}
	var out sentBack
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.sendBack(ctx, actor, workspace, id)
		return err
	})
	if err != nil {
		return UpgradeView{}, err
	}
	s.d.Logger.InfoContext(ctx, "upgrade sent back", slog.String("workspace_id", workspace.String()), slog.String("notebook_id", id.String()),
		slog.String("upgrade_id", out.view.Upgrade.ID.String()), slog.String("submission_id", out.submission.String()),
		slog.String("user_id", actor.UserID.String()), slog.Bool("reported", out.reported))
	return out.view, nil
}

// sendBack is the send-back's transaction, steps 2 to 6.
func (s *SendBackUpgrade) sendBack(ctx context.Context, actor shared.Actor, workspace, id uuid.UUID) (sentBack, error) {
	state, now, err := s.d.locked(ctx, actor, workspace, id)
	if err != nil {
		return sentBack{}, err
	}
	at := now.Truncate(time.Microsecond)
	if state.Upgrade.State != domain.UpgradeReviewing {
		return sentBack{}, domain.ErrUpgradeState
	}
	up, ok, err := s.d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return sentBack{}, err
	case !ok || up.ID != state.Upgrade.ID || up.State != state.Upgrade.State:
		return sentBack{}, fmt.Errorf("wiki: the notebook %s's upgrade %s has no row in its state %s, a defect", id, state.Upgrade.ID,
			state.Upgrade.State)
	}
	review, err := domain.ReviewOf(up, at, s.d.SubmitTimeout)
	switch {
	case err != nil:
		return sentBack{}, err
	case review == domain.ReviewChecking:
		return sentBack{}, domain.ErrUpgradeSubmitting
	case up.SubmissionID == nil:
		return sentBack{}, fmt.Errorf("wiki: the upgrade %s reviewing has no submission, a defect", up.ID)
	}
	if err := s.d.States.SetState(ctx, id, up.ID, domain.UpgradeUpgrading, actor.UserID, at); err != nil {
		return sentBack{}, err
	}
	ok, err = s.d.Rows.SendBack(ctx, up.ID, actor.UserID, at, at.Add(-s.d.SubmitTimeout))
	switch {
	case err != nil:
		return sentBack{}, err
	case !ok:
		return sentBack{}, fmt.Errorf("wiki: the upgrade %s in review has no row to send back, a defect", up.ID)
	}
	if err := s.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return sentBack{}, err
	}
	view, err := s.d.view(ctx, id, up.ID)
	return sentBack{view: view, submission: *up.SubmissionID, reported: up.Report != nil}, err
}
