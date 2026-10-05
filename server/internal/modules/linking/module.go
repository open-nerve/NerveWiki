// Package linking is the module of the link index (v0.1 design 4.4; M6
// design 4): each page's links, tags, properties and aliases, and where
// each link resolves to, kept with every write of the pages. Its root is
// what bootstrap sees: New for the HTTP side, the index's reads (M6/P5);
// NewIndex, the page module's observer; NewRewrite, its participant;
// NewNotebookDeletion, its part in the notebook module's deletion;
// PageFacts, which reads a page's facts from the Markdown's; ResolveLinks,
// where a reading view's links lead; NewAdmin, the rebuild of the indexes
// (nervewiki reindex); Actions for the composition's checks.
package linking

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/http"
	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// What the index's reads read of the other modules: bootstrap wires the
// notebook module's notebooks and the page module's tree.
type (
	// NotebookWorkspaces reads the notebooks' workspaces: bootstrap hands
	// notebook.NewNotebooks to it.
	NotebookWorkspaces = app.NotebookWorkspaces
	// PageTree reads a notebook's tree, whole or by keys and ids, and a
	// page's notebook: bootstrap hands page.NewLinkTargets to it.
	PageTree = app.PageTree
)

// Deps are what the HTTP side, the index's reads, needs.
type Deps struct {
	Pool       *pgxpool.Pool
	Authorizer shared.Authorizer
	Notebooks  NotebookWorkspaces
	Pages      PageTree
	Contents   PageContents
	// MaxDepth is how deep pages nest: bootstrap hands page.MaxDepth to it.
	MaxDepth int
}

// Module is the wired linking module's HTTP side.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the index's reads (M6/P5) and a link's landing (M6/P6): they
// read on the pool, each in its own statements, and parse nothing.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	access := app.Access{Notebooks: d.Notebooks, Pages: d.Pages, Auth: d.Authorizer}
	return &Module{uc: httpadapter.UseCases{
		ListBacklinks:     app.ListBacklinks{Access: access, Reads: store, Contents: d.Contents},
		GetPageProperties: app.GetPageProperties{Access: access, Reads: store},
		ListTags:          app.ListTags{Access: access, Reads: store},
		GetTag:            app.GetTag{Access: access, Reads: store, TagKey: markdownadapter.TagKey},
		ListLinkTargets:   app.ListLinkTargets{Access: access, Reads: store},
		GetLinkLanding:    app.GetLinkLanding{Access: access, Pages: d.Pages, Reads: store, MaxDepth: d.MaxDepth},
	}}
}

// Register mounts the module's routes.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc)
}

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}

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

// What a rewrite of links follows, what it reads and how it writes:
// bootstrap converts the page module's step, adapts its appender and wires
// its reads.
type (
	// Moved is an operation of a page write unit.
	Moved = app.Moved
	// Appender adds a content write to the unit.
	Appender = app.Appender
	// Rewritten is a content a rewrite writes.
	Rewritten = app.Rewritten
	// Locks reads the edit locks of pages: bootstrap hands
	// page.NewLockHolders to it.
	Locks = app.Locks
	// PageContents reads a page's content: bootstrap hands
	// page.NewLinkTargets to it.
	PageContents = app.PageContents
)

// ErrGuardLocked is a write a rewrite adds that the page's edit lock
// refuses: the appender wraps the lock's error with it.
var ErrGuardLocked = app.ErrGuardLocked

// Rewrite is the page module's participant that writes again the links a
// rename or a move would lead elsewhere (M6/P4).
type Rewrite = app.Rewrite

// NewRewrite returns the participant over the pool, the page module's
// reads of the pages, their contents and their edit locks, and its most
// bytes of a page's content, parsing with the server's Markdown within its
// budget.
func NewRewrite(pool *pgxpool.Pool, pages Pages, contents PageContents, locks Locks, maxContent int,
	md *markdown.Markdown, budget *markdown.Budget, logger *slog.Logger,
) Rewrite {
	return app.Rewrite{
		Store: postgresadapter.New(pool), Pages: pages, Contents: contents, Locks: locks, MaxContent: maxContent,
		Parser: markdownadapter.NewParser(md, budget), Logger: logger,
	}
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
