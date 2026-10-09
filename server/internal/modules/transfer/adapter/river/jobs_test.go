package riveradapter

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// runs records the exports it ran.
type runs struct{ ids []uuid.UUID }

func (r *runs) Run(_ context.Context, id uuid.UUID) error {
	r.ids = append(r.ids, id)
	return nil
}

// The export's worker runs the use case on its job's row, within
// transfer.job_timeout rather than River's default.
func TestExportWorkerRunsTheRowWithinTheJobTimeout(t *testing.T) {
	uc := &runs{}
	w := &exportWorker{uc: uc, timeout: 6 * time.Hour}
	id := uuid.NewV7()
	job := &river.Job[ExportArgs]{Args: ExportArgs{JobID: id}}
	if got := w.Timeout(job); got != 6*time.Hour {
		t.Errorf("Timeout = %v, want 6h", got)
	}
	if err := w.Work(context.Background(), job); err != nil || len(uc.ids) != 1 || uc.ids[0] != id {
		t.Errorf("Work = %v, ran %v; want %s", err, uc.ids, id)
	}
	if err := river.AddWorkerSafely(river.NewWorkers(), w); err != nil {
		t.Errorf("AddWorkerSafely = %v", err)
	}
}

// An export is enqueued in its row's transaction alone: outside one, the
// queue refuses before it reaches River.
func TestQueueRefusesOutsideATransaction(t *testing.T) {
	if err := (Queue{}).Export(context.Background(), uuid.NewV7()); err == nil {
		t.Error("Export outside a transaction = nil, want an error")
	}
}

// The queue enqueues an export in its transaction, in the exports' queue,
// one attempt; it holds those River has not finished: not one River
// discarded, nor another kind's job.
func TestTheQueueHoldsTheExportsRiverHasNotFinished(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	inserter, err := jobs.NewInserter(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	q, tx := NewQueue(inserter), postgres.NewTxManager(pool, 5*time.Second)
	kept, dropped := uuid.NewV7(), uuid.NewV7()
	err = tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := q.Export(ctx, kept); err != nil {
			return err
		}
		if err := q.Export(ctx, dropped); err != nil {
			return err
		}
		tx, _ := postgres.TxFrom(ctx)
		return inserter.InsertTx(ctx, tx, expireArgs{}, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	var n, otherwise int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE queue <> $1 OR max_attempts <> 1) FROM river_job WHERE kind = $2`,
		QueueExport, ExportKind).Scan(&n, &otherwise); err != nil || n != 2 || otherwise != 0 {
		t.Errorf("%d exports' jobs, %d not in their queue with one attempt, %v; want 2, none", n, otherwise, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE river_job SET state = 'discarded', attempt = 1, attempted_at = now(), finalized_at = now()
		WHERE args->>'job_id' = $1`, dropped.String()); err != nil {
		t.Fatal(err)
	}

	held, err := q.Held(ctx)
	if err != nil || !slices.Equal(held, []uuid.UUID{kept}) {
		t.Errorf("Held() = %v, %v; want the kept export alone", held, err)
	}
}
