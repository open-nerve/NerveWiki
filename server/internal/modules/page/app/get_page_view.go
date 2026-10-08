package app

import (
	"context"
	"time"
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

// ReadingView is a page's content rendered, the version it was rendered
// from, and when the earliest of the attachments' addresses in it expires,
// zero for none (M7/P3 design 5.7).
type ReadingView struct {
	HTML     string
	Revision int
	Expires  time.Time
}

// NewGetPageView returns the use case.
func NewGetPageView(notebooks Notebooks, nodes Nodes, auth shared.Authorizer, markdown Markdown, budget ParseBudget) *GetPageView {
	return &GetPageView{notebooks: notebooks, nodes: nodes, auth: auth, markdown: markdown, budget: budget}
}

// Execute renders the page id, without a transaction or a lock: as GetPage
// reads it, then its content and version in one statement, parsed and
// rendered for this page at that version, within the parse budget (M4/P4
// review P2). The HTML is not cached: each read renders it (M4 design 4,
// "parse timing"), at a cost the size of the content bounds.
func (g *GetPageView) Execute(ctx context.Context, id uuid.UUID) (ReadingView, error) {
	n, err := readable(ctx, g.notebooks, g.nodes, g.auth, id)
	if err != nil {
		return ReadingView{}, err
	}
	c, err := g.nodes.PageContent(ctx, n.ID)
	if err != nil {
		return ReadingView{}, found(err, domain.ErrNotFound)
	}
	hold, err := g.budget.Take(ctx, len(c.Content))
	if err != nil {
		return ReadingView{}, err
	}
	defer hold.Release()
	r, err := g.markdown.Render(ctx, c.Content, PageRef{NotebookID: n.NotebookID, PageID: n.ID, Revision: c.Revision})
	if err != nil {
		return ReadingView{}, err
	}
	return ReadingView{HTML: r.HTML, Revision: c.Revision, Expires: r.Expires}, nil
}
