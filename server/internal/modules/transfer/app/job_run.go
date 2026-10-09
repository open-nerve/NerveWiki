package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// A job as it runs, an export or an import (M7/P5 design 3.8; M7/P6
// design 3.13): its row moved to running, its heartbeat written every
// beat, which stops it when its cancel was asked or it was deleted, its
// progress and report as it goes, and its end written within bounds.

// The finish's bounds: a job ending as the server stops has until River's
// grace after the shutdown's timeout; any other, its timeout's too, long
// enough.
const (
	finishStopping = 900 * time.Millisecond
	finishWait     = 30 * time.Second
)

// The causes a running job's heartbeat stops it with. A job gone was
// deleted with its notebook, or no longer runs: the rescue failed it.
var (
	errCancelRequested = errors.New("transfer: the job's cancel was asked")
	errGone            = errors.New("transfer: the job was deleted, or no longer runs")
)

// failure is a job's failure as its report tells it, and the error behind
// it, which is logged.
type failure struct {
	code domain.Failure
	err  error
}

func (f *failure) Error() string { return string(f.code) + ": " + f.err.Error() }
func (f *failure) Unwrap() error { return f.err }

func failed(code domain.Failure, err error) error {
	return &failure{code: code, err: err}
}

// jobAttrs are a job's log attributes: which, where, by whom, from where.
func jobAttrs(job domain.Job) []any {
	return []any{slog.String("job_id", job.ID.String()), slog.String("notebook_id", job.NotebookID.String()),
		slog.String("user_id", job.CreatedBy.String()), slog.String("client", string(job.Client))}
}

// authorizeJob decides action again, as job runs, for its starter acting
// through it: an account that can no longer see the notebook, or whose
// role the rule does not allow, fails it. A notebook deleted meanwhile
// deleted the job too.
func authorizeJob(ctx context.Context, notebooks Notebooks, authorizer shared.Authorizer, job domain.Job, action shared.Action) error {
	workspaceID, ok, err := notebooks.WorkspaceOf(ctx, job.NotebookID)
	switch {
	case err != nil:
		return err
	case !ok:
		return errGone
	}
	actor := shared.Actor{UserID: job.CreatedBy, JobID: job.ID}
	_, err = authorizer.Authorize(ctx, actor, action, shared.Target{WorkspaceID: workspaceID, NotebookID: job.NotebookID})
	var denied *shared.Error
	if errors.Is(err, shared.ErrNotVisible) || errors.As(err, &denied) && denied.Kind == shared.KindForbidden {
		return failed(domain.FailureForbidden, err)
	}
	return err
}

// jobRun is a job as it runs.
type jobRun struct {
	rows   Rows
	clock  Clock
	logger *slog.Logger
	// every is how often the heartbeat is written: a second.
	every time.Duration
	job   domain.Job
	done  atomic.Int64
	all   atomic.Int64
	// report and name are the job's as it goes: its counts and problems,
	// and its name, which an export's snapshot may change.
	report domain.Report
	name   string
}

// newJobRun returns job's run, writing its rows, its heartbeat every
// every.
func newJobRun(job domain.Job, rows Rows, clock Clock, logger *slog.Logger, every time.Duration) *jobRun {
	return &jobRun{rows: rows, clock: clock, logger: logger, every: every, job: job, name: job.Name}
}

// progress is the run's progress now.
func (r *jobRun) progress() domain.Progress {
	return domain.Progress{Done: r.done.Load(), Total: r.all.Load()}
}

// beat writes the job's heartbeat and progress every beat until ctx ends,
// and stops the run when the job's cancel was asked, or it was deleted or
// no longer runs. A write that fails is tried again at the next beat: a
// job whose heartbeat stays old is failed by the rescue. The first failure
// of a run of them is logged, and the write that ends it. The channel
// closes as it returns.
func (r *jobRun) beat(ctx context.Context, stop context.CancelCauseFunc) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(r.every)
		defer ticker.Stop()
		failing := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			b, err := r.rows.BeatJob(ctx, r.job.ID, r.clock.Now(), r.progress())
			if err != nil && !errors.Is(err, ErrNoRow) {
				if !failing && ctx.Err() == nil {
					r.logger.WarnContext(ctx, string(r.job.Kind)+" heartbeat not written", slog.String("job_id", r.job.ID.String()),
						slog.Any("error", err))
				}
				failing = true
				continue
			}
			if failing && err == nil {
				r.logger.InfoContext(ctx, string(r.job.Kind)+" heartbeat written again", slog.String("job_id", r.job.ID.String()))
			}
			failing = false
			switch {
			case errors.Is(err, ErrNoRow) || b.Deleted:
				stop(errGone)
			case b.CancelRequested:
				stop(errCancelRequested)
			}
		}
	}()
	return done
}

// finish writes the run's end, err what stopped it: a success, nil err,
// with its report, or as succeed writes it when it is set; a job deleted,
// or no longer running, gets nothing; a cancel asked ends it cancelled;
// anything else failed, its failure in its report. River's context ends
// the job as a timeout or, as the server stops, an interruption. The end
// is written within finishWait, or finishStopping as the server stops.
func (r *jobRun) finish(ctx, running context.Context, err error, attrs []any, succeed func(end context.Context, e Ended) error) error {
	wait := finishWait
	if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		wait = finishStopping
	}
	end, cancel := context.WithTimeout(context.WithoutCancel(ctx), wait)
	defer cancel()
	e := Ended{State: domain.StateSucceeded, At: r.clock.Now(), Name: r.name, Progress: r.progress()}
	if err == nil && succeed != nil {
		return succeed(end, e)
	}
	kind := string(r.job.Kind)
	if err != nil {
		cause := context.Cause(running)
		switch {
		case errors.Is(cause, errGone) || errors.Is(err, errGone):
			r.logger.InfoContext(end, kind+" stopped: the job was deleted, or no longer runs", attrs...)
			return nil
		case errors.Is(cause, errCancelRequested):
			e.State = domain.StateCancelled
		default:
			e.State, e.Report.Failure = domain.StateFailed, r.failureOf(ctx, err)
		}
	}
	report := r.report
	report.Failure = e.Report.Failure
	e.Report = report
	ok, writeErr := r.rows.FinishJob(end, r.job.ID, e)
	if writeErr != nil {
		return fmt.Errorf("write the end of %s %s: %w", kind, r.job.ID, writeErr)
	}
	if !ok {
		r.logger.InfoContext(end, kind+" stopped: the job was deleted, or no longer runs", attrs...)
		return nil
	}
	level := slog.LevelInfo
	if e.Report.Failure == domain.FailureInternal {
		level = slog.LevelError
	}
	r.logger.Log(end, level, kind+" ended", append(attrs, slog.String("state", string(e.State)),
		slog.String("failure", string(e.Report.Failure)), slog.Int64("nodes", e.Progress.Done), slog.Int64("skipped", e.Report.Counts.Skipped),
		slog.Any("error", err))...)
	return nil
}

// failureOf is why the run failed: River's context ended by its timeout or
// by the server's stop, or the run's own failure; any other is internal.
func (r *jobRun) failureOf(ctx context.Context, err error) domain.Failure {
	var f *failure
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return domain.FailureTimeout
	case ctx.Err() != nil:
		return domain.FailureInterrupted
	case errors.As(err, &f):
		return f.code
	}
	return domain.FailureInternal
}
