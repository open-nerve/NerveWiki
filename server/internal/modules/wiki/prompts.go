package wiki

import (
	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// Prompts are the module's MCP prompts, the three playbooks.
func (m *Module) Prompts() []mcpserver.Prompt { return mcpadapter.Prompts(m.playbooks) }
