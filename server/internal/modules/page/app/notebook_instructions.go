package app

import (
	"context"
	"strings"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// NotebookInstructions is what one request's instructions are written
// from (M10 design 4.15): Agents when HasAgents; for an LLM Wiki, its
// source root's path now, without a page's suffix, and its title. It is
// no cache: a rename, a move or a downgrade shows at the next read.
type NotebookInstructions struct {
	NotebookID                      uuid.UUID
	Mode                            domain.Mode
	Agents                          domain.Agents
	HasAgents                       bool
	SourceRootPath, SourceRootTitle string
}

// Read is the notebook's instructions in state, read by the caller: the
// entry's Gate or open_notebook decided the notebook. Its AGENTS page
// through the cache; then, in an LLM Wiki alone, its source root, by one
// point read and one read of the ancestors, which leave the node out. The
// reads share no transaction: a change between them shows until the next
// request, and a root deleted between them is the error of its read.
func (i *Instructions) Read(ctx context.Context, notebookID uuid.UUID, state domain.NotebookState) (NotebookInstructions, error) {
	out := NotebookInstructions{NotebookID: notebookID, Mode: state.Mode}
	var err error
	if out.Agents, out.HasAgents, err = i.Execute(ctx, notebookID); err != nil {
		return NotebookInstructions{}, err
	}
	if state.Mode != domain.ModeLLMWiki {
		return out, nil
	}
	root, err := i.nodes.FindNodeIn(ctx, notebookID, state.SourceRootID)
	if err != nil {
		return NotebookInstructions{}, err
	}
	line, err := i.nodes.Ancestors(ctx, state.SourceRootID)
	if err != nil {
		return NotebookInstructions{}, err
	}
	names := make([]string, 0, len(line)+1)
	for _, a := range line {
		names = append(names, a.Name)
	}
	out.SourceRootPath, out.SourceRootTitle = strings.Join(append(names, root.Name), "/"), root.Name
	return out, nil
}
