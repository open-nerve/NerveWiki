package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serve refuses to start on a storage directory it cannot write, and says
// which directory and which uid (M7/P1 design 3.5; M0/P6 handoff, item 2).
// It makes a missing one.
func TestServeNeedsAStorageDirectoryItCanWrite(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes in any directory")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	cfg := testConfig(t, unreachableDB, false)
	cfg.Storage.Dir = dir

	_, err := newApp(context.Background(), cfg, slog.New(slog.DiscardHandler), sampleMigrations(), testWebUI())

	if err == nil {
		t.Fatal("newApp() on a directory it cannot write succeeded")
	}
	for _, want := range []string{dir, fmt.Sprintf("uid %d", os.Getuid())} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("newApp() = %v, want it to name %q", err, want)
		}
	}

	cfg = testConfig(t, unreachableDB, false)
	cfg.Storage.Dir = filepath.Join(t.TempDir(), "missing", "data")
	a, err := newApp(context.Background(), cfg, slog.New(slog.DiscardHandler), sampleMigrations(), testWebUI())
	if err != nil {
		t.Fatalf("newApp() on a missing directory: %v", err)
	}
	a.close()
	if info, err := os.Stat(cfg.Storage.Dir); err != nil || !info.IsDir() {
		t.Errorf("the storage directory: %v, %v; want it made", info, err)
	}
}
