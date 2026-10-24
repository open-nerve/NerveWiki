package domain

import "uuid"

// Mode is a notebook's mode, as the page module knows it (M10 design
// 4.3): the composition root converts the notebook module's.
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// NotebookState is a notebook's mode and source root, the zero id for
// none: what a write reads of its notebook under the notebook's row lock
// (Write.Notebook, M10/P1/S2), and an unlocked read reads alone.
type NotebookState struct {
	Mode         Mode
	SourceRootID uuid.UUID
}
