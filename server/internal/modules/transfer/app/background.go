package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Background runs the background restores and reverts (M8/P4 design
// 3.6): River's workers of the two kinds call Run with the job's id. The
// page module plans the job and writes its batches; the run cuts the
// plan's steps into batches as an import cuts its nodes, and stops
// between two when its cancel was asked.
type Background struct {
	d BackgroundRunDeps
}

// BackgroundRunDeps are what Background needs.
type BackgroundRunDeps struct {
	Tree Tree
	// Statistics are the page, linking and asset modules', in this order.
	Statistics []Analyzer
	Rows       Rows
	Clock      Clock
	Logger     *slog.Logger
	// Beat is how often a running job writes its heartbeat and progress,
	// and reads whether it was cancelled: a second.
	Beat time.Duration
	// Backoff is the first wait before a batch the server was too busy
	// for is tried again: a second, doubled each time up to maxBackoff.
	Backoff time.Duration
	// Batch bounds a batch: zero is the import's, which production keeps.
	Batch Batch
}

// NewBackground returns the use case.
func NewBackground(d BackgroundRunDeps) *Background {
	return &Background{d: d}
}

// Run runs the restore or the revert id, its starter acting through it. A
// job no longer queued, cancelled or deleted meanwhile, is left as it is.
// A running one plans, then writes its batches, its heartbeat every Beat
// stopping it between two when its cancel was asked or it was deleted; it
// ends succeeded, cancelled or failed with its report, what it wrote
// kept. Run fails only when the job's end cannot be written: the rescue
// fails it later.
func (b *Background) Run(ctx context.Context, id uuid.UUID) error {
	job, err := b.d.Rows.StartJob(ctx, id, b.d.Clock.Now())
	if errors.Is(err, ErrNoRow) {
		return nil
	}
	if err != nil {
		return err
	}
	attrs := logged(job)
	b.d.Logger.InfoContext(ctx, string(job.Kind)+" started", attrs...)
	ctx = shared.WithActor(ctx, shared.Actor{UserID: job.CreatedBy, JobID: job.ID})
	r := &backgroundRun{jobRun: newJobRun(job, b.d.Rows, b.d.Clock, b.d.Logger, b.d.Beat), b: b, bounds: b.d.Batch.bounds(),
		stats: &statistics{analyzers: b.d.Statistics, logger: b.d.Logger, job: job}}
	running, stop := context.WithCancelCause(ctx)
	beating := r.beat(running, stop)
	err = r.run(ctx, running)
	r.stats.end(ctx)
	stop(nil)
	<-beating
	return r.finish(ctx, running, err, append(attrs, r.counted()...), nil)
}

// backgroundRun is a restore or a revert as it runs.
type backgroundRun struct {
	*jobRun
	b      *Background
	bounds Batch
	stats  *statistics
}

// run plans the job and writes its batches: running is River's context as
// the heartbeat may stop it, which every step but a batch's unit heeds.
func (r *backgroundRun) run(ctx, running context.Context) error {
	if r.job.ChangesetID == nil {
		return fmt.Errorf("%s %s without its changeset", r.job.Kind, r.job.ID)
	}
	spec := JobSpec{NotebookID: r.job.NotebookID, Client: r.job.Client, Changeset: *r.job.ChangesetID, Message: r.job.Message}
	var plan JobPlan
	var err error
	switch {
	case r.job.Kind == domain.KindTrashRestore && r.job.RootID != nil:
		plan, err = r.b.d.Tree.PlanRestore(running, spec, *r.job.RootID)
	case r.job.Kind == domain.KindRevert && r.job.TargetChangesetID != nil:
		plan, err = r.b.d.Tree.PlanRevert(running, spec, *r.job.TargetChangesetID)
	default:
		return fmt.Errorf("a background job %s of kind %q", r.job.ID, r.job.Kind)
	}
	if err != nil {
		return r.stopped(err)
	}
	steps, total := plan.Steps(), int64(plan.Total())
	r.all.Store(total)
	r.publish()
	for from := 0; from < len(steps); {
		if running.Err() != nil {
			return context.Cause(running)
		}
		to, done, err := r.batch(ctx, running, spec, plan, from)
		if err != nil {
			return r.stopped(err)
		}
		nodes := int64(0)
		for _, s := range steps[from:to] {
			nodes += int64(s.Nodes)
		}
		r.done.Store(min(r.done.Load()+nodes, total))
		r.count(done)
		r.publish()
		r.stats.wrote(ctx, done.Nodes)
		from = to
	}
	return nil
}

// batch writes the batch from the plan's step from: its steps up to the
// bounds' nodes and bytes, the first always in it, as RunBatch writes
// them, with River's context ctx, which the heartbeat's stop does not
// end. One the server is too busy for is tried again after a wait; one
// past a unit's history or links, halved, at least a step. It answers
// where the next starts.
func (r *backgroundRun) batch(ctx, running context.Context, spec JobSpec, plan JobPlan, from int) (int, BatchDone, error) {
	steps := plan.Steps()
	to, nodes, bytes := from+1, steps[from].Nodes, steps[from].Bytes
	for ; to < len(steps) && nodes+steps[to].Nodes <= r.bounds.Nodes && bytes+steps[to].Bytes <= r.bounds.Bytes; to++ {
		nodes, bytes = nodes+steps[to].Nodes, bytes+steps[to].Bytes
	}
	for {
		var done BatchDone
		err := retry(running, r.b.d.Backoff, func() error {
			if running.Err() != nil {
				return context.Cause(running)
			}
			var err error
			done, err = r.b.d.Tree.RunBatch(ctx, spec, plan, from, to)
			return err
		})
		if errors.Is(err, ErrBatchTooLarge) && to-from > 1 {
			to = from + (to-from)/2
			continue
		}
		return to, done, err
	}
}

// count adds what a batch did to the report.
func (r *backgroundRun) count(done BatchDone) {
	c := &r.report.Counts
	c.Pages, c.Attachments, c.Renamed, c.Skipped = c.Pages+int64(done.Pages), c.Attachments+int64(done.Assets),
		c.Renamed+int64(done.Renamed), c.Skipped+int64(done.Skipped)
	c.Restored, c.Deleted, c.PutBack, c.Written = c.Restored+int64(done.Restored), c.Deleted+int64(done.Deleted),
		c.PutBack+int64(done.PutBack), c.Written+int64(done.Written)
	if done.Placed != nil {
		r.report.Placed = done.Placed
	}
}

// counted are the report's counts as the end's log tells them.
func (r *backgroundRun) counted() []any {
	c := r.report.Counts
	return []any{slog.Int64("pages", c.Pages), slog.Int64("attachments", c.Attachments), slog.Int64("renamed", c.Renamed),
		slog.Int64("restored", c.Restored), slog.Int64("deleted", c.Deleted), slog.Int64("put_back", c.PutBack),
		slog.Int64("written", c.Written)}
}

// stopped is why the job stopped at err, as its report tells it (M8/P4
// design 3.6): the page module's sentinels each their failure, a revert's
// conflicts and the lock it met in the report; a unit refused as an
// import's are; any other internal.
func (r *backgroundRun) stopped(err error) error {
	var problem *shared.Error
	errors.As(err, &problem)
	switch {
	case errors.Is(err, ErrTrashChanged):
		return failed(domain.FailureTrashChanged, err)
	case errors.Is(err, ErrRestoreExpired):
		return failed(domain.FailureRestoreExpired, err)
	case errors.Is(err, ErrNoParent) || errors.Is(err, ErrTooDeep):
		return failed(domain.FailureTreeChanged, err)
	case errors.Is(err, ErrRevertConflict):
		if problem != nil {
			for _, c := range problem.Conflicts[:min(len(problem.Conflicts), domain.MaxConflicts)] {
				r.report.Conflicts = append(r.report.Conflicts, domain.Conflict{NodeID: c.NodeID, Name: c.Name, Reason: c.Reason,
					HolderID: c.HolderID})
			}
			if problem.ConflictCount != nil {
				r.report.ConflictCount = *problem.ConflictCount
			}
		}
		return failed(domain.FailureRevertConflict, err)
	case errors.Is(err, ErrChangesetTrimmed):
		return failed(domain.FailureChangesetTrimmed, err)
	case errors.Is(err, ErrLocked):
		if problem != nil && problem.Lock != nil {
			r.report.Lock = &domain.Lock{PageID: problem.Lock.PageID, UserID: problem.Lock.UserID}
		}
		return failed(domain.FailurePageLocked, err)
	}
	return unitFailure(err)
}
