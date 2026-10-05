// Package linking is the module of the link index (v0.1 design 4.4; M6
// design 4): each page's links, tags, properties and aliases, and where
// each link resolves to, kept with every write of the pages. Its root is
// what bootstrap sees: NewIndex, the page module's observer;
// NewNotebookDeletion, its part in the notebook module's deletion;
// PageFacts, which reads a page's facts from the Markdown's; ResolveLinks,
// where a reading view's links lead; NewAdmin, the rebuild of the indexes
// (nervewiki reindex).
package linking

import (
	"github.com/jackc/pgx/v5/pgxpool"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// What the module reads of the pages, and the page module's changes as it
// follows them: bootstrap converts the page module's values, field by
// field.
type (
	// Pages is what the index reads of a notebook's pages: bootstrap hands
	// page.NewLinkTargets to it.
	Pages = app.Pages
	// Node is a page with its path from the root, itself last.
	Node = domain.Node
	// Step is a page on a path: its id, title key and name.
	Step = domain.Step
	// PagesChanged is a page write unit's changes.
	PagesChanged = app.PagesChanged
	// Change is what a unit did to a node.
	Change = domain.Change
	// Place is where a node is in the tree.
	Place = domain.Place
	// Facts is what the index keeps of a page's content.
	Facts = domain.Facts
	// NotebooksDeleted is notebooks being deleted.
	NotebooksDeleted = app.NotebooksDeleted
)

// LinksChanged is the links event of a unit: bootstrap publishes it on
// the event stream through Publisher.
type LinksChanged = app.LinksChanged

// Publisher publishes the links events.
type Publisher = app.Publisher

// Index is the page module's observer that keeps the index.
type Index = app.Index

// NewIndex returns the index's observer, from the pool, the page module's
// reads and the publisher alone (it parses nothing): the command line's
// compositions build it too.
func NewIndex(pool *pgxpool.Pool, pages Pages, publisher Publisher) Index {
	return app.Index{Store: postgresadapter.New(pool), Pages: pages, Publisher: publisher}
}

// NotebookDeletion is the module's registrant of the notebook module's
// deletion: it drops the notebooks' index.
type NotebookDeletion = app.NotebookDeletion

// NewNotebookDeletion returns the registrant over the pool alone.
func NewNotebookDeletion(pool *pgxpool.Pool) NotebookDeletion {
	return app.NotebookDeletion{Store: postgresadapter.New(pool)}
}

// Extractor is the version of what the index keeps of a page
// (indexed_pages.extractor): a release that changes it asks for nervewiki
// reindex.
const Extractor = domain.Extractor

// PageFacts is what the index keeps of facts, the platform's Markdown
// facts of a page's content that the page module's changes carry; facts
// of another kind, or without the obsidian extension's, are an error.
func PageFacts(facts any) (Facts, error) {
	return markdownadapter.PageFacts(facts)
}

// ResolveLinks is where a page's links lead for its reading view, the
// obsidian extension's Resolve: from the index when it is of the content
// rendered, else anew, over the pool and the page module's reads (M6/P3
// design 6.5). Not New: the command line's compositions reach it too.
func ResolveLinks(pool *pgxpool.Pool, pages Pages) obsidian.Resolve {
	return markdownadapter.Resolve(app.Views{Store: postgresadapter.New(pool), Pages: pages})
}
