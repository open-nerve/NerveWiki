package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// FreezeGuard keeps an upgrading notebook's content read-only (M11 design
// 4.5): a guard of the writes and a vetoer of the openings, first of both.
// A write of a notebook with no upgrade, or of the upgrade's own unit, it
// lets by at once, reading nothing.
type FreezeGuard struct {
	auth  shared.Authorizer
	admin shared.Action
}

// NewFreezeGuard is the guard over auth, the authorizer the units decide
// with, asking admin of the initiator's MCP writes while upgrading: the
// notebook module's notebook.change_mode, which bootstrap hands it.
func NewFreezeGuard(auth shared.Authorizer, admin shared.Action) *FreezeGuard {
	return &FreezeGuard{auth: auth, admin: admin}
}

// GuardWrite judges s by the upgrade its unit read under the notebook's
// lock (domain.FreezeAdmits): a stale unit of an upgrade is
// ErrStaleUpgradeWrite, a defect's error; a refused write
// notebook.upgrading, which the unit marks with the step's first node; the
// initiator's MCP write while upgrading asks admin once a unit (Grants),
// notebook.upgrading when it is not granted, the initiator no admin now.
// Any other error, the database's, is answered as it is.
func (g *FreezeGuard) GuardWrite(ctx context.Context, s Step) error {
	switch domain.FreezeAdmits(s.Notebook, s.UpgradeOwn, s.Client, s.By) {
	case domain.FreezeOpen:
		return nil
	case domain.FreezeStale:
		return domain.ErrStaleUpgradeWrite
	case domain.FreezeRefused:
		return domain.ErrUpgrading
	case domain.FreezeAsk:
	}
	if err := s.Grants.Ask(ctx, g.admin, decideAction(g.auth, s, g.admin)); err != nil {
		if errors.Is(err, shared.Forbidden()) {
			return domain.ErrUpgrading
		}
		return err
	}
	return nil
}

// VetoEditSession refuses every opening in a notebook with an upgrade
// under way, whatever its state (M11 design 4.5): MCP opens no session,
// and the initiator's web is read-only as everyone's is.
func (g *FreezeGuard) VetoEditSession(_ context.Context, o SessionOpening) error {
	if o.Notebook.Upgrade.ID != (uuid.UUID{}) {
		return domain.ErrUpgrading
	}
	return nil
}
