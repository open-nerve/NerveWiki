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
	"strings"
	"testing"
	"time"
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

// An import's archive is listed and deleted as the imports' area's: the
// exports' list has none of it, nor the imports' an export's; one newer
// than asked is left out.
func TestAnImportsArchiveIsListedAndDeletedAsAnImports(t *testing.T) {
	a, store := newArchives(t)
	ctx := context.Background()
	id, export := uuid.NewV7(), uuid.NewV7()
	stored(t, store, id, zipped(t, "", [2]string{"a.md", "# A"}))
	w, err := store.Create(ctx, domain.Archive(domain.KindExport, export))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Commit(); err != nil {
		t.Fatal(err)
	}
	listed := func(kind domain.Kind, before time.Time) []uuid.UUID {
		var ids []uuid.UUID
		if err := a.List(ctx, kind, before, func(id uuid.UUID) error {
			ids = append(ids, id)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return ids
	}
	later := time.Now().Add(time.Hour)
	if imports, exports := listed(domain.KindImport, later), listed(domain.KindExport, later); !slices.Equal(imports, []uuid.UUID{id}) ||
		!slices.Equal(exports, []uuid.UUID{export}) {
		t.Errorf("listed imports %v, exports %v; want each kind's", imports, exports)
	}
	if old := listed(domain.KindImport, time.Now().Add(-time.Hour)); len(old) != 0 {
		t.Errorf("listed before an hour ago %v, want none", old)
	}
	if err := a.Delete(ctx, domain.KindExport, id); err != nil {
		t.Fatal(err)
	}
	if imports := listed(domain.KindImport, later); !slices.Equal(imports, []uuid.UUID{id}) {
		t.Errorf("an export's delete of the id took the import's archive: listed %v", imports)
	}
	if err := a.Delete(ctx, domain.KindImport, id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.OpenImport(ctx, id, 10); !errors.Is(err, app.ErrFileMissing) || len(listed(domain.KindImport, later)) != 0 {
		t.Errorf("OpenImport() of a deleted archive = %v, listed %v; want ErrFileMissing, none", err, listed(domain.KindImport, later))
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
		// After the directory, the end and its comment: no record, though
		// their bytes, zeros, hold none to skip.
		{"a comment of zeros", zipped(t, strings.Repeat("\x00", 100), [2]string{"a.md", "x"}), 1},
		// Its end is past the last 1 KiB: found in the last 65 KiB.
		{"a comment of 2 KiB", zipped(t, strings.Repeat("c", 2048), [2]string{"a.md", "x"}), 1},
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

// with64Locator is data with a zip64 locator before its end, pointing at
// offset, and the end saying a zip64 end holds its counts.
func with64Locator(t *testing.T, data []byte, offset uint64) []byte {
	t.Helper()
	end := endOf(data)
	loc := make([]byte, 20)
	binary.LittleEndian.PutUint32(loc, 0x07064b50)
	binary.LittleEndian.PutUint64(loc[8:], offset)
	binary.LittleEndian.PutUint32(loc[16:], 1)
	out := slices.Concat(data[:end], loc, data[end:])
	binary.LittleEndian.PutUint16(out[end+20+8:], 0xffff)
	binary.LittleEndian.PutUint16(out[end+20+10:], 0xffff)
	return out
}

// understated is data whose end says its directory is 100 bytes: the
// directory is read from where its records start, and counted.
func understated(t *testing.T, data []byte) []byte {
	t.Helper()
	binary.LittleEndian.PutUint32(data[endOf(data)+12:], 100)
	return data
}

// records is where data's directory starts, and its records' offsets.
func records(data []byte) []int {
	at := int(binary.LittleEndian.Uint32(data[endOf(data)+16:]))
	var out []int
	for binary.LittleEndian.Uint32(data[at:]) == 0x02014b50 {
		out = append(out, at)
		at += 46 + int(binary.LittleEndian.Uint16(data[at+28:])) + int(binary.LittleEndian.Uint16(data[at+30:])) +
			int(binary.LittleEndian.Uint16(data[at+32:]))
	}
	return out
}

// twoRecordsOfOne is an archive whose second record points at the first
// entry's local header: both read the same data (M7 closeout A-M1).
func twoRecordsOfOne(t *testing.T) []byte {
	t.Helper()
	data := zipped(t, "", [2]string{"a.md", strings.Repeat("a", 1000)}, [2]string{"b.md", strings.Repeat("a", 1000)})
	binary.LittleEndian.PutUint32(data[records(data)[1]+42:], 0)
	return data
}

// packedPast is an archive whose last record says its entry packs to
// more bytes than lie before the directory, no entry after it: its
// unpacking would be measured against bytes the archive does not hold
// for it (M7 closeout A-M1).
func packedPast(t *testing.T) []byte {
	t.Helper()
	data := zipped(t, "", [2]string{"a.md", "x"}, [2]string{"b.md", strings.Repeat("b", 100_000)})
	binary.LittleEndian.PutUint32(data[records(data)[1]+20:], uint32(len(data))) //nolint:gosec // a small archive
	return data
}

// endOf is where data's directory end is.
func endOf(data []byte) int {
	return bytes.LastIndex(data, []byte{0x50, 0x4b, 0x05, 0x06})
}

// The directory an import reads is at most MaxDirectory, 64 MiB: one of
// records of a name of 8 bytes and a comment of 65,535 each, 65,589 bytes,
// is within it at 1,023 of them and past it at 1,024, which is
// ErrTooManyEntries before archive/zip reads it.
func TestAnImportsDirectoryIsAtMost64MiB(t *testing.T) {
	a, store := newArchives(t)
	comment := strings.Repeat(" ", 65535)
	directory := func(n int) []byte {
		files := make([]any, n)
		for i := range files {
			files[i] = &zip.FileHeader{Name: fmt.Sprintf("c%04d.md", i), Method: zip.Store, Comment: comment}
		}
		return zipped(t, "", files...)
	}
	if _, err := opened(t, a, store, directory(1023), 2000); err != nil {
		t.Errorf("OpenImport() of a directory of 1,023 records = %v, want it read", err)
	}
	if _, err := opened(t, a, store, directory(1024), 2000); !errors.Is(err, app.ErrTooManyEntries) {
		t.Errorf("OpenImport() of a directory of 1,024 records = %v, want ErrTooManyEntries", err)
	}
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
		{"a directory larger than read, its end saying less", understated(t, zipped(t, "", files...)), 100, 500, app.ErrTooManyEntries},
		{"no zip", []byte("PK but no archive at all, whatever its length"), 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"an end pointing outside", outside, 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"a zip64 end past the archive", with64Locator(t, zipped(t, "", [2]string{"a.md", "x"}), 1<<40), 100000, archiveadapter.MaxDirectory,
			app.ErrNotZip},
		// archive/zip ignores the locator; the end's 65,535 records are not there.
		{"a zip64 end before the archive", with64Locator(t, zipped(t, "", [2]string{"a.md", "x"}), 1<<63), 100000, archiveadapter.MaxDirectory,
			app.ErrNotZip},
		{"records fewer than the end says", short, 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"two records of one entry's data", twoRecordsOfOne(t), 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"a packed size past the entry's data", packedPast(t), 10, archiveadapter.MaxDirectory, app.ErrNotZip},
		{"empty entries side by side", zipped(t, "", [2]string{"a.md", ""}, &zip.FileHeader{Name: "b.png", Method: zip.Store},
			[2]string{"c.md", "x"}), 10, archiveadapter.MaxDirectory, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := opened(t, archiveadapter.WithMaxDirectory(a, tt.largest), store, tt.data, tt.most)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("OpenImport() = %v, want %v", err, tt.want)
			}
		})
	}
}
