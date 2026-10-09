package asset

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	filesadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/sniff"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// Blobs are the attachments' files as another module reads and writes
// them (M7 design 4.1): an export reads which file each attachment of its
// snapshot has, then the files, outside it (M7/P5 design 3.8); an import
// writes each file before its unit, and its row in the unit (M7/P6 design
// 3.5). Built on the pool, the store and a logger alone.
type Blobs struct {
	rows   *postgresadapter.Store
	files  filesadapter.Files
	writes *app.Blobs
}

// Blob is an attachment's file as Blobs.Of reads it: its id, and when it
// was written.
type Blob struct {
	ID      uuid.UUID
	Created time.Time
}

// ErrNoFile is a blob whose file is not in the store.
var ErrNoFile = errors.New("asset: the attachment's file is not in the store")

// NewBlobs returns the blobs of pool's rows and store's files, logging to
// logger what no answer tells.
func NewBlobs(pool *pgxpool.Pool, store storage.Store, logger *slog.Logger) Blobs {
	rows, files := postgresadapter.New(pool), filesadapter.New(store)
	return Blobs{rows: rows, files: files, writes: app.NewBlobs(files, rows, sniff.Sniffer{}, logger)}
}

// File is a file Put wrote: its blob's id, its type as the server
// determined it, its size and SHA-256, an image's size in pixels (0 when
// not known).
type File struct {
	ID            uuid.UUID
	MIME          string
	Bytes         int64
	SHA256        []byte
	Width, Height int
}

// Owner is what Attach gives a file's row: the attachment's node, its
// notebook, who created it and when.
type Owner struct {
	NodeID, NotebookID, CreatedBy uuid.UUID
	CreatedAt                     time.Time
}

// ErrTooLarge is a file Put read more than its largest of.
var ErrTooLarge = domain.ErrTooLarge

// ErrStorageFull is a file the store has no room for.
var ErrStorageFull = domain.ErrStorageFull

// Put writes the file r brings, named name, as a new blob, its SHA-256
// computed as the bytes pass, its type told by name's extension and its
// first bytes: ErrTooLarge past maxBytes, ErrStorageFull, or r's error,
// each leaving no file. A failed commit may leave the file at its key,
// which the orphan sweep deletes.
func (b Blobs) Put(ctx context.Context, name string, r io.Reader, maxBytes int64) (File, error) {
	blob, err := b.writes.Put(ctx, name, r, maxBytes)
	var read *app.ReadError
	if errors.As(err, &read) {
		err = read.Err
	}
	if err != nil {
		return File{}, err
	}
	return File{ID: blob.ID, MIME: blob.MIME, Bytes: blob.Bytes, SHA256: blob.SHA256, Width: blob.Width, Height: blob.Height}, nil
}

// Attach writes the row of f, owned by o, in the transaction ctx carries:
// the unit that created o's node.
func (b Blobs) Attach(ctx context.Context, f File, o Owner) error {
	return b.writes.Attach(ctx, domain.Blob{ID: f.ID, NodeID: o.NodeID, NotebookID: o.NotebookID, MIME: f.MIME, Bytes: f.Bytes,
		SHA256: f.SHA256, Width: f.Width, Height: f.Height, CreatedBy: o.CreatedBy, CreatedAt: o.CreatedAt})
}

// Drop deletes f's file, which no row holds: its unit was refused.
func (b Blobs) Drop(ctx context.Context, f File) error {
	return b.writes.Drop(ctx, domain.Blob{ID: f.ID})
}

// blobsBatch is how many nodes one read of the rows asks about.
const blobsBatch = 10000

// Of is the blob of each attachment of nodeIDs not deleted, of notebookID,
// by node, read in the caller's transaction; a node of none is not among
// them.
func (b Blobs) Of(ctx context.Context, notebookID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]Blob, error) {
	out := make(map[uuid.UUID]Blob, len(nodeIDs))
	for start := 0; start < len(nodeIDs); start += blobsBatch {
		got, err := b.rows.BlobsOfNodes(ctx, nodeIDs[start:min(start+blobsBatch, len(nodeIDs))])
		if err != nil {
			return nil, err
		}
		for node, blob := range got {
			if blob.NotebookID == notebookID {
				out[node] = Blob{ID: blob.ID, Created: blob.CreatedAt}
			}
		}
	}
	return out, nil
}

// Open opens the file of blob, or answers ErrNoFile.
func (b Blobs) Open(ctx context.Context, blob uuid.UUID) (io.ReadCloser, error) {
	f, err := b.files.Open(ctx, domain.Key(blob))
	if errors.Is(err, app.ErrNoFile) {
		return nil, ErrNoFile
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}
