// Package mcpadapter projects the wiki's existing lint runtime into MCP tools.
package mcpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// LintUseCase is the same runtime used by the module's other callers.
type LintUseCase interface {
	Execute(context.Context, app.LintScope, app.LintQuery) (domain.LintResult, error)
}

// UpgradeUseCase is an upgrade's check and its submission (M11/P2 design
// 3.14), the same use cases REST's operations call.
type UpgradeUseCase interface {
	CheckUpgrade(ctx context.Context, id uuid.UUID, query app.LintQuery) (app.UpgradeChecked, error)
	SubmitUpgrade(ctx context.Context, id uuid.UUID) (app.UpgradeView, error)
}

// Tools exposes both read-only checks, an upgrade's check and its
// submission in both entries, sharing the caller's owner.
func Tools(lint LintUseCase, upgrades UpgradeUseCase) []mcpserver.Tool {
	t := tools{lint: lint, upgrades: upgrades}
	read := mcpserver.Annotations{ReadOnly: true}
	return []mcpserver.Tool{
		mcpserver.NewTool(mcpserver.Spec{Name: "lint", Title: "Check the notebook's health", Description: lintDescription,
			Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.check),
		mcpserver.NewTool(mcpserver.Spec{Name: "pending_sources", Title: "List undigested sources", Description: pendingSourcesDescription,
			Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.pendingSources),
		mcpserver.NewTool(mcpserver.Spec{Name: "upgrade_verify", Title: "Check the notebook's upgrade",
			Description: upgradeVerifyDescription, Annotations: read, Entry: mcpserver.BothEntries, Notebook: true}, t.verify),
		mcpserver.NewTool(mcpserver.Spec{Name: "upgrade_submit", Title: "Submit the notebook's upgrade",
			Description: upgradeSubmitDescription, Annotations: mcpserver.Annotations{}, Entry: mcpserver.BothEntries, Notebook: true},
			t.submit),
	}
}

type tools struct {
	lint     LintUseCase
	upgrades UpgradeUseCase
}

const lintDescription = `Check this notebook's health from a snapshot for this call. Omit rule for counts and previews of the applicable rules; limit and cursor require rule. Set rule to read its items, then continue with next_cursor; pages may move between calls. count_capped marks a lower bound, not the end of pagination. target_cut marks a shortened display target, not its full identity; use item_key. Page and source items include their full paths.`

const pendingSourcesDescription = `List an LLM Wiki's undigested sources from a snapshot for this call. LLM Wiki only: a standard notebook is refused. This is the undigested_sources health check, with each source's path, kind, bytes and original created_at; empty source pages are omitted. Continue with next_cursor; pages may move between calls. Each item keeps its complete item_key and cursor.`

const upgradeVerifyDescription = `Check this notebook's upgrade to an LLM Wiki, from a snapshot for this call, against an LLM Wiki's contract with the source root the upgrade proposes. Omit rule for every gate, each health check's count and first items, and the counts; limit and cursor require rule. Set rule to read its items, then continue with next_cursor. passed is true once every gate passes: then upgrade_submit. The notebook's admins only, while it is upgrading or reviewing.`

const upgradeSubmitDescription = `Submit this notebook's upgrade for its admins' review: it freezes the notebook for everyone, your writes too, and checks it again from that moment; once every gate passes it stays in review with its report kept, an admin approves or rejects it in the web. Otherwise it is upgrading again, refused with wiki.upgrade_not_ready (detail names the gates) or wiki.upgrade_stale_index. Run upgrade_verify first. wiki.upgrade_state whose detail says a previous submission is still being checked: wait, then upgrade_verify.`
