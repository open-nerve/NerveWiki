package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// fullDisk is a temporary file on a disk that has run out of space: its
// writes, or only its sync, fail with ENOSPC, or with errno when set.
type fullDisk struct {
	name     string
	syncOnly bool
	errno    syscall.Errno
}

func (f fullDisk) Write(p []byte) (int, error) {
	if f.syncOnly {
		return len(p), nil
	}
	if f.errno != 0 {
		return 0, &fs.PathError{Op: "write", Path: f.name, Err: f.errno}
	}
	return 0, &fs.PathError{Op: "write", Path: f.name, Err: syscall.ENOSPC}
}

func (f fullDisk) Sync() error {
	if f.errno != 0 {
		return &fs.PathError{Op: "sync", Path: f.name, Err: f.errno}
	}
	return &fs.PathError{Op: "sync", Path: f.name, Err: syscall.ENOSPC}
}

func (fullDisk) Close() error   { return nil }
func (f fullDisk) Name() string { return f.name }

func TestRunningOutOfSpaceIsFullAndLeavesNothing(t *testing.T) {
	l, err := OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(t.TempDir(), "partial")
	if err := os.WriteFile(tmp, []byte("part"), 0o600); err != nil {
		t.Fatal(err)
	}

	w := &localWriter{l: l, area: "blobs", name: "a", file: fullDisk{name: tmp}}
	if _, err := w.Write([]byte("more")); !errors.Is(err, ErrFull) || !errors.Is(err, syscall.ENOSPC) {
		t.Errorf("Write = %v, want ErrFull keeping ENOSPC", err)
	}
	w = &localWriter{l: l, area: "blobs", name: "a", file: fullDisk{name: tmp, errno: syscall.EDQUOT}}
	if _, err := w.Write([]byte("more")); !errors.Is(err, ErrFull) || !errors.Is(err, syscall.EDQUOT) {
		t.Errorf("Write over the quota = %v, want ErrFull keeping EDQUOT", err)
	}

	w = &localWriter{l: l, area: "blobs", name: "a", file: fullDisk{name: tmp, syncOnly: true}}
	if err := w.Commit(); !errors.Is(err, ErrFull) {
		t.Errorf("Commit = %v, want ErrFull", err)
	}
	if _, err := os.Stat(tmp); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the temporary file after the failed commit: %v, want it gone", err)
	}
	if _, err := os.Stat(l.path("blobs", "a")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the file at the key: %v, want none", err)
	}
}

// onFullDisk makes real files whose writes fail with ENOSPC.
func onFullDisk(dir, pattern string) (tempFile, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return fullDisk{name: f.Name()}, nil
}

// A disk out of space is no reason to refuse to start: the store opens,
// deleting what a stopped process left half written, the likely cause, and
// its writes answer ErrFull.
func TestAFullDiskOpensAndDropsWhatWasLeft(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "imports", tmpDir, "a.zip.123")
	if err := os.MkdirAll(filepath.Dir(left), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("half an import"), 0o600); err != nil {
		t.Fatal(err)
	}

	l, err := openLocal(dir, 0, onFullDisk)
	if err != nil {
		t.Fatalf("openLocal on a full disk: %v", err)
	}
	if full := l.FullAtOpen(); !errors.Is(full, syscall.ENOSPC) {
		t.Errorf("FullAtOpen() = %v, want the probe's ENOSPC", full)
	}
	if _, err := os.Stat(filepath.Dir(left)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("imports/.tmp after opening: %v, want it gone", err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Errorf("the directory holds %v, %v; want the area alone, no probe left", entries, err)
	}
	w, err := l.Create(t.Context(), "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); !errors.Is(err, ErrFull) || strings.Count(err.Error(), "storage:") != 1 {
		t.Errorf("Write = %v, want ErrFull, said once to be the store's", err)
	}
	if err := w.Abort(); err != nil {
		t.Error(err)
	}
}

// The full disk is the one opening finds after all its deletions: a
// half-written file that filled it, once deleted, leaves no warning, nor
// does an area probed before it, as blobs comes before imports.
func TestFullAtOpenIsAfterTheDeletions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o750); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(dir, "imports", tmpDir, "a.zip.123")
	if err := os.MkdirAll(filepath.Dir(left), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(left, []byte("the import that filled the disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	fullWhileLeft := func(dir, pattern string) (tempFile, error) {
		if _, err := os.Stat(left); err == nil {
			return onFullDisk(dir, pattern)
		}
		return osCreateTemp(dir, pattern)
	}
	l, err := openLocal(dir, 0, fullWhileLeft)
	if err != nil {
		t.Fatal(err)
	}
	if full := l.FullAtOpen(); full != nil {
		t.Errorf("FullAtOpen() = %v, though the deletions made room", full)
	}
}

// An area full on its own, a mount or a quota of its own, is found full.
func TestFullAtOpenFindsAnAreaFullOnItsOwn(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o750); err != nil {
		t.Fatal(err)
	}
	areaFull := func(dir, pattern string) (tempFile, error) {
		if filepath.Base(dir) != tmpDir {
			return osCreateTemp(dir, pattern)
		}
		f, err := onFullDisk(dir, pattern)
		if err != nil {
			return nil, err
		}
		return fullDisk{name: f.Name(), errno: syscall.EDQUOT}, nil
	}
	l, err := openLocal(dir, 0, areaFull)
	if err != nil {
		t.Fatalf("openLocal with an area over its quota: %v", err)
	}
	if full := l.FullAtOpen(); !errors.Is(full, syscall.EDQUOT) {
		t.Errorf("FullAtOpen() = %v, want the area's EDQUOT", full)
	}
}

// A file's writes read the free space every freeCheck bytes, and stop at
// the store's minimum as Create does: a file far larger than an
// attachment does not fill the disk. A read that fails skips its check.
func TestWritesStopAtTheMinimumOfFreeSpace(t *testing.T) {
	l, err := OpenLocal(t.TempDir(), 100)
	if err != nil {
		t.Fatal(err)
	}
	free, reads := int64(1000), 0
	l.free = func(string) (int64, error) { reads++; return free, nil }
	l.freeCheck = 4

	w, err := l.Create(t.Context(), "exports/a")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"ab", "cd", "ef", "gh"} {
		if _, err := w.Write([]byte(p)); err != nil {
			t.Fatalf("Write(%q) = %v", p, err)
		}
	}
	if reads != 2 {
		t.Errorf("the free space read %d times, want 2: at Create and once 4 bytes went", reads)
	}
	l.free = func(string) (int64, error) { reads++; return 0, errors.New("the file system did not answer") }
	if _, err := w.Write([]byte("ij")); err != nil || reads != 3 {
		t.Errorf("Write as the free space is not read = %v after %d reads, want it written after 3", err, reads)
	}
	l.free = func(string) (int64, error) { reads++; return free, nil }
	free = 99
	if _, err := w.Write([]byte("kl")); err != nil {
		t.Errorf("Write(%q) = %v, want it written unchecked", "kl", err)
	}
	if _, err := w.Write([]byte("mn")); !errors.Is(err, ErrFull) {
		t.Errorf("Write past the minimum = %v, want ErrFull", err)
	}
	if err := w.Abort(); err != nil {
		t.Error(err)
	}
	if _, err := l.Create(t.Context(), "exports/b"); !errors.Is(err, ErrFull) {
		t.Errorf("Create past the minimum = %v, want ErrFull", err)
	}
}
