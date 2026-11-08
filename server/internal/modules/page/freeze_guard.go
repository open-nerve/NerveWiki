package page

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// FreezeGuard keeps an upgrading notebook's content read-only (M11 design
// 4.5): bootstrap registers it first of the guards of the writes and of
// the vetoers of the openings, before the mode guard and the edit lock.
type FreezeGuard = app.FreezeGuard

// NewFreezeGuard is the guard over the composition root's authorizer, the
// one the units decide with, asking admin, the notebook module's
// notebook.change_mode, of an upgrade's initiator's MCP writes (v0.1
// design 13.1 item 21's exception, as NewModeGuard: a registrant not built
// from the pool alone).
func NewFreezeGuard(_ *pgxpool.Pool, auth shared.Authorizer, admin shared.Action) *FreezeGuard {
	return app.NewFreezeGuard(auth, admin)
}
