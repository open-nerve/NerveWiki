package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// Purger deletes one table's soft-deleted rows for the purge (M2/P4 design
// 3.4). Every table with deleted_at has one (v0.1 design 13.1, item 6);
// its module provides it, and bootstrap lists them from leaf to root.
type Purger struct {
	// Table is the table it purges.
	Table string
	// Purge deletes up to batch rows deleted before before, and returns how
	// many it deleted. It skips the rows another transaction holds. Within
	// its module, it skips those still referenced by rows a purger before it
	// skipped: a foreign key's ON DELETE CASCADE would wait for them. Its
	// queries cannot see another module's tables, so a key from one is ON
	// DELETE RESTRICT: while such a row is left, the batch fails, and a
	// later run deletes it (v0.1 design 13.1, item 6).
	Purge func(ctx context.Context, before time.Time, batch int) (int, error)
}

// PurgeConfig is the purge's schedule and retention.
type PurgeConfig struct {
	// Interval is jobs.purge_interval.
	Interval time.Duration
	// Retention is jobs.purge_retention: how long a soft-deleted row stays.
	Retention time.Duration
	Logger    *slog.Logger
}

// PurgeKind is the kind of the purge's job, and the id of its schedule.
const PurgeKind = "platform.purge_soft_deleted"

// purgeBatch is the most rows one statement deletes: each batch is a short
// statement of its own, however many rows have outlived the retention.
const purgeBatch = 1000

// PurgeJob is the periodic purge: when the server starts, then every
// cfg.Interval. purgers come leaf to root, the tables that reference
// another before it.
func PurgeJob(purgers []Purger, cfg PurgeConfig) Job {
	w := &purgeWorker{purgers: purgers, cfg: cfg}
	return Job{
		Add: func(workers *river.Workers) error { return river.AddWorkerSafely(workers, w) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(cfg.Interval),
			func() (river.JobArgs, *river.InsertOpts) { return purgeArgs{}, nil },
			&river.PeriodicJobOpts{ID: PurgeKind, RunOnStart: true}),
	}
}

// purgeArgs are the purge job's arguments: none.
type purgeArgs struct{}

func (purgeArgs) Kind() string { return PurgeKind }

// purgeWorker runs the purge.
type purgeWorker struct {
	river.WorkerDefaults[purgeArgs]
	purgers []Purger
	cfg     PurgeConfig
}

// Work purges the rows deleted longer than the retention ago. A failure
// makes River retry the job.
func (w *purgeWorker) Work(ctx context.Context, _ *river.Job[purgeArgs]) error {
	return purge(ctx, w.purgers, time.Now().Add(-w.cfg.Retention), w.cfg.Logger)
}

// purge runs the purgers in order and logs what each deleted. The first
// failure stops it before the purgers after, whose tables the failed one's
// rows may still reference.
func purge(ctx context.Context, purgers []Purger, before time.Time, logger *slog.Logger) error {
	for _, p := range purgers {
		deleted, err := purgeTable(ctx, p, before)
		if deleted > 0 {
			logger.InfoContext(ctx, "soft-deleted rows purged", slog.String("table", p.Table), slog.Int("rows", deleted))
		}
		if err != nil {
			return fmt.Errorf("purge %s: %w", p.Table, err)
		}
	}
	return nil
}

// purgeTable runs p batch after batch until one comes back short, and
// returns how many rows it deleted: those of the batches before a failure
// too, which are gone.
func purgeTable(ctx context.Context, p Purger, before time.Time) (int, error) {
	deleted := 0
	for {
		n, err := p.Purge(ctx, before, purgeBatch)
		deleted += n
		if err != nil || n < purgeBatch {
			return deleted, err
		}
	}
}
