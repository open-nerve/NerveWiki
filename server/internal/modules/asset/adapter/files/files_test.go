package files_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// The store's files in the module's terms: a file round trips, a missing
// one is app.ErrNoFile, and a store keeping more room than the disk has is
// domain.ErrStorageFull.
func TestFilesSpeakTheModulesTerms(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	f := files.New(store)
	w, err := f.Create(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
	r, err := f.Open(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if b, _ := io.ReadAll(r); string(b) != "abc" || r.Size() != 3 {
		t.Errorf("read %q of %d bytes, want abc", b, r.Size())
	}
	if _, err := f.Open(ctx, "blobs/b"); !errors.Is(err, app.ErrNoFile) {
		t.Errorf("Open(missing) = %v, want ErrNoFile", err)
	}
	if err := f.Delete(ctx, "blobs/a"); err != nil {
		t.Fatal(err)
	}
	if free, err := f.Free(ctx); err != nil || free <= 0 {
		t.Errorf("Free() = %d, %v", free, err)
	}

	full, err := storage.OpenLocal(t.TempDir(), 1<<62)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := files.New(full).Create(ctx, "blobs/c"); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Create() on a full store = %v, want storage_full", err)
	}
}

// fullStore's writers run out of room: storage.ErrFull wrapped, as Local
// answers ENOSPC.
type fullStore struct{ storage.Store }

func (fullStore) Create(context.Context, string) (storage.Writer, error) { return fullWriter{}, nil }

type fullWriter struct{}

func (fullWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("%w: no space left on device", storage.ErrFull)
}
func (fullWriter) Commit() error { return fmt.Errorf("%w: no space left on device", storage.ErrFull) }
func (fullWriter) Abort() error  { return nil }

// A write or a commit that runs out of room is domain.ErrStorageFull too.
func TestAWriteThatRunsOutOfRoomIsStorageFull(t *testing.T) {
	w, err := files.New(fullStore{}).Create(context.Background(), "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abc")); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Write() = %v, want storage_full", err)
	}
	if err := w.Commit(); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Commit() = %v, want storage_full", err)
	}
}
