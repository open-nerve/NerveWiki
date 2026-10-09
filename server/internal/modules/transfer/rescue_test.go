package transfer_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
)

// The rescue as the module's root wires it, on a real database and River
// (M7/P5 design 3.12): as the jobs start, the running jobs the last process
// left fail; then a run fails the running jobs whose heartbeat is older
// than Deps.HeartbeatTimeout and the queued exports River no longer holds,
// leaving those that beat and those River holds.
func TestTheRescueFailsTheJobsThatNoLongerRun(t *testing.T) {
	r := newRoot(t)
	bob, carol, dave := r.user(t, "bob"), r.user(t, "carol"), r.user(t, "dave")
	left := r.job(t, bob, "running", time.Now())
	inserter, stop := r.start(t, nil, time.Hour, time.Minute)
	defer stop()
	if state, failure := r.state(t, left); state != "failed" || failure != "interrupted" {
		t.Fatalf("the job the last process left is %s %s, want failed interrupted as the jobs start", state, failure)
	}

	stale, beating := r.job(t, bob, "running", time.Now().Add(-2*time.Minute)), r.job(t, carol, "running", time.Now())
	dropped, held := r.job(t, dave, "queued", time.Time{}), r.job(t, r.alice, "queued", time.Time{})
	r.enqueue(t, inserter, riveradapter.ExportArgs{JobID: held}, &river.InsertOpts{Queue: transfer.QueueExport, MaxAttempts: 1,
		ScheduledAt: time.Now().Add(time.Hour)})
	r.enqueue(t, inserter, rescueNow{}, nil)
	ctx := context.Background()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		var done bool
		if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM river_job WHERE kind = $1 AND state = 'completed')", riveradapter.RescueKind).
			Scan(&done); err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the rescue did not run")
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
	} {
		if state, failure := r.state(t, tt.id); state != tt.state || failure != tt.reason {
			t.Errorf("the job %s is %s %s, want %s %s", tt.name, state, failure, tt.state, tt.reason)
		}
	}
}

// rescueNow is the rescue's job, enqueued now rather than at its interval.
type rescueNow struct{}

func (rescueNow) Kind() string { return riveradapter.RescueKind }

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
	id := uuid.NewV7()
	var started *time.Time
	if state == "running" {
		started = &beat
	}
	if _, err := r.pool.Exec(context.Background(), `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, started_at, heartbeat_at, created_at)
		VALUES ($1, $2, 'export', $3, 'Eng', $4, 'api', $5, $5, now())`, id, r.eng, state, by, started); err != nil {
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
