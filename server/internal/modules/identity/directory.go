package identity

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// Profile is what other accounts see of an account.
type Profile = domain.Profile

// Directory is the other modules' unlocked reads of accounts (M2 design 5,
// M2/P3 design 3.6): each one statement, no lock, outside the lock order.
type Directory interface {
	// Profiles returns the profiles of the accounts ids, by id; an id of
	// no account is left out.
	Profiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Profile, error)
	// AccountIDByEmail returns the id of the account of email, a normalized
	// address, and whether there is one, deactivated or not.
	AccountIDByEmail(ctx context.Context, email string) (uuid.UUID, bool, error)
}

// NewDirectory returns the Directory over pool alone: bootstrap builds it
// before the modules that use it.
func NewDirectory(pool *pgxpool.Pool) Directory {
	return postgresadapter.New(pool)
}
