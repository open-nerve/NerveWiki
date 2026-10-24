package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// pageNotebooks is the notebook module's Notebooks as the page module reads
// them (M10/P1 design 3.12, 3.18): the workspace as it is; the state, read
// unlocked or under each lock, converted field by field into the page
// module's own, each module with its mode (v0.1 design 13.1 item 11).
type pageNotebooks struct {
	notebook.Notebooks
}

func (n pageNotebooks) StateOf(ctx context.Context, id uuid.UUID) (page.NotebookState, bool, error) {
	return toPageState(n.Notebooks.StateOf(ctx, id))
}

func (n pageNotebooks) ShareByID(ctx context.Context, id uuid.UUID) (page.NotebookState, bool, error) {
	return toPageState(n.ShareState(ctx, id))
}

func (n pageNotebooks) LockByID(ctx context.Context, id uuid.UUID) (page.NotebookState, bool, error) {
	return toPageState(n.LockState(ctx, id))
}

// toPageState is the notebook module's answer of a state as the page module's:
// none found, or an error, with the zero state.
func toPageState(s notebook.State, ok bool, err error) (page.NotebookState, bool, error) {
	if err != nil || !ok {
		return page.NotebookState{}, ok, err
	}
	return page.NotebookState{Mode: page.Mode(s.Mode), SourceRootID: s.SourceRootID}, true, nil
}
