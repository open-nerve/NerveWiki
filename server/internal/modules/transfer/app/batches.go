package app

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The writing jobs' batches (M7/P6 design 3.13; M8/P4 design 3.6): an
// import's, a restore's and a revert's, each written in units of the page
// module, which hold the notebook's row as they run, so each is bounded.
// A unit the server is too busy for is tried again after a wait; the
// statistics of the tables they fill are refreshed as they go.

// Batch bounds a writing job's batch: its nodes, its pages' contents'
// bytes and their links; a step past them alone is a batch of its own.
// Zero takes the import's, batchNodes, batchBytes and batchLinks (M7/P6
// design 3.3), which production keeps: a test makes them smaller.
type Batch struct {
	Nodes int
	Bytes int64
	Links int
}

// The import's bounds of a batch, and the job's other paces: the
// statistics are refreshed every analyzeEvery nodes written; a parse or a
// unit the server is too busy for waits at most maxBackoff before it is
// tried again.
const (
	batchNodes   = 100
	batchBytes   = 8 << 20
	batchLinks   = 20000
	analyzeEvery = 10000
	maxBackoff   = 30 * time.Second
)

// bounds are b, each zero the import's.
func (b Batch) bounds() Batch {
	return Batch{Nodes: cmp.Or(b.Nodes, batchNodes), Bytes: cmp.Or(b.Bytes, batchBytes), Links: cmp.Or(b.Links, batchLinks)}
}

// retry runs do until it answers anything but 503: after a wait of
// backoff, doubled each time up to maxBackoff, until ctx ends.
func retry(ctx context.Context, backoff time.Duration, do func() error) error {
	wait := backoff
	for {
		err := do()
		if !busy(err) {
			return err
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return context.Cause(ctx)
		case <-t.C:
		}
		wait = min(2*wait, maxBackoff)
	}
}

// busy reports whether err is the server's 503: its parse budget, or a
// lock, too busy.
func busy(err error) bool {
	var e *shared.Error
	return errors.As(err, &e) && e.Kind == shared.KindUnavailable
}

// uncertain reports whether a unit that failed with err, which its do's
// own failure rolled back when rolledBack, may have been written all the
// same: it failed neither in do nor refused, so its commit may have
// failed after the database took it.
func uncertain(err error, rolledBack bool) bool {
	var refused *shared.Error
	return err != nil && !rolledBack && !errors.As(err, &refused) && !errors.Is(err, ErrChangesetReverted)
}

// unitFailure is why a unit failed: forbidden for a job whose account can
// no longer write in the notebook, or see it; changeset_reverted for a
// revert of the job's changeset; the failure it found; any other is
// internal.
func unitFailure(err error) error {
	var e *shared.Error
	switch {
	case errors.As(err, &e) && (e.Kind == shared.KindForbidden || e.Kind == shared.KindNotFound || e.Kind == shared.KindUnauthenticated):
		return failed(domain.FailureForbidden, err)
	case errors.Is(err, ErrChangesetReverted):
		return failed(domain.FailureChangesetReverted, err)
	}
	return err
}

// statistics refreshes the planner's statistics of the tables a writing
// job fills, the page, linking and asset modules' (M7/P6 design 3.6):
// every analyzeEvery nodes it wrote, and as it ends. A failure is logged,
// the job going on.
type statistics struct {
	analyzers []Analyzer
	logger    *slog.Logger
	job       domain.Job
	// unanalyzed is how many nodes were written since the statistics were
	// last refreshed.
	unanalyzed int
}

// wrote counts n nodes written, and refreshes the statistics once
// analyzeEvery are.
func (s *statistics) wrote(ctx context.Context, n int) {
	if s.unanalyzed += n; s.unanalyzed >= analyzeEvery {
		s.refresh(ctx)
	}
}

// end refreshes the statistics of the nodes written since they last were,
// unless ctx ended: the server stops.
func (s *statistics) end(ctx context.Context) {
	if s.unanalyzed > 0 && ctx.Err() == nil {
		s.refresh(ctx)
	}
}

func (s *statistics) refresh(ctx context.Context) {
	for _, a := range s.analyzers {
		if err := a.Analyze(ctx); err != nil {
			s.logger.WarnContext(ctx, string(s.job.Kind)+" statistics not refreshed", slog.String("job_id", s.job.ID.String()),
				slog.Any("error", err))
		}
	}
	s.unanalyzed = 0
}
