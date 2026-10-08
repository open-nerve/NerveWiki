// Package storagetest is the contract every storage.Store passes (M7/P1
// design 5): a file is visible whole once committed and never before, it
// reads back as written, and keys never reach outside the store. Only test
// code may import it (enforced by internal/archtest).
package storagetest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// Run runs the contract on the stores open makes, a new empty one for each
// subtest.
func Run(t *testing.T, open func(t *testing.T) storage.Store) {
	t.Helper()
	tests := []struct {
		name string
		test func(t *testing.T, s storage.Store)
	}{
		{"a file is visible once committed and never before", committedIsVisible},
		{"an aborted file leaves nothing", abortedLeavesNothing},
		{"only one of commit and abort takes effect", oneEnding},
		{"a file reads at any offset", readsAtOffsets},
		{"committing at a key replaces its file", commitReplaces},
		{"a deleted file is gone and deleting none succeeds", deletes},
		{"list names an area's files modified before a time", lists},
		{"list stops at the first error of each", listStops},
		{"keys outside the rules write nothing", badKeys},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.test(t, open(t)) })
	}
}

// put commits content at key.
func put(t *testing.T, s storage.Store, key string, content []byte) {
	t.Helper()
	w, err := s.Create(context.Background(), key)
	if err != nil {
		t.Fatalf("Create(%q): %v", key, err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

// read reads the whole file at key.
func read(t *testing.T, s storage.Store, key string) []byte {
	t.Helper()
	f, err := s.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("Open(%q): %v", key, err)
	}
	defer closeFile(t, f)
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	if f.Size() != int64(len(got)) {
		t.Errorf("Size() = %d, read %d bytes", f.Size(), len(got))
	}
	return got
}

// listed is every key of area that List names for before.
func listed(t *testing.T, s storage.Store, area string, before time.Time) []string {
	t.Helper()
	var keys []string
	if err := s.List(context.Background(), area, before, func(key string) error {
		keys = append(keys, key)
		return nil
	}); err != nil {
		t.Fatalf("List(%q): %v", area, err)
	}
	slices.Sort(keys)
	return keys
}

// closeFile closes f, failing the test when it cannot.
func closeFile(t *testing.T, f io.Closer) {
	t.Helper()
	if err := f.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func absent(t *testing.T, s storage.Store, key string) {
	t.Helper()
	if f, err := s.Open(context.Background(), key); !errors.Is(err, storage.ErrNotFound) {
		if err == nil {
			closeFile(t, f)
		}
		t.Errorf("Open(%q) = %v, want ErrNotFound", key, err)
	}
}

// later is a time after every file a test writes.
func later() time.Time { return time.Now().Add(24 * time.Hour) }

func committedIsVisible(t *testing.T, s storage.Store) {
	ctx := context.Background()
	w, err := s.Create(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"first ", "second ", "third"} {
		if _, err := w.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	absent(t, s, "blobs/a")
	if keys := listed(t, s, "blobs", later()); len(keys) != 0 {
		t.Errorf("before the commit List = %q, want none", keys)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, "blobs/a"); string(got) != "first second third" {
		t.Errorf("read %q", got)
	}
	f, err := s.Open(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFile(t, f)
	if f.ModTime().IsZero() || f.ModTime().After(time.Now().Add(time.Minute)) {
		t.Errorf("ModTime() = %v", f.ModTime())
	}
	if keys := listed(t, s, "blobs", later()); !slices.Equal(keys, []string{"blobs/a"}) {
		t.Errorf("List = %q, want [blobs/a]", keys)
	}
}

func abortedLeavesNothing(t *testing.T, s storage.Store) {
	w, err := s.Create(context.Background(), "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("lost")); err != nil {
		t.Fatal(err)
	}
	if err := w.Abort(); err != nil {
		t.Fatal(err)
	}
	absent(t, s, "blobs/a")
	if keys := listed(t, s, "blobs", later()); len(keys) != 0 {
		t.Errorf("List = %q, want none", keys)
	}
}

func oneEnding(t *testing.T, s storage.Store) {
	ctx := context.Background()
	committed, err := s.Create(ctx, "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	if err := committed.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := committed.Commit(); err == nil {
		t.Error("a second Commit succeeded")
	}
	if err := committed.Abort(); err == nil {
		t.Error("Abort after Commit succeeded")
	}
	if _, err := committed.Write([]byte("x")); err == nil {
		t.Error("Write after Commit succeeded")
	}
	if got := read(t, s, "blobs/a"); len(got) != 0 {
		t.Errorf("the committed empty file reads %q", got)
	}
	aborted, err := s.Create(ctx, "blobs/b")
	if err != nil {
		t.Fatal(err)
	}
	if err := aborted.Abort(); err != nil {
		t.Fatal(err)
	}
	if err := aborted.Commit(); err == nil {
		t.Error("Commit after Abort succeeded")
	}
	if err := aborted.Abort(); err == nil {
		t.Error("a second Abort succeeded")
	}
	absent(t, s, "blobs/b")
}

func readsAtOffsets(t *testing.T, s storage.Store) {
	content := bytes.Repeat([]byte("0123456789"), 1000)
	put(t, s, "blobs/a", content)
	f, err := s.Open(context.Background(), "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFile(t, f)
	part := make([]byte, 5)
	if _, err := f.ReadAt(part, 4321); err != nil || string(part) != "12345" {
		t.Errorf("ReadAt(4321) = %q, %v", part, err)
	}
	if pos, err := f.Seek(9995, io.SeekStart); err != nil || pos != 9995 {
		t.Fatalf("Seek = %d, %v", pos, err)
	}
	rest, err := io.ReadAll(f)
	if err != nil || string(rest) != "56789" {
		t.Errorf("after Seek read %q, %v", rest, err)
	}
	if f.Size() != int64(len(content)) {
		t.Errorf("Size() = %d, want %d", f.Size(), len(content))
	}
}

func commitReplaces(t *testing.T, s storage.Store) {
	put(t, s, "exports/a.zip", []byte("old"))
	put(t, s, "exports/a.zip", []byte("new"))
	if got := read(t, s, "exports/a.zip"); string(got) != "new" {
		t.Errorf("read %q, want new", got)
	}
	if keys := listed(t, s, "exports", later()); !slices.Equal(keys, []string{"exports/a.zip"}) {
		t.Errorf("List = %q", keys)
	}
}

func deletes(t *testing.T, s storage.Store) {
	ctx := context.Background()
	put(t, s, "blobs/a", []byte("a"))
	put(t, s, "blobs/b", []byte("b"))
	if err := s.Delete(ctx, "blobs/a"); err != nil {
		t.Fatal(err)
	}
	absent(t, s, "blobs/a")
	if got := read(t, s, "blobs/b"); string(got) != "b" {
		t.Errorf("the other file reads %q", got)
	}
	if err := s.Delete(ctx, "blobs/a"); err != nil {
		t.Errorf("deleting a deleted file: %v", err)
	}
	if err := s.Delete(ctx, "blobs/never"); err != nil {
		t.Errorf("deleting a file never written: %v", err)
	}
}

func lists(t *testing.T, s storage.Store) {
	put(t, s, "blobs/a", []byte("a"))
	put(t, s, "blobs/b", []byte("b"))
	put(t, s, "imports/c.zip", []byte("c"))
	if keys := listed(t, s, "blobs", later()); !slices.Equal(keys, []string{"blobs/a", "blobs/b"}) {
		t.Errorf("List(blobs) = %q", keys)
	}
	if keys := listed(t, s, "imports", later()); !slices.Equal(keys, []string{"imports/c.zip"}) {
		t.Errorf("List(imports) = %q", keys)
	}
	if keys := listed(t, s, "exports", later()); len(keys) != 0 {
		t.Errorf("List of an area never written = %q", keys)
	}
	f, err := s.Open(context.Background(), "blobs/a")
	if err != nil {
		t.Fatal(err)
	}
	modified := f.ModTime()
	closeFile(t, f)
	for _, key := range listed(t, s, "blobs", modified) {
		if key == "blobs/a" {
			t.Errorf("List before its modification time names %s", key)
		}
	}
	if keys := listed(t, s, "blobs", modified.Add(time.Nanosecond)); !slices.Contains(keys, "blobs/a") {
		t.Errorf("List just after its modification time = %q, want blobs/a in it", keys)
	}
	if keys := listed(t, s, "blobs", time.Unix(0, 0)); len(keys) != 0 {
		t.Errorf("List before 1970 = %q", keys)
	}
}

func listStops(t *testing.T, s storage.Store) {
	put(t, s, "blobs/a", []byte("a"))
	put(t, s, "blobs/b", []byte("b"))
	stop := errors.New("stop")
	calls := 0
	err := s.List(context.Background(), "blobs", later(), func(string) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Errorf("List = %v after %d calls, want the error of each after one", err, calls)
	}
}

func badKeys(t *testing.T, s storage.Store) {
	ctx := context.Background()
	for _, key := range []string{
		"", "blobs", "blobs/", "/blobs/a", "blobs/../a", "../blobs/a", "blobs/a/b", "Blobs/a", "blobs/A",
		"blobs/.a", "blobs/a b", "blobs/a\x00", "blobs/a\\b", "b1obs/a", "blobs/" + string(bytes.Repeat([]byte("a"), 129)),
	} {
		if w, err := s.Create(ctx, key); err == nil {
			_ = w.Abort()
			t.Errorf("Create(%q) succeeded", key)
		}
		if f, err := s.Open(ctx, key); err == nil || errors.Is(err, storage.ErrNotFound) {
			if err == nil {
				closeFile(t, f)
			}
			t.Errorf("Open(%q) = %v, want a key error", key, err)
		}
		if err := s.Delete(ctx, key); err == nil {
			t.Errorf("Delete(%q) succeeded", key)
		}
	}
	for _, area := range []string{"", "Blobs", "blobs/a", "..", "b1obs"} {
		if err := s.List(ctx, area, later(), func(string) error { return nil }); err == nil {
			t.Errorf("List(%q) succeeded", area)
		}
	}
	put(t, s, "blobs/"+string(bytes.Repeat([]byte("a"), 128)), nil)
}
