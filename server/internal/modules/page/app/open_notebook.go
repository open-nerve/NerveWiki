package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// OpenNotebook opens a notebook for MCP's open_notebook (M9 design 4.6):
// node.list, then its instructions and an overview, unlocked.
type OpenNotebook struct {
	gate         *NotebookGate
	notebooks    Notebooks
	overview     OverviewStore
	instructions *Instructions
}

// NewOpenNotebook returns the use case: its decision is the notebook
// entry's gate, its instructions the entry's, with their one cache; its
// mode the notebooks' (M10/P1 design 3.12).
func NewOpenNotebook(gate *NotebookGate, notebooks Notebooks, overview OverviewStore, instructions *Instructions) *OpenNotebook {
	return &OpenNotebook{gate: gate, notebooks: notebooks, overview: overview, instructions: instructions}
}

// Opened is an open notebook: what its instructions are written from, its
// mode among them, which the adapter writes; its pages and attachments,
// its last change, nil when nothing was ever written in it, and the root's
// first nodes, at most MaxOverviewRoot in the tree's order, MoreRoot when
// others follow.
type Opened struct {
	Instructions NotebookInstructions
	Pages        int64
	Attachments  int64
	LastChange   *time.Time
	Root         []RootNode
	MoreRoot     bool
}

// RootNode is a node at the notebook's root, by its path.
type RootNode struct {
	Virtual bool
	Path    string
	Kind    domain.Kind
}

// Execute opens the notebook notebookID, in its order (M9/P1 design
// 3.17.3; M10/P1 design 3.12): the actor and the notebook,
// notebook.not_found for none, a deleted one, one the caller cannot see
// and the zero id, before anything of its content is read; its mode, read
// unlocked, notebook.not_found for one deleted since; its counts; its last
// change; the root's first nodes; its instructions.
func (o *OpenNotebook) Execute(ctx context.Context, notebookID uuid.UUID) (Opened, error) {
	if err := o.gate.Execute(ctx, notebookID); err != nil {
		return Opened{}, err
	}
	state, ok, err := o.notebooks.StateOf(ctx, notebookID)
	switch {
	case err != nil:
		return Opened{}, err
	case !ok:
		return Opened{}, domain.ErrNotebookNotFound
	}
	var out Opened
	if out.Pages, out.Attachments, err = o.overview.NotebookCounts(ctx, notebookID); err != nil {
		return Opened{}, err
	}
	if out.LastChange, err = o.overview.NotebookLastChange(ctx, notebookID); err != nil {
		return Opened{}, err
	}
	root, err := o.overview.FirstChildren(ctx, notebookID, domain.MaxOverviewRoot+1)
	if err != nil {
		return Opened{}, err
	}
	out.Root = make([]RootNode, 0, domain.MaxOverviewRoot)
	if state.Mode == domain.ModeLLMWiki {
		out.Root = append(out.Root, RootNode{Path: "index.md", Kind: domain.KindPage, Virtual: true}, RootNode{Path: "log.md", Kind: domain.KindPage, Virtual: true})
	}
	room := domain.MaxOverviewRoot - len(out.Root)
	if len(root) > room {
		root, out.MoreRoot = root[:room], true
	}
	for _, n := range root {
		out.Root = append(out.Root, RootNode{Path: domain.PathOf(nil, n), Kind: n.Kind})
	}
	if out.Instructions, err = o.instructions.Read(ctx, notebookID, state); err != nil {
		return Opened{}, err
	}
	return out, nil
}
