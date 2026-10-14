package bootstrap

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

// The exports' background jobs as the server's jobs start (M7/P5 design
// 3.12): the rescue of the jobs left running, the expiry, the sweep of
// the archives no export keeps, and the purge. Each test seeds its jobs
// through SQL and their archives in the store before serve starts.

// seededJob is a job the tests seed, an export unless of another kind, of
// a notebook of its own named after it: in state, created, started and
// ended ago, its archive in the store, as old as archiveAgo, when
// archived; a background restore of an item, or a revert of a changeset,
// with its changeset (M8/P4 design 3.2), and the report its heartbeat
// wrote when running and reported. A job of no state has no row: its
// archive is an orphan's.
type seededJob struct {
	name       string
	id         uuid.UUID
	of         domain.Kind
	state      string
	ago        time.Duration
	deleted    bool // with its notebook, at the job's time
	archived   bool
	archiveAgo time.Duration
	reported   string
}

// kind is the job's kind.
func (j seededJob) kind() domain.Kind {
	return cmp.Or(j.of, domain.KindExport)
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
			var root, changeset, target *uuid.UUID
			switch j.kind() {
			case domain.KindTrashRestore:
				root, changeset = new(uuid.NewV7()), new(uuid.NewV7())
			case domain.KindRevert:
				changeset, target = new(uuid.NewV7()), new(uuid.NewV7())
			}
			var reported *string
			if j.reported != "" {
				reported = &j.reported
			}
			exec(`INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, started_at, heartbeat_at, finished_at,
				report, result_bytes, created_at, deleted_at, root_id, changeset_id, target_changeset_id)
				SELECT $1, $2, $8, $3::text, $4, $5, 'web', CASE WHEN $3 <> 'queued' THEN $6::timestamptz END,
					CASE WHEN $3 <> 'queued' THEN $6::timestamptz END, CASE WHEN $3 NOT IN ('queued', 'running') THEN $6::timestamptz END,
					CASE WHEN $3 NOT IN ('queued', 'running') THEN '{"failure": null, "counts": {"pages": 1, "attachments": 0, "renamed": 0,
						"missing": 0, "skipped": 0}, "problems": [], "problems_truncated": false}'::jsonb ELSE $12::jsonb END,
					CASE WHEN $3 IN ('succeeded', 'expired') THEN 3 END, $6, $7, $9, $10, $11`,
				j.id, nb, j.state, j.name, user, at, deletedAt, j.kind(), root, changeset, target, reported)
		}
		if !j.archived {
			continue
		}
		w, err := store.Create(ctx, domain.Archive(j.kind(), j.id))
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
			if err := os.Chtimes(archivesOf(t, dir)[j.kind()][j.id.String()], old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// archivesOf are the archives in the store at dir, each kind's paths by
// job.
func archivesOf(t *testing.T, dir string) map[domain.Kind]map[string]string {
	t.Helper()
	imports := map[string]string{}
	for _, path := range storedImports(t, dir) {
		imports[strings.TrimSuffix(filepath.Base(path), ".zip")] = path
	}
	return map[domain.Kind]map[string]string{domain.KindExport: storedArchives(t, dir), domain.KindImport: imports}
}

// archivedJobs are the names of the seeded jobs whose archive is in the
// store at dir.
func archivedJobs(t *testing.T, dir string, seeded []seededJob) []string {
	t.Helper()
	files := archivesOf(t, dir)
	var out []string
	for _, j := range seeded {
		if _, ok := files[j.kind()][j.id.String()]; ok {
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
// A background restore's and revert's keep the report their heartbeat
// wrote (M8/P4 design 3.6).
func TestServeFailsTheJobsLeftRunning(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	reported := `{"failure": null, "counts": {"pages": 2, "attachments": 1, "renamed": 0, "missing": 0, "skipped": 1, "restored": 4,
		"deleted": 5, "put_back": 0, "written": 0}, "problems": [], "problems_truncated": false}`
	seedJobs(t, pool, cfg.Storage.Dir, []seededJob{
		{name: "beating", id: uuid.NewV7(), state: "running"},
		{name: "waiting", id: uuid.NewV7(), state: "queued"},
		{name: "restoring", id: uuid.NewV7(), of: domain.KindTrashRestore, state: "running", reported: reported},
		{name: "reverting", id: uuid.NewV7(), of: domain.KindRevert, state: "running", reported: reported},
	})

	startApp(t, cfg, migrations.FS())

	// The jobs start beside HTTP: the rescue runs a moment after serve
	// answers.
	want := []string{"beating failed", "restoring failed", "reverting failed", "waiting queued"}
	for deadline := time.Now().Add(10 * time.Second); !slices.Equal(jobStates(t, pool), want); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("jobs %q, want the running ones failed, the queued one queued", jobStates(t, pool))
		}
	}
	if got := queryStrings(t, pool, "SELECT name || ' ' || (report->>'failure') || ' ' || (report->'counts'->>'pages') || ' ' || "+
		"coalesce(report->'counts'->>'restored', '-') || ' ' || coalesce(report->'counts'->>'deleted', '-') FROM transfer_jobs "+
		"WHERE state = 'failed' AND finished_at IS NOT NULL ORDER BY name"); !slices.Equal(got,
		[]string{"beating interrupted 0 0 0", "restoring interrupted 2 4 5", "reverting interrupted 2 4 5"}) {
		t.Errorf("the failures %q, want each interrupted, ended, the background jobs' counts kept", got)
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
// export's, however old its file. So the imports': it deletes an old
// archive no import's row has, or whose import ended, and leaves a newer
// one, and a queued import's, which keeps no export's archive of its id.
func TestTheSweepDeletesTheArchivesNoJobKeeps(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	seeded := []seededJob{
		{name: "orphan", id: uuid.NewV7(), archived: true, archiveAgo: 48 * time.Hour},
		{name: "new orphan", id: uuid.NewV7(), archived: true, archiveAgo: time.Hour},
		{name: "expired", id: uuid.NewV7(), state: "expired", ago: 48 * time.Hour, archived: true, archiveAgo: 48 * time.Hour},
		{name: "live", id: uuid.NewV7(), state: "succeeded", ago: time.Hour, archived: true, archiveAgo: 48 * time.Hour},
		{name: "upload", id: uuid.NewV7(), of: domain.KindImport, archived: true, archiveAgo: 48 * time.Hour},
		{name: "new upload", id: uuid.NewV7(), of: domain.KindImport, archived: true, archiveAgo: time.Hour},
		{name: "imported", id: uuid.NewV7(), of: domain.KindImport, state: "failed", ago: 48 * time.Hour, archived: true, archiveAgo: 48 * time.Hour},
		{name: "importing", id: uuid.NewV7(), of: domain.KindImport, state: "queued", archived: true, archiveAgo: 48 * time.Hour},
	}
	// An export's archive of the queued import's id, no export's row: the
	// import keeps none of it.
	seeded = append(seeded, seededJob{name: "importing's export", id: seeded[len(seeded)-1].id, archived: true, archiveAgo: 48 * time.Hour})
	seedJobs(t, pool, cfg.Storage.Dir, seeded)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, "transfer.sweep_orphan_archives")

	if got := archivedJobs(t, cfg.Storage.Dir, seeded); !slices.Equal(got, []string{"new orphan", "live", "new upload", "importing"}) {
		t.Errorf("archives of %q, want the new orphans', the live export's and the queued import's", got)
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

// The runner's settings: the exports and the imports in their queues, of
// jobs.export_workers and jobs.import_workers, and River's rescue past
// their timeout by an hour (M7/P5 design 3.2), so that a running job is
// never taken for stuck and discarded.
func TestTheJobsHaveTheirQueuesAndOutlastRiversRescue(t *testing.T) {
	cfg := config.Config{Jobs: config.JobsConfig{ShutdownTimeout: time.Second, ExportWorkers: 3, ImportWorkers: 2},
		Transfer: config.TransferConfig{JobTimeout: 6 * time.Hour}}
	got := jobsConfig(cfg, slog.New(slog.DiscardHandler))
	if !maps.Equal(got.Queues, map[string]int{transfer.QueueExport: 3, transfer.QueueImport: 2}) || got.RescueAfter != 7*time.Hour ||
		got.ShutdownTimeout != time.Second || got.Logger == nil {
		t.Errorf("jobs.Config = %+v, want transfer_export of 3 workers, transfer_import of 2, the rescue after 7h", got)
	}
}

// No job lasts as long as the attachments' orphan sweep waits for a file's
// row: an import stores a batch's files before the unit that writes their
// rows, within its job (Codex M7 review R2). The hour is far past a
// unit's COMMIT, which database.commit_timeout bounds past the job's end.
func TestNoJobOutlastsTheOrphanSweep(t *testing.T) {
	if config.MaxJobTimeout+time.Hour > asset.SweepAge {
		t.Errorf("transfer.job_timeout may be %s; the orphan sweep deletes a file %s old", config.MaxJobTimeout, asset.SweepAge)
	}
}

// The composition root hands the transfer module the page module's
// actions, and bounds the nodes an import makes by the items a background
// revert reverts (M8/P4 design 2.2 (a)). transferDeps sets no batch:
// newApp's, which wireApp sets, is the import's, which the whole-program
// tests pin through newApp (TestABackgroundRestoreThroughServe's two
// batches; M8/P4A review B-M1).
func TestTransferDepsLeaveTheBatchesTheImports(t *testing.T) {
	d := transferDeps(testConfig(t, "", false), nil, slog.New(slog.DiscardHandler), nil, nil, nil, nil, nil)
	if d.Batch != (transfer.Batch{}) {
		t.Errorf("transferDeps' batch = %+v, want none: newApp's is the one production uses", d.Batch)
	}
	if d.RestoreAction != page.ActionRestore || d.RevertAction != page.ActionRevert {
		t.Errorf("transferDeps' actions = %s, %s; want the page module's restore and revert", d.RestoreAction, d.RevertAction)
	}
	if d.ImportMaxNodes != page.MaxJobItems {
		t.Errorf("transferDeps' nodes of an import = %d, want the page module's MaxJobItems, %d", d.ImportMaxNodes, page.MaxJobItems)
	}
}
