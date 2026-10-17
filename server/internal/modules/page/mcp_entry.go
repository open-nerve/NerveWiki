package page

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
)

// MCPEntry is the notebook entry's decisions, for mcpserver (M9 design
// 4.1): Gate, and the instructions of a handshake (M9/P1 S3).
type MCPEntry struct {
	gate *app.NotebookGate
}

// MCPEntry is the module's MCPEntry: bootstrap hands it to mcpserver,
// which it satisfies by structure.
func (m *Module) MCPEntry() MCPEntry {
	return MCPEntry{gate: m.gate}
}

// Gate lets an MCP client into the notebook notebookID: node.list, or
// notebook.not_found, which the zero id is without a read.
func (e MCPEntry) Gate(ctx context.Context, notebookID uuid.UUID) error {
	return e.gate.Execute(ctx, notebookID)
}

// Instructions are the notebook's instructions for a handshake: none yet,
// the AGENTS page's come with M9/P1 S3.
func (e MCPEntry) Instructions(context.Context, uuid.UUID) (string, error) {
	return "", nil
}
