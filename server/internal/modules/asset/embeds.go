package asset

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/mac"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/asset/app"
)

// Embeds is what a reading view and a page's properties show of the
// attachments their links lead to (M7/P3 design 5.4, 5.6), built on the
// pool and the content key alone: bootstrap builds the Markdown, whose
// rendering asks it, before the modules.
type Embeds struct {
	embeds *app.Embeds
}

// Embed is what a reading view shows of an attachment: its type and size,
// an image's width and height in pixels (0 when not known), its content's
// address, signed, shown inline, and when that expires.
type Embed struct {
	MIME          string
	Bytes         int64
	Width, Height int
	URL           string
	Expires       time.Time
}

// NewEmbeds returns Embeds over pool, its addresses signed with contentKey
// (ContentKeyInfo's derivation) as of clock's time.
func NewEmbeds(pool *pgxpool.Pool, contentKey []byte, clock Clock) Embeds {
	return Embeds{embeds: app.NewEmbeds(postgresadapter.New(pool), macadapter.New(contentKey), clock)}
}

// Of is each of ids that is an attachment of notebookID not deleted, by
// id; one it is not is left out. It decides no access: ids are those the
// links of a page of notebookID resolve to, for a reader of the page.
func (e Embeds) Of(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Embed, error) {
	embedded, err := e.embeds.Of(ctx, notebookID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]Embed, len(embedded))
	for id, x := range embedded {
		b := x.Blob
		out[id] = Embed{
			MIME: b.MIME, Bytes: b.Bytes, Width: b.Width, Height: b.Height,
			URL: httpadapter.ContentURL(id, b.ID, x.Signed, false), Expires: x.Signed.Expires,
		}
	}
	return out, nil
}
