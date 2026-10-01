package notebook

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
)

// WorkspaceMembershipEnd is an account's workspace memberships ending, as
// the workspace module tells it: bootstrap converts
// workspace.MembershipEnd, Voluntary when the account leaves or is
// deactivated.
type WorkspaceMembershipEnd = app.WorkspaceMembershipEnd

// WorkspaceMembershipRestore is an account's workspace membership active
// again, field by field as the workspace module tells it: bootstrap
// converts workspace.MembershipRestore.
type WorkspaceMembershipRestore = app.WorkspaceMembershipRestore

// WorkspaceSlugs names workspaces: bootstrap hands workspace.NewWorkspaces
// to it.
type WorkspaceSlugs = app.WorkspaceSlugs

// MembershipEnd is the module's registrant of the workspace module's
// membership end (M3/P3 design 3.2): rule two as its vetoer, the account's
// notebook memberships ended, and the notebooks it leaves ownerless, as
// its subscriber.
type MembershipEnd = app.MembershipEnd

// MembershipRestore is the module's registrant of the workspace module's
// membership restore (M3/P3 design 3.2): the notebooks the account left
// ownerless returned to it.
type MembershipRestore = app.MembershipRestore

// NewMembershipEnd builds the registrant from the pool and workspaces, a
// port on the pool alone (v0.1 design 13.1, item 21), with the
// visibility's subscribers it calls. It runs in the end's transaction: the
// store finds it in the context.
func NewMembershipEnd(pool *pgxpool.Pool, workspaces WorkspaceSlugs, subscribers []VisibilitySubscriber) MembershipEnd {
	return app.MembershipEnd{Holdings: postgresadapter.New(pool), Workspaces: workspaces, Subscribers: subscribers}
}

// NewMembershipRestore builds the registrant from the pool alone, with the
// visibility's subscribers it calls. It runs in the restore's transaction.
func NewMembershipRestore(pool *pgxpool.Pool, subscribers []VisibilitySubscriber) MembershipRestore {
	store := postgresadapter.New(pool)
	return app.MembershipRestore{Returner: store, Audit: store, Subscribers: subscribers}
}
