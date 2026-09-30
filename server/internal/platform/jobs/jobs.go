// Package jobs runs the modules' background jobs on River (M1/P4 design
// 3.3): the server's client, which works the jobs and enqueues the periodic
// ones. It imports River and pgx, and no other platform package.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Job is one kind of job a module works: its worker and, when the job is
// periodic, its schedule. A module's river adapter builds it.
type Job struct {
	// Add registers the job's worker, e.g. with river.AddWorkerSafely.
	Add func(*river.Workers) error
	// Periodic, when set, enqueues the job on its schedule. River's leader
	// enqueues it, so one runs per schedule however many servers there are.
	Periodic *river.PeriodicJob
}

// Config is the runner's settings.
type Config struct {
	// ShutdownTimeout is how long Stop lets the running jobs finish before
	// it cancels their contexts: jobs.shutdown_timeout.
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

const (
	// maxWorkers is how many jobs run at once. M1 has one periodic job.
	maxWorkers = 2
	// cancelGrace is how long Stop waits, once ShutdownTimeout has passed and
	// the jobs' contexts are cancelled, for the jobs to return.
	cancelGrace = time.Second
)

// client is what the runner uses of *river.Client.
type client interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Runner is the server's River client.
type Runner struct {
	client          client
	logger          *slog.Logger
	shutdownTimeout time.Duration

	started bool
	// release cancels the context River started with, once Stop has
	// stopped it.
	release context.CancelFunc
}

// New builds the client on pool for jobs; Start runs it.
func New(pool *pgxpool.Pool, cfg Config, jobs []Job) (*Runner, error) {
	workers := river.NewWorkers()
	var periodic []*river.PeriodicJob
	for _, j := range jobs {
		if err := j.Add(workers); err != nil {
			return nil, fmt.Errorf("register a job: %w", err)
		}
		if j.Periodic != nil {
			periodic = append(periodic, j.Periodic)
		}
	}
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:       cfg.Logger,
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers:      workers,
		PeriodicJobs: periodic,
		// Stopping lets the running jobs finish for this long, then cancels
		// their contexts.
		SoftStopTimeout: cfg.ShutdownTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create the jobs client: %w", err)
	}
	return newRunner(c, cfg), nil
}

func newRunner(c client, cfg Config) *Runner {
	return &Runner{client: c, logger: cfg.Logger, shutdownTimeout: cfg.ShutdownTimeout}
}

// Start starts the client. ctx bounds the start alone: cancelling it while
// River's Start runs (a SELECT 1 that the database never answers) ends the
// start, so that it cannot hold the shutdown. Once started, the client runs
// until Stop, whatever becomes of ctx: Stop stops it with the client's Stop
// alone, and River then cancels its own contexts with its stop cause, which
// its services take as a stop and clean up after (the reindexer drops the
// index that an interrupted REINDEX CONCURRENTLY leaves behind). A ctx
// cancelled just as River's Start returns still cancels the context River
// runs with: interrupting a start under way cannot avoid that window.
//
// Call Start once; call Stop after Start has returned.
func (r *Runner) Start(ctx context.Context) error {
	running, release := context.WithCancel(context.WithoutCancel(ctx))
	unwatch := context.AfterFunc(ctx, release)
	err := r.client.Start(running)
	unwatch()
	if err != nil {
		release()
		return fmt.Errorf("start the jobs: %w", err)
	}
	r.started, r.release = true, release
	r.logger.InfoContext(ctx, "jobs started", slog.Duration("shutdown_timeout", r.shutdownTimeout))
	return nil
}

// Stop stops the client when it started: no job is fetched any more, the
// running ones get ShutdownTimeout to finish and then their contexts are
// cancelled. It returns once they have returned, or with an error when they
// still run cancelGrace after that.
func (r *Runner) Stop(ctx context.Context) error {
	if !r.started {
		return nil
	}
	defer r.release()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.shutdownTimeout+cancelGrace)
	defer cancel()
	if err := r.client.Stop(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("jobs still running %s after jobs.shutdown_timeout (%s): %w", cancelGrace, r.shutdownTimeout, err)
		}
		return err
	}
	r.logger.InfoContext(ctx, "jobs stopped")
	return nil
}
