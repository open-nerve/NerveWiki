// Package riveradapter works the asset module's background jobs on River
// (M7/P2 design 3.8): each worker only runs a use case.
package riveradapter

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// SweepKind is the kind of the job that deletes the files no row holds,
// and the id of its schedule.
const SweepKind = "asset.sweep_orphan_files"

// SweepInterval is how often the sweep runs.
const SweepInterval = 24 * time.Hour

// SweepTimeout bounds a run: River's minute would cut a walk of a large
// store short, and the next run walks it from the start again. It stays
// under the hour after which River takes a job still running for stuck
// and runs it again (RescueStuckJobsAfter, which platform/jobs leaves at
// its default).
const SweepTimeout = 50 * time.Minute

// SweepUseCase is the use case the job runs: app.Sweep.
type SweepUseCase interface {
	Run(ctx context.Context) (int, error)
}

// SweepArgs are the job's arguments: none.
type SweepArgs struct{}

// Kind is SweepKind.
func (SweepArgs) Kind() string { return SweepKind }

// SweepWorker runs the sweep.
type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	uc SweepUseCase
}

// NewSweepWorker returns the worker of uc.
func NewSweepWorker(uc SweepUseCase) *SweepWorker {
	return &SweepWorker{uc: uc}
}

// Timeout is SweepTimeout.
func (*SweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration { return SweepTimeout }

// Work deletes the files no row holds. A failure makes River retry the
// job.
func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	_, err := w.uc.Run(ctx)
	return err
}

// SweepJob is the periodic sweep: when the server starts, then every
// SweepInterval.
func SweepJob(uc SweepUseCase) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, NewSweepWorker(uc)) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: SweepKind, RunOnStart: true}),
	}
}
