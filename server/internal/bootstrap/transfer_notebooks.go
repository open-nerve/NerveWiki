package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
)

// transferNotebooks is the notebook module's Notebooks as the transfer
// module reads them (M11/P1 design 3.5): the workspace and the name as
// they are; the state, unlocked or under the row's FOR SHARE, converted
// into the transfer module's, its upgrade under way and whether it rolls
// back (v0.1 design 13.1 item 11).
type transferNotebooks struct {
	notebook.Notebooks
}

func (n transferNotebooks) ShareByID(ctx context.Context, id uuid.UUID) (transfer.NotebookState, bool, error) {
	return toTransferState(n.ShareState(ctx, id))
}

func (n transferNotebooks) StateOf(ctx context.Context, id uuid.UUID) (transfer.NotebookState, bool, error) {
	return toTransferState(n.Notebooks.StateOf(ctx, id))
}

// toTransferState is the notebook module's answer of a state as the
// transfer module's: none found, or an error, with the zero state.
func toTransferState(s notebook.State, ok bool, err error) (transfer.NotebookState, bool, error) {
	if err != nil || !ok {
		return transfer.NotebookState{}, ok, err
	}
	return transfer.NotebookState{UpgradeID: s.Upgrade.ID, RollingBack: s.Upgrade.State == notebook.UpgradeRollingBack}, true, nil
}
