package riveradapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

type fakeDrain struct {
	ctx context.Context
	err error
}

func (f *fakeDrain) Drain(ctx context.Context) (app.SearchRun, error) {
	f.ctx = ctx
	return app.SearchRun{Written: 3}, f.err
}

// The worker runs the job's run with the job's context, its failure the
// job's; a run may take two minutes.
func TestSearchIndexWorkerRunsTheUseCase(t *testing.T) {
	boom := errors.New("connection reset")
	uc := &fakeDrain{err: boom}
	ctx := context.WithValue(context.Background(), ctxKey{}, "job")
	w := riveradapter.NewSearchIndexWorker(uc)

	err := w.Work(ctx, &river.Job[riveradapter.SearchIndexArgs]{})

	if !errors.Is(err, boom) || uc.ctx == nil || uc.ctx.Value(ctxKey{}) != "job" {
		t.Errorf("Work() = %v, want the run's error, run with the job's context", err)
	}
	if got := w.Timeout(nil); got != 2*time.Minute {
		t.Errorf("Timeout() = %v, want 2m", got)
	}
}

// The job is on its own queue, one queued or running at most; a completed
// run holds back no later one (M8/P5 design 3.4): River 0.47's default
// ByState holds completed, and must hold available, pending, running and
// scheduled.
func TestSearchIndexJobIsUniqueButForTheCompleted(t *testing.T) {
	opts := riveradapter.SearchIndexInsertOpts()
	states := opts.UniqueOpts.ByState
	for _, s := range []rivertype.JobState{rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled} {
		if !slices.Contains(states, s) {
			t.Errorf("ByState %v lacks %s", states, s)
		}
	}
	if slices.Contains(states, rivertype.JobStateCompleted) || opts.Queue != riveradapter.SearchIndexQueue {
		t.Errorf("InsertOpts = %+v, want the queue %s and no completed among ByState", opts, riveradapter.SearchIndexQueue)
	}
	job := riveradapter.SearchIndexJob(&fakeDrain{}, time.Second)
	workers := river.NewWorkers()
	if err := job.Add(workers); err != nil || job.Periodic == nil {
		t.Fatalf("Add() = %v, periodic %v; want the worker and a schedule", err, job.Periodic)
	}
	if err := job.Add(workers); err == nil {
		t.Errorf("a second Add() = nil, want the kind %s refused", riveradapter.SearchIndexKind)
	}
}
