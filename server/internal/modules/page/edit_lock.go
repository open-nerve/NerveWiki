package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// EditLock is the edit lock (M5 design 4.1, 4.4): bootstrap registers it
// as a vetoer of the openings and a guard of the writes.
type EditLock = app.EditLock

// NewEditLock returns the lock over pool, naming its holders by names.
func NewEditLock(pool *pgxpool.Pool, names Names) *EditLock {
	return app.NewEditLock(postgresadapter.New(pool), names)
}
