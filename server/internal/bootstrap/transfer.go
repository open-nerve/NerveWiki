package bootstrap

import (
	"context"
	"errors"
	"io"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// The exports' parts (M7/P5 design 3.8, 3.14): the transfer module reads a
// notebook's tree through the page module's ExportNodes and the
// attachments' files through the asset module's Blobs, and follows the
// notebook module's deletions. The modules do not import each other, so
// their values meet here.

// transferNodes is the page module's ExportNodes as the transfer module
// reads them.
type transferNodes struct {
	page.ExportNodes
}

// Scope is the nodes of an export.
func (n transferNodes) Scope(ctx context.Context, notebookID uuid.UUID, root *uuid.UUID) ([]transfer.Node, error) {
	got, err := n.ExportNodes.Scope(ctx, notebookID, root)
	if err != nil {
		return nil, err
	}
	out := make([]transfer.Node, len(got))
	for i, x := range got {
		out[i] = transfer.Node(x)
	}
	return out, nil
}

// transferBlobs is the asset module's Blobs as the transfer module reads
// them.
type transferBlobs struct {
	asset.Blobs
}

// Of is the blob of each attachment of nodeIDs, and when it was written.
func (b transferBlobs) Of(ctx context.Context, notebookID uuid.UUID, nodeIDs []uuid.UUID) (map[uuid.UUID]transfer.Blob, error) {
	got, err := b.Blobs.Of(ctx, notebookID, nodeIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]transfer.Blob, len(got))
	for node, x := range got {
		out[node] = transfer.Blob(x)
	}
	return out, nil
}

// Open opens a blob's file; a file not in the store is transfer's
// ErrFileMissing, which the export reports and goes on.
func (b transferBlobs) Open(ctx context.Context, blob uuid.UUID) (io.ReadCloser, error) {
	r, err := b.Blobs.Open(ctx, blob)
	if errors.Is(err, asset.ErrNoFile) {
		return nil, transfer.ErrFileMissing
	}
	return r, err
}

// transferNotebookDeletion is the jobs' part in a notebook's deletion as
// the notebook module calls it.
type transferNotebookDeletion struct {
	transfer transfer.NotebookDeletion
}

func (d transferNotebookDeletion) NotebookDeleted(ctx context.Context, x notebook.NotebookDeletion) error {
	return d.transfer.NotebooksDeleted(ctx, x.NotebookIDs, x.At)
}
