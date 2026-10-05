package linking

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// What the rebuild reads of the other modules, and what it tells
// (nervewiki reindex, M6/P3 design 3.6).
type (
	// Tx runs the rebuild's transactions: bootstrap hands
	// postgres.NewTxManager to it.
	Tx = app.Tx
	// Contents is what the rebuild reads and repairs of the pages:
	// bootstrap hands page.NewLinkTargets to it.
	Contents = app.Contents
	// Clash is siblings whose names would share a title key.
	Clash = app.Clash
	// NamedNode is a node by its id and name.
	NamedNode = app.NamedNode
	// Notebooks is what the rebuild reads and locks of the notebooks:
	// bootstrap hands notebook.NewNotebooks to it.
	Notebooks = app.Notebooks
	// Rebuilt is a notebook's rebuild.
	Rebuilt = app.Rebuilt
)

// ErrNoNotebook is a rebuild of a notebook that is not there, or is
// deleted.
var ErrNoNotebook = app.ErrNoNotebook

// AdminDeps are what the command line gives the rebuild.
type AdminDeps struct {
	Pool      *pgxpool.Pool
	Tx        Tx
	Pages     Pages
	Contents  Contents
	Notebooks Notebooks
	Publisher Publisher
	// Markdown and Budget are the one parse of Markdown, with the
	// registered extensions, and its budget, as serve's.
	Markdown *markdown.Markdown
	Budget   *markdown.Budget
}

// Admin rebuilds the notebooks' indexes.
type Admin struct {
	rebuild app.Rebuild
}

// NewAdmin returns the rebuild of the indexes.
func NewAdmin(d AdminDeps) Admin {
	return Admin{rebuild: app.Rebuild{
		Tx:        d.Tx,
		Index:     app.Index{Store: postgresadapter.New(d.Pool), Pages: d.Pages, Publisher: d.Publisher},
		Contents:  d.Contents,
		Notebooks: d.Notebooks,
		Parser:    markdownadapter.NewParser(d.Markdown, d.Budget),
	}}
}

// Rebuild rebuilds the index of the notebook id from its pages, in one
// transaction: ErrNoNotebook for a notebook that is not there; nothing,
// with the clashes, when siblings' title keys would clash.
func (a Admin) Rebuild(ctx context.Context, id uuid.UUID) (Rebuilt, error) {
	return a.rebuild.Notebook(ctx, id)
}
