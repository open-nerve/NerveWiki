package domain

import "uuid"

// Mode is a notebook's mode, as the page module knows it (M10 design
// 4.3): the composition root converts the notebook module's.
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// UpgradeState is a notebook's upgrade's state, the notebook module's
// (M11 design 4.3): the composition root converts it.
type UpgradeState string

const (
	UpgradeUpgrading   UpgradeState = "upgrading"
	UpgradeReviewing   UpgradeState = "reviewing"
	UpgradeRollingBack UpgradeState = "rolling_back"
)

// NotebookUpgrade is a notebook's upgrade under way, the zero value for
// none: what the freeze reads (M11 design 4.5).
type NotebookUpgrade struct {
	ID    uuid.UUID
	State UpgradeState
	By    uuid.UUID
}

// NotebookState is a notebook's mode and source root, the zero id for
// none, and its upgrade under way, the zero value for none: what a write
// reads of its notebook under the notebook's row lock (Write.Notebook,
// M10/P1/S2; M11 design 4.5), and an unlocked read reads alone.
type NotebookState struct {
	Mode         Mode
	SourceRootID uuid.UUID
	Upgrade      NotebookUpgrade
}
