package identity

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
)

// The extension points identity offers the other modules (M1 design 8).
// The other modules do not import identity: bootstrap adapts their
// implementations to these types, and hands them Accounts.
type (
	// Deactivation is an account being deactivated.
	Deactivation = app.Deactivation
	// DeactivationVetoer may refuse a deactivation, under the account row
	// lock and before any write.
	DeactivationVetoer = app.DeactivationVetoer
	// DeactivationSubscriber follows a deactivation, in its transaction.
	DeactivationSubscriber = app.DeactivationSubscriber
)

// Accounts is what a module that gives an account new access calls first
// in its transaction (M1/P3 design 3.6): ShareActiveAccount locks the
// account row FOR SHARE and confirms the account is active, so a
// deactivation and the new access run one after the other.
type Accounts interface {
	ShareActiveAccount(ctx context.Context, id uuid.UUID) error
}

// NewAccounts returns Accounts over pool alone: bootstrap builds it before
// the modules that use it.
func NewAccounts(pool *pgxpool.Pool) Accounts {
	return app.ActiveAccounts{Accounts: postgresadapter.New(pool)}
}
