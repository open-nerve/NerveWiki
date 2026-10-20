// Package linking is the module of the link index (v0.1 design 4.4; M6
// design 4): each page's links, tags, properties and aliases, and where
// each link resolves to, kept with every write of the pages. Its root is
// what bootstrap sees: New for the HTTP side, the index's reads (M6/P5);
// NewIndex, the page module's observer; NewRewrite, its participant;
// NewNotebookDeletion, its part in the notebook module's deletion;
// PageFacts, which reads a page's facts from the Markdown's; ResolveLinks,
// where a reading view's links lead; NewLinkedPages, which pages an
// export's links lead to; NewStatistics, the statistics an import updates;
// NewAdmin, the rebuild of the indexes (nervewiki reindex); Actions for
// the composition's checks.
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
	// AttachmentAddresses gives the attachments' contents' addresses:
	// bootstrap adapts the asset module's Embeds to it.
	AttachmentAddresses = app.AttachmentAddresses
	// AttachmentAddress is an attachment's content's address.
	AttachmentAddress = app.AttachmentAddress
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
	// Assets gives a property link to an attachment its content's address.
	Assets AttachmentAddresses
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
		GetPageProperties: app.GetPageProperties{Access: access, Reads: store, Assets: d.Assets},
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
	// Node is a page or an attachment with its path from the root, itself
	// last.
	Node = domain.Node
	// Step is a node on a path, a page but for an attachment's last: its
	// id, title key and name.
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
	// PageSizes reads the sizes of pages' contents: bootstrap hands
	// page.NewLinkTargets to it.
	PageSizes = app.PageSizes
)

// ErrGuardLocked is a write a rewrite adds that the page's edit lock
// refuses: the appender wraps the lock's error with it.
var ErrGuardLocked = app.ErrGuardLocked

// A batch's links (M9/P3 design 3.7): what its deferred rewrite follows
// besides its changes, and the rewrite of its plan.
type (
	// WrittenPage is a page a batch wrote, as its deferred rewrite follows
	// it.
	WrittenPage = app.WrittenPage
	// LinksBefore is where a page's links led at a moment of a batch's
	// plan.
	LinksBefore = app.LinksBefore
	// Resolution is where a link leads.
	Resolution = domain.Resolution
	// TooManyRewrites is a batch's deferred rewrite past its bound, with
	// the pages, links or bytes counted.
	TooManyRewrites = app.TooManyRewrites
	// BatchLinks resolves and rewrites links for a batch's plan.
	BatchLinks = app.BatchLinks
	// LinksOn is a content of a page of a batch's plan to resolve.
	LinksOn = app.LinksOn
	// LinksMoved is a content of a page of a batch's plan to rewrite.
	LinksMoved = app.LinksMoved
	// Link is a link of a page's content, as its facts hold it.
	Link = domain.Link
)

// ErrTooManyRewrites is a batch's deferred rewrite past Moved.MaxPages,
// Moved.MaxLinks or Moved.MaxBytes, which bootstrap answers as the page
// module's page.batch_too_large.
var ErrTooManyRewrites = app.ErrTooManyRewrites

// NewBatchLinks returns the plan's links over the pool alone (M9 design
// 4.1): the tree it resolves on is its caller's, the plan's, and its parse
// its caller's too; maxContent is the most bytes of a page's content.
func NewBatchLinks(pool *pgxpool.Pool, maxContent int, logger *slog.Logger) BatchLinks {
	return app.BatchLinks{Store: postgresadapter.New(pool), MaxContent: maxContent, Logger: logger}
}

// Rewrite is the page module's participant that writes again the links a
// rename or a move would lead elsewhere (M6/P4).
type Rewrite = app.Rewrite

// NewRewrite returns the participant over the pool, the page module's
// reads of the pages, their contents, their contents' sizes and their edit
// locks, and its most bytes of a page's content, parsing with the server's
// Markdown within its budget.
func NewRewrite(pool *pgxpool.Pool, pages Pages, contents PageContents, sizes PageSizes, locks Locks, maxContent int,
	md *markdown.Markdown, budget *markdown.Budget, logger *slog.Logger,
) Rewrite {
	return app.Rewrite{
		Store: postgresadapter.New(pool), Pages: pages, Contents: contents, Sizes: sizes, Locks: locks, MaxContent: maxContent,
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

// AssetLinks writes how a wikilink is written to lead to attachments alone
// (M7/P3 design 4.6): the asset module's links, which bootstrap wires.
type AssetLinks = app.AssetLinks

// Attachments is what AssetLinks reads of the nodes, the page module's.
type Attachments = app.Attachments

// Attachment is an attachment as Attachments reads it.
type Attachment = app.Attachment

// NewAssetLinks returns AssetLinks over the page module's reads, in the
// caller's transaction or on the pool.
func NewAssetLinks(attachments Attachments) AssetLinks {
	return app.AssetLinks{Attachments: attachments}
}

// ResolveLinks is where a page's links lead for its reading view, the
// obsidian extension's Resolve: from the index when it is of the content
// rendered, else anew, over the pool and the page module's reads (M6/P3
// design 6.5). Not New: the command line's compositions reach it too.
func ResolveLinks(pool *pgxpool.Pool, pages Pages) obsidian.Resolve {
	return markdownadapter.Resolve(app.Views{Store: postgresadapter.New(pool), Pages: pages})
}
