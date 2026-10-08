// Package files keeps the attachments' files in the platform's store, its
// errors in the module's terms.
package files

import (
	"context"
	"errors"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// Files is app.Files over a storage.Store.
type Files struct {
	store storage.Store
}

// New returns the files of store.
func New(store storage.Store) Files {
	return Files{store: store}
}

// Create implements app.Files.
func (f Files) Create(ctx context.Context, key string) (app.FileWriter, error) {
	w, err := f.store.Create(ctx, key)
	if err != nil {
		return nil, full(err)
	}
	return writer{w: w}, nil
}

// Open implements app.Files.
func (f Files) Open(ctx context.Context, key string) (app.File, error) {
	file, err := f.store.Open(ctx, key)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, app.ErrNoFile
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}

// List implements app.StoredFiles.
func (f Files) List(ctx context.Context, area string, before time.Time, each func(key string) error) error {
	return f.store.List(ctx, area, before, each)
}

// Delete implements app.Files.
func (f Files) Delete(ctx context.Context, key string) error {
	return f.store.Delete(ctx, key)
}

// Free implements app.Files.
func (f Files) Free(ctx context.Context) (int64, error) {
	return f.store.Free(ctx)
}

// writer is a storage.Writer whose running out of room is
// domain.ErrStorageFull.
type writer struct {
	w storage.Writer
}

func (w writer) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	return n, full(err)
}

func (w writer) Commit() error { return full(w.w.Commit()) }
func (w writer) Abort() error  { return w.w.Abort() }

// full is err, or domain.ErrStorageFull for the store's storage.ErrFull.
func full(err error) error {
	if errors.Is(err, storage.ErrFull) {
		return domain.ErrStorageFull
	}
	return err
}
