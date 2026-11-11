package domain

import (
	"fmt"
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The refusals of an upgrade's start and read (M11/P1 design 3.14), in the
// order the start judges them, and of its abandonment and its rollback's
// end (3.20).
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
	// whose wanted counts the baseline would get wrong, or the check of a
	// submission (M11/P2 design 3.12): the text names neither.
	ErrUpgradeStaleIndex = shared.NewError(shared.KindConflict, "wiki.upgrade_stale_index",
		"The link index is out of date: ask a server admin to run nervewiki reindex, then try again.")
	// ErrUpgradeNotFound: the notebook has no upgrade under way.
	ErrUpgradeNotFound = shared.NewError(shared.KindNotFound, "wiki.upgrade_not_found", "The notebook has no upgrade under way.")
	// ErrUpgradeState: the notebook's upgrade is not in a state that
	// allows the operation (M11/P1 design 3.20): an abandonment of none,
	// an end of a rollback of one not rolling back.
	ErrUpgradeState = shared.NewError(shared.KindConflict, "wiki.upgrade_state", "The upgrade is not in a state that allows this.")
	// ErrUpgradeSubmitting: wiki.upgrade_state while the upgrade's previous
	// submission is still checked (M11 design 4.8 step 1, A2-M2).
	ErrUpgradeSubmitting = shared.NewError(shared.KindConflict, "wiki.upgrade_state",
		"The previous submission is still being checked: check again in a moment.")
	// ErrUpgradeNotReady: a submission's check did not pass, the upgrade is
	// upgrading again (M11 design 4.8); UpgradeNotReady names the gates.
	ErrUpgradeNotReady = shared.NewError(shared.KindConflict, "wiki.upgrade_not_ready",
		"The upgrade does not pass its checks: it is upgrading again. Check it, then submit it again.")
	// ErrRollbackWriting: the transfer module's transfer.busy, the
	// upgrade's rollback queued or running, as its continuation or its
	// end finds it.
	ErrRollbackWriting = shared.NewError(shared.KindConflict, "transfer.busy",
		"The upgrade's rollback is queued or running: continue or end it once it has stopped.")
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

// UpgradeNotReady is ErrUpgradeNotReady naming the gates that did not pass,
// by their keys only (M11/P2 design 3.12: no title, no path).
func UpgradeNotReady(failed []UpgradeGateKey) error {
	e := *ErrUpgradeNotReady
	keys := make([]string, len(failed))
	for i, k := range failed {
		keys[i] = string(k)
	}
	e.Detail = "These gates did not pass: " + strings.Join(keys, ", ") +
		". The upgrade is upgrading again: check it, then submit it again."
	return &e
}
