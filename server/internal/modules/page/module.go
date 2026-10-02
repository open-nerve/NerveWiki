// Package page is the module of a notebook's pages: the tree of nodes,
// their content, the changesets and versions of their writes (v0.1 design
// 3.5, 3.6, 3.8; M4 design). Its root is what bootstrap sees: New for the
// HTTP side; NewNotebookDeletion, its part in the notebook module's
// deletion; Purgers for the purge; Actions for the composition's checks.
package page

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http"
	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Clock tells the time.
type Clock = app.Clock

// Tx runs the write units' transactions: bootstrap hands
// postgres.NewTxManager to it.
type Tx = app.Tx

// Workspaces is what the module reads of workspaces: bootstrap hands
// workspace.NewWorkspaces to it.
type Workspaces = app.Workspaces

// Notebooks is what the module reads of notebooks: bootstrap hands
// notebook.NewNotebooks to it.
type Notebooks = app.Notebooks

// The module's extension points (M4 design 8): bootstrap composes their
// registrants in registrants.go.
type (
	Write        = app.Write
	Options      = app.Options
	Step         = app.Step
	WriteGuard   = app.WriteGuard
	Participant  = app.Participant
	Appender     = app.Appender
	Event        = app.Event
	PageObserver = app.PageObserver
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         Tx
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Workspaces Workspaces
	Notebooks  Notebooks
	// Markdown is the one parse and rendering of Markdown, with the
	// registered extensions (M4 design 8).
	Markdown *markdown.Markdown
	// The registrants of the extension points.
	Guards       []WriteGuard
	Participants []Participant
	Observers    []PageObserver
}

// Module is the wired page module.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the module: every write runs in the one writer's units.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	writer := app.NewWriter(app.WriterDeps{
		Tx: d.Tx, Clock: d.Clock, Auth: d.Authorizer, Workspaces: d.Workspaces, Notebooks: d.Notebooks,
		Nodes: store, NodeWriter: store, Changesets: store,
		Guards: d.Guards, Participants: d.Participants, Observers: d.Observers,
	})
	return &Module{uc: httpadapter.UseCases{
		ListNodes:   app.NewListNodes(d.Notebooks, store, d.Authorizer),
		CreatePage:  app.NewCreatePage(writer, store, d.Logger),
		GetPage:     app.NewGetPage(d.Notebooks, store, d.Authorizer),
		GetPageView: app.NewGetPageView(d.Notebooks, store, d.Authorizer, markdownadapter.New(d.Markdown)),
		RenameNode:  app.NewRenameNode(writer, store, d.Logger),
		MoveNode:    app.NewMoveNode(writer, store, d.Logger),
		DeleteNode:  app.NewDeleteNode(writer, store, d.Logger),
	}}
}

// Register mounts the module's API on router, the root router from
// httpserver.NewRouter, behind api's per-route middlewares.
func (m *Module) Register(router *httpserver.Router, api *httpserver.API) {
	httpadapter.Register(router, api, m.uc)
}

// Actions are the module's actions: bootstrap checks they are the access
// module's rule table.
func Actions() []shared.Action {
	return domain.Actions()
}
