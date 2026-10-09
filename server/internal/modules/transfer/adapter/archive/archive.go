// Package archiveadapter keeps the jobs' archives in the platform's store
// (M7/P5 design 3.8): an export writes its zip file there as it goes,
// visible once committed, at exports/<job id>.zip; its errors in the
// module's terms.
package archiveadapter

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"log/slog"
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
}

// New returns the archives of store.
func New(store storage.Store, logger *slog.Logger) Archives {
	return Archives{store: store, logger: logger}
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
