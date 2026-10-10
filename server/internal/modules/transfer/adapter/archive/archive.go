// Package archiveadapter keeps the jobs' archives in the platform's store
// (M7/P5 design 3.8; M7/P6 design 3.10): an export writes its zip file
// there as it goes, visible once committed, at exports/<job id>.zip; an
// import's request writes its zip at imports/<job id>.zip, which its job
// reads, the directory checked before archive/zip reads it; their errors
// in the module's terms.
package archiveadapter

import (
	"archive/zip"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// Archives is app.Archives over a storage.Store.
type Archives struct {
	store  storage.Store
	logger *slog.Logger
	// maxDirectory is the largest directory of an import's archive read:
	// MaxDirectory.
	maxDirectory int64
}

// New returns the archives of store.
func New(store storage.Store, logger *slog.Logger) Archives {
	return Archives{store: store, logger: logger, maxDirectory: MaxDirectory}
}

// Create implements app.Archives.
func (a Archives) Create(ctx context.Context, id uuid.UUID) (app.Archive, error) {
	w, err := a.store.Create(ctx, domain.Archive(domain.KindExport, id))
	if err != nil {
		return nil, full(err)
	}
	c := &counter{w: w}
	return &archive{w: w, counter: c, zip: zip.NewWriter(c)}, nil
}

// Open implements app.Archives.
func (a Archives) Open(ctx context.Context, id uuid.UUID) (app.ArchiveFile, error) {
	f, err := a.store.Open(ctx, domain.Archive(domain.KindExport, id))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, app.ErrFileMissing
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Delete implements app.Archives.
func (a Archives) Delete(ctx context.Context, kind domain.Kind, id uuid.UUID) error {
	return a.store.Delete(ctx, domain.Archive(kind, id))
}

// List implements app.Archives.
func (a Archives) List(ctx context.Context, kind domain.Kind, before time.Time, each func(id uuid.UUID) error) error {
	area := string(kind) + "s"
	return a.store.List(ctx, area, before, func(key string) error {
		name, _ := strings.CutPrefix(key, area+"/")
		name, ok := strings.CutSuffix(name, ".zip")
		id, err := uuid.Parse(name)
		if !ok || err != nil || domain.Archive(kind, id) != key {
			a.logger.WarnContext(ctx, "a file of the archives' area that is no job's", slog.String("key", key))
			return nil
		}
		return each(id)
	})
}

// Free implements app.Archives.
func (a Archives) Free(ctx context.Context) (int64, error) {
	return a.store.Free(ctx)
}

// archive is a zip file written into a storage.Writer.
type archive struct {
	w       storage.Writer
	counter *counter
	zip     *zip.Writer
}

// Add implements app.Archive. Go's archive/zip marks a name that is not
// ASCII as UTF-8, and turns to zip64 past 4 GiB.
func (a *archive) Add(path string, modified time.Time, stored bool, r io.Reader) error {
	h := &zip.FileHeader{Name: path, Modified: modified, Method: zip.Deflate}
	if stored || r == nil {
		h.Method = zip.Store
	}
	w, err := a.zip.CreateHeader(h)
	if err != nil {
		return full(err)
	}
	if r == nil {
		return nil
	}
	_, err = io.Copy(w, r)
	return full(err)
}

// Commit implements app.Archive.
func (a *archive) Commit() (int64, error) {
	if err := a.zip.Close(); err != nil {
		_ = a.w.Abort()
		return 0, full(err)
	}
	if err := a.w.Commit(); err != nil {
		return 0, full(err)
	}
	return a.counter.n, nil
}

// Abort implements app.Archive.
func (a *archive) Abort() error {
	return a.w.Abort()
}

// counter counts the bytes written through it.
type counter struct {
	w io.Writer
	n int64
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// full is err, or domain.ErrStorageFull for the store's storage.ErrFull.
func full(err error) error {
	if errors.Is(err, storage.ErrFull) {
		return domain.ErrStorageFull
	}
	return err
}

// Upload implements app.Archives.
func (a Archives) Upload(ctx context.Context, id uuid.UUID) (app.Upload, error) {
	w, err := a.store.Create(ctx, domain.Archive(domain.KindImport, id))
	if err != nil {
		return nil, full(err)
	}
	return upload{w: w}, nil
}

// upload is an import's archive written into a storage.Writer.
type upload struct {
	w storage.Writer
}

func (u upload) Write(p []byte) (int, error) {
	n, err := u.w.Write(p)
	return n, full(err)
}

func (u upload) Commit() error { return full(u.w.Commit()) }
func (u upload) Abort() error  { return u.w.Abort() }

// directoryEnds is what archive/zip reads of an archive besides its
// directory: its end, searched in the last 65 KiB, the zip64 records, and
// what its buffer reads ahead.
const directoryEnds = 128 << 10

// OpenImport implements app.Archives: the directory's end read, its
// records counted (zipdir.go), then archive/zip reads it, bounded to the
// bytes counted and its end's; once read, the entries' data is not.
func (a Archives) OpenImport(ctx context.Context, id uuid.UUID, most int) (app.ImportArchive, error) {
	f, err := a.store.Open(ctx, domain.Archive(domain.KindImport, id))
	if errors.Is(err, storage.ErrNotFound) {
		return nil, app.ErrFileMissing
	}
	if err != nil {
		return nil, err
	}
	z, err := readImport(f, most, a.maxDirectory)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &importArchive{f: f, z: z}, nil
}

// readImport reads f's directory, of at most most records and largest
// bytes: app.ErrNotZip, app.ErrTooManyEntries.
func readImport(f storage.File, most int, largest int64) (*zip.Reader, error) {
	size := f.Size()
	d, err := readEnd(f, size)
	switch {
	case errors.Is(err, errNotZip):
		return nil, fmt.Errorf("%w: %w", app.ErrNotZip, err)
	case err != nil:
		return nil, err
	case d.records > uint64(most) || d.size > uint64(largest): //nolint:gosec // positive settings
		return nil, app.ErrTooManyEntries
	}
	n, read, refused := countRecords(f, size, d.start, most, largest)
	if n > most || read > largest {
		return nil, app.ErrTooManyEntries
	}
	b := &bounded{r: f, left: d.probed + read + refused + directoryEnds}
	z, err := zip.NewReader(b, size)
	b.lift()
	switch {
	case errors.Is(err, errBounded):
		return nil, app.ErrTooManyEntries
	case err != nil && !errors.Is(err, zip.ErrInsecurePath):
		return nil, fmt.Errorf("%w: %w", app.ErrNotZip, err)
	case len(z.File) != n:
		// The pre-read reads the records as archive/zip does: no archive found yet tells them apart.
		return nil, fmt.Errorf("%w: %d entries read, %d counted", app.ErrNotZip, len(z.File), n)
	}
	if err := checkData(z, d.start); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrNotZip, err)
	}
	return z, nil
}

// errOverlap is an archive whose entries' data overlap, or reach into its
// directory: no archiver writes one.
var errOverlap = errors.New("zip: the entries' data overlap, or reach into the directory")

// checkData checks that the data of z's entries, each from where it
// starts as long as the directory says it is packed, lie apart and before
// the directory, at start (M7 closeout A-M1): each entry's packed bytes
// are then bytes of the archive of its own, which domain.TooCompressed
// measures its unpacking by, and no data is read for two entries. An
// entry whose local header cannot be read is left out: it is unreadable
// as it is opened.
func checkData(z *zip.Reader, start int64) error {
	type span struct{ from, to int64 }
	spans := make([]span, 0, len(z.File))
	for _, f := range z.File {
		from, err := f.DataOffset()
		if err != nil {
			continue
		}
		if f.CompressedSize64 > uint64(start) || from+int64(f.CompressedSize64) > start { //nolint:gosec // start is positive, the size bounded by it
			return errOverlap
		}
		spans = append(spans, span{from: from, to: from + int64(f.CompressedSize64)}) //nolint:gosec // bounded above
	}
	slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.from, b.from) })
	for i := 1; i < len(spans); i++ {
		if spans[i].from < spans[i-1].to {
			return errOverlap
		}
	}
	return nil
}

// errBounded is a read of an archive's directory past what was counted.
var errBounded = errors.New("zip: the directory goes on past the records counted")

// bounded lets left bytes of r be read, until lifted: archive/zip reads
// no more of a directory than was counted.
type bounded struct {
	r      io.ReaderAt
	left   int64
	lifted bool
}

func (b *bounded) ReadAt(p []byte, off int64) (int, error) {
	if !b.lifted && int64(len(p)) > b.left {
		return 0, errBounded
	}
	n, err := b.r.ReadAt(p, off)
	if !b.lifted {
		b.left -= int64(n)
	}
	return n, err
}

func (b *bounded) lift() { b.lifted = true }

// importArchive is an import's archive read by archive/zip.
type importArchive struct {
	f storage.File
	z *zip.Reader
}

// Entries implements app.ImportArchive: a name ending in "/" or "\" is a
// folder, and so is a folder's mode; a mode neither a folder's nor a
// regular file's is special; the first bit of the flags tells an
// encrypted one.
func (a *importArchive) Entries() []domain.RawEntry {
	out := make([]domain.RawEntry, len(a.z.File))
	for i, f := range a.z.File {
		mode := f.Mode()
		folder := strings.HasSuffix(f.Name, "/") || strings.HasSuffix(f.Name, `\`) || mode.IsDir()
		out[i] = domain.RawEntry{Index: i, Name: f.Name, Folder: folder, Special: !folder && !mode.IsRegular(), Encrypted: f.Flags&0x1 != 0,
			Method: f.Method}
	}
	return out
}

// Packed implements app.ImportArchive.
func (a *importArchive) Packed(i int) int64 {
	return int64(min(a.z.File[i].CompressedSize64, 1<<63-1)) //nolint:gosec // bounded
}

// Open implements app.ImportArchive.
func (a *importArchive) Open(i int) (io.ReadCloser, error) {
	return a.z.File[i].Open()
}

// Close implements app.ImportArchive.
func (a *importArchive) Close() error {
	return a.f.Close()
}
