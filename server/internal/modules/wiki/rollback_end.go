package wiki

import (
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// An upgrade's rollback's end (M11/P1 design 3.20): the page module's part
// in it, which bootstrap fills, what a rollback left, and the end the
// transfer module calls as the rollback's job ends.
type (
	RollbackPages = app.RollbackPages
	RollbackLeft  = domain.RollbackLeft
	RollbackEnd   = app.RollbackEnd
)

// NewRollbackEnd returns the end of the upgrades' rollbacks, over pool's
// rows of the module, the notebooks' workspaces, the notebook module's
// upgrade's columns and the page module's part: bootstrap builds it before
// the transfer module, whose rollbacks' jobs end through it.
func NewRollbackEnd(pool *pgxpool.Pool, notebooks Notebooks, states UpgradeStates, pages RollbackPages, clock Clock,
	logger Logger,
) *RollbackEnd {
	return app.NewRollbackEnd(app.RollbackEndDeps{Notebooks: notebooks, States: states, Pages: pages, Rows: postgresadapter.New(pool),
		Clock: clock, Logger: logger})
}
