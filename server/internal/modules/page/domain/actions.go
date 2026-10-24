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
	// ActionReleaseEditLock ends whichever edit session holds a page's
	// lock (M5 design 4.2).
	ActionReleaseEditLock shared.Action = "page.release_edit_lock"
	// ActionToggleTask ticks or clears a task item of a page's content
	// (M5/P6): a write of its content, by the same rule.
	ActionToggleTask shared.Action = "page.toggle_task"
	// ActionHistory reads a notebook's history: a page's versions, alive
	// or in the trash (M8 design 4.14).
	ActionHistory shared.Action = "page.history"
	// ActionRestoreVersion writes a version's content as a page's.
	ActionRestoreVersion shared.Action = "page.restore_version"
	// ActionRestore brings a trash item back, a page or an attachment with
	// what was deleted with it (M8 design 4.14).
	ActionRestore shared.Action = "node.restore"
	// ActionRevert reverts a changeset of the notebook, whoever made it
	// (M8 design 4.14).
	ActionRevert shared.Action = "changeset.revert"
	// ActionSearch searches a workspace, each notebook of it the caller
	// sees (M8/P5 design 3.8): a workspace's action, any member's.
	ActionSearch shared.Action = "page.search"
	// ActionBatch writes a batch of a notebook over MCP, its ops one change
	// (M9 design 4.11): a writer's, as each op's own action is, page.create,
	// page.write, node.move, node.rename and node.delete, so that its ops
	// are not decided one by one. When one of those comes to have another
	// rule than a writer's, the batch decides each op's (M9/P3 design
	// 3.13.4).
	ActionBatch shared.Action = "page.batch"
	// ActionManageSource is an LLM Wiki's source area's tree operations,
	// its source root's own rename and move among them (M10 design 4.7):
	// the mode guard asks it beyond the operation's own action, once a
	// unit, of a step that needs it (CheckContract). No operation is it.
	ActionManageSource shared.Action = "page.manage_source"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{ActionList, ActionRead, ActionCreate, ActionRename, ActionMove, ActionDelete, ActionWrite, ActionEdit,
		ActionReleaseEditLock, ActionToggleTask, ActionHistory, ActionRestoreVersion, ActionRestore, ActionRevert, ActionSearch, ActionBatch,
		ActionManageSource}
}
