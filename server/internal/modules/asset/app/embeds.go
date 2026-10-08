package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/domain"
)

// Embeds reads what a reading view shows of attachments (M7/P3 design 5.4):
// their files' rows and their contents' addresses, signed. It decides no
// access: the ids are those a page's links resolve to, within the page's
// notebook, for a reader of the page, who may read its attachments
// (asset.read is a reader's, as page.read is).
type Embeds struct {
	rows   Rows
	signer Signer
	clock  Clock
}

// Embedded is an attachment's row and its content's address, signed.
type Embedded struct {
	Blob   domain.Blob
	Signed Signed
}

// NewEmbeds returns the use case.
func NewEmbeds(rows Rows, signer Signer, clock Clock) *Embeds {
	return &Embeds{rows: rows, signer: signer, clock: clock}
}

// Of is each of ids that is an attachment of notebookID not deleted, by
// id: its row, which an attachment not deleted has, of its notebook, read
// in one statement, unlocked; the addresses signed as of one time. One
// without a row is left out.
func (e *Embeds) Of(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Embedded, error) {
	blobs, err := e.rows.BlobsOfNodes(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := e.clock.Now()
	out := make(map[uuid.UUID]Embedded, len(blobs))
	for id, b := range blobs {
		if b.NotebookID == notebookID {
			out[id] = Embedded{Blob: b, Signed: e.signer.Sign(now, id, b.ID)}
		}
	}
	return out, nil
}
