package riveradapter

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"
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
