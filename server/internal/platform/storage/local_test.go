package storage_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage/storagetest"
)

func openLocal(t *testing.T) *storage.Local {
	t.Helper()
	l, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLocalPassesTheContract(t *testing.T) {
	storagetest.Run(t, func(t *testing.T) storage.Store { return openLocal(t) })
}

// shard is where Local keeps the file name of area under dir.
func shard(dir, area, name string) string {
	sum := sha256.Sum256([]byte(name))
	h := hex.EncodeToString(sum[:2])
	return filepath.Join(dir, area, h[:2], h[2:], name)
}

func TestAFileLiesInItsShard(t *testing.T) {
	dir := t.TempDir()
	l, err := storage.OpenLocal(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, key := range []string{"blobs/0199c1a2-7b3c-7d4e-8f50-1a2b3c4d5e6f", "exports/a.zip"} {
		w, err := l.Create(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(key)); err != nil {
			t.Fatal(err)
		}
		if err := w.Commit(); err != nil {
			t.Fatal(err)
		}
		area, name, _ := strings.Cut(key, "/")
		got, err := os.ReadFile(shard(dir, area, name))
		if err != nil || string(got) != key {
			t.Errorf("%s in its shard: %q, %v", key, got, err)
		}
	}
}

// A file being written lies in its area's .tmp, which Abort leaves empty.
func TestAFileBeingWrittenLiesInItsAreasTemporaryDirectory(t *testing.T) {
	dir := t.TempDir()
	l, err := storage.OpenLocal(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	w, err := l.Create(context.Background(), "imports/a.zip")
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "imports", ".tmp")
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 1 {
		t.Fatalf("imports/.tmp holds %v, %v; want the one file", entries, err)
	}
	if err := w.Abort(); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Errorf("imports/.tmp after Abort holds %v, %v; want nothing", entries, err)
	}
}

func TestOpeningDropsWhatAStoppedProcessLeftHalfWritten(t *testing.T) {
	dir := t.TempDir()
	l, err := storage.OpenLocal(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, key := range []string{"blobs/a", "imports/b.zip"} {
		w, err := l.Create(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("half")); err != nil {
			t.Fatal(err)
		}
		// Neither committed nor aborted: the process stopped here.
	}
	committed, err := l.Create(ctx, "blobs/kept")
	if err != nil {
		t.Fatal(err)
	}
	if err := committed.Commit(); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(stray, []byte("not the store's"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, ".probe-123")
	if err := os.WriteFile(probe, []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := storage.OpenLocal(dir, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(probe); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a probe left by a stopped process: %v, want it gone", err)
	}
	for _, area := range []string{"blobs", "imports"} {
		if _, err := os.Stat(filepath.Join(dir, area, ".tmp")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s/.tmp after opening: %v, want it gone", area, err)
		}
	}
	if _, err := os.Stat(shard(dir, "blobs", "kept")); err != nil {
		t.Errorf("the committed file: %v", err)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("a file that is not the store's: %v", err)
	}
}

func TestOpeningMakesAMissingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	if _, err := storage.OpenLocal(dir, 0); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("the directory: %v, %v", info, err)
	}
}

// A store whose directory, or an area in it, the process cannot write, as
// one restored as another user, does not open, and says why.
func TestOpeningADirectoryItCannotWriteNamesItAndTheUID(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes in any directory")
	}
	for name, area := range map[string]string{"the directory": "", "an area": "blobs"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := storage.OpenLocal(dir, 0); err != nil {
				t.Fatal(err)
			}
			locked := filepath.Join(dir, area)
			if err := os.MkdirAll(locked, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(locked, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
			_, err := storage.OpenLocal(dir, 0)
			if err == nil {
				t.Fatal("opened a directory it cannot write")
			}
			for _, want := range []string{locked, fmt.Sprintf("uid %d", os.Getuid()), fmt.Sprintf("gid %d", os.Getgid())} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestOpeningAFileFails(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.OpenLocal(file, 0); err == nil {
		t.Error("opened a file as the store's directory")
	}
}

func TestOpeningNeedsADirectoryAndANonNegativeMinimum(t *testing.T) {
	if _, err := storage.OpenLocal("", 0); err == nil {
		t.Error("opened no directory")
	}
	if _, err := storage.OpenLocal(t.TempDir(), -1); err == nil {
		t.Error("opened with a negative minimum")
	}
}

func TestAWriteThatWouldLeaveTooLittleFreeIsFull(t *testing.T) {
	ctx := context.Background()
	free, err := openLocal(t).Free(ctx)
	if err != nil || free <= 0 {
		t.Fatalf("Free() = %d, %v", free, err)
	}
	l, err := storage.OpenLocal(t.TempDir(), math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if w, err := l.Create(ctx, "blobs/a"); !errors.Is(err, storage.ErrFull) {
		if err == nil {
			_ = w.Abort()
		}
		t.Errorf("Create = %v, want ErrFull", err)
	}
	enough, err := storage.OpenLocal(t.TempDir(), free/2)
	if err != nil {
		t.Fatal(err)
	}
	w, err := enough.Create(ctx, "blobs/a")
	if err != nil {
		t.Fatalf("Create with half the free space kept: %v", err)
	}
	_ = w.Abort()
}
