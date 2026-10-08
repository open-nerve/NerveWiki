package postgresadapter_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// fixture is a database with alice, acme, its notebooks eng and ops, and
// eng's attachment photo.png and ops' report.pdf.
type fixture struct {
	pool            *pgxpool.Pool
	alice, eng, ops uuid.UUID
	photo, report   uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f := fixture{pool: pool, alice: uuid.NewV7(), eng: uuid.NewV7(), ops: uuid.NewV7(), photo: uuid.NewV7(), report: uuid.NewV7()}
	acme := uuid.NewV7()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", []any{f.alice}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, f.alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now()), ($4, $2, 'Ops', $3, $3, now(), now())", []any{f.eng, acme, f.alice, f.ops}},
		{`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, 'asset', 'photo.png', 'photo.png', 0, $3, $3, now(), now()), ($4, $5, 'asset', 'report.pdf', 'report.pdf', 0, $3, $3, now(), now())`,
			[]any{f.photo, f.eng, f.alice, f.report, f.ops}},
	} {
		if _, err := pool.Exec(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}
	return f
}

func (f fixture) blob(node, notebook uuid.UUID, width, height int) domain.Blob {
	return domain.Blob{ID: uuid.NewV7(), NodeID: node, NotebookID: notebook, MIME: "image/png", Bytes: 3, SHA256: bytes.Repeat([]byte{9}, 32),
		Width: width, Height: height, CreatedBy: f.alice, CreatedAt: time.Date(2026, 10, 8, 10, 0, 0, 123456000, time.UTC)}
}

// A row reads back as written, its size or none, alone or among others; a
// node without a row not deleted has none.
func TestBlobRowsRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := postgresadapter.New(f.pool)
	photo, report := f.blob(f.photo, f.eng, 640, 480), f.blob(f.report, f.ops, 0, 0)
	for _, b := range []domain.Blob{photo, report} {
		if err := store.CreateBlob(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []domain.Blob{photo, report} {
		got, err := store.BlobOfNode(ctx, want.NodeID)
		if err != nil || got.ID != want.ID || got.NotebookID != want.NotebookID || got.MIME != want.MIME || got.Bytes != want.Bytes ||
			!bytes.Equal(got.SHA256, want.SHA256) || got.Width != want.Width || got.Height != want.Height || got.CreatedBy != f.alice ||
			!got.CreatedAt.Equal(want.CreatedAt) {
			t.Errorf("BlobOfNode() = %+v, %v; want %+v", got, err, want)
		}
	}
	both, err := store.BlobsOfNodes(ctx, []uuid.UUID{f.photo, f.report, uuid.NewV7()})
	if err != nil || len(both) != 2 || both[f.photo].ID != photo.ID || both[f.photo].Width != 640 || both[f.report].ID != report.ID {
		t.Errorf("BlobsOfNodes() = %+v, %v; want photo's and report's rows", both, err)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE asset_blobs SET deleted_at = now() WHERE node_id = $1", f.photo); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uuid.UUID{f.photo, uuid.NewV7()} {
		if _, err := store.BlobOfNode(ctx, id); !errors.Is(err, app.ErrNoRow) {
			t.Errorf("BlobOfNode(deleted or none) = %v, want ErrNoRow", err)
		}
	}
	if left, err := store.BlobsOfNodes(ctx, []uuid.UUID{f.photo, f.report}); err != nil || len(left) != 1 || left[f.report].ID != report.ID {
		t.Errorf("BlobsOfNodes() after photo's deletion = %+v, %v; want report's row", left, err)
	}
}

// The table keeps a row to its node's notebook, one row a node, and a
// width with its height.
func TestBlobRowsKeepTheirRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := postgresadapter.New(f.pool)
	if err := store.CreateBlob(ctx, f.blob(f.photo, f.ops, 0, 0)); err == nil {
		t.Error("a row of eng's node in ops was written, want the key to nodes to refuse it")
	}
	if err := store.CreateBlob(ctx, f.blob(f.photo, f.eng, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateBlob(ctx, f.blob(f.photo, f.eng, 0, 0)); err == nil {
		t.Error("a second row of a node was written, want it refused")
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, width, created_by_id, created_at)
		VALUES ($1, $2, $3, 'image/png', 1, $4, 10, $5, now())`, uuid.NewV7(), f.report, f.ops, bytes.Repeat([]byte{1}, 32), f.alice); err == nil {
		t.Error("a width without a height was written, want it refused")
	}
}
