package app

import (
	"context"
	"errors"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Grants is a unit's memo of the actions asked beyond its own decision
// (M10 design 4.6): the mode guard is one registrant that every unit
// shares, which cannot keep an answer, so the unit keeps it. Ask asks
// an action at most once a unit, the first time a step needs it; a step
// of a standard notebook, or one that touches no source, never does.
// Only the mode guard and the unit's own locked checks ask; any
// extension may read the answers. It is read in the unit's transaction,
// so it sees the members the unit's own decision saw. A precheck before
// the unit asks on its own and keeps nothing here: the unit asks again
// under its locks.
type Grants struct {
	allowed map[shared.Action]bool
}

// Ask is ask's answer for action, ask called the first time only: nil
// for allowed, shared.Forbidden() for denied (ask answered it, or
// shared.ErrNotVisible), both kept for the unit; any other error, the
// database's, is answered as it is and kept not, since it rolls the unit
// back.
func (g *Grants) Ask(ctx context.Context, action shared.Action, ask func(ctx context.Context) error) error {
	allowed, asked := g.allowed[action]
	if !asked {
		err := ask(ctx)
		switch {
		case err == nil:
			allowed = true
		case errors.Is(err, shared.Forbidden()), errors.Is(err, shared.ErrNotVisible):
		default:
			return err
		}
		if g.allowed == nil {
			g.allowed = map[shared.Action]bool{}
		}
		g.allowed[action] = allowed
	}
	if !allowed {
		return shared.Forbidden()
	}
	return nil
}
