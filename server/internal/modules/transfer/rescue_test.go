package transfer_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// The rescue as the module's root wires it, on a real database and River
// (M7/P5 design 3.12): as the jobs start, the running jobs the last process
// left fail, a background restore's and revert's too (M8/P4 design 3.6);
// then a run fails the running jobs whose heartbeat is older than
// Deps.HeartbeatTimeout and the queued jobs River no longer holds, of each
// kind, leaving those that beat and those River holds.
func TestTheRescueFailsTheJobsThatNoLongerRun(t *testing.T) {
	r := newRoot(t)
	bob, carol, dave := r.user(t, "bob"), r.user(t, "carol"), r.user(t, "dave")
	left := r.job(t, bob, "running", time.Now())
	restoring := r.jobOf(t, carol, domain.KindTrashRestore, "running", time.Now(), r.notebook(t))
	reverting := r.jobOf(t, dave, domain.KindRevert, "running", time.Now(), r.notebook(t))
	inserter, stop := r.start(t, nil, time.Hour, time.Minute)
	defer stop()
	for _, id := range []uuid.UUID{left, restoring, reverting} {
		if state, failure := r.state(t, id); state != "failed" || failure != "interrupted" {
			t.Fatalf("the job %s the last process left is %s %s, want failed interrupted as the jobs start", id, state, failure)
		}
	}

	stale, beating := r.job(t, bob, "running", time.Now().Add(-2*time.Minute)), r.job(t, carol, "running", time.Now())
	dropped, held := r.job(t, dave, "queued", time.Time{}), r.job(t, r.alice, "queued", time.Time{})
	r.enqueue(t, inserter, riveradapter.ExportArgs{JobID: held}, &river.InsertOpts{Queue: transfer.QueueExport, MaxAttempts: 1,
		ScheduledAt: time.Now().Add(time.Hour)})
	// A queued restore and revert River holds, and two it dropped, in
	// notebooks of their own: one writing job a notebook.
	restoreHeld := r.jobOf(t, bob, domain.KindTrashRestore, "queued", time.Time{}, r.notebook(t))
	revertHeld := r.jobOf(t, bob, domain.KindRevert, "queued", time.Time{}, r.notebook(t))
	restoreDropped := r.jobOf(t, carol, domain.KindTrashRestore, "queued", time.Time{}, r.notebook(t))
	revertDropped := r.jobOf(t, carol, domain.KindRevert, "queued", time.Time{}, r.notebook(t))
	later := &river.InsertOpts{Queue: transfer.QueueImport, MaxAttempts: 1, ScheduledAt: time.Now().Add(time.Hour)}
	r.enqueue(t, inserter, riveradapter.TrashRestoreArgs{JobID: restoreHeld}, later)
	r.enqueue(t, inserter, riveradapter.RevertArgs{JobID: revertHeld}, later)
	ran := r.rescues(t)
	r.enqueue(t, inserter, rescueNow{}, nil)
	for deadline := time.Now().Add(15 * time.Second); r.rescues(t) == ran; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			var states []string
			if err := r.pool.QueryRow(context.Background(), "SELECT coalesce(array_agg(state || ' ' || errors::text), '{}') FROM river_job WHERE kind = $1",
				riveradapter.RescueKind).Scan(&states); err != nil {
				t.Fatal(err)
			}
			t.Fatalf("the rescue did not complete: %v", states)
		}
	}

	for _, tt := range []struct {
		name          string
		id            uuid.UUID
		state, reason string
	}{
		{"its heartbeat old", stale, "failed", "interrupted"},
		{"beating", beating, "running", ""},
		{"dropped by River", dropped, "failed", "interrupted"},
		{"held by River", held, "queued", ""},
		{"a restore held by River", restoreHeld, "queued", ""},
		{"a revert held by River", revertHeld, "queued", ""},
		{"a restore dropped by River", restoreDropped, "failed", "interrupted"},
		{"a revert dropped by River", revertDropped, "failed", "interrupted"},
	} {
		if state, failure := r.state(t, tt.id); state != tt.state || failure != tt.reason {
			t.Errorf("the job %s is %s %s, want %s %s", tt.name, state, failure, tt.state, tt.reason)
		}
	}
}

// rescueNow is the rescue's job, enqueued now rather than at its interval.
type rescueNow struct{}

func (rescueNow) Kind() string { return riveradapter.RescueKind }

// rescues is how many of the rescue's River jobs completed.
func (r root) rescues(t *testing.T) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), "SELECT count(*) FROM river_job WHERE kind = $1 AND state = 'completed'", riveradapter.RescueKind).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// user adds the account name.
func (r root) user(t *testing.T, name string) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	if _, err := r.pool.Exec(context.Background(), "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, $2, 'x', $3, now(), now())",
		id, name+"@corp.com", name); err != nil {
		t.Fatal(err)
	}
	return id
}

// job adds by's export of Eng in state, queued or running, a running one's
// heartbeat at beat; no River job is enqueued.
func (r root) job(t *testing.T, by uuid.UUID, state string, beat time.Time) uuid.UUID {
	t.Helper()
	return r.jobOf(t, by, domain.KindExport, state, beat, r.eng)
}

// jobOf adds by's job of kind in notebook, in state, queued or running, a
// running one's heartbeat at beat, a restore's or a revert's with its
// changeset and target; no River job is enqueued.
func (r root) jobOf(t *testing.T, by uuid.UUID, kind domain.Kind, state string, beat time.Time, notebook uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	var started *time.Time
	var root, changeset, target *uuid.UUID
	if state == "running" {
		started = &beat
	}
	switch kind {
	case domain.KindTrashRestore:
		root, changeset = new(uuid.NewV7()), new(uuid.NewV7())
	case domain.KindRevert:
		changeset, target = new(uuid.NewV7()), new(uuid.NewV7())
	}
	if _, err := r.pool.Exec(context.Background(), `INSERT INTO transfer_jobs (id, notebook_id, root_id, kind, state, name, created_by_id, client,
		started_at, heartbeat_at, created_at, changeset_id, target_changeset_id)
		VALUES ($1, $2, $3, $4, $5, 'Eng', $6, 'api', $7, $7, now(), $8, $9)`, id, notebook, root, string(kind), state, by, started, changeset,
		target); err != nil {
		t.Fatal(err)
	}
	return id
}

// notebook adds a notebook of acme's, alice's.
func (r root) notebook(t *testing.T) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	if _, err := r.pool.Exec(context.Background(), `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT $1, workspace_id, 'Other', created_by_id, created_by_id, now(), now() FROM notebooks WHERE id = $2`, id, r.eng); err != nil {
		t.Fatal(err)
	}
	return id
}

// enqueue enqueues args with opts on River.
func (r root) enqueue(t *testing.T, inserter *jobs.Inserter, args river.JobArgs, opts *river.InsertOpts) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := inserter.InsertTx(ctx, tx, args, opts); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// state is the job id's state and its failure, if any.
func (r root) state(t *testing.T, id uuid.UUID) (string, string) {
	t.Helper()
	var state, failure string
	if err := r.pool.QueryRow(context.Background(), "SELECT state, coalesce(report->>'failure', '') FROM transfer_jobs WHERE id = $1", id).
		Scan(&state, &failure); err != nil {
		t.Fatal(err)
	}
	return state, failure
}
