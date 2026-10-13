package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// GetVersionView reads a version's reading view: GET
// /api/v0/pages/{page_id}/versions/{revision}/view (M8/P1 design 3.5).
type GetVersionView struct {
	r        VersionReads
	markdown Markdown
	budget   ParseBudget
}

// NewGetVersionView returns the use case.
func NewGetVersionView(r VersionReads, markdown Markdown, budget ParseBudget) *GetVersionView {
	return &GetVersionView{r: r, markdown: markdown, budget: budget}
}

// Execute renders the version of the page id holding revision, found as
// GetVersion finds it, for this page at the version's own revision, within
// the parse budget, as GetPageView renders the page's content: its links
// resolved on the tree as it is now.
func (g *GetVersionView) Execute(ctx context.Context, id uuid.UUID, revision int) (ReadingView, error) {
	n, err := readableAny(ctx, g.r.Notebooks, g.r.Nodes, g.r.Auth, id, domain.ActionHistory)
	if err != nil {
		return ReadingView{}, err
	}
	v, err := findVersion(ctx, g.r.Versions, n.ID, revision)
	if err != nil {
		return ReadingView{}, err
	}
	content, err := g.r.Versions.VersionContent(ctx, v.ID)
	if err != nil {
		return ReadingView{}, found(err, domain.ErrVersionNotFound)
	}
	hold, err := g.budget.Take(ctx, len(content))
	if err != nil {
		return ReadingView{}, err
	}
	defer hold.Release()
	r, err := g.markdown.Render(ctx, content, PageRef{NotebookID: n.NotebookID, PageID: n.ID, Revision: v.Revision})
	if err != nil {
		return ReadingView{}, err
	}
	return ReadingView{HTML: r.HTML, Revision: v.Revision, Expires: r.Expires}, nil
}
