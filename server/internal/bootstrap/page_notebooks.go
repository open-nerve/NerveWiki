package bootstrap

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
)

// pageNotebooks is the notebook module's Notebooks as the page module reads
// them (M10/P1 design 3.12): the workspace and the locks as they are, the
// state converted field by field into the page module's own, each module
// with its mode (v0.1 design 13.1 item 11).
type pageNotebooks struct {
	notebook.Notebooks
}

func (n pageNotebooks) StateOf(ctx context.Context, id uuid.UUID) (page.NotebookState, bool, error) {
	s, ok, err := n.Notebooks.StateOf(ctx, id)
	if err != nil || !ok {
		return page.NotebookState{}, ok, err
	}
	return page.NotebookState{Mode: page.Mode(s.Mode), SourceRootID: s.SourceRootID}, true, nil
}
