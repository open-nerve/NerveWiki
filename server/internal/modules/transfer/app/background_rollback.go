package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// An upgrade's rollback as its job runs and ends (M11/P1 design 3.19): a
// pass at a time, the page module planning each; the end written with the
// upgrade's, in one transaction, the notebook's row locked first.

// errRollbackLeft is a rollback whose pass left no less than before it,
// or whose end found something left (M11/P1 design 2.1 item 12): the job
// stops, internal, its end written as a stop's.
var errRollbackLeft = errors.New("transfer: the rollback left something")

// errNoLongerRuns rolls back a rollback's end whose job no longer runs:
// the rescue failed it meanwhile.
var errNoLongerRuns = errors.New("transfer: the rollback's job no longer runs")

// rollback runs spec's rollback a pass at a time: each planned, then its
// batches written as a restore's or a revert's are, the progress's total
// grown by each pass's, an info line "rollback pass" after it; Done ends
// it. A pass that leaves no less than before it stops the job, internal:
// each pass deletes or puts back some of what is left, else there would
// be no end. ErrJobGone, the upgrade ended meanwhile (M11 decision 233),
// goes to finish as it is, which writes no end.
func (r *backgroundRun) rollback(ctx, running context.Context, spec JobSpec) error {
	before := 0
	for pass := 1; ; pass++ {
		if running.Err() != nil {
			return context.Cause(running)
		}
		plan, p, err := r.b.d.Tree.PlanRollback(running, spec)
		switch {
		case errors.Is(err, ErrJobGone):
			return err
		case err != nil:
			return r.stopped(err)
		case p.Done:
			return nil
		case pass > 1 && p.Left.Sum() >= before:
			return fmt.Errorf("%w: %d left after a pass, %d before it", errRollbackLeft, p.Left.Sum(), before)
		}
		before = p.Left.Sum()
		deleted := r.report.Counts.Deleted
		r.all.Add(int64(plan.Total()))
		r.publish()
		if err := r.batches(ctx, running, spec, plan); err != nil {
			return err
		}
		first := int64(0)
		if p.Deletes {
			first = r.report.Counts.Deleted - deleted
		}
		r.logger.InfoContext(ctx, "rollback pass", slog.String("job_id", r.job.ID.String()), slog.String("upgrade_id", spec.UpgradeID.String()),
			slog.Int("pass", pass), slog.Int64("deleted_first", first), slog.Int("items", p.Items), slog.Int("steps", len(plan.Steps())))
	}
}

// rollbackSucceeded ends the rollback's job and its upgrade in one
// transaction: the notebook's row first, as a deletion takes it before
// the job's (jobs.sql); the job's end; then the upgrade ended when nothing
// is left; once it committed, the upgrade's end's line, then its "rollback
// ended" line, as the export's succeed logs its own (finish logs nothing
// past a succeed). The job's end comes before the upgrade's, so that the
// upgrade ends, and the wiki module logs it, only in a transaction that
// writes the job's end and commits (M11/P1 S3 review A-N1): a job no
// longer running (the rescue failed it) rolls it all back, and the
// "stopped" line is logged. Something left rolls it all back and ends the job failed,
// internal, as a stop's end, rollbackStopped, writes it. A notebook
// deleted meanwhile leaves no row to lock: nothing is written, End is not
// called, and the "stopped" line is logged.
func (r *backgroundRun) rollbackSucceeded(ctx, running, end context.Context, e Ended, attrs []any) error {
	e.Report = r.report
	locked, left := false, false
	var committed func()
	err := r.b.d.Tx.WithinTx(end, func(ctx context.Context) error {
		var err error
		if locked, err = r.b.d.Notebooks.LockByID(ctx, r.job.NotebookID); err != nil || !locked {
			return err
		}
		ok, err := r.rows.FinishJob(ctx, r.job.ID, e)
		switch {
		case err != nil:
			return err
		case !ok:
			return errNoLongerRuns
		}
		if left, committed, err = r.b.d.RollbackEnd.End(ctx, r.job.NotebookID, *r.job.UpgradeID); err != nil {
			return err
		}
		if left {
			return errRollbackLeft
		}
		return nil
	})
	switch {
	case errors.Is(err, errRollbackLeft):
		return r.finish(ctx, running, fmt.Errorf("%w as it ended", errRollbackLeft), attrs, nil)
	case errors.Is(err, errNoLongerRuns) || err == nil && !locked:
		r.logger.InfoContext(end, "rollback stopped: the job was deleted, or no longer runs", attrs...)
		return nil
	case err != nil:
		return fmt.Errorf("write the success of rollback %s: %w", r.job.ID, err)
	}
	if committed != nil {
		committed()
	}
	r.logger.InfoContext(end, "rollback ended", append(attrs, slog.String("state", string(domain.StateSucceeded)),
		slog.String("failure", ""), slog.Int64("nodes", e.Progress.Done), slog.Int64("skipped", e.Report.Counts.Skipped))...)
	return nil
}

// rollbackStopped writes a stopped rollback's end in one transaction: the
// notebook's row, the job's end, then RollbackEnd.Stopped when the end was
// written (M11 design 4.10, decision 228). No notebook row: false, nothing
// written, Stopped not called.
func (r *backgroundRun) rollbackStopped(end context.Context, e Ended) (bool, error) {
	ok := false
	err := r.b.d.Tx.WithinTx(end, func(ctx context.Context) error {
		locked, err := r.b.d.Notebooks.LockByID(ctx, r.job.NotebookID)
		if err != nil || !locked {
			return err
		}
		if ok, err = r.rows.FinishJob(ctx, r.job.ID, e); err != nil || !ok {
			return err
		}
		return r.b.d.RollbackEnd.Stopped(ctx, r.job.NotebookID, *r.job.UpgradeID)
	})
	if err != nil {
		return false, err
	}
	return ok, nil
}
