package domain

import (
	"fmt"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The refusals of an upgrade's start and read (M11/P1 design 3.14), in the
// order the start judges them.
var (
	// ErrAlreadyLLMWiki: the notebook is an LLM Wiki already.
	ErrAlreadyLLMWiki = shared.NewError(shared.KindConflict, "notebook.already_llm_wiki", "The notebook is already an LLM Wiki.")
	// ErrUpgrading: the notebook has an upgrade under way, another
	// admin's start first among them; the page module's text.
	ErrUpgrading = shared.NewError(shared.KindConflict, "notebook.upgrading",
		"The notebook is being upgraded to an LLM Wiki: its content is read-only until the upgrade ends.")
	// ErrWriting: the transfer module's transfer.busy, a job that writes
	// the notebook's tree queued or running, as the downgrade answers it.
	ErrWriting = shared.NewError(shared.KindConflict, "transfer.busy",
		"An import, a restore or a revert of the notebook is queued or running: start the upgrade once it is done.")
	// ErrUpgradePagesLocked: pages of the notebook are being edited;
	// UpgradePagesLocked names them.
	ErrUpgradePagesLocked = shared.NewError(shared.KindConflict, "wiki.upgrade_pages_locked",
		"Pages of the notebook are being edited: the upgrade can start once their editors leave or an admin unlocks them.")
	// ErrUpgradeTitleTaken: a node at the root titled raw or AGENTS is an
	// attachment, where the upgrade would write the page.
	ErrUpgradeTitleTaken = shared.NewError(shared.KindConflict, "wiki.upgrade_title_taken",
		"A node at the root titled raw or AGENTS is an attachment: rename it, then start the upgrade.")
	// ErrUpgradeStaleIndex: pages of the notebook have an old link index,
	// whose wanted counts the baseline would get wrong.
	ErrUpgradeStaleIndex = shared.NewError(shared.KindConflict, "wiki.upgrade_stale_index",
		"The link index is out of date: a server admin runs nervewiki reindex, then the upgrade can start.")
	// ErrUpgradeNotFound: the notebook has no upgrade under way.
	ErrUpgradeNotFound = shared.NewError(shared.KindNotFound, "wiki.upgrade_not_found", "The notebook has no upgrade under way.")
)

// UpgradePagesLocked is ErrUpgradePagesLocked naming locks, at most 100,
// one a page, its detail saying how many pages, total, are being edited.
func UpgradePagesLocked(locks []shared.LockHolder, total int) error {
	e := *ErrUpgradePagesLocked
	e.Locks = locks
	e.Detail = fmt.Sprintf("%d pages of the notebook are being edited: the upgrade can start once their editors leave or an admin unlocks them.", total)
	if total == 1 {
		e.Detail = "A page of the notebook is being edited: the upgrade can start once its editor leaves or an admin unlocks it."
	}
	return &e
}
