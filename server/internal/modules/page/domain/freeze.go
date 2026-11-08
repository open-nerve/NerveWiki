package domain

import "uuid"

// Freeze is what a notebook's upgrade does to a write (M11 design 4.5).
type Freeze int

const (
	// FreezeOpen: no upgrade, or the upgrade's own unit: nothing asked.
	FreezeOpen Freeze = iota
	// FreezeStale: a unit of an upgrade that is not the notebook's now.
	FreezeStale
	// FreezeRefused: notebook.upgrading.
	FreezeRefused
	// FreezeAsk: the initiator's MCP write while upgrading, admitted to
	// the guardrails when notebook.change_mode is granted.
	FreezeAsk
)

// FreezeAdmits is the freeze of a write by by from client, of the unit of
// the upgrade own (the zero id for none), in a notebook in state n: a unit
// of an upgrade not the notebook's now is stale, whatever else; a notebook
// with no upgrade, or the upgrade's own unit, is open; reviewing and
// rolling back refuse every other write; upgrading asks of its initiator's
// MCP write and refuses the rest, the initiator's web and API writes too.
// It reads no kind of changeset: a revert is refused as an edit is.
func FreezeAdmits(n NotebookState, own uuid.UUID, client Client, by uuid.UUID) Freeze {
	switch {
	case own != (uuid.UUID{}) && own != n.Upgrade.ID:
		return FreezeStale
	case n.Upgrade.ID == (uuid.UUID{}), own == n.Upgrade.ID:
		return FreezeOpen
	case n.Upgrade.State != UpgradeUpgrading:
		return FreezeRefused
	case client.IsMCP() && by == n.Upgrade.By:
		return FreezeAsk
	}
	return FreezeRefused
}
