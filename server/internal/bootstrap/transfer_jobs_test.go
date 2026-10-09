package bootstrap

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The exports' background jobs as the server's jobs start (M7/P5 design
// 3.12): the rescue of the jobs left running, the expiry, the sweep of
// the archives no export keeps, and the purge. Each test seeds its jobs
// through SQL and their archives in the store before serve starts.

// seededJob is an export the tests seed, of a notebook of its own named
// after it: in state, created, started and ended ago, its archive in the
// store, as old as archiveAgo, when archived. A job of no state has no row:
// its archive is an orphan's.
type seededJob struct {
	name       string
	id         uuid.UUID
	state      string
	ago        time.Duration
	deleted    bool // with its notebook, at the job's time
	archived   bool
	archiveAgo time.Duration
}

// seedJobs writes alice, acme, the jobs and their notebooks into the
// database at pool and the store at dir.
func seedJobs(t *testing.T, pool *pgxpool.Pool, dir string, seeded []seededJob) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	user, workspace := uuid.NewV7(), uuid.NewV7()
	exec("INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@example.com', 'x', 'Alice', now(), now())", user)
	exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())",
		workspace, user)
	notebook := "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at, deleted_at) " +
		"VALUES ($1, $2, $3, $4, $4, now(), now(), $5)"
	store, err := storage.OpenLocal(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range seeded {
		at := time.Now().Add(-j.ago)
		if j.state != "" {
			nb := uuid.NewV7()
			var deletedAt *time.Time
			if j.deleted {
				deletedAt = &at
			}
			exec(notebook, nb, workspace, j.name, user, deletedAt)
			exec(`INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, started_at, heartbeat_at, finished_at,
				report, result_bytes, created_at, deleted_at)
				SELECT $1, $2, 'export', $3::text, $4, $5, 'web', CASE WHEN $3 <> 'queued' THEN $6::timestamptz END,
					CASE WHEN $3 <> 'queued' THEN $6::timestamptz END, CASE WHEN $3 NOT IN ('queued', 'running') THEN $6::timestamptz END,
					CASE WHEN $3 NOT IN ('queued', 'running') THEN '{"failure": null, "counts": {"pages": 1, "attachments": 0, "renamed": 0,
						"missing": 0, "skipped": 0}, "problems": [], "problems_truncated": false}'::jsonb END,
					CASE WHEN $3 IN ('succeeded', 'expired') THEN 3 END, $6, $7`,
				j.id, nb, j.state, j.name, user, at, deletedAt)
		}
		if !j.archived {
			continue
		}
		w, err := store.Create(ctx, "exports/"+j.id.String()+".zip")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("zip")); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
		if j.archiveAgo != 0 {
			old := time.Now().Add(-j.archiveAgo)
			if err := os.Chtimes(storedArchives(t, dir)[j.id.String()], old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// archivedJobs are the names of the seeded jobs whose archive is in the
// store at dir.
func archivedJobs(t *testing.T, dir string, seeded []seededJob) []string {
	t.Helper()
	files := storedArchives(t, dir)
	var out []string
	for _, j := range seeded {
		if _, ok := files[j.id.String()]; ok {
			out = append(out, j.name)
		}
	}
	return out
}

// jobStates are the seeded jobs' rows, each its name and state, by name.
func jobStates(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	return queryStrings(t, pool, "SELECT name || ' ' || state FROM transfer_jobs ORDER BY name")
}

// As serve starts, before River works any job, every job left running is
// failed, interrupted, whatever its heartbeat; a queued one stays queued.
func TestServeFailsTheJobsLeftRunning(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	seedJobs(t, pool, cfg.Storage.Dir, []seededJob{
		{name: "beating", id: uuid.NewV7(), state: "running"},
		{name: "waiting", id: uuid.NewV7(), state: "queued"},
	})

	startApp(t, cfg, migrations.FS())

	// The jobs start beside HTTP: the rescue runs a moment after serve
	// answers.
	want := []string{"beating failed", "waiting queued"}
	for deadline := time.Now().Add(10 * time.Second); !slices.Equal(jobStates(t, pool), want); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("jobs %q, want the running one failed, the queued one queued", jobStates(t, pool))
		}
	}
	if got := queryStrings(t, pool, "SELECT report->>'failure' FROM transfer_jobs WHERE state = 'failed' AND finished_at IS NOT NULL"); !slices.Equal(got,
		[]string{"interrupted"}) {
		t.Errorf("the failure %q, want interrupted, ended", got)
	}
}

// The expiry expires the exports that succeeded longer than
// transfer.export_ttl ago, and deletes their archives; a newer one stays.
func TestTheExpiryExpiresTheOldExports(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	seeded := []seededJob{
		{name: "old", id: uuid.NewV7(), state: "succeeded", ago: cfg.Transfer.ExportTTL + time.Hour, archived: true},
		{name: "new", id: uuid.NewV7(), state: "succeeded", ago: cfg.Transfer.ExportTTL - time.Hour, archived: true},
	}
	seedJobs(t, pool, cfg.Storage.Dir, seeded)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, "transfer.expire_exports")

	if got := jobStates(t, pool); !slices.Equal(got, []string{"new succeeded", "old expired"}) {
		t.Errorf("jobs %q, want old expired, new succeeded", got)
	}
	if got := archivedJobs(t, cfg.Storage.Dir, seeded); !slices.Equal(got, []string{"new"}) {
		t.Errorf("archives of %q, want new's", got)
	}
}

// The sweep deletes the archives older than a day that no export that
// succeeded keeps: an orphan's, an expired one's. It leaves a newer
// orphan, which an export may still be about to succeed with, and a live
// export's, however old its file.
func TestTheSweepDeletesTheArchivesNoExportKeeps(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	seeded := []seededJob{
		{name: "orphan", id: uuid.NewV7(), archived: true, archiveAgo: 48 * time.Hour},
		{name: "new orphan", id: uuid.NewV7(), archived: true, archiveAgo: time.Hour},
		{name: "expired", id: uuid.NewV7(), state: "expired", ago: 48 * time.Hour, archived: true, archiveAgo: 48 * time.Hour},
		{name: "live", id: uuid.NewV7(), state: "succeeded", ago: time.Hour, archived: true, archiveAgo: 48 * time.Hour},
	}
	seedJobs(t, pool, cfg.Storage.Dir, seeded)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, "transfer.sweep_orphan_archives")

	if got := archivedJobs(t, cfg.Storage.Dir, seeded); !slices.Equal(got, []string{"new orphan", "live"}) {
		t.Errorf("archives of %q, want the new orphan's and the live export's", got)
	}
}

// The purge takes the jobs deleted longer than the retention ago, their
// archives first, before their notebooks, which their rows reference; a
// job deleted since stays with its archive and its notebook.
func TestThePurgeDeletesTheJobsBeforeTheirNotebooks(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	seeded := []seededJob{
		{name: "old", id: uuid.NewV7(), state: "succeeded", ago: cfg.Jobs.PurgeRetention + 24*time.Hour, deleted: true, archived: true},
		{name: "recent", id: uuid.NewV7(), state: "succeeded", ago: cfg.Jobs.PurgeRetention - 24*time.Hour, deleted: true, archived: true},
		{name: "live", id: uuid.NewV7(), state: "succeeded", ago: time.Hour, archived: true},
	}
	seedJobs(t, pool, cfg.Storage.Dir, seeded)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, jobs.PurgeKind)

	if got := jobStates(t, pool); !slices.Equal(got, []string{"live succeeded", "recent succeeded"}) {
		t.Errorf("jobs %q, want recent and live", got)
	}
	if got := archivedJobs(t, cfg.Storage.Dir, seeded); !slices.Equal(got, []string{"recent", "live"}) {
		t.Errorf("archives of %q, want recent's and live's", got)
	}
	if got := queryStrings(t, pool, "SELECT name FROM notebooks ORDER BY name"); !slices.Equal(got, []string{"live", "recent"}) {
		t.Errorf("notebooks %q, want live's and recent's", got)
	}
}
