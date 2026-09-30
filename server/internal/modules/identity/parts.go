package identity

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	argon2adapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/argon2"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
)

// parts are what New and NewAdmin build alike from the pool, the password
// settings and the deactivation's registrants (M1/P4 design 3.6).
type parts struct {
	store  *postgresadapter.Store
	hasher *argon2adapter.Hasher
	// rules are built once: building them splits the common-password list.
	rules *domain.PasswordRules
	steps app.DeactivationSteps
}

func newParts(pool *pgxpool.Pool, password PasswordHashing, logger *slog.Logger,
	vetoers []DeactivationVetoer, subscribers []DeactivationSubscriber,
) parts {
	store := postgresadapter.New(pool)
	return parts{
		store:  store,
		hasher: argon2adapter.New(argon2adapter.Params(password), logger),
		rules:  domain.NewPasswordRules(),
		steps:  app.DeactivationSteps{Users: store, Sessions: store, Vetoers: vetoers, Subscribers: subscribers},
	}
}
