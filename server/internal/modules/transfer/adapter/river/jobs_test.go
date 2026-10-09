package riveradapter

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// runs records the jobs it ran.
type runs struct{ ids []uuid.UUID }

func (r *runs) Run(_ context.Context, id uuid.UUID) error {
	r.ids = append(r.ids, id)
	return nil
}

// The export's and the import's workers run the use case on their job's
// row, within transfer.job_timeout rather than River's default.
func TestTheWorkersRunTheRowWithinTheJobTimeout(t *testing.T) {
	uc := &runs{}
	exports, imports := &jobWorker[ExportArgs]{uc: uc, timeout: 6 * time.Hour}, &jobWorker[ImportArgs]{uc: uc, timeout: 6 * time.Hour}
	exported, imported := uuid.NewV7(), uuid.NewV7()
	export, imp := &river.Job[ExportArgs]{Args: ExportArgs{JobID: exported}}, &river.Job[ImportArgs]{Args: ImportArgs{JobID: imported}}
	if a, b := exports.Timeout(export), imports.Timeout(imp); a != 6*time.Hour || b != 6*time.Hour {
		t.Errorf("Timeout = %v, %v; want 6h", a, b)
	}
	if err := exports.Work(context.Background(), export); err != nil {
		t.Fatal(err)
	}
	if err := imports.Work(context.Background(), imp); err != nil || !slices.Equal(uc.ids, []uuid.UUID{exported, imported}) {
		t.Errorf("Work = %v, ran %v; want %s, %s", err, uc.ids, exported, imported)
	}
	w := river.NewWorkers()
	if err := ExportJob(uc, time.Hour).Add(w); err != nil {
		t.Errorf("ExportJob's Add = %v", err)
	}
	if err := ImportJob(uc, time.Hour).Add(w); err != nil {
		t.Errorf("ImportJob's Add = %v", err)
	}
}

// A job is enqueued in its row's transaction alone: outside one, the
// queue refuses before it reaches River.
func TestQueueRefusesOutsideATransaction(t *testing.T) {
	if err := (Queue{}).Export(context.Background(), uuid.NewV7()); err == nil {
		t.Error("Export outside a transaction = nil, want an error")
	}
	if err := (Queue{}).Import(context.Background(), uuid.NewV7()); err == nil {
		t.Error("Import outside a transaction = nil, want an error")
	}
}

// The queue enqueues an export or an import in its transaction, in its
// kind's queue, one attempt; it holds those of a kind River has not
// finished: not one River discarded, nor another kind's job.
func TestTheQueueHoldsTheJobsRiverHasNotFinished(t *testing.T) {
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
	kept, dropped, imported := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	err = tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := q.Export(ctx, kept); err != nil {
			return err
		}
		if err := q.Export(ctx, dropped); err != nil {
			return err
		}
		if err := q.Import(ctx, imported); err != nil {
			return err
		}
		tx, _ := postgres.TxFrom(ctx)
		return inserter.InsertTx(ctx, tx, expireArgs{}, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[string]struct {
		queue string
		n     int
	}{ExportKind: {QueueExport, 2}, ImportKind: {QueueImport, 1}} {
		var n, otherwise int
		if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE queue <> $1 OR max_attempts <> 1) FROM river_job WHERE kind = $2`,
			want.queue, kind).Scan(&n, &otherwise); err != nil || n != want.n || otherwise != 0 {
			t.Errorf("%d jobs of %s, %d not in their queue with one attempt, %v; want %d, none", n, kind, otherwise, err, want.n)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE river_job SET state = 'discarded', attempt = 1, attempted_at = now(), finalized_at = now()
		WHERE args->>'job_id' = $1`, dropped.String()); err != nil {
		t.Fatal(err)
	}

	held, err := q.Held(ctx, domain.KindExport)
	if err != nil || !slices.Equal(held, []uuid.UUID{kept}) {
		t.Errorf("Held(export) = %v, %v; want the kept export alone", held, err)
	}
	held, err = q.Held(ctx, domain.KindImport)
	if err != nil || !slices.Equal(held, []uuid.UUID{imported}) {
		t.Errorf("Held(import) = %v, %v; want the import alone", held, err)
	}
}
