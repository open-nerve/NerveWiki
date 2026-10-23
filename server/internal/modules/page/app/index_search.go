package app

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// IndexSearch is the search's job (M8/P5 design 3.4): it builds the rows of
// the nodes the units queued, as they are when it reads them, and takes the
// nodes off the queue at the versions it read. One run at a time: two would
// lose a write, the one that read earlier writing its row after the later
// one's.
type IndexSearch struct {
	tx     Tx
	index  SearchIndex
	clock  Clock
	logger *slog.Logger
	// minRebuild is how soon after it built a node's row the job builds it
	// again: domain.MinSearchRebuild for the server's job; none for the
	// tests' runs, which empty the queue.
	minRebuild time.Duration
	// unanalyzed is how many rows the runs wrote since the job last
	// analyzed page_search: a run of fewer than domain.SearchAnalyzeRows
	// leaves its count to the next, so that a build in many short runs is
	// analyzed as one in a few long ones (M8/P5A review B-N4).
	unanalyzed atomic.Int64
	// bulkFrom is the notebook the last run's pass over the nodes queued in
	// bulk ended in, after which the next begins; nil before any.
	bulkFrom atomic.Pointer[uuid.UUID]
}

// NewIndexSearch returns the job's use case.
func NewIndexSearch(tx Tx, index SearchIndex, clock Clock, logger *slog.Logger, minRebuild time.Duration) *IndexSearch {
	return &IndexSearch{tx: tx, index: index, clock: clock, logger: logger, minRebuild: minRebuild}
}

// SearchRun is what a run of the job did: the rows it wrote and deleted,
// the nodes it found current and took off, those it left queued, built
// within minRebuild or whose write failed. Locked is a run that did nothing,
// another holding the lock.
type SearchRun struct {
	Written, Deleted, Kept, Skipped, Failed int
	Locked                                  bool
}

// Drain runs the job once: under its lock, on its lock's connection, a pass
// over the nodes queued exact, those of the users' writes, by node id, then
// one over those queued in bulk, so that a user's write never waits behind
// an import's or a background job's thousands; each queued node read once,
// a batch at a time, until the queue's end or domain.SearchRunBudget, after
// which it starts no batch. The bulk pass goes notebook by notebook, by
// notebook id, then node id (M8 Codex review R1, fix round 2: B-3): a
// notebook's rows written together touch its own keys in the GIN indexes
// alone, where rows by node id touch every notebook's (an upgrade's rebuild
// of 299,200 pages of 5,000 notebooks in 16 minutes, not 29, and 0.44 MB of
// WAL a page, not 1.1), and a notebook is built, and says so, once its own
// rows are. The pass begins after the notebook the last run's ended in and
// comes round to it: no notebook's thousands hold the others back more than
// a run at a time. A batch's rows are written in one transaction, each
// alone again when that fails: a node whose write fails then is logged and
// left queued, still a candidate of every search, for the next run to try
// again. The nodes it took are taken off in a second, short transaction, at
// the versions it read: one a unit queued again since stays.
func (j *IndexSearch) Drain(ctx context.Context) (SearchRun, error) {
	ctx, release, ok, err := j.index.LockSearch(ctx)
	if err != nil {
		return SearchRun{}, err
	}
	if !ok {
		return SearchRun{Locked: true}, nil
	}
	defer release()
	var run SearchRun
	start := j.clock.Now()
	for _, bulk := range []bool{false, true} {
		// after is the last node decided; the bulk pass's from the notebook
		// the last run's ended in, past all its nodes, and, once the pass
		// has come round to the start, until it.
		var after QueuedNode
		var from, until *uuid.UUID
		if bulk {
			if from = j.bulkFrom.Load(); from != nil {
				after = QueuedNode{NotebookID: *from, ID: uuid.Max()}
			}
		}
		for j.clock.Now().Sub(start) < domain.SearchRunBudget {
			queued, err := j.index.QueuedSearch(ctx, bulk, after, domain.SearchBatchNodes)
			if err != nil {
				return run, err
			}
			if len(queued) == 0 && from != nil && until == nil {
				// The pass comes round to the start, until the notebook it
				// began after.
				until, after = from, QueuedNode{}
				if queued, err = j.index.QueuedSearch(ctx, bulk, after, domain.SearchBatchNodes); err != nil {
					return run, err
				}
			}
			if until != nil {
				queued = queued[:countUntil(queued, *until)]
			}
			if len(queued) == 0 {
				break
			}
			written := run.Written
			if after, err = j.batch(ctx, queued, &run); err != nil {
				return run, err
			}
			if bulk {
				notebook := after.NotebookID
				j.bulkFrom.Store(&notebook)
			}
			if j.unanalyzed.Add(int64(run.Written-written)) >= domain.SearchAnalyzeRows {
				if err := j.index.AnalyzeSearch(ctx); err != nil {
					return run, err
				}
				j.unanalyzed.Store(0)
			}
		}
	}
	return run, nil
}

// countUntil is how many of the queued nodes, in the bulk pass's order,
// are of notebook or one before it.
func countUntil(queued []QueuedNode, notebook uuid.UUID) int {
	n := 0
	for n < len(queued) && queued[n].NotebookID.Compare(notebook) <= 0 {
		n++
	}
	return n
}

// batch decides on the queued nodes in order and builds, writes and takes
// off those it decided on, the contents it reads within
// domain.SearchBatchBytes; it returns the last node it decided on, after
// which the pass goes on.
func (j *IndexSearch) batch(ctx context.Context, queued []QueuedNode, run *SearchRun) (QueuedNode, error) {
	ids := make([]uuid.UUID, len(queued))
	for i, q := range queued {
		ids[i] = q.ID
	}
	read, err := j.index.SearchSources(ctx, ids)
	if err != nil {
		return QueuedNode{}, err
	}
	sources := make(map[uuid.UUID]SearchSource, len(read))
	for _, s := range read {
		sources[s.ID] = s
	}
	now := j.clock.Now()
	var deleted, pages, kept, skipped []uuid.UUID
	var build []SearchSource
	var done []QueuedNode
	bytes, last := 0, queued[len(queued)-1]
	for i, q := range queued {
		s, ok := sources[q.ID]
		switch {
		case !ok || s.Gone:
			if ok && s.Built != nil {
				deleted = append(deleted, q.ID)
			}
			done = append(done, q)
			continue
		case s.Built != nil && s.Built.Name == s.Name && s.Built.Revision == s.Revision &&
			s.Built.Tokenizer == domain.SearchTokenizer:
			kept = append(kept, q.ID)
			done = append(done, q)
			continue
		case s.Built != nil && now.Sub(s.Built.At) < j.minRebuild:
			skipped = append(skipped, q.ID)
			continue
		}
		if len(build) > 0 && bytes+s.Bytes > domain.SearchBatchBytes {
			last = queued[i-1]
			break
		}
		bytes += s.Bytes
		build = append(build, s)
		if s.Kind == domain.KindPage {
			pages = append(pages, s.ID)
		}
		done = append(done, q)
	}
	contents, past, err := j.index.SearchContents(ctx, pages, domain.SearchBatchBytes)
	if err != nil {
		return QueuedNode{}, err
	}
	if past != (uuid.UUID{}) {
		// Grown since its source told its size, past the batch's bytes with
		// the pages before it (M9 Codex review D2): the batch ends before
		// it, its node and those after it left queued for the next batch,
		// which reads their sizes again, decides on them and counts them
		// (fix round 1, B-N2): this one neither deletes their rows nor
		// counts them. The first page is never past it: the batch's bytes
		// hold a page's largest content.
		at := make(map[uuid.UUID]int, len(queued))
		for i, q := range queued {
			at[q.ID] = i
		}
		if cut := at[past]; cut > 0 {
			after := func(id uuid.UUID) bool { return at[id] >= cut }
			build = slices.DeleteFunc(build, func(s SearchSource) bool { return after(s.ID) })
			done = slices.DeleteFunc(done, func(q QueuedNode) bool { return after(q.ID) })
			deleted, kept, skipped = slices.DeleteFunc(deleted, after), slices.DeleteFunc(kept, after), slices.DeleteFunc(skipped, after)
			last = queued[cut-1]
		}
	}
	run.Kept, run.Skipped = run.Kept+len(kept), run.Skipped+len(skipped)
	rows := make([]SearchRow, 0, len(build))
	missing := map[uuid.UUID]bool{}
	for _, s := range build {
		row := SearchRow{NodeID: s.ID, Name: s.Name, Tokenizer: domain.SearchTokenizer, BuiltAt: now}
		var content *string
		if s.Kind == domain.KindPage {
			c, ok := contents[s.ID]
			if !ok {
				// Deleted since its source was read: the deletion queued it
				// again, and the next run deletes its row.
				missing[s.ID] = true
				continue
			}
			content, row.Revision = &c.Content, c.Revision
		}
		row.Title, row.Body, row.Partial = domain.SearchTokens(s.Name, content)
		rows = append(rows, row)
	}
	failed, err := j.write(ctx, rows, deleted)
	if err != nil {
		return QueuedNode{}, err
	}
	run.Written += len(rows) - countIn(rows, failed, func(r SearchRow) uuid.UUID { return r.NodeID })
	run.Deleted += len(deleted) - countIn(deleted, failed, func(id uuid.UUID) uuid.UUID { return id })
	take := make([]QueuedNode, 0, len(done))
	for _, q := range done {
		switch {
		case failed[q.ID]:
			run.Failed++
		case missing[q.ID]:
			run.Skipped++
		default:
			take = append(take, q)
		}
	}
	if len(take) > 0 {
		if err := j.index.DequeueSearch(ctx, take); err != nil {
			return QueuedNode{}, err
		}
	}
	return last, nil
}

// write writes the rows and deletes the rows of deleted in one transaction;
// when that fails, each alone in a transaction of its own, and returns those
// that fail again, which it logs. ctx's end fails it, and so does a
// transaction that cannot begin: the run's connection is gone, and a node
// tried alone on it would fail for want of it, not for anything of its
// own; the run ends with that one error, its nodes left queued (M8/P5A
// fix check M-1), holding the batch's failure too, which may be what ended
// the connection: a FATAL, a COMMIT past its timeout (fix check N2-1).
func (j *IndexSearch) write(ctx context.Context, rows []SearchRow, deleted []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(rows) == 0 && len(deleted) == 0 {
		return nil, nil
	}
	batchErr := j.tx.WithinTx(ctx, func(ctx context.Context) error { return j.index.WriteSearchRows(ctx, rows, deleted) })
	if batchErr == nil || ctx.Err() != nil || shared.TxNotBegun(batchErr) {
		return nil, batchErr
	}
	failed := map[uuid.UUID]bool{}
	one := func(id uuid.UUID, rows []SearchRow, deleted []uuid.UUID) error {
		err := j.tx.WithinTx(ctx, func(ctx context.Context) error { return j.index.WriteSearchRows(ctx, rows, deleted) })
		switch {
		case err == nil || ctx.Err() != nil:
			return err
		case shared.TxNotBegun(err):
			return fmt.Errorf("the batch's write: %w; a node's alone: %w", batchErr, err)
		}
		failed[id] = true
		j.logger.LogAttrs(ctx, slog.LevelError, "a node's search row is not written: the node stays queued",
			slog.String("node_id", id.String()), slog.Any("error", err))
		return nil
	}
	for _, r := range rows {
		if err := one(r.NodeID, []SearchRow{r}, nil); err != nil {
			return nil, err
		}
	}
	for _, id := range deleted {
		if err := one(id, nil, []uuid.UUID{id}); err != nil {
			return nil, err
		}
	}
	return failed, nil
}

// countIn is how many of xs have their id in set.
func countIn[T any](xs []T, set map[uuid.UUID]bool, id func(T) uuid.UUID) int {
	n := 0
	for _, x := range xs {
		if set[id(x)] {
			n++
		}
	}
	return n
}
