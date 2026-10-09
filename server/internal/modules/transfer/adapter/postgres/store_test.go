package postgresadapter_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// fixture is a database with alice and bob, and acme's notebooks eng and
// ops.
type fixture struct {
	pool       *pgxpool.Pool
	store      *postgresadapter.Store
	tx         *postgres.TxManager
	alice, bob uuid.UUID
	eng, ops   uuid.UUID
}

// t0Unix is when the fixtures' jobs begin, and at a time after it.
const t0Unix = 1791540000

func at(d time.Duration) time.Time { return time.Unix(t0Unix, 123456000).UTC().Add(d) }

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{pool: pool, store: postgresadapter.New(pool), tx: postgres.NewTxManager(pool, 5*time.Second),
		alice: uuid.NewV7(), bob: uuid.NewV7(), eng: uuid.NewV7(), ops: uuid.NewV7()}
	acme := uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now()), " +
			"($2, 'bob@corp.com', 'x', 'x', now(), now())", []any{f.alice, f.bob}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, f.alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now()), ($4, $2, 'Ops', $3, $3, now(), now())", []any{f.eng, acme, f.alice, f.ops}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	return f
}

// export creates a queued export of notebook by by, made at.
func (f fixture) export(t *testing.T, notebook, by uuid.UUID, at time.Time) domain.Job {
	t.Helper()
	j := domain.Job{ID: uuid.NewV7(), NotebookID: notebook, Kind: domain.KindExport, State: domain.StateQueued, Name: "Eng", CreatedBy: by,
		Client: domain.ClientWeb, CreatedAt: at}
	if err := f.store.CreateJob(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	return j
}

func (f fixture) start(t *testing.T, id uuid.UUID, at time.Time) {
	t.Helper()
	if _, err := f.store.StartJob(context.Background(), id, at); err != nil {
		t.Fatal(err)
	}
}

// hold locks the job id's row FOR UPDATE in a transaction of its own, until
// the function it answers ends it.
func (f fixture) hold(t *testing.T, id uuid.UUID) func() {
	t.Helper()
	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT 1 FROM transfer_jobs WHERE id = $1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	release := func() { _ = tx.Rollback(ctx) }
	t.Cleanup(release)
	return release
}

// skipping is the context of a statement that skips the rows held: one
// that waits for them fails within seconds, not at the tests' timeout.
func skipping(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func (f fixture) succeed(t *testing.T, id uuid.UUID, at time.Time) {
	t.Helper()
	bytes := int64(42)
	ok, err := f.store.FinishJob(context.Background(), id, app.Ended{State: domain.StateSucceeded, At: at, Name: "Eng", ResultBytes: &bytes,
		Progress: domain.Progress{Done: 2, Total: 2}})
	if err != nil || !ok {
		t.Fatalf("FinishJob() = %v, %v", ok, err)
	}
}

// A job moves through its states by the statements that name the state
// they move it from; each reads back as written, its report too.
func TestAJobMovesThroughItsStates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	root := uuid.NewV7()
	j := domain.Job{ID: uuid.NewV7(), NotebookID: f.eng, RootID: &root, Kind: domain.KindExport, State: domain.StateQueued, Name: "Page",
		CreatedBy: f.alice, Client: domain.ClientAPI, CreatedAt: at(0)}
	if err := f.store.CreateJob(ctx, j); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.FindJob(ctx, j.ID)
	if err != nil || got.State != domain.StateQueued || *got.RootID != root || got.Client != domain.ClientAPI || got.Name != "Page" ||
		!got.CreatedAt.Equal(at(0)) || got.Started != nil || got.Report != nil {
		t.Fatalf("FindJob() = %+v, %v", got, err)
	}
	if _, err := f.store.BeatJob(ctx, j.ID, at(0), domain.Progress{}); !errors.Is(err, app.ErrNoRow) {
		t.Errorf("BeatJob() of a queued job = %v, want ErrNoRow", err)
	}

	started, err := f.store.StartJob(ctx, j.ID, at(time.Second))
	if err != nil || started.State != domain.StateRunning || !started.Started.Equal(at(time.Second)) || !started.Heartbeat.Equal(*started.Started) {
		t.Fatalf("StartJob() = %+v, %v", started, err)
	}
	if _, err := f.store.StartJob(ctx, j.ID, at(0)); !errors.Is(err, app.ErrNoRow) {
		t.Errorf("StartJob() twice = %v, want ErrNoRow", err)
	}
	beat, err := f.store.BeatJob(ctx, j.ID, at(2*time.Second), domain.Progress{Done: 1, Total: 3})
	if err != nil || beat != (app.Beat{}) {
		t.Errorf("BeatJob() = %+v, %v", beat, err)
	}
	if ok, err := f.store.RequestCancel(ctx, j.ID, at(3*time.Second)); !ok || err != nil {
		t.Fatalf("RequestCancel() = %v, %v", ok, err)
	}
	if ok, err := f.store.RequestCancel(ctx, j.ID, at(4*time.Second)); !ok || err != nil {
		t.Fatalf("RequestCancel() again = %v, %v", ok, err)
	}
	beat, err = f.store.BeatJob(ctx, j.ID, at(5*time.Second), domain.Progress{Done: 2, Total: 3})
	if err != nil || !beat.CancelRequested || beat.Deleted {
		t.Errorf("BeatJob() after the cancel = %+v, %v; want the cancel read", beat, err)
	}
	report := domain.Report{Counts: domain.Counts{Pages: 2}, Problems: []domain.Problem{{Path: "a/", Code: domain.ProblemRenamed, To: "a 2/"}}}
	ok, err := f.store.FinishJob(ctx, j.ID, app.Ended{State: domain.StateCancelled, At: at(6 * time.Second), Report: report, Name: "Page 2",
		Progress: domain.Progress{Done: 2, Total: 3}})
	if !ok || err != nil {
		t.Fatalf("FinishJob() = %v, %v", ok, err)
	}
	got, err = f.store.FindJob(ctx, j.ID)
	if err != nil || got.State != domain.StateCancelled || !got.CancelRequested.Equal(at(3*time.Second)) || got.Name != "Page 2" ||
		got.Progress != (domain.Progress{Done: 2, Total: 3}) || got.Report == nil || got.Report.Counts.Pages != 2 ||
		!slices.Equal(got.Report.Problems, report.Problems) || got.Report.Failure != "" || got.ResultBytes != nil {
		t.Errorf("FindJob() = %+v (report %+v), %v", got, got.Report, err)
	}
	if ok, err := f.store.FinishJob(ctx, j.ID, app.Ended{State: domain.StateFailed, At: at(0), Name: "x"}); ok || err != nil {
		t.Errorf("FinishJob() of an ended job = %v, %v; want false", ok, err)
	}
	if ok, err := f.store.RequestCancel(ctx, j.ID, at(0)); ok || err != nil {
		t.Errorf("RequestCancel() of an ended job = %v, %v; want false", ok, err)
	}
}

// A queued job is cancelled at once; River then finds it so.
func TestAQueuedJobIsCancelledAtOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	j := f.export(t, f.eng, f.alice, at(0))
	if ok, err := f.store.RequestCancel(ctx, j.ID, at(0)); ok || err != nil {
		t.Errorf("RequestCancel() of a queued job = %v, %v; want false", ok, err)
	}
	if ok, err := f.store.CancelQueued(ctx, j.ID, at(0), domain.Report{}); !ok || err != nil {
		t.Fatalf("CancelQueued() = %v, %v", ok, err)
	}
	// The table's check keeps cancel_requested_at for a job that started.
	if got, err := f.store.FindJob(ctx, j.ID); err != nil || got.State != domain.StateCancelled || got.CancelRequested != nil {
		t.Errorf("FindJob() = %+v, %v; want cancelled, no cancel asked of it running", got, err)
	}
	if _, err := f.store.StartJob(ctx, j.ID, at(0)); !errors.Is(err, app.ErrNoRow) {
		t.Errorf("StartJob() of a cancelled job = %v, want ErrNoRow", err)
	}
	if ok, err := f.store.CancelQueued(ctx, j.ID, at(0), domain.Report{}); ok || err != nil {
		t.Errorf("CancelQueued() twice = %v, %v; want false", ok, err)
	}
}

// One export queued or running for a reader in a notebook: a second is
// domain.ErrBusy, whatever was counted; another reader's, or one in
// another notebook, or after the first ended, goes.
func TestOneExportAtOnceForAReaderInANotebook(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	first := f.export(t, f.eng, f.alice, at(0))
	second := domain.Job{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.KindExport, State: domain.StateQueued, Name: "Eng", CreatedBy: f.alice,
		Client: domain.ClientWeb, CreatedAt: at(0)}
	if err := f.store.CreateJob(ctx, second); !errors.Is(err, domain.ErrBusy) {
		t.Errorf("CreateJob() of a second export = %v, want ErrBusy", err)
	}
	if busy, err := f.store.Exporting(ctx, f.eng, f.alice); !busy || err != nil {
		t.Errorf("Exporting() = %v, %v; want true", busy, err)
	}
	f.export(t, f.eng, f.bob, at(0))
	f.export(t, f.ops, f.alice, at(0))
	f.start(t, first.ID, at(0))
	if err := f.store.CreateJob(ctx, second); !errors.Is(err, domain.ErrBusy) {
		t.Errorf("CreateJob() beside a running export = %v, want ErrBusy", err)
	}
	f.succeed(t, first.ID, at(0))
	if err := f.store.CreateJob(ctx, second); err != nil {
		t.Errorf("CreateJob() once the first ended = %v", err)
	}
	if n, err := f.store.CountActive(ctx); n != 3 || err != nil {
		t.Errorf("CountActive() = %d, %v; want 3", n, err)
	}
}

// done is an export of notebook by by made at, run and succeeded.
func (f fixture) done(t *testing.T, notebook, by uuid.UUID, at time.Time) domain.Job {
	t.Helper()
	j := f.export(t, notebook, by, at)
	f.start(t, j.ID, at)
	f.succeed(t, j.ID, at)
	return j
}

func ids(jobs []domain.Job) []uuid.UUID {
	out := make([]uuid.UUID, len(jobs))
	for i, j := range jobs {
		out[i] = j.ID
	}
	return out
}

// A notebook's jobs come newest first, those of one time by id, a page at
// a time, the account's alone when asked, their reports without their
// problems; a deleted job, and another notebook's, are in no list.
func TestTheJobsList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a1 := f.done(t, f.eng, f.alice, at(0))
	b1 := f.done(t, f.eng, f.bob, at(time.Minute))
	a2 := f.export(t, f.eng, f.alice, at(2*time.Minute))
	f.start(t, a2.ID, at(2*time.Minute))
	report := domain.Report{Counts: domain.Counts{Pages: 1, Renamed: 1}, Problems: []domain.Problem{{Path: "a/", Code: domain.ProblemRenamed, To: "a 2/"}}}
	if ok, err := f.store.FinishJob(ctx, a2.ID, app.Ended{State: domain.StateCancelled, At: at(2 * time.Minute), Name: "Eng", Report: report}); !ok || err != nil {
		t.Fatalf("FinishJob() = %v, %v", ok, err)
	}
	b2 := f.export(t, f.eng, f.bob, at(2*time.Minute)) // the same time: by id
	f.done(t, f.ops, f.alice, at(3*time.Minute))

	all, err := f.store.ListJobs(ctx, f.eng, nil, nil, 10)
	if want := []uuid.UUID{b2.ID, a2.ID, b1.ID, a1.ID}; err != nil || !slices.Equal(ids(all), want) {
		t.Fatalf("ListJobs() = %v, %v; want %v", ids(all), err, want)
	}
	if r := all[1].Report; r == nil || r.Counts != report.Counts || r.Problems != nil {
		t.Errorf("the listed report = %+v, want its counts without its problems", r)
	}
	if got, err := f.store.FindJob(ctx, a2.ID); err != nil || got.Report == nil || !slices.Equal(got.Report.Problems, report.Problems) {
		t.Errorf("FindJob() = %+v, %v; want the report's problems", got.Report, err)
	}
	page, err := f.store.ListJobs(ctx, f.eng, nil, nil, 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("ListJobs() = %v, %v", page, err)
	}
	last := page[1]
	rest, err := f.store.ListJobs(ctx, f.eng, nil, &app.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}, 2)
	if want := []uuid.UUID{b1.ID, a1.ID}; err != nil || !slices.Equal(ids(rest), want) {
		t.Errorf("the next page = %v, %v; want %v", ids(rest), err, want)
	}
	tie, err := f.store.ListJobs(ctx, f.eng, nil, &app.Cursor{CreatedAt: b2.CreatedAt, ID: b2.ID}, 1)
	if want := []uuid.UUID{a2.ID}; err != nil || !slices.Equal(ids(tie), want) {
		t.Errorf("the page after b2 = %v, %v; want a2, of its time", ids(tie), err)
	}
	alices, err := f.store.ListJobs(ctx, f.eng, &f.alice, nil, 10)
	if want := []uuid.UUID{a2.ID, a1.ID}; err != nil || !slices.Equal(ids(alices), want) {
		t.Errorf("alice's = %v, %v; want %v", ids(alices), err, want)
	}

	if err := f.store.DeleteJobsOfNotebooks(ctx, []uuid.UUID{f.eng}, at(0)); err != nil {
		t.Fatal(err)
	}
	if left, err := f.store.ListJobs(ctx, f.eng, nil, nil, 10); len(left) != 0 || err != nil {
		t.Errorf("ListJobs() of a deleted notebook = %v, %v; want none", ids(left), err)
	}
	if _, err := f.store.FindJob(ctx, a1.ID); !errors.Is(err, app.ErrNoRow) {
		t.Errorf("FindJob() of a deleted job = %v, want ErrNoRow", err)
	}
	if n, err := f.store.CountActive(ctx); n != 0 || err != nil {
		t.Errorf("CountActive() = %d, %v; want the deleted not counted", n, err)
	}
}

// A deleted job, running, reads its deletion back at its next heartbeat;
// it is not finished, nor started when queued.
func TestADeletedJobStops(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	running, queued := f.export(t, f.eng, f.alice, at(0)), f.export(t, f.eng, f.bob, at(0))
	f.start(t, running.ID, at(0))
	if err := f.store.DeleteJobsOfNotebooks(ctx, []uuid.UUID{f.eng}, at(0)); err != nil {
		t.Fatal(err)
	}
	beat, err := f.store.BeatJob(ctx, running.ID, at(0), domain.Progress{})
	if err != nil || !beat.Deleted {
		t.Errorf("BeatJob() = %+v, %v; want the deletion read", beat, err)
	}
	if ok, err := f.store.FinishJob(ctx, running.ID, app.Ended{State: domain.StateFailed, At: at(0), Name: "x"}); ok || err != nil {
		t.Errorf("FinishJob() of a deleted job = %v, %v; want false", ok, err)
	}
	if _, err := f.store.StartJob(ctx, queued.ID, at(0)); !errors.Is(err, app.ErrNoRow) {
		t.Errorf("StartJob() of a deleted job = %v, want ErrNoRow", err)
	}
	if got, err := f.store.InterruptJobs(ctx, nil, at(0), domain.Report{Failure: domain.FailureInterrupted}); len(got) != 0 || err != nil {
		t.Errorf("InterruptJobs() = %v, %v; want the deleted left", got, err)
	}
}

// A success expires the account's earlier successes in the notebook, no
// other's; the expiry takes the exports that succeeded before a time, a
// batch at a time, skipping the rows another transaction holds; the
// archives kept are the successes'.
func TestExportsExpire(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	old := f.done(t, f.eng, f.alice, at(0))
	bobs := f.done(t, f.eng, f.bob, at(0))
	ops := f.done(t, f.ops, f.alice, at(0))
	latest := f.done(t, f.eng, f.alice, at(time.Hour))
	running := f.export(t, f.eng, f.alice, at(time.Hour))
	f.start(t, running.ID, at(time.Hour))

	expired, err := f.store.ExpireOthers(ctx, f.eng, f.alice, latest.ID)
	if err != nil || !slices.Equal(expired, []uuid.UUID{old.ID}) {
		t.Errorf("ExpireOthers() = %v, %v; want the older alone", expired, err)
	}
	live, err := f.store.LiveArchives(ctx, []uuid.UUID{old.ID, bobs.ID, ops.ID, latest.ID, running.ID, uuid.NewV7()})
	if want := []uuid.UUID{bobs.ID, ops.ID, latest.ID}; err != nil || !sameSet(live, want) {
		t.Errorf("LiveArchives() = %v, %v; want %v", live, err, want)
	}
	release := f.hold(t, ops.ID)
	first, err := f.store.ExpireExports(skipping(t), at(time.Minute), 10)
	if err != nil || !slices.Equal(first, []uuid.UUID{bobs.ID}) {
		t.Fatalf("ExpireExports() = %v, %v; want bob's, ops' held", first, err)
	}
	release()
	second, err := f.store.ExpireExports(ctx, at(time.Minute), 1)
	if err != nil || !slices.Equal(second, []uuid.UUID{ops.ID}) {
		t.Errorf("ExpireExports() = %v, %v; want ops' of a batch of one", second, err)
	}
	got, err := f.store.FindJob(ctx, bobs.ID)
	if err != nil || got.State != domain.StateExpired || got.ResultBytes == nil {
		t.Errorf("FindJob() = %+v, %v; want expired with its bytes", got, err)
	}
}

func sameSet(a, b []uuid.UUID) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.SortFunc(a, func(x, y uuid.UUID) int { return x.Compare(y) })
	slices.SortFunc(b, func(x, y uuid.UUID) int { return x.Compare(y) })
	return slices.Equal(a, b)
}

// The rescue fails every running job at the start, and those whose
// heartbeat is old after, skipping the rows another transaction holds;
// queued jobs and ended ones are left.
func TestTheRescueFailsTheRunningJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	stale, fresh, queued := f.export(t, f.eng, f.alice, at(0)), f.export(t, f.eng, f.bob, at(0)), f.export(t, f.ops, f.alice, at(0))
	f.start(t, stale.ID, at(0))
	f.start(t, fresh.ID, at(time.Minute))
	report := domain.Report{Failure: domain.FailureInterrupted}
	release := f.hold(t, stale.ID)
	if got, err := f.store.InterruptJobs(skipping(t), nil, at(time.Hour), report); err != nil || len(got) != 1 || got[0].ID != fresh.ID {
		t.Errorf("InterruptJobs() with the stale one held = %+v, %v; want the fresh one alone", got, err)
	}
	release()
	if _, err := f.pool.Exec(ctx, "UPDATE transfer_jobs SET state = 'running', finished_at = NULL, report = NULL WHERE id = $1", fresh.ID); err != nil {
		t.Fatal(err)
	}

	before := at(30 * time.Second)
	got, err := f.store.InterruptJobs(ctx, &before, at(time.Hour), report)
	if err != nil || len(got) != 1 || got[0].ID != stale.ID || got[0].NotebookID != f.eng || got[0].CreatedBy != f.alice || got[0].Client != domain.ClientWeb {
		t.Errorf("InterruptJobs(before) = %+v, %v; want the stale one", got, err)
	}
	failed, err := f.store.FindJob(ctx, stale.ID)
	if err != nil || failed.State != domain.StateFailed || failed.Report == nil || failed.Report.Failure != domain.FailureInterrupted {
		t.Errorf("FindJob() = %+v, %v", failed, err)
	}
	got, err = f.store.InterruptJobs(ctx, nil, at(time.Hour), report)
	if err != nil || len(got) != 1 || got[0].ID != fresh.ID {
		t.Errorf("InterruptJobs(all) = %+v, %v; want the fresh one", got, err)
	}
	if j, err := f.store.FindJob(ctx, queued.ID); err != nil || j.State != domain.StateQueued {
		t.Errorf("the queued job = %+v, %v; want it left", j, err)
	}
}

// The rescue reads the queued exports, not an import, and fails those of
// them it is given that are still queued, skipping the rows another
// transaction holds; a deleted job is neither read nor failed.
func TestTheRescueFailsTheQueuedExportsRiverDropped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dropped, held, running := f.export(t, f.eng, f.alice, at(0)), f.export(t, f.ops, f.alice, at(0)), f.export(t, f.eng, f.bob, at(0))
	f.start(t, running.ID, at(0))
	imported := domain.Job{ID: uuid.NewV7(), NotebookID: f.eng, Kind: domain.KindImport, State: domain.StateQueued, Name: "Eng", CreatedBy: f.alice,
		Client: domain.ClientWeb, CreatedAt: at(0)}
	if err := f.store.CreateJob(ctx, imported); err != nil {
		t.Fatal(err)
	}
	report := domain.Report{Failure: domain.FailureInterrupted}

	queued, err := f.store.QueuedExports(ctx)
	if err != nil || !sameSet(queued, []uuid.UUID{dropped.ID, held.ID}) {
		t.Errorf("QueuedExports() = %v, %v; want the two queued exports", queued, err)
	}
	release := f.hold(t, held.ID)
	got, err := f.store.FailQueued(skipping(t), []uuid.UUID{dropped.ID, held.ID, running.ID}, at(time.Hour), report)
	if err != nil || len(got) != 1 || got[0].ID != dropped.ID || got[0].NotebookID != f.eng || got[0].CreatedBy != f.alice || got[0].Client != domain.ClientWeb {
		t.Errorf("FailQueued() = %+v, %v; want the dropped one, the held one skipped, the running one left", got, err)
	}
	release()
	failed, err := f.store.FindJob(ctx, dropped.ID)
	if err != nil || failed.State != domain.StateFailed || failed.Report == nil || failed.Report.Failure != domain.FailureInterrupted ||
		!failed.Finished.Equal(at(time.Hour)) || failed.Started != nil {
		t.Errorf("FindJob() = %+v, %v; want failed, never started", failed, err)
	}
	if j, err := f.store.FindJob(ctx, running.ID); err != nil || j.State != domain.StateRunning {
		t.Errorf("the running job = %+v, %v; want it left", j, err)
	}
	if err := f.store.DeleteJobsOfNotebooks(ctx, []uuid.UUID{f.ops}, at(0)); err != nil {
		t.Fatal(err)
	}
	if queued, err := f.store.QueuedExports(ctx); err != nil || len(queued) != 0 {
		t.Errorf("QueuedExports() = %v, %v; want none, the deleted one left", queued, err)
	}
	if got, err := f.store.FailQueued(ctx, []uuid.UUID{held.ID}, at(time.Hour), report); err != nil || len(got) != 0 {
		t.Errorf("FailQueued() of a deleted job = %+v, %v; want none", got, err)
	}
}

// The purge's batch holds its rows until its transaction ends: a second
// purge skips them.
func TestThePurgeTakesTheDeletedJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a, b := f.done(t, f.eng, f.alice, at(0)), f.export(t, f.eng, f.bob, at(0))
	f.export(t, f.ops, f.alice, at(0))
	if err := f.store.DeleteJobsOfNotebooks(ctx, []uuid.UUID{f.eng}, at(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.ExpiredJobs(ctx, at(time.Second), 10); err == nil {
		t.Error("ExpiredJobs() on the pool = nil error, want it refused")
	}
	err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
		got, err := f.store.ExpiredJobs(ctx, at(time.Second), 10)
		if err != nil || len(got) != 2 || got[a.ID] != domain.KindExport || got[b.ID] != domain.KindExport {
			t.Errorf("ExpiredJobs() = %v, %v; want eng's two", got, err)
		}
		inner := f.tx.WithinTx(skipping(t), func(other context.Context) error {
			skipped, err := f.store.ExpiredJobs(other, at(time.Second), 10)
			if err != nil || len(skipped) != 0 {
				t.Errorf("another purge's ExpiredJobs() = %v, %v; want the locked rows skipped", skipped, err)
			}
			return nil
		})
		if inner != nil {
			return inner
		}
		n, err := f.store.DeleteJobs(ctx, []uuid.UUID{a.ID, b.ID})
		if n != 2 || err != nil {
			t.Errorf("DeleteJobs() = %d, %v", n, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var left int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM transfer_jobs").Scan(&left); err != nil || left != 1 {
		t.Errorf("%d jobs left, %v; want ops' alone", left, err)
	}
}

// The queue's lock serializes the creations: one waits for the other's
// transaction.
func TestTheQueueLockSerializesCreations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.store.LockQueue(ctx); err == nil {
		t.Error("LockQueue() on the pool = nil error, want it refused")
	}
	held, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- f.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := f.store.LockQueue(ctx); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	locked := make(chan error, 1)
	go func() {
		locked <- f.tx.WithinTx(ctx, func(ctx context.Context) error { return f.store.LockQueue(ctx) })
	}()
	pgtest.WaitForLockWaits(t, f.pool, 1, 10*time.Second)
	select {
	case err := <-locked:
		t.Fatalf("a second LockQueue() returned %v while the first held it", err)
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-locked:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the second LockQueue() still waits after the first ended")
	}
}
