package riveradapter

import (
	"context"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// SearchIndexKind is the kind of the search's job, which builds the rows of
// the nodes queued for it (M8/P5 design 3.4), and the id of its schedule.
const SearchIndexKind = "page.index_search"

// SearchIndexQueue is the job's queue, of one worker: the job runs one at a
// time, and holds neither of the default queue's two workers.
const SearchIndexQueue = "page_search"

// SearchIndexTimeout bounds a run: a run starts no batch after a minute
// (domain.SearchRunBudget), and River's minute would cut its last batch
// short. It stays far under the hour after which River takes a job still
// running for stuck.
const SearchIndexTimeout = 2 * time.Minute

// SearchIndexUseCase is the use case the job runs: app.IndexSearch.
type SearchIndexUseCase interface {
	Drain(ctx context.Context) (app.SearchRun, error)
}

// SearchIndexArgs are the job's arguments: none.
type SearchIndexArgs struct{}

// Kind is SearchIndexKind.
func (SearchIndexArgs) Kind() string { return SearchIndexKind }

// SearchIndexInsertOpts are the job's: on its queue, one queued or running
// at most. Unlike River's default, a completed run holds back no later one:
// the schedule's next insert would wait a day behind it.
func SearchIndexInsertOpts() *river.InsertOpts {
	return &river.InsertOpts{Queue: SearchIndexQueue, UniqueOpts: river.UniqueOpts{ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}}}
}

// SearchIndexWorker runs the job.
type SearchIndexWorker struct {
	river.WorkerDefaults[SearchIndexArgs]
	uc SearchIndexUseCase
}

// NewSearchIndexWorker returns the worker of uc.
func NewSearchIndexWorker(uc SearchIndexUseCase) *SearchIndexWorker {
	return &SearchIndexWorker{uc: uc}
}

// Timeout is SearchIndexTimeout.
func (*SearchIndexWorker) Timeout(*river.Job[SearchIndexArgs]) time.Duration {
	return SearchIndexTimeout
}

// Work runs the job once. A failure makes River retry it; the next run on
// the schedule goes on from the queue as it is.
func (w *SearchIndexWorker) Work(ctx context.Context, _ *river.Job[SearchIndexArgs]) error {
	_, err := w.uc.Drain(ctx)
	return err
}

// SearchIndexJob is the periodic job: when the server starts, then every
// interval (page.search_index_interval).
func SearchIndexJob(uc SearchIndexUseCase, interval time.Duration) jobs.Job {
	return jobs.Job{
		Add: func(w *river.Workers) error { return river.AddWorkerSafely(w, NewSearchIndexWorker(uc)) },
		Periodic: river.NewPeriodicJob(river.PeriodicInterval(interval),
			func() (river.JobArgs, *river.InsertOpts) { return SearchIndexArgs{}, SearchIndexInsertOpts() },
			&river.PeriodicJobOpts{ID: SearchIndexKind, RunOnStart: true}),
	}
}
