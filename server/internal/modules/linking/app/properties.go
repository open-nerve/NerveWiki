package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPageProperties reads a page's properties and their links from the
// index: GET /api/v0/pages/{page_id}/properties (M6/P5 design 4).
type GetPageProperties struct {
	Access Access
	Reads  Reads
}

// Execute returns the properties of the page id: the page and the decision
// first. A page the index does not have, before nervewiki reindex, has a
// valid frontmatter and none. A read takes no lock and opens no
// transaction; the index's rows are read in one statement.
func (g GetPageProperties) Execute(ctx context.Context, id uuid.UUID) (Properties, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return Properties{}, err
	}
	if _, err := g.Access.page(ctx, actor, id, domain.ActionReadProperties); err != nil {
		return Properties{}, err
	}
	p, ok, err := g.Reads.Properties(ctx, id)
	if err != nil || !ok {
		return Properties{Valid: true}, err
	}
	return p, nil
}
