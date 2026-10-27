package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ModeGuard keeps a notebook's mode's contract in each of its writes (M10
// design 4.6): bootstrap registers it as a guard of the writes, before the
// edit lock.
type ModeGuard = app.ModeGuard

// NewModeGuard is the guard over the composition root's authorizer, the
// one the units decide with (v0.1 design 13.1 item 21's exception: a
// registrant not built from the pool alone). Its source reader uses the
// opening's transaction, under the notebook and page locks.
func NewModeGuard(pool *pgxpool.Pool, auth shared.Authorizer) *ModeGuard {
	return app.NewModeGuard(auth, postgresadapter.New(pool))
}
