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

// Profiles reads accounts' profiles for the other modules' member lists (M2
// design 5): one statement, no lock, outside the lock order.
type Profiles interface {
	// Profiles returns the profiles of the accounts ids, by id; an id of
	// no account is left out.
	Profiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Profile, error)
}

// NewProfiles returns Profiles over pool alone: bootstrap builds it before
// the modules that use it.
func NewProfiles(pool *pgxpool.Pool) Profiles {
	return postgresadapter.New(pool)
}
