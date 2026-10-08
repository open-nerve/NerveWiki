package app

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// Content opens an attachment's content at its signed address: GET
// /api/v0/assets/{node_id}/content (M7/P2 design 3.6). No token is read:
// the address is the grant, signed by a read that decided asset.read.
type Content struct {
	nodes  Nodes
	blobs  *Blobs
	signer Signer
	clock  Clock
	logger *slog.Logger
}

// NewContent returns the use case.
func NewContent(nodes Nodes, blobs *Blobs, signer Signer, clock Clock, logger *slog.Logger) *Content {
	return &Content{nodes: nodes, blobs: blobs, signer: signer, clock: clock, logger: logger}
}

// Address is a content's address, its query read: the node, its file, when
// the address expires in Unix seconds, whether it downloads, and its
// signature.
type Address struct {
	Node      uuid.UUID
	Blob      uuid.UUID
	Expires   int64
	Download  bool
	Signature string
}

// Opened is a content opened: the file, its row, the attachment's name,
// and how long the address has left.
type Opened struct {
	File File
	Blob domain.Blob
	Name string
	Left time.Duration
}

// Open opens the content at a. An address the signer did not sign, or
// expired; a node that is none, is deleted, or is no attachment; and a
// row that is gone, or holds another file, are domain.ErrContentNotFound,
// alike; so is a file gone from the store, logged as a warning. The
// signature is checked before anything is read.
func (c *Content) Open(ctx context.Context, a Address) (Opened, error) {
	now := c.clock.Now()
	if !c.signer.Valid(now, a.Node, a.Blob, a.Expires, a.Download, a.Signature) {
		return Opened{}, domain.ErrContentNotFound
	}
	n, ok, err := c.nodes.Node(ctx, a.Node)
	switch {
	case err != nil:
		return Opened{}, err
	case !ok || !n.Asset:
		return Opened{}, domain.ErrContentNotFound
	}
	f, b, err := c.blobs.Open(ctx, a.Node, a.Blob)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return Opened{}, domain.ErrContentNotFound
	case errors.Is(err, ErrNoFile):
		c.logger.WarnContext(ctx, "asset file missing", slog.String("blob_id", b.ID.String()), slog.String("node_id", n.ID.String()))
		return Opened{}, domain.ErrContentNotFound
	case err != nil:
		return Opened{}, err
	}
	return Opened{File: f, Blob: b, Name: n.Name, Left: time.Unix(a.Expires, 0).Sub(now)}, nil
}
