package asset_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	pngenc "image/png"
	"io"
	"log/slog"
	"testing"
	"testing/iotest"
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
	b := asset.NewBlobs(pool, store, slog.New(slog.DiscardHandler))

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

// An import writes each attachment's file, its type and an image's size
// told by the server, then its row in its unit's transaction; a file past
// its largest leaves nothing, a reader's error comes back as it is, and a
// file whose unit was refused is dropped.
func TestAnImportWritesTheAttachmentsFiles(t *testing.T) {
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
	alice, acme, eng, node := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", []any{alice}},
		{"INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())", []any{acme, alice}},
		{"INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, 'Eng', $3, $3, now(), now())", []any{eng, acme, alice}},
		{`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, 'asset', 'dot.png', 'dot.png', 0, $3, $3, now(), now())`, []any{node, eng, alice}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	b := asset.NewBlobs(pool, store, slog.New(slog.DiscardHandler))
	var png bytes.Buffer
	if err := pngenc.Encode(&png, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	f, err := b.Put(ctx, "dot.png", bytes.NewReader(png.Bytes()), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(png.Bytes())
	if f.MIME != "image/png" || f.Bytes != int64(png.Len()) || !bytes.Equal(f.SHA256, sum[:]) || f.Width != 3 || f.Height != 2 {
		t.Errorf("Put() = %+v, want a 3×2 PNG of %d bytes", f, png.Len())
	}
	at := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	err = postgres.NewTxManager(pool, time.Second).WithinTx(ctx, func(ctx context.Context) error {
		return b.Attach(ctx, f, asset.Owner{NodeID: node, NotebookID: eng, CreatedBy: alice, CreatedAt: at})
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Of(ctx, eng, []uuid.UUID{node})
	if err != nil || got[node].ID != f.ID || !got[node].Created.Equal(at) {
		t.Errorf("Of() = %v, %v; want the file attached at %v", got, err, at)
	}
	var width int
	if err := pool.QueryRow(ctx, "SELECT width FROM asset_blobs WHERE id = $1 AND mime = 'image/png' AND byte_size = $2", f.ID, png.Len()).Scan(&width); err != nil || width != 3 {
		t.Errorf("the row's width = %d, %v; want 3", width, err)
	}

	if _, err := b.Put(ctx, "big.bin", bytes.NewReader(make([]byte, 11)), 10); !errors.Is(err, asset.ErrTooLarge) {
		t.Errorf("Put() past the largest = %v, want ErrTooLarge", err)
	}
	failed := errors.New("the zip's entry failed")
	if _, err := b.Put(ctx, "broken.bin", io.MultiReader(bytes.NewReader([]byte("abc")), iotest.ErrReader(failed)), 10); !errors.Is(err, failed) {
		t.Errorf("Put() of a reader failing = %v, want its error", err)
	}
	dropped, err := b.Put(ctx, "refused.bin", bytes.NewReader([]byte("abc")), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Drop(ctx, dropped); err != nil {
		t.Fatal(err)
	}
	var left []string
	if err := store.List(ctx, "blobs", time.Now().Add(time.Hour), func(key string) error {
		left = append(left, key)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != "blobs/"+f.ID.String() {
		t.Errorf("the store holds %v, want the attached file alone", left)
	}
}
