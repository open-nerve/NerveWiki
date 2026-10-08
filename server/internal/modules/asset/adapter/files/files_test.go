package files_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// The store's files in the module's terms: a file round trips, a missing
// or deleted one is app.ErrNoFile, and a store keeping more room than the
// disk has is domain.ErrStorageFull.
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
	var listed []string
	if err := f.List(ctx, "blobs", time.Now().Add(time.Minute), func(key string) error {
		listed = append(listed, key)
		return nil
	}); err != nil || len(listed) != 1 || listed[0] != "blobs/a" {
		t.Errorf("List() = %q, %v; want blobs/a", listed, err)
	}
	if err := f.Delete(ctx, "blobs/a"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Open(ctx, "blobs/a"); !errors.Is(err, app.ErrNoFile) {
		t.Errorf("Open(deleted) = %v, want ErrNoFile", err)
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

// spyStore records what it is asked and fails each call with err, but
// Create; its writers take every byte, and record an abort.
type spyStore struct {
	err     error
	free    int64
	calls   []string
	before  time.Time
	aborted bool
}

func (s *spyStore) Create(_ context.Context, key string) (storage.Writer, error) {
	s.calls = append(s.calls, "Create "+key)
	return spyWriter{s}, nil
}

func (s *spyStore) Open(_ context.Context, key string) (storage.File, error) {
	s.calls = append(s.calls, "Open "+key)
	return nil, s.err
}

func (s *spyStore) Delete(_ context.Context, key string) error {
	s.calls = append(s.calls, "Delete "+key)
	return s.err
}

func (s *spyStore) List(_ context.Context, area string, before time.Time, each func(string) error) error {
	s.calls, s.before = append(s.calls, "List "+area), before
	if err := each(area + "/k"); err != nil {
		return err
	}
	return s.err
}

func (s *spyStore) Free(context.Context) (int64, error) { return s.free, s.err }

type spyWriter struct{ s *spyStore }

func (w spyWriter) Write(p []byte) (int, error) { return len(p), w.s.err }
func (w spyWriter) Commit() error               { return w.s.err }
func (w spyWriter) Abort() error {
	w.s.aborted = true
	return w.s.err
}

// Each call reaches the store, its answer back: a write's count, the free
// bytes, the list's area, time and keys, an abort; and the store's
// failures, but not found and full, as themselves.
func TestFilesPassTheStoreThrough(t *testing.T) {
	ctx := context.Background()
	disk := errors.New("input/output error")
	s := &spyStore{err: disk, free: 12345}
	f := files.New(s)
	w, err := f.Create(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := w.Write([]byte("abc")); n != 3 || !errors.Is(err, disk) || errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Write() = %d, %v; want 3, the store's failure", n, err)
	}
	if err := w.Commit(); !errors.Is(err, disk) || errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Commit() = %v, want the store's failure", err)
	}
	if err := w.Abort(); !errors.Is(err, disk) || !s.aborted {
		t.Errorf("Abort() = %v, aborted %v; want the store's failure, aborted", err, s.aborted)
	}
	if file, err := f.Open(ctx, "blobs/a"); file != nil || !errors.Is(err, disk) || errors.Is(err, app.ErrNoFile) {
		t.Errorf("Open() = %v, %v; want the store's failure", file, err)
	}
	if err := f.Delete(ctx, "blobs/a"); !errors.Is(err, disk) {
		t.Errorf("Delete() = %v, want the store's failure", err)
	}
	if free, err := f.Free(ctx); free != 12345 || !errors.Is(err, disk) {
		t.Errorf("Free() = %d, %v; want 12345, the store's failure", free, err)
	}
	before := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	var listed []string
	err = f.List(ctx, "blobs", before, func(key string) error { listed = append(listed, key); return nil })
	if !errors.Is(err, disk) || !s.before.Equal(before) || len(listed) != 1 || listed[0] != "blobs/k" {
		t.Errorf("List() = %v, before %v, listed %q; want the store's failure, %v, blobs/k", err, s.before, listed, before)
	}
	if want := []string{"Create blobs/a", "Open blobs/a", "Delete blobs/a", "List blobs"}; !slices.Equal(s.calls, want) {
		t.Errorf("calls %q, want %q", s.calls, want)
	}
}
