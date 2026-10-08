package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// Blobs keeps the attachments' files and their rows (M7/P2 design 3.4):
// an upload's, and from P5 an import's and an export's.
type Blobs struct {
	files   Files
	rows    Rows
	sniffer Sniffer
	logger  *slog.Logger
}

// NewBlobs returns them.
func NewBlobs(files Files, rows Rows, sniffer Sniffer, logger *slog.Logger) *Blobs {
	return &Blobs{files: files, rows: rows, sniffer: sniffer, logger: logger}
}

// headBytes is how much of a file's start sniffing reads.
const headBytes = 512

// ReadError is a file that failed to arrive: reading it failed, not
// writing it. Err is the reader's error.
type ReadError struct {
	Err error
}

func (e *ReadError) Error() string { return "asset: read the file: " + e.Err.Error() }
func (e *ReadError) Unwrap() error { return e.Err }

// Put writes the file r brings, named name, as a new blob, computing its
// SHA-256 as the bytes pass: domain.ErrTooLarge past max bytes, the
// store's domain.ErrStorageFull, a *ReadError when r fails, each leaving
// no file: a write that cannot be aborted is logged, its bytes left in the
// store's files being written until the next start. Once committed, its
// type is told by name's extension and its first bytes, and an image's
// size is read from the file. A failed Commit may leave the file at its
// key: the orphan sweep deletes it.
func (b *Blobs) Put(ctx context.Context, name string, r io.Reader, maxBytes int64) (domain.Blob, error) {
	id := uuid.NewV7()
	w, err := b.files.Create(ctx, domain.Key(id))
	if err != nil {
		return domain.Blob{}, err
	}
	sum := sha256.New()
	head := make([]byte, 0, headBytes)
	buf := make([]byte, 32<<10)
	var n int64
	for {
		k, rerr := r.Read(buf)
		if k > 0 {
			if n += int64(k); n > maxBytes {
				return domain.Blob{}, b.abort(ctx, id, w, domain.ErrTooLarge)
			}
			chunk := buf[:k]
			sum.Write(chunk)
			head = append(head, chunk[:min(len(chunk), headBytes-len(head))]...)
			if _, err := w.Write(chunk); err != nil {
				return domain.Blob{}, b.abort(ctx, id, w, err)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return domain.Blob{}, b.abort(ctx, id, w, &ReadError{Err: rerr})
		}
	}
	if err := w.Commit(); err != nil {
		return domain.Blob{}, err
	}
	blob := domain.Blob{ID: id, MIME: domain.TypeOf(name, b.sniffer.Sniff(head)), Bytes: n, SHA256: sum.Sum(nil)}
	if domain.HasSize(blob.MIME) {
		blob.Width, blob.Height = b.size(ctx, id)
	}
	return blob, nil
}

// abort aborts the write w of the blob id, which failed with err, and
// answers err; an abort that fails is logged, as no answer tells it.
func (b *Blobs) abort(ctx context.Context, id uuid.UUID, w FileWriter, err error) error {
	if aerr := w.Abort(); aerr != nil {
		b.logger.WarnContext(ctx, "an upload's partial file is not deleted", slog.String("blob_id", id.String()), slog.Any("error", aerr))
	}
	return err
}

// size is the width and height of the image blob id, 0 and 0 when they
// cannot be read or are larger than domain.MaxSide.
func (b *Blobs) size(ctx context.Context, id uuid.UUID) (int, int) {
	f, err := b.files.Open(ctx, domain.Key(id))
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	w, h, ok := b.sniffer.Size(f)
	if !ok || w < 1 || h < 1 || w > domain.MaxSide || h > domain.MaxSide {
		return 0, 0
	}
	return w, h
}

// Attach writes the row of blob, its node, notebook, uploader and time
// set, in ctx's transaction.
func (b *Blobs) Attach(ctx context.Context, blob domain.Blob) error {
	return b.rows.CreateBlob(ctx, blob)
}

// Open opens the file blobID of the attachment nodeID's row not deleted,
// and answers the row: domain.ErrNotFound without one, or when its file is
// another; ErrNoFile, with the row, when the file is gone.
func (b *Blobs) Open(ctx context.Context, nodeID, blobID uuid.UUID) (File, domain.Blob, error) {
	blob, err := b.rows.BlobOfNode(ctx, nodeID)
	switch {
	case errors.Is(err, ErrNoRow):
		return nil, domain.Blob{}, domain.ErrNotFound
	case err != nil:
		return nil, domain.Blob{}, err
	case blob.ID != blobID:
		return nil, domain.Blob{}, domain.ErrNotFound
	}
	f, err := b.files.Open(ctx, domain.Key(blob.ID))
	if err != nil {
		return nil, blob, err
	}
	return f, blob, nil
}

// Drop deletes blob's file, which no row holds: a unit that refused the
// attachment rolled its row back.
func (b *Blobs) Drop(ctx context.Context, blob domain.Blob) error {
	return b.files.Delete(ctx, domain.Key(blob.ID))
}
