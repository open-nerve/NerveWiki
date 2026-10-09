package archiveadapter_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"uuid"

	archiveadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/archive"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

func newArchives(t *testing.T) (archiveadapter.Archives, *storage.Local) {
	t.Helper()
	store, err := storage.OpenLocal(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return archiveadapter.New(store, slog.New(slog.DiscardHandler)), store
}

// An archive is a zip file at exports/<id>.zip once committed: its entries
// as added, a name that is not ASCII marked UTF-8, a file stored as it is
// or deflated, a folder's entry; its bytes counted. Until it commits, and
// after an abort, nothing is there.
func TestAnArchiveIsAZipFile(t *testing.T) {
	archives, store := newArchives(t)
	ctx := context.Background()
	id := uuid.NewV7()
	a, err := archives.Create(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	text := strings.Repeat("纪要 ", 1000)
	for _, f := range []struct {
		path   string
		stored bool
		r      io.Reader
	}{
		{"笔记/会议纪要.md", false, strings.NewReader(text)},
		{"笔记/图.png", true, strings.NewReader(text)},
		{"笔记/只有子页/", true, nil},
	} {
		if err := a.Add(f.path, modified, f.stored, f.r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := archives.Open(ctx, id); !errors.Is(err, app.ErrFileMissing) {
		t.Errorf("Open() before the commit = %v, want ErrFileMissing", err)
	}
	n, err := a.Commit()
	if err != nil {
		t.Fatal(err)
	}
	f, err := archives.Open(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if f.Size() != n {
		t.Errorf("Commit() = %d bytes, the file has %d", n, f.Size())
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name   string
		method uint16
	}{{"笔记/会议纪要.md", zip.Deflate}, {"笔记/图.png", zip.Store}, {"笔记/只有子页/", zip.Store}}
	if len(z.File) != len(want) {
		t.Fatalf("%d entries, want %d", len(z.File), len(want))
	}
	for i, w := range want {
		e := z.File[i]
		if e.Name != w.name || e.Method != w.method || e.Flags&0x800 == 0 || !e.Modified.Equal(modified) {
			t.Errorf("entry %d = %q method %d flags %#x modified %v; want %q, %d, UTF-8", i, e.Name, e.Method, e.Flags, e.Modified, w.name, w.method)
		}
	}
	r, err := z.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if string(got) != text || z.File[0].CompressedSize64 >= uint64(len(text)) {
		t.Errorf("the page reads %d bytes, %d compressed; want the text, deflated", len(got), z.File[0].CompressedSize64)
	}

	aborted, err := archives.Create(ctx, uuid.NewV7())
	if err != nil {
		t.Fatal(err)
	}
	if err := aborted.Add("x.md", modified, false, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if err := aborted.Abort(); err != nil {
		t.Fatal(err)
	}
	var listed []uuid.UUID
	if err := archives.List(ctx, domain.KindExport, time.Now().Add(time.Hour), func(id uuid.UUID) error {
		listed = append(listed, id)
		return nil
	}); err != nil || len(listed) != 1 || listed[0] != id {
		t.Errorf("List() = %v, %v; want the committed archive alone", listed, err)
	}
	if err := archives.Delete(ctx, domain.KindExport, id); err != nil {
		t.Fatal(err)
	}
	if _, err := archives.Open(ctx, id); !errors.Is(err, app.ErrFileMissing) {
		t.Errorf("Open() after the delete = %v, want ErrFileMissing", err)
	}
	if err := archives.Delete(ctx, domain.KindExport, id); err != nil {
		t.Errorf("Delete() of none = %v, want nil", err)
	}
	_ = store
}

// A file of the exports' area that is no archive's is left out of the
// list.
func TestTheListLeavesOtherFiles(t *testing.T) {
	archives, store := newArchives(t)
	ctx := context.Background()
	w, err := store.Create(ctx, "exports/notes.zip")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
	err = archives.List(ctx, domain.KindExport, time.Now().Add(time.Hour), func(id uuid.UUID) error {
		t.Errorf("listed %s", id)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}
