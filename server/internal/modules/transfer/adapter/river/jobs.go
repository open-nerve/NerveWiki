// Package riveradapter works the transfer module's background jobs on
// River (M7/P5 design 3.2, 3.8, 3.12): each worker only runs a use case.
// The exports run in a queue of their own, each its job's timeout, one
// attempt; the request enqueues them in its own transaction.
package riveradapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The kinds of the module's jobs, and the ids of the periodic ones'
// schedules.
const (
	ExportKind = "transfer.export"
	RescueKind = "transfer.rescue_interrupted"
	ExpireKind = "transfer.expire_exports"
	SweepKind  = "transfer.sweep_orphan_archives"
)

// QueueExport is the exports' queue: a few hours' export holds neither the
// periodic jobs nor an import.
const QueueExport = "transfer_export"

// The periodic jobs' intervals and timeouts. The sweep's stays under the
// rescue River gives any job, its timeout and an hour.
const (
	RescueInterval = 5 * time.Minute
	ExpireInterval = 15 * time.Minute
	ExpireTimeout  = 10 * time.Minute
	SweepInterval  = 24 * time.Hour
	SweepTimeout   = 50 * time.Minute
)

// ExportArgs are an export's job's arguments: its row's id.
type ExportArgs struct {
	JobID uuid.UUID `json:"job_id"`
}

// Kind is ExportKind.
func (ExportArgs) Kind() string { return ExportKind }

// ExportUseCase is the use case the export's worker runs: app.Export.
type ExportUseCase interface {
	Run(ctx context.Context, id uuid.UUID) error
}

// exportWorker runs an export, within timeout.
type exportWorker struct {
	river.WorkerDefaults[ExportArgs]
	uc      ExportUseCase
	timeout time.Duration
}

// Timeout is transfer.job_timeout.
func (w *exportWorker) Timeout(*river.Job[ExportArgs]) time.Duration { return w.timeout }

// Work runs the export. Its one attempt spent, a failure is not tried
// again: the job's row tells it.
func (w *exportWorker) Work(ctx context.Context, j *river.Job[ExportArgs]) error {
	return w.uc.Run(ctx, j.Args.JobID)
}

// ExportJob is the exports' worker, each run within timeout.
func ExportJob(uc ExportUseCase, timeout time.Duration) jobs.Job {
	return jobs.Job{Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, &exportWorker{uc: uc, timeout: timeout}) }}
}

// Queue enqueues the exports in the caller's transaction, with the
// insert-only client, and tells which River still holds.
type Queue struct {
	inserter *jobs.Inserter
}

// NewQueue returns the queue of inserter.
func NewQueue(inserter *jobs.Inserter) Queue {
	return Queue{inserter: inserter}
}

// Export enqueues the export id in the exports' queue, one attempt, in the
// transaction ctx carries: committed with the job's row or rolled back
// with it.
func (q Queue) Export(ctx context.Context, id uuid.UUID) error {
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		return errors.New("enqueue an export: not in a transaction")
	}
	return q.inserter.InsertTx(ctx, tx, ExportArgs{JobID: id}, &river.InsertOpts{Queue: QueueExport, MaxAttempts: 1})
}

// Held is the ids of the exports River has not finished.
func (q Queue) Held(ctx context.Context) ([]uuid.UUID, error) {
	args, err := q.inserter.Unfinished(ctx, ExportKind)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(args))
	for _, a := range args {
		var x ExportArgs
		if err := json.Unmarshal(a, &x); err != nil {
			return nil, fmt.Errorf("an export's River job: %w", err)
		}
		ids = append(ids, x.JobID)
	}
	return ids, nil
}

// RescueUseCase is the rescue's use case: app.Rescue.
type RescueUseCase interface {
	AtStart(ctx context.Context) error
	Run(ctx context.Context) error
}

type rescueArgs struct{}

func (rescueArgs) Kind() string { return RescueKind }

type rescueWorker struct {
	river.WorkerDefaults[rescueArgs]
	uc RescueUseCase
}

func (w *rescueWorker) Work(ctx context.Context, _ *river.Job[rescueArgs]) error {
	return w.uc.Run(ctx)
}

// RescueJob fails the interrupted jobs: every running one as the server
// starts, before River works any job; then every RescueInterval those
// whose heartbeat is old.
func RescueJob(uc RescueUseCase) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, &rescueWorker{uc: uc}) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(RescueInterval),
			func() (river.JobArgs, *river.InsertOpts) { return rescueArgs{}, nil },
			&river.PeriodicJobOpts{ID: RescueKind}),
		Start: uc.AtStart,
	}
}

// CountUseCase is a periodic use case that tells what it did: app.Expire,
// app.Sweep.
type CountUseCase interface {
	Run(ctx context.Context) (int, error)
}

type expireArgs struct{}

func (expireArgs) Kind() string { return ExpireKind }

type expireWorker struct {
	river.WorkerDefaults[expireArgs]
	uc CountUseCase
}

func (*expireWorker) Timeout(*river.Job[expireArgs]) time.Duration { return ExpireTimeout }

func (w *expireWorker) Work(ctx context.Context, _ *river.Job[expireArgs]) error {
	_, err := w.uc.Run(ctx)
	return err
}

// ExpireJob expires the exports past transfer.export_ttl: when the server
// starts, then every ExpireInterval.
func ExpireJob(uc CountUseCase) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, &expireWorker{uc: uc}) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(ExpireInterval),
			func() (river.JobArgs, *river.InsertOpts) { return expireArgs{}, nil },
			&river.PeriodicJobOpts{ID: ExpireKind, RunOnStart: true}),
	}
}

type sweepArgs struct{}

func (sweepArgs) Kind() string { return SweepKind }

type sweepWorker struct {
	river.WorkerDefaults[sweepArgs]
	uc CountUseCase
}

func (*sweepWorker) Timeout(*river.Job[sweepArgs]) time.Duration { return SweepTimeout }

func (w *sweepWorker) Work(ctx context.Context, _ *river.Job[sweepArgs]) error {
	_, err := w.uc.Run(ctx)
	return err
}

// SweepJob deletes the archives no job keeps: when the server starts, then
// every SweepInterval.
func SweepJob(uc CountUseCase) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, &sweepWorker{uc: uc}) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(SweepInterval),
			func() (river.JobArgs, *river.InsertOpts) { return sweepArgs{}, nil },
			&river.PeriodicJobOpts{ID: SweepKind, RunOnStart: true}),
	}
}
