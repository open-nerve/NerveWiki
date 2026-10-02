package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPageContent reads a page's content: GET
// /api/v0/pages/{page_id}/content (M4/P4 design 3.6).
type GetPageContent struct {
	notebooks Notebooks
	nodes     Nodes
	auth      shared.Authorizer
}

// NewGetPageContent returns the use case.
func NewGetPageContent(notebooks Notebooks, nodes Nodes, auth shared.Authorizer) *GetPageContent {
	return &GetPageContent{notebooks: notebooks, nodes: nodes, auth: auth}
}

// Execute reads the content of the page id, byte for byte as written, with
// its version and hash, without a transaction or a lock, as GetPage reads
// the page. A page that does not exist, is deleted, or whose notebook the
// caller has no role in is page.not_found.
func (g *GetPageContent) Execute(ctx context.Context, id uuid.UUID) (PageContent, error) {
	n, err := readable(ctx, g.notebooks, g.nodes, g.auth, id)
	if err != nil {
		return PageContent{}, err
	}
	c, err := g.nodes.PageContent(ctx, n.ID)
	if err != nil {
		return PageContent{}, found(err, domain.ErrNotFound)
	}
	return c, nil
}
