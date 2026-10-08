package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Upload uploads an attachment: POST /api/v0/notebooks/{notebook_id}/assets
// (M7/P2 design 3.5), in the three steps its handler takes as the
// request's parts come. Check before the file, Store the file, Create its
// node and row.
type Upload struct {
	tree   Tree
	blobs  *Blobs
	files  Files
	signer Signer
	links  Links
	logger *slog.Logger
	// maxBytes is asset.max_bytes, minFree storage.min_free_bytes.
	maxBytes, minFree int64
}

// UploadDeps are what Upload needs.
type UploadDeps struct {
	Tree     Tree
	Blobs    *Blobs
	Files    Files
	Signer   Signer
	Links    Links
	Logger   *slog.Logger
	MaxBytes int64
	MinFree  int64
}

// NewUpload returns the use case.
func NewUpload(d UploadDeps) *Upload {
	return &Upload{tree: d.Tree, blobs: d.Blobs, files: d.Files, signer: d.Signer, links: d.Links, logger: d.Logger, maxBytes: d.MaxBytes,
		minFree: d.MinFree}
}

// Request is an upload into NotebookID, under ParentID (nil: the root),
// named Name, from Client (web or api).
type Request struct {
	NotebookID uuid.UUID
	ParentID   *uuid.UUID
	Name       string
	Client     string
}

func (u *Upload) node(req Request, meta FileMeta) NewNode {
	return NewNode{NotebookID: req.NotebookID, ParentID: req.ParentID, Name: req.Name, Meta: meta, Action: domain.ActionUpload,
		Client: req.Client}
}

// Check decides req before its file is read, unlocked: what the page
// module's Check answers (notebook.not_found, forbidden, the name, the
// parent, page.title_taken), then a store with less free space than it
// keeps, storage_full.
func (u *Upload) Check(ctx context.Context, req Request) error {
	if err := u.tree.Check(ctx, u.node(req, FileMeta{})); err != nil {
		return err
	}
	free, err := u.files.Free(ctx)
	if err != nil {
		return err
	}
	if free < u.minFree {
		return domain.ErrStorageFull
	}
	return nil
}

// Store writes the file r brings (Blobs.Put): domain.ErrTooLarge past
// asset.max_bytes, domain.ErrStorageFull, *ReadError. Its type is told by
// the name as the node keeps it, a title's form (shared.CheckTitle), which
// Check has passed.
func (u *Upload) Store(ctx context.Context, req Request, r io.Reader) (domain.Blob, error) {
	name := req.Name
	if title, f := shared.CheckTitle("name", name); f == nil {
		name = title
	}
	return u.blobs.Put(ctx, name, r, u.maxBytes)
}

// Create creates req's node, last among its siblings, with the row of
// blob in the node's unit, and answers the attachment, its address signed
// as of the unit's time, its link as the unit's tree has it (M7/P3 design
// 4.6). A refusal of the unit, a *shared.Error, rolled the row back: the
// file is deleted. Any other error leaves it, for the orphan sweep: a
// COMMIT whose outcome is unknown may have kept the row (M7 design 4.4).
func (u *Upload) Create(ctx context.Context, req Request, blob domain.Blob) (Asset, error) {
	meta := FileMeta{MIME: blob.MIME, Bytes: blob.Bytes, SHA256: blob.SHA256}
	var link string
	n, err := u.tree.CreateAsset(ctx, u.node(req, meta), func(ctx context.Context, n Node) error {
		blob.NodeID, blob.NotebookID, blob.CreatedBy, blob.CreatedAt = n.ID, n.NotebookID, n.CreatedBy, n.CreatedAt
		if err := u.blobs.Attach(ctx, blob); err != nil {
			return err
		}
		links, err := u.links.Of(ctx, n.NotebookID, []uuid.UUID{n.ID})
		if err != nil {
			return err
		}
		var ok bool
		if link, ok = links[n.ID]; !ok {
			return fmt.Errorf("%w %s", ErrNoLink, n.ID)
		}
		return nil
	})
	if err != nil {
		var refused *shared.Error
		if errors.As(err, &refused) {
			u.Discard(ctx, blob)
		}
		return Asset{}, err
	}
	u.logger.InfoContext(ctx, "asset uploaded", slog.String("notebook_id", n.NotebookID.String()), slog.String("node_id", n.ID.String()),
		slog.String("blob_id", blob.ID.String()), slog.String("user_id", n.CreatedBy.String()), slog.String("mime", blob.MIME),
		slog.Int64("bytes", blob.Bytes), slog.String("client", req.Client))
	return Asset{Node: n, Blob: blob, Signed: u.signer.Sign(n.CreatedAt, n.ID, blob.ID), Link: link}, nil
}

// Discard deletes the file of blob, which no row holds, logging a
// failure: the orphan sweep deletes it then.
func (u *Upload) Discard(ctx context.Context, blob domain.Blob) {
	if err := u.blobs.Drop(ctx, blob); err != nil {
		u.logger.ErrorContext(ctx, "an upload's file is not deleted", slog.String("blob_id", blob.ID.String()), slog.Any("error", err))
	}
}

// Asset is an attachment as the API answers it: its node, its file, its
// address signed, and how a wikilink leads to it alone.
type Asset struct {
	Node   Node
	Blob   domain.Blob
	Signed Signed
	Link   string
}
