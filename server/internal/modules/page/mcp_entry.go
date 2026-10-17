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
	instructions *app.Instructions
}

// MCPEntry is the module's MCPEntry: bootstrap hands it to mcpserver,
// which it satisfies by structure.
func (m *Module) MCPEntry() MCPEntry {
	return MCPEntry{gate: m.gate, instructions: m.instructions}
}

// Gate lets an MCP client into the notebook notebookID: node.list, or
// notebook.not_found, which the zero id is without a read.
func (e MCPEntry) Gate(ctx context.Context, notebookID uuid.UUID) error {
	return e.gate.Execute(ctx, notebookID)
}

// Instructions are the notebook's instructions for a handshake, which its
// Gate let in (M9 design 4.6): its AGENTS page's, within
// MaxInstructionsBytes as the handshake's text counts them, the note
// included when cut; the built-in ones when it has none.
func (e MCPEntry) Instructions(ctx context.Context, notebookID uuid.UUID) (string, error) {
	a, ok, err := e.instructions.Execute(ctx, notebookID)
	switch {
	case err != nil:
		return "", err
	case !ok:
		return mcpadapter.NotebookInstructions, nil
	}
	return domain.AgentsText(a, func(s string) bool { return mcpserver.TextBytes(s) <= domain.MaxInstructionsBytes }), nil
}

// AccountInstructions are the account entry's instructions, the built-in
// ones in English (M9/P1 design 3.15.4): bootstrap hands them to
// mcpserver.
func AccountInstructions() string {
	return mcpadapter.AccountInstructions
}
