package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// backgroundWorld is a background restore's or revert's surroundings: the
// export's world, the page module's tree, the statistics, and the bounds
// of a batch.
type backgroundWorld struct {
	*world
	tree  *jobTree
	stats *analyzer
	batch app.Batch
}

func newBackgroundWorld() *backgroundWorld {
	w := newWorld()
	return &backgroundWorld{world: w, tree: newJobTree(nil), stats: &analyzer{}}
}

func (w *backgroundWorld) runner() *app.Background {
	return app.NewBackground(app.BackgroundRunDeps{Tree: w.tree, Statistics: []app.Analyzer{w.stats}, Rows: w.rows, Clock: fixedClock{now()},
		Logger: w.logger, Beat: 5 * time.Millisecond, Backoff: time.Millisecond, Batch: w.batch})
}

// queued adds bob's job of kind in eng, queued: a restore of an item, a
// revert of a changeset with a message, the changeset it writes decided.
func (w *backgroundWorld) queued(kind domain.Kind) domain.Job {
	changeset, target, message := uuid.NewV7(), uuid.NewV7(), "Undo"
	j := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: kind, State: domain.StateQueued, Name: "Box", CreatedBy: w.bob,
		Client: domain.ClientAPI, CreatedAt: now(), ChangesetID: &changeset}
	if kind == domain.KindRevert {
		j.TargetChangesetID, j.Message, j.Name = &target, &message, "import"
	} else {
		item := uuid.NewV7()
		j.RootID = &item
	}
	w.rows.add(j)
	return j
}

// run runs the job, and answers its row as it ended.
func (w *backgroundWorld) run(t *testing.T, j domain.Job) domain.Job {
	t.Helper()
	if err := w.runner().Run(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	return w.rows.get(j.ID)
}

// A background job plans, then writes its plan's steps in batches of the
// bounds' nodes and bytes, a step past them alone a batch of its own,
// each a unit of the job's changeset, from its client, its starter acting
// as the job; its progress the steps' nodes of all, its report the
// batches' counts and the item's root's place. A restore plans its item,
// a revert its changeset, its message in each unit.
func TestABackgroundJobWritesItsPlanInBatches(t *testing.T) {
	for _, tt := range []struct {
		name  string
		batch app.Batch
		steps []app.JobStep
		want  [][2]int
	}{
		{"the import's bounds", app.Batch{}, steps(250, 10), [][2]int{{0, 100}, {100, 200}, {200, 250}}},
		{"bytes", app.Batch{Bytes: 100}, []app.JobStep{{Nodes: 1, Bytes: 60}, {Nodes: 1, Bytes: 40}, {Nodes: 1, Bytes: 30}, {Nodes: 1, Bytes: 200},
			{Nodes: 1, Bytes: 10}}, [][2]int{{0, 2}, {2, 3}, {3, 4}, {4, 5}}},
		{"a subtree deleted", app.Batch{Nodes: 3}, []app.JobStep{{Nodes: 1}, {Nodes: 150}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}, {Nodes: 1}},
			[][2]int{{0, 1}, {1, 2}, {2, 5}, {5, 6}}},
	} {
		for _, kind := range []domain.Kind{domain.KindTrashRestore, domain.KindRevert} {
			t.Run(tt.name+", "+string(kind), func(t *testing.T) {
				w := newBackgroundWorld()
				w.batch, w.tree.steps = tt.batch, tt.steps
				j := w.queued(kind)
				got := w.run(t, j)
				total := int64(plan(tt.steps).Total())
				if got.State != domain.StateSucceeded || got.Progress != (domain.Progress{Done: total, Total: total}) ||
					got.Report.Counts.Pages != total || got.Report.Placed == nil || got.Report.Failure != "" {
					t.Errorf("job %s, progress %+v, report %+v; want succeeded, %d of %d", got.State, got.Progress, got.Report, total, total)
				}
				if !slices.Equal(w.tree.written(), tt.want) {
					t.Errorf("batches %v, want %v", w.tree.written(), tt.want)
				}
				wantPlanned := j.RootID
				if kind == domain.KindRevert {
					wantPlanned = j.TargetChangesetID
				}
				if !slices.Equal(w.tree.planned, []uuid.UUID{*wantPlanned}) {
					t.Errorf("planned %v, want %v", w.tree.planned, *wantPlanned)
				}
				for i, s := range w.tree.specs {
					if s != (app.JobSpec{NotebookID: w.eng, Client: domain.ClientAPI, Changeset: *j.ChangesetID, Message: j.Message}) ||
						w.tree.actors[i] != (shared.Actor{UserID: w.bob, JobID: j.ID}) {
						t.Errorf("batch %d of %+v by %+v, want the job's changeset and message, by bob as the job", i, s, w.tree.actors[i])
					}
				}
			})
		}
	}
}

// A batch the server is too busy for is tried again; one past a unit's
// history or links is halved, at least a step, the next cut from where it
// ends again; a step alone too large is the page module's defect.
func TestABackgroundJobsBatchIsTriedAgainOrHalved(t *testing.T) {
	w := newBackgroundWorld()
	w.batch, w.tree.steps, w.tree.busy = app.Batch{Nodes: 4}, steps(6, 0), 2
	if got := w.run(t, w.queued(domain.KindRevert)); got.State != domain.StateSucceeded || got.Progress.Done != 6 {
		t.Errorf("job %s, progress %+v; want succeeded", got.State, got.Progress)
	}
	if want := [][2]int{{0, 4}, {0, 4}, {0, 4}, {4, 6}}; !slices.Equal(w.tree.written(), want) {
		t.Errorf("batches %v, want %v", w.tree.written(), want)
	}

	w = newBackgroundWorld()
	w.batch, w.tree.steps, w.tree.halving = app.Batch{Nodes: 8}, steps(10, 0), 3
	if got := w.run(t, w.queued(domain.KindTrashRestore)); got.State != domain.StateSucceeded || got.Report.Counts.Pages != 10 {
		t.Errorf("job %s, report %+v; want succeeded, 10 pages", got.State, got.Report)
	}
	if want := [][2]int{{0, 8}, {0, 4}, {0, 2}, {2, 10}, {2, 6}, {2, 4}, {4, 10}, {4, 7}, {7, 10}}; !slices.Equal(w.tree.written(), want) {
		t.Errorf("batches %v, want %v", w.tree.written(), want)
	}

	w = newBackgroundWorld()
	w.tree.steps = steps(3, 0)
	w.tree.fail[0] = app.ErrBatchTooLarge
	w.batch = app.Batch{Nodes: 1}
	if got := w.run(t, w.queued(domain.KindRevert)); got.State != domain.StateFailed || got.Report.Failure != domain.FailureInternal {
		t.Errorf("job %s, report %+v; want failed internal", got.State, got.Report)
	}
}

// What stops a background job, as its plan or a batch answers it: each
// the failure its report tells, what it wrote before kept and counted; a
// revert's conflicts, the first 100 and their count, and the lock it met.
func TestABackgroundJobStopsWithItsReason(t *testing.T) {
	page, holder, node := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	conflicts := make([]shared.Conflict, 101)
	for i := range conflicts {
		conflicts[i] = shared.Conflict{NodeID: node, Name: fmt.Sprintf("n%03d", i), Reason: "changed"}
	}
	conflicts[0].Reason, conflicts[0].HolderID = "title_taken", &holder
	count := 140
	conflict := fmt.Errorf("%w: %w", app.ErrRevertConflict, &shared.Error{Kind: shared.KindConflict, Code: "page.revert_conflict",
		Conflicts: conflicts, ConflictCount: &count})
	lock := fmt.Errorf("%w: %w", app.ErrLocked, &shared.Error{Kind: shared.KindConflict, Code: "page.locked",
		Lock: &shared.LockHolder{PageID: page, UserID: holder, DisplayName: "Ada"}})
	for _, tt := range []struct {
		name string
		err  error
		want domain.Failure
	}{
		{"the trash item changed", app.ErrTrashChanged, domain.FailureTrashChanged},
		{"the item expired", app.ErrRestoreExpired, domain.FailureRestoreExpired},
		{"a parent gone", app.ErrNoParent, domain.FailureTreeChanged},
		{"too deep", app.ErrTooDeep, domain.FailureTreeChanged},
		{"conflicts", conflict, domain.FailureRevertConflict},
		{"trimmed", app.ErrChangesetTrimmed, domain.FailureChangesetTrimmed},
		{"locked", lock, domain.FailurePageLocked},
		{"its changeset reverted", app.ErrChangesetReverted, domain.FailureChangesetReverted},
		{"forbidden", shared.Forbidden(), domain.FailureForbidden},
		{"the notebook gone", domain.ErrNotebookNotFound, domain.FailureForbidden},
		{"a name taken", &shared.Error{Kind: shared.KindConflict, Code: "page.title_taken"}, domain.FailureInternal},
		{"anything else", errors.New("connection reset"), domain.FailureInternal},
	} {
		for _, at := range []string{"plan", "batch"} {
			t.Run(tt.name+", "+at, func(t *testing.T) {
				w := newBackgroundWorld()
				w.batch, w.tree.steps = app.Batch{Nodes: 2}, steps(5, 0)
				written := int64(0)
				if at == "plan" {
					w.tree.planErr = tt.err
				} else {
					w.tree.fail[2], written = tt.err, 2
				}
				got := w.run(t, w.queued(domain.KindRevert))
				if got.State != domain.StateFailed || got.Report.Failure != tt.want || got.Report.Counts.Pages != written ||
					got.Progress.Done != written {
					t.Errorf("job %s, progress %+v, report %+v; want failed %s, %d written", got.State, got.Progress, got.Report, tt.want, written)
				}
				r := got.Report
				switch tt.want {
				case domain.FailureRevertConflict:
					if len(r.Conflicts) != domain.MaxConflicts || r.ConflictCount != count ||
						r.Conflicts[0] != (domain.Conflict{NodeID: node, Name: "n000", Reason: "title_taken", HolderID: &holder}) {
						t.Errorf("conflicts %d of %d, the first %+v", len(r.Conflicts), r.ConflictCount, r.Conflicts[0])
					}
				case domain.FailurePageLocked:
					if r.Lock == nil || *r.Lock != (domain.Lock{PageID: page, UserID: holder}) {
						t.Errorf("lock %+v, want the page's holder", r.Lock)
					}
				default:
					if r.Conflicts != nil || r.ConflictCount != 0 || r.Lock != nil {
						t.Errorf("report %+v, want no conflicts nor lock", r)
					}
				}
			})
		}
	}
}

// A cancel asked as a batch runs lets it end, then stops the job between
// two: the batches written kept and counted, the job cancelled. A job
// deleted as it runs, with its notebook, stops so too, writing no end.
func TestABackgroundJobStopsBetweenTwoBatches(t *testing.T) {
	for _, tt := range []struct {
		name string
		stop func(w *backgroundWorld, j domain.Job)
		seen func(r *row) bool
		want domain.State
	}{
		{"cancelled", func(w *backgroundWorld, j domain.Job) {
			if _, err := w.rows.RequestCancel(context.Background(), j.ID, now()); err != nil {
				panic(err)
			}
		}, func(r *row) bool { return r.job.CancelRequested != nil }, domain.StateCancelled},
		{"deleted", func(w *backgroundWorld, j domain.Job) { w.rows.set(j.ID, func(r *row) { r.deleted = true }) },
			func(r *row) bool { return r.deleted }, domain.StateRunning},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newBackgroundWorld()
			w.tree.steps = steps(300, 0)
			j := w.queued(domain.KindTrashRestore)
			read := make(chan struct{})
			w.rows.onBeat = func() {
				w.rows.mu.Lock()
				seen := tt.seen(w.rows.jobs[j.ID])
				w.rows.mu.Unlock()
				if seen {
					select {
					case <-read:
					default:
						close(read)
					}
				}
			}
			w.tree.onBatch = func(n int) {
				if n == 2 {
					tt.stop(w, j)
					<-read
				}
			}
			got := w.run(t, j)
			if got.State != tt.want || len(w.tree.written()) != 2 {
				t.Errorf("job %s, batches %v; want %s after two batches", got.State, w.tree.written(), tt.want)
			}
			if tt.want == domain.StateCancelled && (got.Report.Counts.Pages != 200 || got.Progress != (domain.Progress{Done: 200, Total: 300})) {
				t.Errorf("progress %+v, report %+v; want two batches counted", got.Progress, got.Report)
			}
		})
	}
}

// A job no longer queued, cancelled or deleted before it ran, is left
// unplanned; one without the changeset it writes is the server's defect.
func TestABackgroundJobNotQueuedIsLeft(t *testing.T) {
	w := newBackgroundWorld()
	j := w.queued(domain.KindRevert)
	w.rows.set(j.ID, func(r *row) { r.job.State = domain.StateCancelled })
	if got := w.run(t, j); got.State != domain.StateCancelled || len(w.tree.planned) != 0 {
		t.Errorf("job %s, planned %v; want it left", got.State, w.tree.planned)
	}
	j = w.queued(domain.KindTrashRestore)
	w.rows.set(j.ID, func(r *row) { r.job.ChangesetID = nil })
	if got := w.run(t, j); got.State != domain.StateFailed || got.Report.Failure != domain.FailureInternal || len(w.tree.planned) != 0 {
		t.Errorf("job %s, report %+v; want failed internal, unplanned", got.State, got.Report)
	}
}

// A job of nothing to change succeeds at once, 0 of 0, writing nothing.
func TestABackgroundJobOfNothingToChange(t *testing.T) {
	w := newBackgroundWorld()
	got := w.run(t, w.queued(domain.KindRevert))
	if got.State != domain.StateSucceeded || got.Progress != (domain.Progress{}) || len(w.tree.written()) != 0 || w.stats.count() != 0 {
		t.Errorf("job %s, progress %+v, batches %v, statistics %d; want succeeded, nothing written", got.State, got.Progress,
			w.tree.written(), w.stats.count())
	}
}

// The statistics are refreshed every 10,000 nodes written and at the end.
func TestABackgroundJobRefreshesTheStatistics(t *testing.T) {
	w := newBackgroundWorld()
	w.tree.steps = steps(10050, 0)
	if got := w.run(t, w.queued(domain.KindTrashRestore)); got.State != domain.StateSucceeded || w.stats.count() != 2 {
		t.Errorf("job %s, statistics refreshed %d times; want twice", got.State, w.stats.count())
	}
}

// The job's logs tell what it starts on and its counts, not its name nor
// its message.
func TestABackgroundJobLogsTheIDsNotTheNames(t *testing.T) {
	for _, kind := range []domain.Kind{domain.KindTrashRestore, domain.KindRevert} {
		w := newBackgroundWorld()
		var l logs
		w.logger = l.logger()
		w.tree.steps = steps(3, 0)
		j := w.queued(kind)
		w.rows.set(j.ID, func(r *row) { r.job.Name = "Secret box" })
		w.run(t, j)
		text, on := l.String(), "root_id="
		if kind == domain.KindRevert {
			on = "target_changeset_id="
		}
		if !strings.Contains(text, string(kind)+" started") || !strings.Contains(text, string(kind)+" ended") || !strings.Contains(text, on) ||
			!strings.Contains(text, "pages=3") || !strings.Contains(text, j.ID.String()) || strings.Contains(text, "Secret") ||
			strings.Contains(text, "Undo") {
			t.Errorf("logs %q", text)
		}
	}
}

// The job's view names the holder of the lock that stopped it, with its
// starter, by their names.
func TestABackgroundJobsLockIsNamed(t *testing.T) {
	w := newBackgroundWorld()
	j := w.queued(domain.KindRevert)
	ada := uuid.NewV7()
	w.rows.set(j.ID, func(r *row) {
		r.job.State, r.job.Report = domain.StateFailed, &domain.Report{Failure: domain.FailurePageLocked, Lock: &domain.Lock{PageID: w.spec, UserID: ada}}
	})
	got, err := app.NewReads(app.ReadsDeps{Authorizer: w.auth, Notebooks: w.notebooks, Names: names{w.bob: "Bob", ada: "Ada"}, Signer: signer{},
		Clock: fixedClock{now()}, Rows: w.rows, ExportTTL: ttl}).Get(as(w.bob), j.ID)
	if err != nil || got.CreatedByName != "Bob" || got.LockedByName != "Ada" {
		t.Errorf("Get() = %+v, %v; want Bob's job locked by Ada", got, err)
	}
}
