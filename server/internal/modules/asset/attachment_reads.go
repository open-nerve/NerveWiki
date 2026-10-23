package asset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// ErrNotFound is asset.not_found: an attachment that does not exist, is
// deleted, or whose notebook the caller cannot read in.
var ErrNotFound = domain.ErrNotFound

// AttachmentReads reads an attachment for page's MCP read (M9 design 4.1,
// 4.9): decided on asset.read as GET /api/v0/assets/{node_id} decides it.
// Its bytes come through the module's files, not the public download's
// route: no asset_content bucket, no signed address in between.
type AttachmentReads struct {
	reads     *app.Reads
	blobs     *app.Blobs
	sniffer   app.Sniffer
	publicURL string
	logger    *slog.Logger
}

// AttachmentReads are the module's: bootstrap adapts them to page's
// AttachmentReader.
func (m *Module) AttachmentReads() AttachmentReads {
	return AttachmentReads{reads: m.reads, blobs: m.blobs, sniffer: m.sniffer, publicURL: m.publicURL, logger: m.logger}
}

// Attachment is an attachment as read tells it: its node and file, its
// name, type and how a link writes it ("" without an extension), its size
// and SHA-256, its row's sides (0 when the server read none), and its
// content's download address signed, absolute, with when it expires; ""
// and the zero time without server.public_url.
type Attachment struct {
	NodeID, BlobID   uuid.UUID
	Name, MIME, Link string
	Bytes            int64
	SHA256           []byte
	Width, Height    int
	URL              string
	Expires          time.Time
}

// Read reads the attachment nodeID: asset.not_found for none and for one
// the caller cannot read; its row, its link's writing, and its content's
// address signed now, absolute: server.public_url and the content route's
// path and query, the download's (d=1), whose answer names the file; ""
// without public_url.
func (a AttachmentReads) Read(ctx context.Context, nodeID uuid.UUID) (Attachment, error) {
	got, err := a.reads.Get(ctx, nodeID)
	if err != nil {
		return Attachment{}, err
	}
	n, b := got.Node, got.Blob
	out := Attachment{NodeID: n.ID, BlobID: b.ID, Name: n.Name, MIME: b.MIME, Link: got.Link, Bytes: b.Bytes, SHA256: b.SHA256,
		Width: b.Width, Height: b.Height}
	if a.publicURL != "" {
		out.URL, out.Expires = a.publicURL+httpadapter.ContentURL(n.ID, b.ID, got.Signed, true), got.Signed.Expires
	}
	return out, nil
}

// Content reads the file of at, at most max bytes, a larger one an error:
// ErrNoFile, logged at warn as a download logs it, when the store has none;
// asset.not_found when its row went, or holds another file, since Read; an
// error when its size is not its row's. It judges no access: Read did. The
// sides are the row's or, for an image whose row has none (a WebP uploaded
// before M9/P1), read from data's header, as an upload keeps them; 0 and 0
// when neither knows them.
func (a AttachmentReads) Content(ctx context.Context, at Attachment, max int64) (data []byte, width, height int, err error) {
	f, _, err := a.blobs.Open(ctx, at.NodeID, at.BlobID)
	switch {
	case errors.Is(err, app.ErrNoFile):
		a.logger.WarnContext(ctx, "asset file missing", slog.String("blob_id", at.BlobID.String()), slog.String("node_id", at.NodeID.String()))
		return nil, 0, 0, ErrNoFile
	case err != nil:
		return nil, 0, 0, err
	}
	defer func() { _ = f.Close() }()
	data, err = io.ReadAll(io.LimitReader(f, max+1))
	switch {
	case err != nil:
		return nil, 0, 0, fmt.Errorf("asset: read the file of blob %s: %w", at.BlobID, err)
	case int64(len(data)) > max:
		return nil, 0, 0, fmt.Errorf("asset: the file of blob %s is larger than %d bytes", at.BlobID, max)
	case int64(len(data)) != at.Bytes:
		return nil, 0, 0, fmt.Errorf("asset: the file of blob %s read %d bytes, its row has %d", at.BlobID, len(data), at.Bytes)
	}
	width, height = at.Width, at.Height
	if width == 0 && domain.HasSize(at.MIME) {
		width, height = app.ImageSize(a.sniffer, bytes.NewReader(data))
	}
	return data, width, height, nil
}
