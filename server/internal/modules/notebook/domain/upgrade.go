package domain

import (
	"time"
	"uuid"
)

// UpgradeState is the state of a notebook's upgrade to an LLM Wiki under
// way (M11 design 4.3): no mode, the notebook standard throughout.
type UpgradeState string

const (
	UpgradeUpgrading   UpgradeState = "upgrading"
	UpgradeReviewing   UpgradeState = "reviewing"
	UpgradeRollingBack UpgradeState = "rolling_back"
)

// Upgrade is a notebook's upgrade under way, the zero value for none:
// its id, state, initiator and start (notebooks_upgrade_check sets the
// four together). Comparable, as Notebook and State are compared whole.
type Upgrade struct {
	ID    uuid.UUID
	State UpgradeState
	By    uuid.UUID
	At    time.Time
}
