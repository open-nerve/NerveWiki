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

// MemberProfiles reads accounts' profiles for the member list: bootstrap
// adapts identity.NewProfiles to it.
type (
	MemberProfiles = app.MemberProfiles
	Profile        = app.Profile
)

// The module's extension points (M2 design 8): bootstrap composes their
// registrants in registrants.go.
type (
	MembershipEnd               = app.MembershipEnd
	EndCause                    = app.EndCause
	MembershipEndVetoer         = app.MembershipEndVetoer
	MembershipEndSubscriber     = app.MembershipEndSubscriber
	WorkspaceDeletion           = app.WorkspaceDeletion
	WorkspaceDeletionSubscriber = app.WorkspaceDeletionSubscriber
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Accounts   Accounts
	Profiles   MemberProfiles
	// CreationEnabled is workspace.creation_enabled.
	CreationEnabled bool
	// The registrants of the extension points.
	MembershipEndVetoers     []MembershipEndVetoer
	MembershipEndSubscribers []MembershipEndSubscriber
	DeletionSubscribers      []WorkspaceDeletionSubscriber
}

// Module is the wired workspace module.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the module.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	auth := d.Authorizer
	ender := app.MembershipEnder{Members: store, Vetoers: d.MembershipEndVetoers, Subscribers: d.MembershipEndSubscribers}
	return &Module{uc: httpadapter.UseCases{
		ListWorkspaces: app.NewListWorkspaces(store),
		CreateWorkspace: app.NewCreateWorkspace(app.CreateWorkspaceDeps{
			Workspaces: store, Accounts: d.Accounts, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger, CreationEnabled: d.CreationEnabled,
		}),
		GetWorkspace: app.NewGetWorkspace(store, auth),
		CheckSlug:    app.NewCheckSlug(store),
		UpdateWorkspace: app.NewUpdateWorkspace(app.UpdateWorkspaceDeps{
			Locker: store, Workspaces: store, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		DeleteWorkspace: app.NewDeleteWorkspace(app.DeleteWorkspaceDeps{
			Locker: store, Workspaces: store, Members: store, Subscribers: d.DeletionSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListMembers: app.NewListMembers(app.ListMembersDeps{Workspaces: store, Members: store, Profiles: d.Profiles, Auth: auth}),
		UpdateMember: app.NewUpdateMember(app.UpdateMemberDeps{
			Locker: store, Finder: store, Members: store, Profiles: d.Profiles, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		RemoveMember: app.NewRemoveMember(app.RemoveMemberDeps{
			Locker: store, Finder: store, Ender: ender, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		LeaveWorkspace: app.NewLeaveWorkspace(app.LeaveWorkspaceDeps{
			Locker: store, Finder: store, Ender: ender, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
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
