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

// The exports' and the imports' parts (M7/P5 design 3.8, 3.14; M7/P6
// design 3.3, 3.5, 3.16): the transfer module reads a notebook's tree
// through the page module's ExportNodes and the attachments' files through
// the asset module's Blobs, writes an import's nodes through the page
// module's TreeWrites and its files through the asset module's Blobs, and
// follows the notebook module's deletions. The modules do not import each
// other, so their values meet here.

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

// transferTree is the page module's TreeWrites as the transfer module's
// imports write through it.
type transferTree struct {
	tree page.TreeWrites
}

func (t transferTree) CheckContent(content string) error {
	return t.tree.CheckContent(content)
}

func (t transferTree) Parse(ctx context.Context, content string) (transfer.Parsed, error) {
	p, err := t.tree.Parse(ctx, content)
	if err != nil {
		return nil, err
	}
	return parsedPage{p}, nil
}

func (t transferTree) Import(ctx context.Context, spec transfer.ImportSpec, do func(ctx context.Context, u transfer.ImportUnit) error) (uuid.UUID, error) {
	return t.tree.Import(ctx, page.ImportSpec{NotebookID: spec.NotebookID, Action: spec.Action, Client: page.Client(spec.Client),
		Changeset: spec.Changeset}, func(ctx context.Context, u page.ImportUnit) error {
		return do(ctx, importUnit{u})
	})
}

// parsedPage is a page's content the page module parsed.
type parsedPage struct {
	p page.Parsed
}

func (p parsedPage) Links() int { return p.p.Links }
func (p parsedPage) Release()   { p.p.Release() }

// importUnit is the page module's import unit as the transfer module's
// import calls it.
type importUnit struct {
	u page.ImportUnit
}

func (i importUnit) CreatePage(ctx context.Context, p transfer.ImportedPage) (transfer.CreatedNode, error) {
	parsed, ok := p.Parsed.(parsedPage)
	if !ok {
		return transfer.CreatedNode{}, errors.New("an import's page not parsed by the page module")
	}
	n, err := i.u.CreatePage(ctx, page.ImportedPage{ParentID: p.ParentID, Name: p.Name, Content: p.Content, Parsed: parsed.p})
	return createdNode(n), treeError(err)
}

func (i importUnit) CreateAsset(ctx context.Context, a transfer.ImportedAsset,
	after func(ctx context.Context, n transfer.CreatedNode) error,
) (transfer.CreatedNode, error) {
	n, err := i.u.CreateAsset(ctx, page.ImportedAsset{ParentID: a.ParentID, Name: a.Name,
		Meta: page.AssetMeta{MIME: a.File.MIME, Bytes: a.File.Bytes, SHA256: a.File.SHA256}},
		func(ctx context.Context, n page.NodeInfo) error { return after(ctx, createdNode(n)) })
	return createdNode(n), treeError(err)
}

func createdNode(n page.NodeInfo) transfer.CreatedNode {
	return transfer.CreatedNode{ID: n.ID, Name: n.Name, CreatedAt: n.CreatedAt}
}

// treeError is the page module's error as the import reads it: a page too
// deep, a parent gone.
func treeError(err error) error {
	switch {
	case errors.Is(err, page.ErrTooDeep):
		return transfer.ErrTooDeep
	case errors.Is(err, page.ErrNoParent):
		return transfer.ErrNoParent
	}
	return err
}

// transferAttachments is the asset module's Blobs as the transfer module's
// imports write their files through it.
type transferAttachments struct {
	blobs asset.Blobs
}

func (a transferAttachments) Put(ctx context.Context, name string, r io.Reader, maxBytes int64) (transfer.File, error) {
	f, err := a.blobs.Put(ctx, name, r, maxBytes)
	switch {
	case errors.Is(err, asset.ErrTooLarge):
		return transfer.File{}, transfer.ErrTooLarge
	case errors.Is(err, asset.ErrStorageFull):
		return transfer.File{}, transfer.ErrStorageFull
	case err != nil:
		return transfer.File{}, err
	}
	return transfer.File(f), nil
}

func (a transferAttachments) Attach(ctx context.Context, f transfer.File, o transfer.Owner) error {
	return a.blobs.Attach(ctx, asset.File(f), asset.Owner(o))
}

func (a transferAttachments) Drop(ctx context.Context, f transfer.File) error {
	return a.blobs.Drop(ctx, asset.File(f))
}
