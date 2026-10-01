// Package workspace is the module of workspaces, their members and
// invitations (v0.1 design 3.2; M2 design). Its root is what bootstrap
// sees: New for the HTTP side; NewMemberships for the access module's
// facts; NewWorkspaces for the modules inside a workspace;
// NewInvitationCheck for identity's sign-up policy, and InvitationKeyInfo,
// the info of the key it derives for the invitations; NewDeactivation, its
// part in identity's deactivation; NewAdmin for the command line; Purgers
// for the purge; Actions and Reserved for the composition's checks.
package workspace

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/mac"
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

// Directory is identity's unlocked reads of accounts, as the module reads
// them: the profiles of the member list, which tell a member's address too,
// and the account of an address invited. bootstrap adapts
// identity.NewDirectory to it.
type Directory interface {
	app.MemberProfiles
	app.AccountFinder
}

// Profile is what the module reads of an account.
type Profile = app.Profile

// The module's extension points (M2 design 8; the addition and the role
// change, M3 design 8): bootstrap composes their registrants in
// registrants.go.
type (
	MembershipEnd                = app.MembershipEnd
	EndCause                     = app.EndCause
	MembershipEndVetoer          = app.MembershipEndVetoer
	MembershipEndSubscriber      = app.MembershipEndSubscriber
	WorkspaceDeletion            = app.WorkspaceDeletion
	WorkspaceDeletionSubscriber  = app.WorkspaceDeletionSubscriber
	MembershipRestore            = app.MembershipRestore
	MembershipRestoreSubscriber  = app.MembershipRestoreSubscriber
	MembershipAddition           = app.MembershipAddition
	MembershipAdditionSubscriber = app.MembershipAdditionSubscriber
	MemberRoleChange             = app.MemberRoleChange
	MemberRoleChangeSubscriber   = app.MemberRoleChangeSubscriber
)

// The causes of a membership end: the notebook module's rule two refuses
// only what the account does itself (M3 design 4), which bootstrap tells
// by them.
const (
	EndRemoved     = app.EndRemoved
	EndLeft        = app.EndLeft
	EndDeactivated = app.EndDeactivated
)

// Deps are what bootstrap gives the module.
type Deps struct {
	Pool       *pgxpool.Pool
	Tx         shared.TxManager
	Clock      Clock
	Logger     *slog.Logger
	Authorizer shared.Authorizer
	Accounts   Accounts
	Directory  Directory
	// InvitationKey is the invitations' MAC key, derived from the signing
	// key with InvitationKeyInfo.
	InvitationKey []byte
	// CreationEnabled is workspace.creation_enabled.
	CreationEnabled bool
	// The registrants of the extension points.
	MembershipEndVetoers          []MembershipEndVetoer
	MembershipEndSubscribers      []MembershipEndSubscriber
	DeletionSubscribers           []WorkspaceDeletionSubscriber
	MembershipRestoreSubscribers  []MembershipRestoreSubscriber
	MembershipAdditionSubscribers []MembershipAdditionSubscriber
	MemberRoleChangeSubscribers   []MemberRoleChangeSubscriber
}

// Module is the wired workspace module.
type Module struct {
	uc httpadapter.UseCases
}

// New wires the module.
func New(d Deps) *Module {
	store := postgresadapter.New(d.Pool)
	auth := d.Authorizer
	tokens := macadapter.New(d.InvitationKey)
	ender := app.MembershipEnder{Members: store, Invitations: store, Profiles: d.Directory,
		Vetoers: d.MembershipEndVetoers, Subscribers: d.MembershipEndSubscribers}
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
			Locker: store, Workspaces: store, Members: store, Invitations: store, Subscribers: d.DeletionSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListMembers: app.NewListMembers(app.ListMembersDeps{Workspaces: store, Members: store, Profiles: d.Directory, Auth: auth}),
		UpdateMember: app.NewUpdateMember(app.UpdateMemberDeps{
			Locker: store, Finder: store, Members: store, Profiles: d.Directory, Subscribers: d.MemberRoleChangeSubscribers,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		RemoveMember: app.NewRemoveMember(app.RemoveMemberDeps{
			Locker: store, Finder: store, Ender: ender, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		LeaveWorkspace: app.NewLeaveWorkspace(app.LeaveWorkspaceDeps{
			Locker: store, Finder: store, Ender: ender, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		ListInvitations: app.NewListInvitations(app.ListInvitationsDeps{Workspaces: store, Invitations: store, Tokens: tokens, Auth: auth}),
		CreateInvitation: app.NewCreateInvitation(app.CreateInvitationDeps{
			Sharer: store, Accounts: d.Directory, Members: store, Invitations: store, Tokens: tokens,
			Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		DeleteInvitation: app.NewDeleteInvitation(app.DeleteInvitationDeps{
			Finder: store, Sharer: store, Invitations: store, Auth: auth, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		PreviewInvitation: app.NewPreviewInvitation(app.PreviewInvitationDeps{Tokens: tokens, Invitations: store, Workspaces: store}),
		AcceptInvitation: app.NewAcceptInvitation(app.AcceptInvitationDeps{
			Tokens: tokens, Finder: store, Accounts: d.Accounts, Locker: store, Invitations: store, Members: store, Updater: store,
			Restored: d.MembershipRestoreSubscribers, Added: d.MembershipAdditionSubscribers, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
	}}
}

// PublicOperations are the module's routes that need no token.
func (m *Module) PublicOperations() []string {
	return httpadapter.PublicOperations()
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
