// Package notebook is the module of notebooks and their members (v0.1
// design 3.3; M3 design). Its root is what bootstrap sees: New for the HTTP
// side; NewFacts for the access module's facts; NewWorkspaceDeletion, its
// part in the workspace module's deletion; Purgers for the purge; Actions
// for the composition's checks.
package notebook

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Clock tells the time.
type Clock = app.Clock

// Workspaces is what the module reads of workspaces: bootstrap hands
// workspace.NewWorkspaces to it.
type Workspaces = app.Workspaces

// The module's extension point (M3 design 8): bootstrap composes its
// registrants in registrants.go.
type (
	NotebookDeletion           = app.NotebookDeletion
	NotebookDeletionSubscriber = app.NotebookDeletionSubscriber
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Workspaces Workspaces
	// The registrants of the extension point.
	DeletionSubscribers []NotebookDeletionSubscriber
}

// Module is the wired notebook module.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the module.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	auth := d.Authorizer
	return &Module{uc: httpadapter.UseCases{
		ListNotebooks: app.NewListNotebooks(d.Workspaces, store, auth),
		CreateNotebook: app.NewCreateNotebook(app.CreateNotebookDeps{
			Workspaces: d.Workspaces, Notebooks: store, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		GetNotebook: app.NewGetNotebook(store, auth),
		UpdateNotebook: app.NewUpdateNotebook(app.UpdateNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		DeleteNotebook: app.NewDeleteNotebook(app.DeleteNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Subscribers: d.DeletionSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
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
