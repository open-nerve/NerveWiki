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

// The attachments' jobs run as the server's jobs start (M7/P2 design 3.8):
// the purge deletes a file before its row, and the orphan sweep the old
// files no row holds. Each test seeds its attachments through SQL and the
// store before serve starts.

// seededAsset is an attachment the tests seed: its node and file's ids,
// when its node and row were deleted (zero: not), whether it has a row,
// and whether its file is in the store and is old.
type seededAsset struct {
	name                  string
	node, blob            uuid.UUID
	deleted               time.Duration // ago; 0 for not deleted
	rowless, unfiled, old bool
}

// seedAssets writes alice, acme, its notebook Eng and the attachments
// into the database at pool and the store at dir.
func seedAssets(t *testing.T, pool *pgxpool.Pool, dir string, assets []seededAsset) {
	t.Helper()
	ctx := context.Background()
	user, workspace, notebook := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@example.com', 'x', 'Alice', now(), now())",
			[]any{user}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())",
			[]any{workspace, user}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now())",
			[]any{notebook, workspace, user}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	store, err := storage.OpenLocal(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range assets {
		var deletedAt *time.Time
		if a.deleted != 0 {
			at := time.Now().Add(-a.deleted)
			deletedAt = &at
		}
		if _, err := pool.Exec(ctx, `INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
			created_at, updated_at, deleted_at) VALUES ($1, $2, 'asset', $3, $3, $4, $5, $5, now(), now(), $6)`,
			a.node, notebook, a.name, i, user, deletedAt); err != nil {
			t.Fatal(err)
		}
		if !a.rowless {
			if _, err := pool.Exec(ctx, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at,
				deleted_at) VALUES ($1, $2, $3, 'text/plain', 1, sha256('x'), $4, now(), $5)`, a.blob, a.node, notebook, user, deletedAt); err != nil {
				t.Fatal(err)
			}
		}
		if a.unfiled {
			continue
		}
		w, err := store.Create(ctx, "blobs/"+a.blob.String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
		if a.old {
			ago := time.Now().Add(-48 * time.Hour)
			if err := os.Chtimes(fileOf(t, dir, a.blob), ago, ago); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// fileOf is the path of the file of blob in the store at dir.
func fileOf(t *testing.T, dir string, blob uuid.UUID) string {
	t.Helper()
	path, ok := storedBlobs(t, dir)[blob.String()]
	if !ok {
		t.Fatalf("the file of %s is not in the store", blob)
	}
	return path
}

// filed are the names of the attachments whose file is in the store at
// dir.
func filed(t *testing.T, dir string, assets []seededAsset) []string {
	t.Helper()
	files := storedBlobs(t, dir)
	var out []string
	for _, a := range assets {
		if _, ok := files[a.blob.String()]; ok {
			out = append(out, a.name)
		}
	}
	return out
}

// awaitJob waits until a job of kind has completed, and fails t if a run
// of it failed before.
func awaitJob(t *testing.T, pool *pgxpool.Pool, kind string) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); count(t, pool, "SELECT count(*) FROM river_job WHERE kind = $1 AND state = 'completed'",
		kind) == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not complete; jobs: %v", kind, queryStrings(t, pool, "SELECT kind || ' ' || state FROM river_job"))
		}
	}
	if errs := queryStrings(t, pool, "SELECT array_to_string(errors, ' ') FROM river_job WHERE kind = $1 AND errors IS NOT NULL", kind); len(errs) != 0 {
		t.Errorf("%s failed before it completed: %v", kind, errs)
	}
}

// The purge takes the attachments deleted longer than the retention ago,
// their files first, then their rows, then their nodes (M2/P4 handoff to
// M7): one whose file an earlier run deleted, its row left, goes too. One
// deleted since stays with its file, as does one not deleted.
func TestThePurgeDeletesTheAttachmentsFilesThenRows(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	assets := []seededAsset{
		{name: "old", node: uuid.NewV7(), blob: uuid.NewV7(), deleted: 61 * 24 * time.Hour},
		{name: "half-purged", node: uuid.NewV7(), blob: uuid.NewV7(), deleted: 61 * 24 * time.Hour, unfiled: true},
		{name: "recent", node: uuid.NewV7(), blob: uuid.NewV7(), deleted: 59 * 24 * time.Hour},
		{name: "live", node: uuid.NewV7(), blob: uuid.NewV7()},
	}
	seedAssets(t, pool, cfg.Storage.Dir, assets)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, jobs.PurgeKind)

	if got := queryStrings(t, pool, "SELECT n.name FROM asset_blobs b JOIN nodes n ON n.id = b.node_id ORDER BY n.name"); !slices.Equal(got, []string{"live", "recent"}) {
		t.Errorf("rows %q, want live's and recent's", got)
	}
	if got := queryStrings(t, pool, "SELECT name FROM nodes ORDER BY name"); !slices.Equal(got, []string{"live", "recent"}) {
		t.Errorf("nodes %q, want live and recent", got)
	}
	if got := filed(t, cfg.Storage.Dir, assets); !slices.Equal(got, []string{"recent", "live"}) {
		t.Errorf("files of %q, want recent's and live's", got)
	}
	checkAssets(t, pool, cfg.Storage.Dir)
}

// The orphan sweep deletes the files older than a day that no row holds;
// it leaves a newer one, an upload's that may still get its row, and the
// files of rows, deleted or not.
func TestTheSweepDeletesTheOldFilesNoRowHolds(t *testing.T) {
	url := pgtest.NewDatabase(t)
	pool := connect(t, url)
	cfg := testConfig(t, url, false)
	assets := []seededAsset{
		{name: "orphan", node: uuid.NewV7(), blob: uuid.NewV7(), rowless: true, old: true},
		{name: "new orphan", node: uuid.NewV7(), blob: uuid.NewV7(), rowless: true},
		{name: "held", node: uuid.NewV7(), blob: uuid.NewV7(), old: true},
		{name: "deleted", node: uuid.NewV7(), blob: uuid.NewV7(), deleted: time.Hour, old: true},
	}
	seedAssets(t, pool, cfg.Storage.Dir, assets)

	startApp(t, cfg, migrations.FS())
	awaitJob(t, pool, "asset.sweep_orphan_files")

	if got := filed(t, cfg.Storage.Dir, assets); !slices.Equal(got, []string{"new orphan", "held", "deleted"}) {
		t.Errorf("files of %q, want all but the old orphan's", got)
	}
}
