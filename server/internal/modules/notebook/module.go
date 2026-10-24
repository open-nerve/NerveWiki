// Package notebook is the module of notebooks and their members (v0.1
// design 3.3; M3 design). Its root is what bootstrap sees: New for the HTTP
// side; NewFacts for the access module's facts; NewWorkspaceDeletion and
// NewWorkspaceMemberEvents, its part in the workspace module's deletion,
// addition and role change; Purgers for the purge; Actions for the
// composition's checks; Tools, the module's MCP tools.
package notebook

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http"
	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/mcp"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Clock tells the time.
type Clock = app.Clock

// Workspaces is what the module reads of workspaces: bootstrap hands
// workspace.NewWorkspaces to it.
type Workspaces = app.Workspaces

// WorkspaceMembers is what the module reads of a workspace's memberships:
// bootstrap hands workspace.NewMemberships to it.
type WorkspaceMembers = app.WorkspaceMembers

// AccountWorkspaces are the workspaces an account is a member of, which
// MCP's notebooks lists the notebooks of: bootstrap converts workspace's
// Memberships.
type (
	AccountWorkspaces = app.AccountWorkspaces
	AccountWorkspace  = app.AccountWorkspace
)

// Profile is what the module reads of an account.
type Profile = app.Profile

// MemberProfiles reads the accounts' profiles: bootstrap converts
// identity's directory.
type MemberProfiles = app.MemberProfiles

// The module's extension points (M3 design 8; M10 design 8): bootstrap
// composes their registrants in registrants.go, the creation's seeders in
// wire.go.
type (
	NotebookCreation           = app.NotebookCreation
	Seeder                     = app.Seeder
	NotebookDeletion           = app.NotebookDeletion
	NotebookDeletionSubscriber = app.NotebookDeletionSubscriber
	VisibilityChange           = app.VisibilityChange
	VisibilitySubscriber       = app.VisibilitySubscriber
	NotebookActivity           = app.NotebookActivity
	NotebookActivitySource     = app.NotebookActivitySource
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool             *pgxpool.Pool
	Tx               shared.TxManager
	Clock            Clock
	Logger           *slog.Logger
	Authorizer       shared.Authorizer
	Workspaces       Workspaces
	WorkspaceMembers WorkspaceMembers
	Profiles         MemberProfiles
	// AccountWorkspaces are the caller's workspaces of MCP's notebooks: it
	// must be there.
	AccountWorkspaces AccountWorkspaces
	// Seeders are the creation's registrants (Seeder): New panics without
	// one, a fault of its wiring, as without AccountWorkspaces.
	Seeders []Seeder
	// The registrants of the extension points.
	DeletionSubscribers   []NotebookDeletionSubscriber
	VisibilitySubscribers []VisibilitySubscriber
	ActivitySources       []NotebookActivitySource
}

// Module is the wired notebook module.
type Module struct {
	uc        httpadapter.UseCases
	notebooks *app.ListAccountNotebooks // MCP's notebooks (Tools)
}

// New wires the module. A module without the account's workspaces, or
// without a seeder, is a fault of its wiring.
func New(d Deps) *Module {
	switch {
	case d.AccountWorkspaces == nil:
		panic("notebook: no account workspaces")
	case len(d.Seeders) == 0:
		panic("notebook: no seeders")
	}
	store := postgresadapter.New(d.Pool)
	auth := d.Authorizer
	visibility := d.VisibilitySubscribers
	return &Module{uc: httpadapter.UseCases{
		ListNotebooks: app.NewListNotebooks(d.Workspaces, store, auth),
		CreateNotebook: app.NewCreateNotebook(app.CreateNotebookDeps{
			Workspaces: d.Workspaces, Notebooks: store, Seeders: d.Seeders, Subscribers: visibility, Auth: auth, Tx: d.Tx, Clock: d.Clock,
			Logger: d.Logger,
		}),
		GetNotebook: app.NewGetNotebook(store, auth),
		UpdateNotebook: app.NewUpdateNotebook(app.UpdateNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Subscribers: visibility,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		DeleteNotebook: app.NewDeleteNotebook(app.DeleteNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Subscribers: d.DeletionSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListMembers: app.NewListMembers(app.ListMembersDeps{Notebooks: store, Members: store, Profiles: d.Profiles, Auth: auth}),
		AddMember: app.NewAddMember(app.AddMemberDeps{
			Workspaces: d.Workspaces, WorkspaceMembers: d.WorkspaceMembers, Finder: store, Notebooks: store, Writer: store,
			Profiles: d.Profiles, Subscribers: visibility, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		UpdateMember: app.NewUpdateMember(app.UpdateMemberDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Members: store, Writer: store, Profiles: d.Profiles,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		RemoveMember: app.NewRemoveMember(app.RemoveMemberDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Members: store, Writer: store, Subscribers: visibility,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		LeaveNotebook: app.NewLeaveNotebook(app.LeaveNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Writer: store, Subscribers: visibility,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListOwnerless: app.NewListOwnerlessNotebooks(app.ListOwnerlessNotebooksDeps{
			Workspaces: d.Workspaces, Notebooks: store, Profiles: d.Profiles, Activities: d.ActivitySources, Auth: auth,
		}),
		TakeOver: app.NewTakeOverNotebook(app.TakeOverNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Writer: store, Ownerless: store, Audit: store,
			Subscribers: visibility, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		DeleteOwnerless: app.NewDeleteOwnerlessNotebook(app.DeleteOwnerlessNotebookDeps{
			Workspaces: d.Workspaces, Finder: store, Notebooks: store, Audit: store, Subscribers: d.DeletionSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListAuditEvents: app.NewListNotebookAuditEvents(app.ListNotebookAuditEventsDeps{
			Workspaces: d.Workspaces, Audit: store, Profiles: d.Profiles, Auth: auth,
		}),
	}, notebooks: app.NewListAccountNotebooks(d.AccountWorkspaces, store)}
}

// Tools are the module's MCP tools: notebooks (M9 design 4.6).
func (m *Module) Tools() []mcpserver.Tool {
	return mcpadapter.Tools(m.notebooks)
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
