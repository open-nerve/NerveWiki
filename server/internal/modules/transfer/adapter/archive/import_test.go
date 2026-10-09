package archiveadapter_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"testing"
	"uuid"

	archiveadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/archive"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// zipped is a zip archive of files, each a name and content, written by
// archive/zip in order: a name ending in "/" a folder; a header given
// instead is written as it is.
func zipped(t *testing.T, comment string, files ...any) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if comment != "" {
		if err := w.SetComment(comment); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		var h *zip.FileHeader
		var content string
		switch f := f.(type) {
		case [2]string:
			h, content = &zip.FileHeader{Name: f[0], Method: zip.Deflate}, f[1]
		case *zip.FileHeader:
			h = f
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stored writes data as the import id's archive.
func stored(t *testing.T, store *storage.Local, id uuid.UUID, data []byte) {
	t.Helper()
	w, err := store.Create(context.Background(), domain.Archive(domain.KindImport, id))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
}

// opened opens data as an import's archive of at most most entries.
func opened(t *testing.T, a archiveadapter.Archives, store *storage.Local, data []byte, most int) (app.ImportArchive, error) {
	t.Helper()
	id := uuid.NewV7()
	stored(t, store, id, data)
	ar, err := a.OpenImport(context.Background(), id, most)
	if err == nil {
		t.Cleanup(func() { _ = ar.Close() })
	}
	return ar, err
}

// An import's request writes its archive, visible once committed; a
// write the store has no room for is domain.ErrStorageFull.
func TestAnImportsArchiveIsUploaded(t *testing.T) {
	a, _ := newArchives(t)
	id := uuid.NewV7()
	u, err := a.Upload(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	data := zipped(t, "", [2]string{"a.md", "# A"})
	if _, err := u.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := u.Commit(); err != nil {
		t.Fatal(err)
	}
	ar, err := a.OpenImport(context.Background(), id, 10)
	if err != nil {
		t.Fatal(err)
	}
	_ = ar.Close()
	dropped := uuid.NewV7()
	u, err = a.Upload(context.Background(), dropped)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := u.Abort(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenImport(context.Background(), dropped, 10); !errors.Is(err, app.ErrFileMissing) {
		t.Errorf("an aborted upload's archive = %v, want ErrFileMissing", err)
	}
	full, err := storage.OpenLocal(t.TempDir(), 1<<62)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := archiveadapter.New(full, nil).Upload(context.Background(), uuid.NewV7()); !errors.Is(err, domain.ErrStorageFull) {
		t.Errorf("Upload() on a store without room = %v, want ErrStorageFull", err)
	}
}

// The entries as the directory lists them: names as they are, folders by
// a name ending in "/" or "\" or by their mode, a symbolic link special,
// the first flag encrypted, the method (archive/zip stores a folder);
// each opens as it unpacks, and Packed is its packed bytes.
func TestAnImportsArchiveListsItsEntries(t *testing.T) {
	a, store := newArchives(t)
	link := &zip.FileHeader{Name: "link.md", Method: zip.Store}
	link.SetMode(fs.ModeSymlink | 0o777)
	locked := &zip.FileHeader{Name: "locked.md", Method: zip.Store, Flags: 0x1}
	folder := &zip.FileHeader{Name: "Folder", Method: zip.Store}
	folder.SetMode(fs.ModeDir | 0o755)
	data := zipped(t, "", [2]string{"a.md", "# A, " + string(bytes.Repeat([]byte("a"), 1000))}, [2]string{"B/", ""}, [2]string{`C\`, ""},
		link, locked, folder, &zip.FileHeader{Name: "stored.png", Method: zip.Store})
	ar, err := opened(t, a, store, data, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.RawEntry{
		{Index: 0, Name: "a.md", Method: domain.MethodDeflate},
		{Index: 1, Name: "B/", Folder: true},
		{Index: 2, Name: `C\`, Folder: true, Method: domain.MethodDeflate},
		{Index: 3, Name: "link.md", Special: true},
		{Index: 4, Name: "locked.md", Encrypted: true},
		{Index: 5, Name: "Folder", Folder: true},
		{Index: 6, Name: "stored.png"},
	}
	if got := ar.Entries(); !slices.Equal(got, want) {
		t.Errorf("Entries() =\n%+v\nwant\n%+v", got, want)
	}
	r, err := ar.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil || len(content) != 1005 || ar.Packed(0) >= 1005 || ar.Packed(0) <= 0 {
		t.Errorf("entry 0 = %d bytes, %v, packed %d; want 1005, packed fewer", len(content), err, ar.Packed(0))
	}
}

// An archive is read wherever its end says its directory is: with a
// comment, zip64, after bytes prepended (an executable's).
func TestAnImportsArchiveIsFoundAsArchiveZipFindsIt(t *testing.T) {
	a, store := newArchives(t)
	many := make([]any, 0, 70000)
	for i := range 70000 {
		many = append(many, &zip.FileHeader{Name: fmt.Sprintf("f%05d.md", i), Method: zip.Store})
	}
	for _, tt := range []struct {
		name  string
		data  []byte
		count int
	}{
		{"a comment", zipped(t, "a comment of the archive", [2]string{"a.md", "x"}), 1},
		{"zip64, more records than 65535", zipped(t, "", many...), 70000},
		{"prepended", append(bytes.Repeat([]byte{'#'}, 4096), zipped(t, "", [2]string{"a.md", "x"}, [2]string{"b.md", "y"})...), 2},
		{"empty", zipped(t, ""), 0},
		{"an end whose size falls short of its directory", shortEnd(t), 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ar, err := opened(t, a, store, tt.data, 100000)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(ar.Entries()); got != tt.count {
				t.Errorf("%d entries, want %d", got, tt.count)
			}
		})
	}
}

// shortEnd is an archive of two entries whose end says its directory is
// 10 bytes shorter than it is: archive/zip, finding a record where the end
// says the directory starts, reads it from there.
func shortEnd(t *testing.T) []byte {
	data := zipped(t, "", [2]string{"a.md", "x"}, [2]string{"b.md", "y"})
	end := endOf(data)
	binary.LittleEndian.PutUint32(data[end+12:], binary.LittleEndian.Uint32(data[end+12:])-10)
	return data
}

// endOf is where data's directory end is.
func endOf(data []byte) int {
	return bytes.LastIndex(data, []byte{0x50, 0x4b, 0x05, 0x06})
}

// An archive of more entries than read, or a larger directory, is
// ErrTooManyEntries before archive/zip reads it; and so is one whose end
// says it holds fewer than its directory does. One that is no zip, or
// whose end points outside it, is ErrNotZip.
func TestAnImportsArchiveRefusesItsDirectory(t *testing.T) {
	a, store := newArchives(t)
	files := make([]any, 20)
	for i := range files {
		files[i] = [2]string{fmt.Sprintf("f%02d.md", i), "x"}
	}
	lying := zipped(t, "", files...)
	end := endOf(lying)
	binary.LittleEndian.PutUint16(lying[end+8:], 3)
	binary.LittleEndian.PutUint16(lying[end+10:], 3)
	outside := zipped(t, "", [2]string{"a.md", "x"})
	binary.LittleEndian.PutUint32(outside[endOf(outside)+12:], 1<<20) // a directory larger than all before its end
	short := zipped(t, "", [2]string{"a.md", "x"}, [2]string{"b.md", "y"})
	directory := int(binary.LittleEndian.Uint32(short[endOf(short)+16:]))
	short[directory+46+len("a.md")] = 'Q' // the second record's signature, broken: archive/zip reads one
	for _, tt := range []struct {
		name    string
		data    []byte
		most    int
		largest int64
		want    error
	}{
		{"more entries than read", zipped(t, "", files...), 19, archiveadapter.MaxDirectory, app.ErrTooManyEntries},
		{"as many as read", zipped(t, "", files...), 20, archiveadapter.MaxDirectory, nil},
		{"an end that says fewer", lying, 10, archiveadapter.MaxDirectory, app.ErrTooManyEntries},
		{"a directory larger than read", zipped(t, "", files...), 100, 500, app.ErrTooManyEntries},
		{"no zip", []byte("PK but no archive at all, whatever its length"), 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"an end pointing outside", outside, 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"records fewer than the end says", short, 10, archiveadapter.MaxDirectory, app.ErrNotZip},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := opened(t, archiveadapter.WithMaxDirectory(a, tt.largest), store, tt.data, tt.most)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("OpenImport() = %v, want %v", err, tt.want)
			}
		})
	}
}
