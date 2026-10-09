package transfer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	riveradapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/river"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The export contributors as the module's root takes them (M7/P5 design
// 3.10), on a real database and River: Deps.Contributors reach the
// exports, run in their order, their files in the archive and meta.json;
// the first error stops the rest; a file where a node is fails the export
// contributor_conflict. Deps.JobTimeout bounds an export's run.

// root is a database with alice's notebook Eng, its page Readme, and the
// store the exports write in.
type root struct {
	pool               *pgxpool.Pool
	store              storage.Store
	alice, eng, readme uuid.UUID
}

func newRoot(t *testing.T) root {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 8})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	r := root{pool: pool, store: store, alice: uuid.NewV7(), eng: uuid.NewV7(), readme: uuid.NewV7()}
	acme := uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'Alice', now(), now())", []any{r.alice}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, r.alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now())", []any{r.eng, acme, r.alice}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	return r
}

// start wires the module on the root's database with the contributors,
// job timeout and heartbeat timeout given, and starts its jobs on River:
// stop stops them.
func (r root) start(t *testing.T, contributors []transfer.ExportContributor, timeout, heartbeat time.Duration) (inserter *jobs.Inserter, stop func()) {
	t.Helper()
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	tx := postgres.NewTxManager(r.pool, 5*time.Second)
	inserter, err := jobs.NewInserter(r.pool, logger)
	if err != nil {
		t.Fatal(err)
	}
	m := transfer.New(transfer.Deps{Pool: r.pool, Tx: tx, Snapshots: tx, Store: r.store, Inserter: inserter, Clock: clock{}, Logger: logger,
		Authorizer: readers{}, Workspaces: workspaces{}, Notebooks: notebooks{eng: r.eng}, Names: names{}, Nodes: nodes{readme: r.readme},
		Linked: linked{}, Blobs: blobs{}, Contributors: contributors, DownloadKey: bytes.Repeat([]byte{1}, 32), ExportTTL: 24 * time.Hour,
		JobTimeout: timeout, HeartbeatTimeout: heartbeat, MaxQueued: 20, MinRate: 1})
	runner, err := jobs.New(r.pool, jobs.Config{ShutdownTimeout: 5 * time.Second, Queues: map[string]int{transfer.QueueExport: 1},
		RescueAfter: time.Hour, Logger: logger}, m.Jobs())
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return inserter, func() {
		if err := runner.Stop(ctx); err != nil {
			t.Error(err)
		}
	}
}

// export runs alice's export of Eng through the module's jobs, contributors
// and timeout given, and answers its row's state and failure once it
// ended, and its archive's entries by name when it succeeded.
func (r root) export(t *testing.T, contributors []transfer.ExportContributor, timeout time.Duration) (string, string, map[string]string) {
	t.Helper()
	ctx := context.Background()
	inserter, stop := r.start(t, contributors, timeout, 5*time.Minute)
	defer stop()

	tx := postgres.NewTxManager(r.pool, 5*time.Second)
	id := uuid.NewV7()
	err := tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := postgres.DB(ctx, r.pool).Exec(ctx, `INSERT INTO transfer_jobs (id, notebook_id, kind, state, name, created_by_id, client, created_at)
			VALUES ($1, $2, 'export', 'queued', 'Eng', $3, 'api', now())`, id, r.eng, r.alice); err != nil {
			return err
		}
		pgxTx, _ := postgres.TxFrom(ctx)
		return inserter.InsertTx(ctx, pgxTx, riveradapter.ExportArgs{JobID: id}, &river.InsertOpts{Queue: transfer.QueueExport, MaxAttempts: 1})
	})
	if err != nil {
		t.Fatal(err)
	}
	var state, failure string
	for deadline := time.Now().Add(15 * time.Second); ; {
		err := r.pool.QueryRow(ctx, `SELECT state, coalesce(report->>'failure', '') FROM transfer_jobs WHERE id = $1`, id).Scan(&state, &failure)
		if err != nil {
			t.Fatal(err)
		}
		if state != "queued" && state != "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the export is still %s", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state != "succeeded" {
		return state, failure, nil
	}
	return state, failure, r.entries(t, id)
}

// entries are the export id's archive's files, by name.
func (r root) entries(t *testing.T, id uuid.UUID) map[string]string {
	t.Helper()
	f, err := r.store.Open(context.Background(), "exports/"+id.String()+".zip")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	z, err := zip.NewReader(f, f.Size())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range z.File {
		rc, err := e.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name] = string(data)
	}
	return out
}

func TestTheModulesContributorsAddToTheExports(t *testing.T) {
	r := newRoot(t)
	var calls calls
	state, failure, files := r.export(t, []transfer.ExportContributor{
		adds{name: "index", calls: &calls, files: map[string]string{"index.md": "# Index"}},
		adds{name: "log", calls: &calls, files: map[string]string{"log/2026.md": "- exported"}},
	}, time.Hour)
	if state != "succeeded" || failure != "" || !slices.Equal(calls.got(), []string{"index", "log"}) {
		t.Fatalf("the export %s %s, the contributors called %v; want succeeded, index then log", state, failure, calls.got())
	}
	if files["Eng/index.md"] != "# Index" || files["Eng/log/2026.md"] != "- exported" || files["Eng/Readme.md"] != "hello" {
		t.Errorf("the archive = %v, want the contributors' files beside the page's", files)
	}
	var meta struct {
		Contributed []string `json:"contributed"`
	}
	if err := json.Unmarshal([]byte(files["Eng/.nerve/meta.json"]), &meta); err != nil || !slices.Equal(meta.Contributed, []string{"index.md", "log/2026.md"}) {
		t.Errorf("meta.json's contributed = %v, %v; want the two files in their order", meta.Contributed, err)
	}
}

func TestTheFirstContributorsErrorStopsTheExport(t *testing.T) {
	r := newRoot(t)
	var calls calls
	state, failure, _ := r.export(t, []transfer.ExportContributor{
		adds{name: "broken", calls: &calls, err: errors.New("the index is broken")},
		adds{name: "log", calls: &calls, files: map[string]string{"log.md": "-"}},
	}, time.Hour)
	if state != "failed" || failure != "internal" || !slices.Equal(calls.got(), []string{"broken"}) {
		t.Errorf("the export %s %s, the contributors called %v; want failed internal, the first alone", state, failure, calls.got())
	}

	state, failure, _ = r.export(t, []transfer.ExportContributor{adds{name: "over", calls: &calls, files: map[string]string{"readme.md": "x"}}}, time.Hour)
	if state != "failed" || failure != "contributor_conflict" {
		t.Errorf("a file where the page's is: the export %s %s, want failed contributor_conflict", state, failure)
	}
}

func TestTheJobTimeoutBoundsAnExport(t *testing.T) {
	r := newRoot(t)
	state, failure, _ := r.export(t, []transfer.ExportContributor{waits{}}, 300*time.Millisecond)
	if state != "failed" || failure != "timeout" {
		t.Errorf("the export %s %s, want failed timeout", state, failure)
	}
}

// calls records the contributors' names as they are called.
type calls struct {
	mu    sync.Mutex
	names []string
}

func (c *calls) add(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.names = append(c.names, name)
}

func (c *calls) got() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.names)
}

// adds is a contributor that adds its files, in their paths' order, or
// fails with err.
type adds struct {
	name  string
	calls *calls
	files map[string]string
	err   error
}

func (a adds) Contribute(_ context.Context, _ transfer.ExportScope, sink transfer.ExportSink) error {
	a.calls.add(a.name)
	if a.err != nil {
		return a.err
	}
	paths := make([]string, 0, len(a.files))
	for p := range a.files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		if err := sink.Add(p, []byte(a.files[p])); err != nil {
			return err
		}
	}
	return nil
}

// waits is a contributor that waits until its export's context ends.
type waits struct{}

func (waits) Contribute(ctx context.Context, _ transfer.ExportScope, _ transfer.ExportSink) error {
	<-ctx.Done()
	return context.Cause(ctx)
}

type clock struct{}

func (clock) Now() time.Time { return time.Now() }

// readers lets every account read every notebook.
type readers struct{}

func (readers) Authorize(context.Context, shared.Actor, shared.Action, shared.Target) (shared.Grant, error) {
	return shared.Grant{WorkspaceRole: shared.WorkspaceMember, NotebookRole: shared.NotebookReader}, nil
}

type workspaces struct{}

func (workspaces) ShareByID(context.Context, uuid.UUID) (bool, error) { return true, nil }

// notebooks has Eng alone.
type notebooks struct{ eng uuid.UUID }

func (n notebooks) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return uuid.UUID{}, id == n.eng, nil
}

func (n notebooks) ShareByID(_ context.Context, id uuid.UUID) (bool, error) { return id == n.eng, nil }

func (n notebooks) NameOf(_ context.Context, id uuid.UUID) (string, bool, error) {
	return "Eng", id == n.eng, nil
}

type names struct{}

func (names) DisplayNames(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}

// nodes is Eng's tree: its page Readme, its content "hello".
type nodes struct{ readme uuid.UUID }

func (n nodes) Page(_ context.Context, _, id uuid.UUID) (string, bool, error) {
	return "Readme", id == n.readme, nil
}

func (n nodes) Depth(_ context.Context, _, id uuid.UUID) (int, bool, error) {
	return 1, id == n.readme, nil
}

func (n nodes) Scope(context.Context, uuid.UUID, *uuid.UUID) ([]transfer.Node, error) {
	return []transfer.Node{{ID: n.readme, Name: "Readme", Bytes: int64(len("hello")), Modified: time.Now()}}, nil
}

func (n nodes) Contents(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{n.readme: "hello"}, nil
}

type linked struct{}

func (linked) Linked(context.Context, []uuid.UUID, []uuid.UUID) ([]uuid.UUID, error) { return nil, nil }

type blobs struct{}

func (blobs) Of(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]transfer.Blob, error) {
	return map[uuid.UUID]transfer.Blob{}, nil
}

func (blobs) Open(context.Context, uuid.UUID) (io.ReadCloser, error) {
	return nil, transfer.ErrFileMissing
}
