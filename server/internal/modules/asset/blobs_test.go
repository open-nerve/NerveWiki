package asset_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// An export reads the blob of each attachment of its notebook not deleted,
// and when it was written, then its file; a file not in the store is
// ErrNoFile.
func TestAnExportReadsTheAttachmentsFiles(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	alice, acme, eng, ops := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	kept, gone, elsewhere, missing := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	blobs := map[uuid.UUID]uuid.UUID{kept: uuid.NewV7(), gone: uuid.NewV7(), elsewhere: uuid.NewV7(), missing: uuid.NewV7()}
	written := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec("INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", alice)
	exec("INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", acme, alice)
	exec("INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $3, 'Eng', $2, $2, now(), now()), ($4, $3, 'Ops', $2, $2, now(), now())",
		eng, alice, acme, ops)
	for node, notebook := range map[uuid.UUID]uuid.UUID{kept: eng, gone: eng, elsewhere: ops, missing: eng} {
		deleted := node == gone
		exec(`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, 'asset', $3, $3, 0, $4, $4, now(), now(), CASE WHEN $5 THEN now() END)`, node, notebook, node.String()+".png", alice, deleted)
		exec(`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at, deleted_at)
			VALUES ($1, $2, $3, 'image/png', 3, sha256('abc'), $4, $6, CASE WHEN $5 THEN now() END)`, blobs[node], node, notebook, alice, deleted, written)
		if node == missing {
			continue
		}
		w, err := store.Create(ctx, "blobs/"+blobs[node].String())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("abc")); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	b := asset.NewBlobs(pool, store)

	got, err := b.Of(ctx, eng, []uuid.UUID{kept, gone, elsewhere, missing, uuid.NewV7()})
	if err != nil || len(got) != 2 || got[kept].ID != blobs[kept] || got[missing].ID != blobs[missing] ||
		!got[kept].Created.Equal(written) || !got[missing].Created.Equal(written) {
		t.Fatalf("Of() = %v, %v; want kept's and missing's, written at %v", got, err, written)
	}
	f, err := b.Open(ctx, got[kept].ID)
	if err != nil {
		t.Fatal(err)
	}
	read, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil || string(read) != "abc" {
		t.Errorf("the file read = %q, %v", read, err)
	}
	if _, err := b.Open(ctx, got[missing].ID); !errors.Is(err, asset.ErrNoFile) {
		t.Errorf("Open() of a file not in the store = %v, want ErrNoFile", err)
	}
}
