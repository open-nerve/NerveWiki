package riveradapter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"

	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/river"
)

type fakeCleanup struct {
	ctx context.Context
	err error
}

func (f *fakeCleanup) Execute(ctx context.Context) (int, error) {
	f.ctx = ctx
	return 3, f.err
}

type ctxKey struct{}

// The worker runs the cleanup with the job's context; a failed cleanup
// fails the job, which River retries.
func TestCleanupWorkerRunsTheUseCase(t *testing.T) {
	boom := errors.New("connection reset")
	uc := &fakeCleanup{err: boom}
	ctx := context.WithValue(context.Background(), ctxKey{}, "job")

	err := riveradapter.NewCleanupWorker(uc).Work(ctx, &river.Job[riveradapter.CleanupArgs]{})

	if !errors.Is(err, boom) || uc.ctx == nil || uc.ctx.Value(ctxKey{}) != "job" {
		t.Errorf("Work() = %v, want the cleanup's error, run with the job's context", err)
	}
}

// The job registers one worker of its kind: a second of the same kind is
// refused, as platform/jobs relies on.
func TestCleanupJobRegistersItsWorker(t *testing.T) {
	job := riveradapter.CleanupJob(&fakeCleanup{}, time.Hour)
	workers := river.NewWorkers()

	if err := job.Add(workers); err != nil {
		t.Fatalf("Add() = %v", err)
	}
	if err := job.Add(workers); err == nil || job.Periodic == nil {
		t.Errorf("a second Add() = %v, periodic %v; want the kind %s refused and a schedule", err, job.Periodic, riveradapter.CleanupKind)
	}
}
