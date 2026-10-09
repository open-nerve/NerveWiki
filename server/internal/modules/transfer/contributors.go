package transfer

import "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/app"

// The export contributors (M7 design 4.10, 8; M7/P5 design 3.10): an
// extension point M7 builds and M10 registers, the LLM Wiki's virtual
// index and log. bootstrap hands the registrants to Deps.Contributors;
// M7's are none.
type (
	// ExportContributor adds files of its own to an export, in its
	// snapshot, in the order registered; its first error stops the export.
	ExportContributor = app.Contributor
	// ExportScope is what an export holds: its notebook, the page exported
	// with its subtree, and its nodes with their paths in the vault.
	ExportScope = app.Scope
	// ExportScopeNode is a node of an export.
	ExportScopeNode = app.ScopeNode
	// ExportSink takes a contributor's files: a path where a node, another
	// contributed file or the export's own .nerve is fails the export.
	ExportSink = app.Sink
)
