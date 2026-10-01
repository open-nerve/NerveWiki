package jobs

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakePurger answers each call with the next of counts, then 0 and err,
// and records the calls in calls.
type fakePurger struct {
	table  string
	counts []int
	err    error
	calls  *[]string
	seen   []time.Time
}

func (f *fakePurger) purger() Purger {
	return Purger{Table: f.table, Purge: func(_ context.Context, before time.Time, batch int) (int, error) {
		*f.calls = append(*f.calls, f.table)
		f.seen = append(f.seen, before)
		if batch != purgeBatch {
			return 0, errors.New("not the purge's batch")
		}
		if len(f.counts) == 0 {
			return 0, f.err
		}
		n := f.counts[0]
		f.counts = f.counts[1:]
		return n, nil
	}}
}

// The purgers run in their order, each batch after batch until one comes
// back short, all with the one cutoff; what a purger deleted is logged.
func TestPurgeRunsThePurgersInOrder(t *testing.T) {
	var calls []string
	children := &fakePurger{table: "children", counts: []int{purgeBatch, 3}, calls: &calls}
	parents := &fakePurger{table: "parents", calls: &calls}
	var logs logBuffer
	before := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	err := purge(context.Background(), []Purger{children.purger(), parents.purger()}, before, newLogger(&logs))

	if err != nil || !slices.Equal(calls, []string{"children", "children", "parents"}) {
		t.Errorf("purge() = %v after %q; want the children twice, then the parents", err, calls)
	}
	for _, at := range slices.Concat(children.seen, parents.seen) {
		if !at.Equal(before) {
			t.Errorf("a purger was asked for rows before %v, want %v", at, before)
		}
	}
	if got := logs.String(); !strings.Contains(got, `msg="soft-deleted rows purged" table=children rows=1003`) || strings.Contains(got, "parents") {
		t.Errorf("logs = %s; want the children's 1003 rows, nothing of the parents", got)
	}
}

// A failure stops the purge before the purgers after it: their tables may
// still be referenced by the rows the failed one left. The batches before
// it are gone, and logged.
func TestPurgeStopsAtAFailure(t *testing.T) {
	var calls []string
	failed := errors.New("connection reset")
	children := &fakePurger{table: "children", counts: []int{purgeBatch}, err: failed, calls: &calls}
	parents := &fakePurger{table: "parents", calls: &calls}
	var logs logBuffer

	err := purge(context.Background(), []Purger{children.purger(), parents.purger()}, time.Now(), newLogger(&logs))

	if !errors.Is(err, failed) || !strings.Contains(err.Error(), "purge children") || !slices.Equal(calls, []string{"children", "children"}) {
		t.Errorf("purge() = %v after %q; want the children's failure on their second batch, the parents untouched", err, calls)
	}
	if got := logs.String(); !strings.Contains(got, `msg="soft-deleted rows purged" table=children rows=1000`) {
		t.Errorf("logs = %s; want the children's first batch, gone before the failure", got)
	}
}

// The worker asks for the rows deleted longer than the retention ago.
func TestPurgeWorkerCutsOffAtTheRetention(t *testing.T) {
	var calls []string
	rows := &fakePurger{table: "rows", calls: &calls}
	retention := 60 * 24 * time.Hour
	w := &purgeWorker{purgers: []Purger{rows.purger()}, cfg: PurgeConfig{Retention: retention, Logger: slog.New(slog.DiscardHandler)}}

	start := time.Now()
	err := w.Work(context.Background(), nil)
	end := time.Now()

	if err != nil || len(rows.seen) != 1 || rows.seen[0].Before(start.Add(-retention)) || rows.seen[0].After(end.Add(-retention)) {
		t.Errorf("Work() = %v asking for rows before %v; want the retention before now", err, rows.seen)
	}
}
