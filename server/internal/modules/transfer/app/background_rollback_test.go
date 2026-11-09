package app_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// rbPass is a pass the page module plans: its steps, what it answers, or
// its error.
type rbPass struct {
	steps []app.JobStep
	pass  app.RollbackPass
	err   error
}

// rbTree is the page module's tree as a rollback's job plans and writes
// it: its passes in order, Done after the last; each batch written as the
// job tree writes it, its nodes deleted in a pass that deletes first, put
// back in another. upgrades are the upgrades each plan was of; running
// the run's context, as the last plan saw it.
type rbTree struct {
	*jobTree
	mu       sync.Mutex
	passes   []rbPass
	at       int
	upgrades []uuid.UUID
	running  context.Context
}

func (t *rbTree) PlanRollback(ctx context.Context, spec app.JobSpec) (app.JobPlan, app.RollbackPass, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.upgrades, t.running = append(t.upgrades, spec.UpgradeID), ctx
	if t.at == len(t.passes) {
		return nil, app.RollbackPass{Done: true}, nil
	}
	p := t.passes[t.at]
	t.at++
	return plan(p.steps), p.pass, p.err
}

func (t *rbTree) RunBatch(ctx context.Context, spec app.JobSpec, p app.JobPlan, from, to int) (app.BatchDone, error) {
	done, err := t.jobTree.RunBatch(ctx, spec, p, from, to)
	t.mu.Lock()
	deletes := t.at > 0 && t.passes[t.at-1].pass.Deletes
	t.mu.Unlock()
	if deletes {
		done.Deleted = done.Nodes
	} else {
		done.PutBack = done.Nodes
	}
	done.Pages, done.Placed = 0, nil
	return done, err
}

// rbEnd is the wiki module's end of the upgrades as a rollback's job ends:
// End answers left, err fails both; each call recorded with its upgrade
// and actor.
type rbEnd struct {
	rec    *recorder
	left   bool
	err    error
	mu     sync.Mutex
	of     []uuid.UUID
	actors []shared.Actor
}

func (e *rbEnd) note(ctx context.Context, call string, upgradeID uuid.UUID) {
	e.rec.add(call)
	actor, _ := shared.RequireActor(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.of, e.actors = append(e.of, upgradeID), append(e.actors, actor)
}

func (e *rbEnd) End(ctx context.Context, _, upgradeID uuid.UUID) (bool, error) {
	e.note(ctx, "RollbackEnd.End", upgradeID)
	return e.left, e.err
}

func (e *rbEnd) Stopped(ctx context.Context, _, upgradeID uuid.UUID) error {
	e.note(ctx, "RollbackEnd.Stopped", upgradeID)
	return e.err
}

// undoTx runs fn in a transaction of the rows: an error rolls its writes
// to them back, as the database would.
type undoTx struct {
	rows *rows
	rec  *recorder
}

func (u undoTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	u.rec.add("begin")
	u.rows.mu.Lock()
	kept := map[uuid.UUID]row{}
	for id, r := range u.rows.jobs {
		kept[id] = *r
	}
	u.rows.mu.Unlock()
	if err := fn(ctx); err != nil {
		u.rows.mu.Lock()
		for id, r := range kept {
			*u.rows.jobs[id] = r
		}
		u.rows.mu.Unlock()
		u.rec.add("rollback")
		return err
	}
	u.rec.add("commit")
	return nil
}

func (u undoTx) WithinSnapshot(ctx context.Context, fn func(ctx context.Context) error) error {
	return errors.New("a rollback's end reads no snapshot")
}

// rbWorld is a rollback's job's surroundings.
type rbWorld struct {
	*backgroundWorld
	rb   *rbTree
	end  *rbEnd
	logs *logs
	up   uuid.UUID
}

func newRbWorld(passes ...rbPass) *rbWorld {
	bw := newBackgroundWorld()
	l := &logs{}
	bw.logger = l.logger()
	bw.notebooks.rec = bw.rec
	return &rbWorld{backgroundWorld: bw, rb: &rbTree{jobTree: bw.tree, passes: passes}, end: &rbEnd{rec: bw.rec}, logs: l,
		up: uuid.NewV7()}
}

func (w *rbWorld) runner() *app.Background {
	return app.NewBackground(app.BackgroundRunDeps{Tree: w.rb, Statistics: []app.Analyzer{w.stats}, Rows: w.rows, Clock: fixedClock{now()},
		Logger: w.logger, Beat: 5 * time.Millisecond, Backoff: time.Millisecond, Batch: w.batch, Tx: undoTx{rows: w.rows, rec: w.rec},
		Notebooks: w.notebooks, RollbackEnd: w.end})
}

// queued adds bob's rollback of the upgrade up in eng, queued.
func (w *rbWorld) queued() domain.Job {
	changeset, up := uuid.NewV7(), w.up
	j := domain.Job{ID: uuid.NewV7(), NotebookID: w.eng, Kind: domain.KindRollback, State: domain.StateQueued, Name: "upgrade",
		CreatedBy: w.bob, Client: domain.ClientWeb, CreatedAt: now(), ChangesetID: &changeset, UpgradeID: &up}
	w.rows.add(j)
	return j
}

func (w *rbWorld) run(t *testing.T, ctx context.Context, j domain.Job) domain.Job {
	t.Helper()
	if err := w.runner().Run(ctx, j.ID); err != nil {
		t.Fatal(err)
	}
	return w.rows.get(j.ID)
}

// ends are the end's calls in order.
func (w *rbWorld) ends() string {
	return strings.Join(w.rec.of("begin", "commit", "rollback", "Notebooks.LockByID", "FinishJob", "RollbackEnd.End",
		"RollbackEnd.Stopped"), ", ")
}

// lines are the log's lines that say text.
func (w *rbWorld) lines(text string) []string {
	var out []string
	for _, line := range strings.Split(w.logs.String(), "\n") {
		if strings.Contains(line, text) {
			out = append(out, line)
		}
	}
	return out
}

// A rollback's job runs a pass at a time (M11/P1 design 3.19): each
// planned for the job's upgrade, its batches written as a restore's, by
// the starter as the job; the progress's total grown by each pass's, the
// report counting what each deleted or put back; after each an info line
// "rollback pass" with the pass, what it deleted first, its items and
// steps; Done ends it, succeeded. A pass that leaves no less than before
// it stops the job failed, internal, the upgrade's stop told, logged at
// the error level.
func TestARollbackRunsPassByPass(t *testing.T) {
	w := newRbWorld(
		rbPass{steps: []app.JobStep{{Nodes: 10}, {Nodes: 10}}, pass: app.RollbackPass{Deletes: true, Left: app.RollbackLeft{Created: 30}}},
		rbPass{steps: []app.JobStep{{Nodes: 10}}, pass: app.RollbackPass{Deletes: true, Left: app.RollbackLeft{Created: 10}}},
		rbPass{steps: steps(3, 0), pass: app.RollbackPass{Left: app.RollbackLeft{Nodes: 3, Pages: 2, Created: 2}, Items: 7}},
	)
	j := w.queued()
	got := w.run(t, context.Background(), j)
	if got.State != domain.StateSucceeded || got.Progress != (domain.Progress{Done: 33, Total: 33}) || got.Report == nil ||
		got.Report.Counts.Deleted != 30 || got.Report.Counts.PutBack != 3 || got.Report.Failure != "" {
		t.Errorf("job %s, progress %+v, report %+v; want succeeded, 33 of 33, 30 deleted, 3 put back", got.State, got.Progress, got.Report)
	}
	if want := slices.Repeat([]uuid.UUID{w.up}, 4); !slices.Equal(w.rb.upgrades, want) {
		t.Errorf("planned %v, want four passes of the upgrade %v", w.rb.upgrades, w.up)
	}
	for i, s := range w.tree.specs {
		if s.UpgradeID != w.up || s.Changeset != *j.ChangesetID || w.tree.actors[i] != (shared.Actor{UserID: w.bob, JobID: j.ID}) {
			t.Errorf("batch %d of %+v by %+v, want the job's upgrade and changeset, by bob as the job", i, s, w.tree.actors[i])
		}
	}
	passes := w.lines(`msg="rollback pass"`)
	for i, want := range []string{"pass=1 deleted_first=20 items=0 steps=2", "pass=2 deleted_first=10 items=0 steps=1",
		"pass=3 deleted_first=0 items=7 steps=3"} {
		if len(passes) != 3 || !strings.Contains(passes[i], want) || !strings.Contains(passes[i], "upgrade_id="+w.up.String()) ||
			!strings.Contains(passes[i], "job_id="+j.ID.String()) || !strings.Contains(passes[i], "level=INFO") {
			t.Errorf("the passes' lines %q, want the %d-th with %s", passes, i+1, want)
			break
		}
	}

	w = newRbWorld(
		rbPass{steps: []app.JobStep{{Nodes: 5}}, pass: app.RollbackPass{Deletes: true, Left: app.RollbackLeft{Created: 5}}},
		rbPass{steps: []app.JobStep{{Nodes: 5}}, pass: app.RollbackPass{Deletes: true, Left: app.RollbackLeft{Nodes: 4, Created: 1}}},
	)
	j = w.queued()
	got = w.run(t, context.Background(), j)
	if got.State != domain.StateFailed || got.Report.Failure != domain.FailureInternal || got.Progress != (domain.Progress{Done: 5, Total: 5}) ||
		len(w.tree.written()) != 1 {
		t.Errorf("a pass that left as much: job %s/%s, progress %+v, batches %v; want failed internal after the first pass", got.State,
			got.Report.Failure, got.Progress, w.tree.written())
	}
	if ends, want := w.ends(), "begin, Notebooks.LockByID, FinishJob, RollbackEnd.Stopped, commit"; ends != want {
		t.Errorf("its end %s, want %s", ends, want)
	}
	if lines := w.lines(`msg="rollback ended"`); len(lines) != 1 || !strings.Contains(lines[0], "level=ERROR") ||
		!strings.Contains(lines[0], "failure=internal") {
		t.Errorf("logged %q, want one error line rollback ended, internal", lines)
	}
}

// A rollback's job's end is written with its upgrade's (M11/P1 design
// 3.19), as the job's starter, in one transaction each, the notebook's
// row locked first: done, the job's end, then the upgrade's End, then an
// info line "rollback ended"; something left rolls that back, and a second
// transaction ends the job failed, internal, and tells the upgrade's stop,
// logged at the error level. Cancelled, timed out, interrupted, refused:
// the job's end, then Stopped, once. A job no longer running at either end
// (the rescue failed it) writes nothing more: End is not called at the
// success, Stopped not at the stop, and the "stopped" line is logged; so
// for a notebook gone, its row not locked. The upgrade ended meanwhile
// (ErrJobGone from the plan) writes nothing at all, the "stopped" line
// alone.
func TestARollbacksEnd(t *testing.T) {
	type outcome struct {
		state   domain.State
		failure domain.Failure
		ends    string
		logged  string
	}
	done := outcome{domain.StateSucceeded, "", "begin, Notebooks.LockByID, FinishJob, RollbackEnd.End, commit", `msg="rollback ended"`}
	stopped := func(state domain.State, failure domain.Failure) outcome {
		return outcome{state, failure, "begin, Notebooks.LockByID, FinishJob, RollbackEnd.Stopped, commit", `msg="rollback ended"`}
	}
	pass := rbPass{steps: steps(2, 0), pass: app.RollbackPass{Deletes: true, Left: app.RollbackLeft{Created: 2}}}
	for _, tt := range []struct {
		name   string
		passes []rbPass
		change func(t *testing.T, w *rbWorld, j domain.Job, cancel func(), deadline *expiring)
		want   outcome
	}{
		{"done", []rbPass{pass}, nil, done},
		{"something left", []rbPass{pass}, func(_ *testing.T, w *rbWorld, _ domain.Job, _ func(), _ *expiring) { w.end.left = true },
			outcome{domain.StateFailed, domain.FailureInternal, "begin, Notebooks.LockByID, FinishJob, RollbackEnd.End, rollback, " +
				"begin, Notebooks.LockByID, FinishJob, RollbackEnd.Stopped, commit", `msg="rollback ended"`}},
		{"cancelled", []rbPass{pass}, func(t *testing.T, w *rbWorld, j domain.Job, _ func(), _ *expiring) {
			w.tree.onBatch = func(int) {
				if _, err := w.rows.RequestCancel(context.Background(), j.ID, now()); err != nil {
					t.Error(err)
				}
				waitFor(t, w.rb)
			}
		}, stopped(domain.StateCancelled, "")},
		{"timed out", []rbPass{pass}, func(t *testing.T, w *rbWorld, _ domain.Job, _ func(), deadline *expiring) {
			w.tree.onBatch = func(int) {
				deadline.expire()
				waitFor(t, w.rb)
			}
		}, stopped(domain.StateFailed, domain.FailureTimeout)},
		{"interrupted", []rbPass{pass}, func(t *testing.T, w *rbWorld, _ domain.Job, cancel func(), _ *expiring) {
			w.tree.onBatch = func(int) { cancel() }
		}, stopped(domain.StateFailed, domain.FailureInterrupted)},
		{"refused", []rbPass{{err: shared.Forbidden()}}, nil, stopped(domain.StateFailed, domain.FailureForbidden)},
		{"rescued as it succeeded", []rbPass{pass}, func(_ *testing.T, w *rbWorld, j domain.Job, _ func(), _ *expiring) {
			w.tree.onBatch = func(n int) {
				if n == 2 {
					w.rows.set(j.ID, func(r *row) { r.job.State = domain.StateFailed })
				}
			}
		}, outcome{domain.StateFailed, "", "begin, Notebooks.LockByID, FinishJob, rollback", "rollback stopped: the job was deleted"}},
		{"rescued as it stopped", []rbPass{pass}, func(_ *testing.T, w *rbWorld, j domain.Job, cancel func(), _ *expiring) {
			w.tree.onBatch = func(int) {
				w.rows.set(j.ID, func(r *row) { r.job.State = domain.StateFailed })
				cancel()
			}
		}, outcome{domain.StateFailed, "", "begin, Notebooks.LockByID, FinishJob, commit", "rollback stopped: the job was deleted"}},
		{"the notebook gone as it succeeded", []rbPass{pass}, func(_ *testing.T, w *rbWorld, _ domain.Job, _ func(), _ *expiring) {
			w.notebooks.unlocked = true
		}, outcome{domain.StateRunning, "", "begin, Notebooks.LockByID, commit", "rollback stopped: the job was deleted"}},
		{"the notebook gone as it stopped", []rbPass{{err: shared.Forbidden()}},
			func(_ *testing.T, w *rbWorld, _ domain.Job, _ func(), _ *expiring) { w.notebooks.unlocked = true },
			outcome{domain.StateRunning, "", "begin, Notebooks.LockByID, commit", "rollback stopped: the job was deleted"}},
		{"the upgrade ended", []rbPass{pass, {err: fmt.Errorf("page: gone: %w", app.ErrJobGone)}}, nil,
			outcome{domain.StateRunning, "", "", "rollback stopped: the job was deleted"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := newRbWorld(tt.passes...)
			w.batch = app.Batch{Nodes: 1}
			j := w.queued()
			deadline := newExpiring()
			ctx, cancel := context.WithCancel(deadline)
			defer cancel()
			if tt.change != nil {
				tt.change(t, w, j, cancel, deadline)
			}
			got := w.run(t, ctx, j)
			failure := domain.Failure("")
			if got.Report != nil {
				failure = got.Report.Failure
			}
			if got.State != tt.want.state || failure != tt.want.failure || w.ends() != tt.want.ends {
				t.Errorf("job %s/%s after %s; want %s/%s after %s", got.State, failure, w.ends(), tt.want.state, tt.want.failure,
					tt.want.ends)
			}
			if lines := w.lines(tt.want.logged); len(lines) != 1 {
				t.Errorf("logged %q, want one line of %s", w.logs.String(), tt.want.logged)
			}
			for i, a := range w.end.actors {
				if a != (shared.Actor{UserID: w.bob, JobID: j.ID}) || w.end.of[i] != w.up {
					t.Errorf("the end's call %d by %+v of %v, want by bob as the job, of the upgrade %v", i, a, w.end.of[i], w.up)
				}
			}
			level := "level=INFO"
			if tt.want.failure == domain.FailureInternal {
				level = "level=ERROR"
			}
			if lines := w.lines(`msg="rollback ended"`); len(lines) == 1 && !strings.Contains(lines[0], level) {
				t.Errorf("the end's line %q, want %s", lines[0], level)
			}
		})
	}
}

// waitFor waits until the heartbeat stopped the run, which the tree's
// last plan saw.
func waitFor(t *testing.T, tree *rbTree) {
	t.Helper()
	tree.mu.Lock()
	running := tree.running
	tree.mu.Unlock()
	select {
	case <-running.Done():
	case <-time.After(3 * time.Second):
		t.Error("the heartbeat did not stop the run")
	}
}
