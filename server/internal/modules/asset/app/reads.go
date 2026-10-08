package app

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Reads reads the attachments, their addresses signed (M7/P2 design 3.7):
// GET /api/v0/assets/{node_id} and GET
// /api/v0/notebooks/{notebook_id}/assets. A read takes no lock and opens no
// transaction.
type Reads struct {
	auth      shared.Authorizer
	notebooks Notebooks
	nodes     Nodes
	rows      Rows
	signer    Signer
	logger    *slog.Logger
}

// ReadsDeps are what Reads needs.
type ReadsDeps struct {
	Authorizer shared.Authorizer
	Notebooks  Notebooks
	Nodes      Nodes
	Rows       Rows
	Signer     Signer
	Logger     *slog.Logger
}

// NewReads returns the use cases.
func NewReads(d ReadsDeps) *Reads {
	return &Reads{auth: d.Authorizer, notebooks: d.Notebooks, nodes: d.Nodes, rows: d.Rows, signer: d.Signer, logger: d.Logger}
}

// AssetPage is a page of a list of attachments, and the cursor of the
// next, "" after the last page.
type AssetPage struct {
	Assets     []Asset
	NextCursor string
}

// Get reads the attachment id: asset.not_found for a node that is none,
// is deleted, or whose notebook the caller cannot read in (asset.read).
// A node without its row, which each attachment's node has, is logged as
// an error and not found.
func (r *Reads) Get(ctx context.Context, id uuid.UUID) (Asset, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Asset{}, err
	}
	n, ok, err := r.nodes.Node(ctx, id)
	switch {
	case err != nil:
		return Asset{}, err
	case !ok || !n.Asset:
		return Asset{}, domain.ErrNotFound
	}
	if err := r.decide(ctx, actor, n.NotebookID, domain.ErrNotFound); err != nil {
		return Asset{}, err
	}
	b, err := r.rows.BlobOfNode(ctx, id)
	switch {
	case errors.Is(err, ErrNoRow):
		r.rowless(ctx, n)
		return Asset{}, domain.ErrNotFound
	case err != nil:
		return Asset{}, err
	}
	return Asset{Node: n, Blob: b, Signed: r.signer.Sign(n.ID, b.ID)}, nil
}

// List lists the attachments under parentID (nil: the root) of the
// notebook, by name key and id, the page that limit and cursor ask for,
// from the first when cursor is nil. A cursor shared.DecodeCursor refuses
// is 400 bad_request, judged first; then the decision (asset.read,
// notebook.not_found); then the parent, page.not_found unless a page of
// the notebook; then the limit, 422 outside 1–100. One node more than
// limit is read to tell whether another page follows. A node without its
// row is logged as an error and left out.
func (r *Reads) List(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, limit *int, cursor *string) (AssetPage, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return AssetPage{}, err
	}
	var after *Cursor
	if cursor != nil {
		after = &Cursor{}
		if err := shared.DecodeCursor(*cursor, after); err != nil {
			return AssetPage{}, err
		}
	}
	if err := r.decide(ctx, actor, notebookID, domain.ErrNotebookNotFound); err != nil {
		return AssetPage{}, err
	}
	if parentID != nil {
		switch page, err := r.nodes.Parent(ctx, notebookID, *parentID); {
		case err != nil:
			return AssetPage{}, err
		case !page:
			return AssetPage{}, domain.ErrParentNotFound
		}
	}
	size, err := shared.PageSize(limit)
	if err != nil {
		return AssetPage{}, err
	}
	nodes, err := r.nodes.Assets(ctx, notebookID, parentID, after, size+1)
	if err != nil {
		return AssetPage{}, err
	}
	var out AssetPage
	if len(nodes) > size {
		nodes = nodes[:size]
		last := nodes[size-1]
		if out.NextCursor, err = shared.EncodeCursor(Cursor{NameKey: last.NameKey, ID: last.ID}); err != nil {
			return AssetPage{}, err
		}
	}
	ids := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	blobs, err := r.rows.BlobsOfNodes(ctx, ids)
	if err != nil {
		return AssetPage{}, err
	}
	out.Assets = make([]Asset, 0, len(nodes))
	for _, n := range nodes {
		b, ok := blobs[n.ID]
		if !ok {
			r.rowless(ctx, n)
			continue
		}
		out.Assets = append(out.Assets, Asset{Node: n, Blob: b, Signed: r.signer.Sign(n.ID, b.ID)})
	}
	return out, nil
}

// decide decides asset.read on the notebook notebookID for actor: notFound
// for a notebook that does not exist, is deleted, or that the caller
// cannot see.
func (r *Reads) decide(ctx context.Context, actor shared.Actor, notebookID uuid.UUID, notFound error) error {
	workspaceID, ok, err := r.notebooks.WorkspaceOf(ctx, notebookID)
	switch {
	case err != nil:
		return err
	case !ok:
		return notFound
	}
	_, err = r.auth.Authorize(ctx, actor, domain.ActionRead, shared.Target{WorkspaceID: workspaceID, NotebookID: notebookID})
	if errors.Is(err, shared.ErrNotVisible) {
		return notFound
	}
	return err
}

// rowless logs an attachment's node without its row: each has one, written
// in the node's unit (M7/P2 design 3.7).
func (r *Reads) rowless(ctx context.Context, n Node) {
	r.logger.ErrorContext(ctx, "attachment node without its row", slog.String("node_id", n.ID.String()),
		slog.String("notebook_id", n.NotebookID.String()))
}
