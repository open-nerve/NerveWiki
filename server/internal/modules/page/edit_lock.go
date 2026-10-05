package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// EditLock is the edit lock (M5 design 4.1, 4.4): bootstrap registers it
// as a vetoer of the openings and a guard of the writes.
type EditLock = app.EditLock

// NewEditLock returns the lock over pool, naming its holders by names.
func NewEditLock(pool *pgxpool.Pool, names Names) *EditLock {
	return app.NewEditLock(postgresadapter.New(pool), names)
}

// ErrLocked is page.locked, which the edit lock answers: bootstrap tells it
// from the other errors of a write another module adds (M6/P4).
var ErrLocked = domain.ErrLocked

// LockHolders reads the edit locks of pages for another module: M6's
// rewrite of links names the pages being edited it meets (M6/P4 design
// 4.1).
type LockHolders = app.LockHolders

// NewLockHolders returns the read over pool, naming the holders by names.
func NewLockHolders(pool *pgxpool.Pool, names Names) *LockHolders {
	return app.NewLockHolders(postgresadapter.New(pool), names)
}
