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
	overview     OverviewStore
	instructions *Instructions
}

// NewOpenNotebook returns the use case: its decision is the notebook
// entry's gate, its instructions the entry's, with their one cache.
func NewOpenNotebook(gate *NotebookGate, overview OverviewStore, instructions *Instructions) *OpenNotebook {
	return &OpenNotebook{gate: gate, overview: overview, instructions: instructions}
}

// Opened is an open notebook: its AGENTS page's text when HasAgents (else
// the built-in instructions, which the adapter writes), its pages and
// attachments, its last change, nil when nothing was ever written in it,
// and the root's first nodes, at most MaxOverviewRoot in the tree's
// order, MoreRoot when others follow.
type Opened struct {
	Agents      domain.Agents
	HasAgents   bool
	Pages       int64
	Attachments int64
	LastChange  *time.Time
	Root        []RootNode
	MoreRoot    bool
}

// RootNode is a node at the notebook's root, by its path.
type RootNode struct {
	Path string
	Kind domain.Kind
}

// Execute opens the notebook notebookID, in its order (M9/P1 design
// 3.17.3): the actor and the notebook, notebook.not_found for none, a
// deleted one, one the caller cannot see and the zero id, before anything
// of its content is read; its counts; its last change; the root's first
// nodes; its instructions.
func (o *OpenNotebook) Execute(ctx context.Context, notebookID uuid.UUID) (Opened, error) {
	if err := o.gate.Execute(ctx, notebookID); err != nil {
		return Opened{}, err
	}
	var out Opened
	var err error
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
	if len(root) > domain.MaxOverviewRoot {
		root, out.MoreRoot = root[:domain.MaxOverviewRoot], true
	}
	out.Root = make([]RootNode, len(root))
	for i, n := range root {
		out.Root[i] = RootNode{Path: domain.PathOf(nil, n), Kind: n.Kind}
	}
	if out.Agents, out.HasAgents, err = o.instructions.Execute(ctx, notebookID); err != nil {
		return Opened{}, err
	}
	return out, nil
}
