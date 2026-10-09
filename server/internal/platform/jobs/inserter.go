package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

// Inserter enqueues jobs for the server's client to work (M7/P5 design
// 3.2): a River client without queues, never started. A request enqueues
// in its own transaction, so that the job exists exactly when the
// request's writes do; River notifies the queue as the transaction
// commits, and the server's client fetches the job.
type Inserter struct {
	client *river.Client[pgx.Tx]
}

// NewInserter builds the insert-only client on pool.
func NewInserter(pool *pgxpool.Pool, logger *slog.Logger) (*Inserter, error) {
	c, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("create the jobs' insert-only client: %w", err)
	}
	return &Inserter{client: c}, nil
}

// InsertTx enqueues args with opts in tx: rolled back, the job is gone
// with it.
func (i *Inserter) InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) error {
	if _, err := i.client.InsertTx(ctx, tx, args, opts); err != nil {
		return fmt.Errorf("enqueue %s: %w", args.Kind(), err)
	}
	return nil
}

// unfinishedPage is how many jobs one read of River's asks for.
const unfinishedPage = 1000

// Unfinished is the encoded arguments of the jobs of kind that River has
// not finished: to be worked, being worked, or to be tried again. A job
// whose own row says it waits asks it whether River still holds it: River
// drops a job whose attempts are spent, however they ended (M7/P5 design
// 3.12).
func (i *Inserter) Unfinished(ctx context.Context, kind string) ([][]byte, error) {
	return i.unfinished(ctx, kind, unfinishedPage)
}

func (i *Inserter) unfinished(ctx context.Context, kind string, page int) ([][]byte, error) {
	params := river.NewJobListParams().Kinds(kind).First(page).States(rivertype.JobStateAvailable, rivertype.JobStatePending,
		rivertype.JobStateRetryable, rivertype.JobStateRunning, rivertype.JobStateScheduled)
	var out [][]byte
	for {
		got, err := i.client.JobList(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("list the unfinished %s jobs: %w", kind, err)
		}
		for _, j := range got.Jobs {
			out = append(out, j.EncodedArgs)
		}
		if len(got.Jobs) < page {
			return out, nil
		}
		params = params.After(got.LastCursor)
	}
}
