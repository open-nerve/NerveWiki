// Package mcpadapter projects the wiki's existing lint runtime into MCP tools.
package mcpadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// LintUseCase is the same runtime used by the module's other callers.
type LintUseCase interface {
	Execute(context.Context, app.LintScope, app.LintQuery) (domain.LintResult, error)
}

// Tools exposes both read-only checks in both entries, sharing the caller's owner.
func Tools(lint LintUseCase) []mcpserver.Tool {
	t := tools{lint: lint}
	read := mcpserver.Annotations{ReadOnly: true}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "lint", Title: "Check the notebook's health", Description: lintDescription,
			Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.check),
		mcpserver.NewTool(mcpserver.Spec{Name: "pending_sources", Title: "List undigested sources", Description: pendingSourcesDescription,
			Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.pendingSources),
	}
}

type tools struct{ lint LintUseCase }

const lintDescription = `Check this notebook's health from a snapshot for this call. Omit rule for counts and previews of the applicable rules; limit and cursor require rule. Set rule to read its items, then continue with next_cursor; pages may move between calls. count_capped marks a lower bound, not the end of pagination. target_cut marks a shortened display target, not its full identity; use item_key. Page and source items include their full paths.`

const pendingSourcesDescription = `List an LLM Wiki's undigested sources from a snapshot for this call. LLM Wiki only: a standard notebook is refused. This is the undigested_sources health check, with each source's path, kind, bytes and original created_at; empty source pages are omitted. Continue with next_cursor; pages may move between calls. Each item keeps its complete item_key and cursor.`
