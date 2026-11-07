package page

import (
	"context"
	"uuid"

	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// MCPEntry is the notebook entry's decisions, for mcpserver (M9 design
// 4.1): Gate, and the instructions of a handshake.
type MCPEntry struct {
	gate         *app.NotebookGate
	notebooks    app.NotebookStates
	instructions *app.Instructions
}

// MCPEntry is the module's MCPEntry: bootstrap hands it to mcpserver,
// which it satisfies by structure.
func (m *Module) MCPEntry() MCPEntry {
	return MCPEntry{gate: m.gate, notebooks: m.notebooks, instructions: m.instructions}
}

// Gate lets an MCP client into the notebook notebookID: node.list, or
// notebook.not_found, which the zero id is without a read.
func (e MCPEntry) Gate(ctx context.Context, notebookID uuid.UUID) error {
	return e.gate.Execute(ctx, notebookID)
}

// Instructions are the notebook's instructions for a handshake, which its
// Gate let in (M9 design 4.6, M10 design 4.15): an LLM Wiki's system
// paragraph; its AGENTS page's text, or the built-in ones when it has
// none; within MaxInstructionsBytes as the handshake's text counts them,
// the note included when cut. A notebook deleted since the Gate is
// notebook.not_found, never read as a standard one.
func (e MCPEntry) Instructions(ctx context.Context, notebookID uuid.UUID) (string, error) {
	state, ok, err := e.notebooks.StateOf(ctx, notebookID)
	switch {
	case err != nil:
		return "", err
	case !ok:
		return "", domain.ErrNotebookNotFound
	}
	i, err := e.instructions.Read(ctx, notebookID, state)
	if err != nil {
		return "", err
	}
	return mcpadapter.NotebookText(i, mcpserver.NotebookEntry, func(string) bool { return true })
}

// AccountInstructions are the account entry's instructions, the built-in
// ones in English (M9/P1 design 3.15.4): bootstrap hands them to
// mcpserver.
func AccountInstructions() string {
	return mcpadapter.AccountInstructions
}
