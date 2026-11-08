package app

import (
	"context"
	"errors"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// UpgradeNodes are an upgrade's rows the freeze reads, in the transaction
// ctx carries.
type UpgradeNodes interface {
	// Protected are those of ids that have a row of the upgrade upgradeID:
	// its baseline's and its protected source root's.
	Protected(ctx context.Context, upgradeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]bool, error)
}

// FreezeGuard keeps an upgrading notebook's content read-only (M11 design
// 4.5): a guard of the writes and a vetoer of the openings, first of both.
// A write of a notebook with no upgrade, or of the upgrade's own unit, it
// lets by at once, reading nothing. The initiator's agent's writes it
// admits it holds to the guardrails (M11 design 4.6).
type FreezeGuard struct {
	auth  shared.Authorizer
	admin shared.Action
	nodes UpgradeNodes
}

// NewFreezeGuard is the guard over auth, the authorizer the units decide
// with, asking admin of the initiator's MCP writes while upgrading: the
// notebook module's notebook.change_mode, which bootstrap hands it; nodes
// read the upgrade's rows its deletion guardrail keeps.
func NewFreezeGuard(auth shared.Authorizer, admin shared.Action, nodes UpgradeNodes) *FreezeGuard {
	return &FreezeGuard{auth: auth, admin: admin, nodes: nodes}
}

// GuardWrite judges s by the upgrade its unit read under the notebook's
// lock (domain.FreezeAdmits): a stale unit of an upgrade is
// ErrStaleUpgradeWrite, a defect's error; a refused write
// notebook.upgrading, which the unit marks with the step's first node; the
// initiator's MCP write while upgrading asks admin once a unit (Grants),
// notebook.upgrading when it is not granted, the initiator no admin now;
// granted, it is held to the guardrails. Any other error, the database's,
// is answered as it is.
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
	return g.guardrails(ctx, s)
}

// guardrails are the initiator's agent's limits (M11 design 4.6): P1's is
// the delete's, which keeps every node the upgrade started with and the
// source root its start created, page.upgrade_protected marked with the
// first of them in the subtree's order (Changes, top-down), not the
// subtree's root, which may be a node the agent created; a node created
// since may go. P2/S1 adds the rest.
func (g *FreezeGuard) guardrails(ctx context.Context, s Step) error {
	if s.Operation != domain.OpDelete {
		return nil
	}
	ids := make([]uuid.UUID, len(s.Changes))
	for i, c := range s.Changes {
		ids[i] = c.NodeID
	}
	protected, err := g.nodes.Protected(ctx, s.Notebook.Upgrade.ID, ids)
	if err != nil {
		return err
	}
	for _, c := range s.Changes {
		if protected[c.NodeID] {
			return domain.Refuse(domain.ErrUpgradeProtected, c.NodeID)
		}
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
