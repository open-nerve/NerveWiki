package asset

import (
	"context"
	"errors"
	"io"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	filesadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/files"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// Blobs are the attachments' files as another module reads them (M7
// design 4.1; M7/P5 design 3.8): an export reads which file each
// attachment of its snapshot has, then the files, outside it. Built on the
// pool and the store alone.
type Blobs struct {
	rows  *postgresadapter.Store
	files filesadapter.Files
}

// ErrNoFile is a blob whose file is not in the store.
var ErrNoFile = errors.New("asset: the attachment's file is not in the store")

// NewBlobs returns the blobs of pool's rows and store's files.
func NewBlobs(pool *pgxpool.Pool, store storage.Store) Blobs {
	return Blobs{rows: postgresadapter.New(pool), files: filesadapter.New(store)}
}

// blobsBatch is how many nodes one read of the rows asks about.
const blobsBatch = 10000

// Of is the blob of each attachment of nodeIDs not deleted, of notebookID,
// by node, read in the caller's transaction; a node of none is not among
// them.
func (b Blobs) Of(ctx context.Context, notebookID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	out := make(map[uuid.UUID]uuid.UUID, len(nodeIDs))
	for start := 0; start < len(nodeIDs); start += blobsBatch {
		got, err := b.rows.BlobsOfNodes(ctx, nodeIDs[start:min(start+blobsBatch, len(nodeIDs))])
		if err != nil {
			return nil, err
		}
		for node, blob := range got {
			if blob.NotebookID == notebookID {
				out[node] = blob.ID
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
