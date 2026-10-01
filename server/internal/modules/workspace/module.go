// Package workspace is the module of workspaces, their members and
// invitations (v0.1 design 3.2; M2 design). Its root is what bootstrap
// sees: New for the HTTP side; NewMemberships for the access module's
// facts; Actions and Reserved for the composition's checks.
package workspace

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Accounts is what identity offers the modules that give an account new
// access: bootstrap hands identity.NewAccounts to the module.
type Accounts = app.Accounts

// Clock tells the time.
type Clock = app.Clock

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Accounts   Accounts
	// CreationEnabled is workspace.creation_enabled.
	CreationEnabled bool
}

// Module is the wired workspace module.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the module.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	return &Module{uc: httpadapter.UseCases{
		ListWorkspaces: app.NewListWorkspaces(store),
		CreateWorkspace: app.NewCreateWorkspace(app.CreateWorkspaceDeps{
			Store: store, Accounts: d.Accounts, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger, CreationEnabled: d.CreationEnabled,
		}),
		GetWorkspace: app.NewGetWorkspace(store, d.Authorizer),
		CheckSlug:    app.NewCheckSlug(store),
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
