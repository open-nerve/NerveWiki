// Package riveradapter works the page module's background jobs on River
// (M4/P4 design 3.5): each worker only runs a use case.
package riveradapter

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// CleanupKind is the kind of the job that deletes the expired edit
// sessions, and the id of its schedule.
const CleanupKind = "page.cleanup_expired_edit_sessions"

// CleanupUseCase is the use case the job runs: app.CleanupEditSessions.
type CleanupUseCase interface {
	Execute(ctx context.Context) (int, error)
}

// CleanupArgs are the job's arguments: none.
type CleanupArgs struct{}

// Kind is CleanupKind.
func (CleanupArgs) Kind() string { return CleanupKind }

// CleanupWorker runs the cleanup.
type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	uc CleanupUseCase
}

// NewCleanupWorker returns the worker of uc.
func NewCleanupWorker(uc CleanupUseCase) *CleanupWorker {
	return &CleanupWorker{uc: uc}
}

// Work deletes the expired edit sessions. A failure makes River retry the job.
func (w *CleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	_, err := w.uc.Execute(ctx)
	return err
}

// CleanupJob is the periodic cleanup: when the server starts, then every
// interval (page.edit_session_cleanup_interval).
func CleanupJob(uc CleanupUseCase, interval time.Duration) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, NewCleanupWorker(uc)) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(interval),
			func() (river.JobArgs, *river.InsertOpts) { return CleanupArgs{}, nil },
			&river.PeriodicJobOpts{ID: CleanupKind, RunOnStart: true}),
	}
}
