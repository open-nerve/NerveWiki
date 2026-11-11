package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// SubmitUpgrade submits a notebook's upgrade for its admins' review:
// POST /api/v0/notebooks/{notebook_id}/upgrade/submit and MCP's
// upgrade_submit (M11 design 4.8, M11/P2 design 3.12).
type SubmitUpgrade struct {
	d     UpgradeDeps
	check *CheckUpgrade
	runs  Runner
}

// NewSubmitUpgrade returns the use case, which checks through check's
// computation, a run of runs keyed by the submission.
func NewSubmitUpgrade(d UpgradeDeps, check *CheckUpgrade, runs Runner) *SubmitUpgrade {
	return &SubmitUpgrade{d: d, check: check, runs: runs}
}

// The outcomes of a submission's second transaction, its info line's
// (M11/P2 design 3.16).
const (
	submitStored     = "stored"
	submitStale      = "stale"
	submitNotReady   = "not_ready"
	submitFailed     = "failed"
	submitSuperseded = "superseded"
)

// submitted is what a submission's first transaction wrote: the upgrade
// as it left it, the submission's id and moment, truncated to the
// microsecond the table keeps.
type submitted struct {
	workspace  uuid.UUID
	up         domain.Upgrade
	submission uuid.UUID
	at         time.Time
}

// submitEnd is what a submission's second transaction decided: its answer,
// a view or an error, and its line's outcome.
type submitEnd struct {
	view    UpgradeView
	answer  error
	outcome string
}

// Execute submits notebook id's upgrade, in the order of M11/P2 design
// 3.12:
//
//  1. the notebook's workspace, unlocked (404);
//  2. the first transaction, the request's: the workspace's row FOR
//     SHARE, the notebook's FOR NO KEY UPDATE, notebook.change_mode (404,
//     403), the clock once, truncated to the microsecond; no upgrade, or
//     one rolling back, wiki.upgrade_state; reviewing with a report
//     wiki.upgrade_state, without one submitted less than SubmitTimeout
//     ago ErrUpgradeSubmitting, else taken over; reviewing, a new
//     submission, the mode's subscribers told;
//  3. apart from the request's cancel, by the earlier of the submission's
//     moment plus SubmitTimeout and the request's deadline;
//  4. the check, a run keyed by the submission, in a snapshot whose
//     upgrade is still this submission's;
//  5. the second transaction: the rows locked again; another submission's
//     upgrade, or none, writes nothing (wiki.upgrade_state); else a
//     failed check, a stale index or a gate not passed sets it upgrading
//     again (its error, wiki.upgrade_stale_index, wiki.upgrade_not_ready),
//     a passed one keeps its report; the mode's subscribers told; then,
//     before the COMMIT, notebook.change_mode again, which decides only
//     the answer;
//  6. that transaction past the deadline is server_busy, its Retry-After
//     the seconds until a submission may take it over, a warn line;
//  7. after the commit, an info line, whatever the answer.
func (s *SubmitUpgrade) Execute(ctx context.Context, id uuid.UUID) (UpgradeView, error) {
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
	var first submitted
	if err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		first, err = s.first(ctx, actor, workspace, id)
		return err
	}); err != nil {
		return UpgradeView{}, err
	}
	deadline := first.at.Add(s.d.SubmitTimeout)
	if req, ok := ctx.Deadline(); ok && req.Before(deadline) {
		deadline = req
	}
	detached, cancel := context.WithDeadline(context.WithoutCancel(ctx), deadline)
	defer cancel()
	up, submission := first.up, first.submission
	result, runErr := s.runs.Do(detached, upgradeKey(up, domain.LintUpgradeSubmit, submission),
		func(worker context.Context) (RunOutput, error) {
			return s.check.compute(worker, up, func(up, now domain.Upgrade) bool {
				return now.ID == up.ID && now.State == domain.UpgradeReviewing && now.SubmissionID != nil &&
					*now.SubmissionID == submission && now.Report == nil
			}, RuleQuery{WantedByPage: true}, domain.LintUpgradeSubmit)
		})
	var end submitEnd
	err = s.d.Tx.WithinTx(detached, func(ctx context.Context) error {
		var err error
		end, err = s.second(ctx, actor, first, result.Check, runErr)
		return err
	})
	ids := []any{slog.String("workspace_id", workspace.String()), slog.String("notebook_id", id.String()),
		slog.String("upgrade_id", up.ID.String()), slog.String("submission_id", submission.String()),
		slog.String("user_id", actor.UserID.String())}
	elapsed := s.d.Clock.Now().Sub(first.at)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && detached.Err() != nil {
			s.d.Logger.WarnContext(ctx, "upgrade submission timed out", append(ids, slog.Int64("elapsed_ms", elapsed.Milliseconds()),
				slog.Int64("timeout_ms", deadline.Sub(first.at).Milliseconds()))...)
			return UpgradeView{}, shared.ServerBusy(s.retryAfter(first.at))
		}
		return UpgradeView{}, err
	}
	check, failed := result.Check, []string{}
	var stale int64
	if check != nil {
		stale = check.StalePages
		for _, key := range check.Gates.Failed() {
			failed = append(failed, string(key))
		}
	}
	s.d.Logger.InfoContext(ctx, "upgrade submitted", append(ids, slog.String("outcome", end.outcome),
		slog.Bool("passed", check != nil && check.Passed), slog.Any("failed_gates", failed), slog.Int64("stale_pages", stale),
		slog.Int64("elapsed_ms", elapsed.Milliseconds()))...)
	if end.answer != nil {
		return UpgradeView{}, end.answer
	}
	return end.view, nil
}

// retryAfter is the Retry-After of a submission at at past its deadline
// (step 6, decision 274): the whole seconds until at plus SubmitTimeout,
// when another submission may take it over, at least one. A deadline the
// request's, earlier, leaves seconds to wait: a retry a second on would
// be "still being checked".
func (s *SubmitUpgrade) retryAfter(at time.Time) time.Duration {
	wait := at.Add(s.d.SubmitTimeout).Sub(s.d.Clock.Now())
	return max(time.Second, time.Duration(math.Ceil(wait.Seconds()))*time.Second)
}

// first is the submission's first transaction, step 2.
func (s *SubmitUpgrade) first(ctx context.Context, actor shared.Actor, workspace, id uuid.UUID) (submitted, error) {
	state, now, err := s.d.locked(ctx, actor, workspace, id)
	if err != nil {
		return submitted{}, err
	}
	at := now.Truncate(time.Microsecond)
	if state.Upgrade.State != domain.UpgradeUpgrading && state.Upgrade.State != domain.UpgradeReviewing {
		return submitted{}, domain.ErrUpgradeState
	}
	up, ok, err := s.d.Rows.Current(ctx, id)
	switch {
	case err != nil:
		return submitted{}, err
	case !ok || up.ID != state.Upgrade.ID || up.State != state.Upgrade.State:
		return submitted{}, fmt.Errorf("wiki: the notebook %s's upgrade %s has no row in its state %s, a defect", id, state.Upgrade.ID,
			state.Upgrade.State)
	}
	// Reviewing: none reported and past the moment, taken over (M11/P3
	// design 3.9).
	if up.State == domain.UpgradeReviewing {
		review, err := domain.ReviewOf(up, at, s.d.SubmitTimeout)
		switch {
		case err != nil:
			return submitted{}, err
		case review == domain.ReviewReported:
			return submitted{}, domain.ErrUpgradeState
		case review == domain.ReviewChecking:
			return submitted{}, domain.ErrUpgradeSubmitting
		}
	}
	submission := uuid.NewV7()
	if err := s.d.States.SetState(ctx, id, up.ID, domain.UpgradeReviewing, actor.UserID, at); err != nil {
		return submitted{}, err
	}
	ok, err = s.d.Rows.Submit(ctx, up.ID, submission, actor.UserID, at, at.Add(-s.d.SubmitTimeout))
	switch {
	case err != nil:
		return submitted{}, err
	case !ok:
		return submitted{}, fmt.Errorf("wiki: the upgrade %s has no row to submit, a defect", up.ID)
	}
	if err := s.d.States.Publish(ctx, ModeChange{WorkspaceID: workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return submitted{}, err
	}
	by := actor.UserID
	up.State, up.SubmissionID, up.SubmittedBy, up.SubmittedAt, up.Report = domain.UpgradeReviewing, &submission, &by, &at, nil
	return submitted{workspace: workspace, up: up, submission: submission, at: at}, nil
}

// second is the submission's second transaction, step 5: what it writes
// and what it answers, the decision before the answer made in it, before
// its COMMIT. A notebook not locked again, gone since, writes nothing and
// is notebook.not_found, as the first transaction's lock would answer.
func (s *SubmitUpgrade) second(ctx context.Context, actor shared.Actor, first submitted, check *domain.UpgradeCheck, runErr error,
) (submitEnd, error) {
	id, upgradeID, submission := first.up.NotebookID, first.up.ID, first.submission
	end := submitEnd{answer: domain.ErrUpgradeState, outcome: submitSuperseded}
	if ok, err := s.d.Workspaces.ShareByID(ctx, first.workspace); err != nil || !ok {
		end.answer = domain.ErrNotebookNotFound
		return end, err
	}
	if _, ok, err := s.d.States.Lock(ctx, id); err != nil || !ok {
		end.answer = domain.ErrNotebookNotFound
		return end, err
	}
	now, ok, err := s.d.Rows.Current(ctx, id)
	if err != nil {
		return submitEnd{}, err
	}
	if !ok || now.ID != upgradeID || now.State != domain.UpgradeReviewing || now.SubmissionID == nil ||
		*now.SubmissionID != submission || now.Report != nil {
		return s.decided(ctx, actor, first.workspace, id, end)
	}
	at := s.d.Clock.Now().UTC()
	switch {
	case runErr != nil || check == nil:
		if runErr == nil {
			runErr = errLintInvariant
		}
		end = submitEnd{answer: runErr, outcome: submitFailed}
	case check.StalePages > 0:
		end = submitEnd{answer: domain.ErrUpgradeStaleIndex, outcome: submitStale}
	case !check.Passed:
		end = submitEnd{answer: domain.UpgradeNotReady(check.Gates.Failed()), outcome: submitNotReady}
	default:
		end = submitEnd{outcome: submitStored}
	}
	if end.outcome == submitStored {
		ok, err = s.d.Rows.KeepReport(ctx, upgradeID, submission, domain.ReportOf(*check), actor.UserID, at)
	} else {
		if err := s.d.States.SetState(ctx, id, upgradeID, domain.UpgradeUpgrading, actor.UserID, at); err != nil {
			return submitEnd{}, err
		}
		ok, err = s.d.Rows.Unsubmit(ctx, upgradeID, submission, actor.UserID, at)
	}
	switch {
	case err != nil:
		return submitEnd{}, err
	case !ok:
		return submitEnd{}, fmt.Errorf("wiki: the upgrade %s's submission %s has no row to end, a defect", upgradeID, submission)
	}
	if err := s.d.States.Publish(ctx, ModeChange{WorkspaceID: first.workspace, NotebookID: id, From: domain.ModeStandard,
		To: domain.ModeStandard, By: actor.UserID, At: at}); err != nil {
		return submitEnd{}, err
	}
	end, err = s.decided(ctx, actor, first.workspace, id, end)
	if err != nil || end.answer != nil {
		return end, err
	}
	end.view, err = s.d.view(ctx, id, upgradeID)
	return end, err
}

// decided is end, its answer the decision before the answer made now, in
// the second transaction (M11/P2 design 3.12 step 5): notebook.not_found
// for a notebook the caller cannot see, a refusal for one they are no
// admin of. It decides only the answer: what the transaction wrote is
// committed all the same.
func (s *SubmitUpgrade) decided(ctx context.Context, actor shared.Actor, workspace, id uuid.UUID, end submitEnd) (submitEnd, error) {
	_, err := s.d.Authorizer.Authorize(ctx, actor, s.d.Action, shared.Target{WorkspaceID: workspace, NotebookID: id})
	var refusal *shared.Error
	switch {
	case errors.Is(err, shared.ErrNotVisible):
		end.answer = domain.ErrNotebookNotFound
	case errors.As(err, &refusal) && refusal.Kind == shared.KindForbidden:
		end.answer = err
	case err != nil:
		return submitEnd{}, err
	}
	return end, nil
}
