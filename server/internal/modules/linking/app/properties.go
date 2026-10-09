package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPageProperties reads a page's properties and their links from the
// index: GET /api/v0/pages/{page_id}/properties (M6/P5 design 4), a link
// to an attachment with its content's address (M7/P3 design 5.6) and when
// the earliest of them expires (M7/P4 design 4.3).
type GetPageProperties struct {
	Access Access
	Reads  Reads
	// Assets gives the attachments' addresses; nil, none has one.
	Assets AttachmentAddresses
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
	seen := map[uuid.UUID]bool{}
	for _, l := range p.Links {
		if l.Asset && !seen[l.NodeID] {
			seen[l.NodeID] = true
			ids = append(ids, l.NodeID)
		}
	}
	if g.Assets == nil || len(ids) == 0 {
		return p, nil
	}
	addresses, err := g.Assets.Addresses(ctx, notebookID, ids)
	if err != nil {
		return Properties{}, err
	}
	for i, l := range p.Links {
		a, ok := addresses[l.NodeID]
		if !ok {
			continue
		}
		p.Links[i].URL, p.Links[i].Inline = a.URL, a.Inline
		if !a.Expires.IsZero() && (p.AssetsExpire.IsZero() || a.Expires.Before(p.AssetsExpire)) {
			p.AssetsExpire = a.Expires
		}
	}
	return p, nil
}
