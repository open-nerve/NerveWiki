package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// GetPageView reads a page's reading view: GET /api/v0/pages/{page_id}/view
// (M4/P3 design 3.9).
type GetPageView struct {
	notebooks Notebooks
	nodes     Nodes
	auth      shared.Authorizer
	markdown  Markdown
	budget    ParseBudget
}

// ReadingView is a page's content rendered, and the version it was
// rendered from.
type ReadingView struct {
	HTML     string
	Revision int
}

// NewGetPageView returns the use case.
func NewGetPageView(notebooks Notebooks, nodes Nodes, auth shared.Authorizer, markdown Markdown, budget ParseBudget) *GetPageView {
	return &GetPageView{notebooks: notebooks, nodes: nodes, auth: auth, markdown: markdown, budget: budget}
}

// Execute renders the page id, without a transaction or a lock: as GetPage
// reads it, then its content and version in one statement, parsed and
// rendered for this page, within the parse budget (M4/P4 review P2). The
// HTML is not cached: each read renders it (M4 design 4, "parse timing"),
// at a cost the size of the content bounds.
func (g *GetPageView) Execute(ctx context.Context, id uuid.UUID) (ReadingView, error) {
	n, err := readable(ctx, g.notebooks, g.nodes, g.auth, id)
	if err != nil {
		return ReadingView{}, err
	}
	c, err := g.nodes.PageContent(ctx, n.ID)
	if err != nil {
		return ReadingView{}, found(err, domain.ErrNotFound)
	}
	release, err := g.budget.Take(ctx, len(c.Content))
	if err != nil {
		return ReadingView{}, err
	}
	defer release()
	html, err := g.markdown.Render(ctx, g.markdown.Parse(c.Content), PageRef{NotebookID: n.NotebookID, PageID: n.ID})
	if err != nil {
		return ReadingView{}, err
	}
	return ReadingView{HTML: html, Revision: c.Revision}, nil
}
