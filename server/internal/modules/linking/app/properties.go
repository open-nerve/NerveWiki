package app

import (
	"context"
	"slices"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPageProperties reads a page's properties and their links from the
// index: GET /api/v0/pages/{page_id}/properties (M6/P5 design 4), a link
// to an attachment with its content's address (M7/P3 design 5.6).
type GetPageProperties struct {
	Access Access
	Reads  Reads
	// Assets gives the attachments' addresses; nil, none has one.
	Assets AttachmentURLs
}

// Execute returns the properties of the page id: the page and the decision
// first. A page the index does not have, before nervewiki reindex, has a
// valid frontmatter and none. A read takes no lock and opens no
// transaction; the index's rows are read in one statement, then the
// addresses of the attachments their links lead to, in one call.
func (g GetPageProperties) Execute(ctx context.Context, id uuid.UUID) (Properties, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Properties{}, err
	}
	notebookID, err := g.Access.page(ctx, actor, id, domain.ActionReadProperties)
	if err != nil {
		return Properties{}, err
	}
	p, ok, err := g.Reads.Properties(ctx, id)
	if err != nil || !ok {
		return Properties{Valid: true}, err
	}
	var ids []uuid.UUID
	for _, l := range p.Links {
		if l.Asset && !slices.Contains(ids, l.NodeID) {
			ids = append(ids, l.NodeID)
		}
	}
	if g.Assets == nil || len(ids) == 0 {
		return p, nil
	}
	urls, err := g.Assets.URLs(ctx, notebookID, ids)
	if err != nil {
		return Properties{}, err
	}
	for i, l := range p.Links {
		if l.Asset {
			p.Links[i].URL = urls[l.NodeID]
		}
	}
	return p, nil
}
