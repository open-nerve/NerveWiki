package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions (M4/P1 design 3.5): each has a row in the access
// module's rule table (bootstrap's actions test). An action on any node,
// which M7's attachments are too, is node's; one on a page alone, page's.
const (
	ActionList   shared.Action = "node.list"
	ActionRead   shared.Action = "page.read"
	ActionCreate shared.Action = "page.create"
	ActionRename shared.Action = "node.rename"
	ActionMove   shared.Action = "node.move"
	ActionDelete shared.Action = "node.delete"
	// ActionWrite writes a page's content.
	ActionWrite shared.Action = "page.write"
	// ActionEdit opens an edit session of a page and keeps it alive; ending
	// one needs only that it is the caller's.
	ActionEdit shared.Action = "page.edit"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{ActionList, ActionRead, ActionCreate, ActionRename, ActionMove, ActionDelete, ActionWrite, ActionEdit}
}
