package page

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ModeGuard keeps a notebook's mode's contract in each of its writes (M10
// design 4.6): bootstrap registers it as a guard of the writes, before the
// edit lock.
type ModeGuard = app.ModeGuard

// NewModeGuard is the guard over the composition root's authorizer, the
// one the units decide with (v0.1 design 13.1 item 21's exception: a
// registrant not built from the pool alone). M10/P2/S1 adds the pool, for
// the reads of the source area's rules.
func NewModeGuard(auth shared.Authorizer) *ModeGuard {
	return app.NewModeGuard(auth)
}
