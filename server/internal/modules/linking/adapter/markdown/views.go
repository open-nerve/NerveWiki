package markdownadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// Views is where a page's links resolve: app.Views.
type Views interface {
	Resolve(ctx context.Context, p app.Page, links []app.Link) (map[int]domain.Resolution, error)
}

// Resolve is the obsidian extension's Resolve over views (M6/P3 design
// 6.5): a page's links, their targets written as PageFacts writes them,
// so that what resolves anew resolves as the index has it. A link that
// resolves to an attachment is left out, as one that resolves to none:
// the extension would write it as a page's (M7/P3 design 4.6), until it
// takes an attachment's.
func Resolve(views Views) obsidian.Resolve {
	return func(ctx context.Context, page markdown.Page, links []obsidian.Link) (map[int]uuid.UUID, error) {
		asked := make([]app.Link, len(links))
		for i, l := range links {
			asked[i] = app.Link{SourceID: page.PageID, Start: l.Range.Start, Target: text(l.Target)}
		}
		p := app.Page{ID: page.PageID, NotebookID: page.NotebookID, Revision: page.Revision}
		rs, err := views.Resolve(ctx, p, asked)
		if err != nil {
			return nil, err
		}
		to := make(map[int]uuid.UUID, len(rs))
		for start, r := range rs {
			if r.ID != uuid.Nil() && !r.Asset {
				to[start] = r.ID
			}
		}
		return to, nil
	}
}
