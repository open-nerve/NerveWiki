package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki"
)

// The LLM Wiki's template (M10 design 4.4): the notebook module's creation
// calls the wiki module's seeder, which writes through the page module's
// template writes. The modules do not import each other, so their values
// meet here.

// wikiSeeds are the page module's template writes as the wiki module's
// template calls them: the two modules do not import each other, so their
// values, alike field by field, meet here.
type wikiSeeds struct {
	tree page.TreeWrites
}

func (s wikiSeeds) Seed(ctx context.Context, x wiki.Seed) error {
	pages := make([]page.PageDraft, len(x.Pages))
	for i, p := range x.Pages {
		pages[i] = page.PageDraft{ID: p.ID, Title: p.Title, Content: p.Content, Facts: p.Parsed.Facts}
	}
	return s.tree.Seed(ctx, page.SeedSpec{NotebookID: x.NotebookID, At: x.At, Client: page.Client(x.Client),
		Message: &x.Message, Pages: pages})
}

func (s wikiSeeds) Parse(content string) (wiki.Parsed, error) {
	facts, err := s.tree.ParseTemplate(content)
	return wiki.Parsed{Facts: facts}, err
}

// wikiTemplate is the wiki module's template as the notebook module's
// seeder: a notebook.NotebookCreation converted, the mode and language
// by their values.
type wikiTemplate struct {
	template *wiki.Template
}

func (t wikiTemplate) NotebookCreated(ctx context.Context, c notebook.NotebookCreation) (uuid.UUID, error) {
	return t.template.NotebookCreated(ctx, wiki.Creation{NotebookID: c.NotebookID, Mode: wiki.Mode(c.Mode),
		SourceRootID: c.SourceRootID, Locale: wiki.Locale(c.Locale), Client: c.Client, At: c.At})
}
