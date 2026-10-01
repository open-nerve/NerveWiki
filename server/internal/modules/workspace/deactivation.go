package workspace

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
)

// Deactivated is an account being deactivated, field by field as
// identity's deactivation tells it: bootstrap converts
// identity.Deactivation.
type Deactivated = app.Deactivated

// Deactivation is the module's registrant of identity's deactivation
// (M2/P4 design 3.1): its VetoDeactivation applies rule two, its
// AccountDeactivated ends the account's memberships, both through the
// membership end's extension point.
type Deactivation = app.Deactivation

// NewDeactivation builds the registrant from the pool alone (v0.1 design
// 13.1, item 21), with the membership end's registrants it calls. It runs
// in the deactivation's transaction: the store finds it in the context.
func NewDeactivation(pool *pgxpool.Pool, vetoers []MembershipEndVetoer, subscribers []MembershipEndSubscriber) Deactivation {
	store := postgresadapter.New(pool)
	// The deactivation reads the address under the account's lock and
	// writes with it: the ender's Profiles, End's unlocked read, stay
	// unused.
	ender := app.MembershipEnder{Members: store, Invitations: store, Vetoers: vetoers, Subscribers: subscribers}
	return app.Deactivation{Locker: store, Standings: store, Lister: store, Ender: ender}
}
