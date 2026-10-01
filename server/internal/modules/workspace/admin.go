package workspace

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// AccountsByEmail is what identity offers the administrator's commands:
// bootstrap hands identity.NewAccounts to NewAdmin.
type AccountsByEmail = app.AccountsByEmail

// Reactivated is what reactivate-member did.
type Reactivated = app.Reactivated

// AdminDeps are what the server administrator's commands need (v0.1 design
// 13.1, item 22): the pool, identity's accounts and the restore's
// registrants; no Authorizer, invitation key or HTTP side.
type AdminDeps struct {
	Pool                         *pgxpool.Pool
	Tx                           shared.TxManager
	Clock                        Clock
	Logger                       *slog.Logger
	Accounts                     AccountsByEmail
	MembershipRestoreSubscribers []MembershipRestoreSubscriber
}

// Admin is the server administrator's use cases behind nervewiki
// workspaces (M2/P4 design 3.3).
type Admin struct {
	create     *app.CreateWorkspaceFor
	reactivate *app.ReactivateMember
}

// NewAdmin wires the administrator's use cases on the pool alone: the
// command line builds no HTTP side, so this is the module's own minimal
// composition, not New's.
func NewAdmin(d AdminDeps) *Admin {
	store := postgresadapter.New(d.Pool)
	return &Admin{
		create: app.NewCreateWorkspaceFor(app.CreateWorkspaceForDeps{
			Workspaces: store, Accounts: d.Accounts, Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
		reactivate: app.NewReactivateMember(app.ReactivateMemberDeps{
			Accounts: d.Accounts, Locker: store, Members: store, Updater: store, Subscribers: d.MembershipRestoreSubscribers,
			Tx: d.Tx, Clock: d.Clock, Logger: d.Logger,
		}),
	}
}

// CreateWorkspace is nervewiki workspaces create: the workspace name with
// slug, with the account of adminEmail as its admin, whatever
// workspace.creation_enabled says.
func (a *Admin) CreateWorkspace(ctx context.Context, name, slug, adminEmail string) (domain.Workspace, error) {
	return a.create.Execute(ctx, name, slug, adminEmail)
}

// ReactivateMember is nervewiki workspaces reactivate-member: the ended
// membership of the account of email in the workspace of slug, active
// again with its role.
func (a *Admin) ReactivateMember(ctx context.Context, slug, email string) (Reactivated, error) {
	return a.reactivate.Execute(ctx, slug, email)
}
