package riveradapter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"

	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/river"
)

type fakeSweep struct {
	ctx context.Context
	err error
}

func (f *fakeSweep) Run(ctx context.Context) (int, error) {
	f.ctx = ctx
	return 3, f.err
}

type ctxKey struct{}

// The worker runs the sweep with the job's context; a failed sweep fails
// the job, which River retries.
func TestSweepWorkerRunsTheUseCase(t *testing.T) {
	boom := errors.New("connection reset")
	uc := &fakeSweep{err: boom}
	ctx := context.WithValue(context.Background(), ctxKey{}, "job")

	err := riveradapter.NewSweepWorker(uc).Work(ctx, &river.Job[riveradapter.SweepArgs]{})

	if !errors.Is(err, boom) || uc.ctx == nil || uc.ctx.Value(ctxKey{}) != "job" {
		t.Errorf("Work() = %v, want the sweep's error, run with the job's context", err)
	}
}

// A run has SweepTimeout, longer than River's minute and shorter than the
// hour after which River runs a job still running again.
func TestSweepWorkerHasItsTimeout(t *testing.T) {
	const rescueAfter = time.Hour // the least RescueStuckJobsAfter serve sets
	got := riveradapter.NewSweepWorker(&fakeSweep{}).Timeout(&river.Job[riveradapter.SweepArgs]{})
	if got != riveradapter.SweepTimeout || got <= time.Minute || got >= rescueAfter {
		t.Errorf("Timeout() = %v, want SweepTimeout, between a minute and an hour", got)
	}
}

// The job registers one worker of its kind, on a schedule.
func TestSweepJobRegistersItsWorker(t *testing.T) {
	job := riveradapter.SweepJob(&fakeSweep{})
	workers := river.NewWorkers()

	if err := job.Add(workers); err != nil {
		t.Fatalf("Add() = %v", err)
	}
	if err := job.Add(workers); err == nil || job.Periodic == nil {
		t.Errorf("a second Add() = %v, periodic %v; want the kind %s refused and a schedule", err, job.Periodic, riveradapter.SweepKind)
	}
}
