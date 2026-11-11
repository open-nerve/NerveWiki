package wiki

import (
	mcpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/mcp"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// Prompts are the module's MCP prompts: the three playbooks of an LLM
// Wiki, and the upgrade's (M11 design 4.13).
func (m *Module) Prompts() []mcpserver.Prompt {
	return mcpadapter.Prompts(m.playbooks, m.upgradePlaybook)
}

// Playbook names one of an LLM Wiki's three prompts.
type Playbook = domain.Playbook

// The LLM Wiki's three playbooks, as the composition root names them.
const (
	PlaybookIngest = domain.PlaybookIngest
	PlaybookQuery  = domain.PlaybookQuery
	PlaybookLint   = domain.PlaybookLint
)

// PlaybookText and UpgradePlaybookText are the texts the prompts answer:
// an LLM Wiki's playbook kind, argument with it when the caller gave one,
// and the upgrade's. The composition root's tests compare each prompt's
// answer with them, whole (M11/P3 design 3.18).
func PlaybookText(kind Playbook, argument *string) string { return domain.PlaybookText(kind, argument) }

// UpgradePlaybookText is the upgrade prompt's text; see PlaybookText.
func UpgradePlaybookText() string { return domain.UpgradePlaybookText() }
