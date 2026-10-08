package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
