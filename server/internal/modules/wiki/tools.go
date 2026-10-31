package wiki

import (
	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// Tools uses this module's existing runtime for both MCP entries.
func (m *Module) Tools() []mcpserver.Tool { return mcpadapter.Tools(m) }
