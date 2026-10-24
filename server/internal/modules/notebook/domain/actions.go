package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions: each has a row in the access module's rule table
// (bootstrap's actions test).
const (
	ActionList         shared.Action = "notebook.list"
	ActionCreate       shared.Action = "notebook.create"
	ActionRead         shared.Action = "notebook.read"
	ActionUpdate       shared.Action = "notebook.update"
	ActionDelete       shared.Action = "notebook.delete"
	ActionListMembers  shared.Action = "notebook_member.list"
	ActionAddMember    shared.Action = "notebook_member.add"
	ActionUpdateMember shared.Action = "notebook_member.update"
	ActionRemoveMember shared.Action = "notebook_member.remove"
	ActionLeave        shared.Action = "notebook.leave"
	// A notebook's mode changes by its admins (M10 design 4.5): P1's
	// downgrade; M11's upgrade may take it too.
	ActionChangeMode shared.Action = "notebook.change_mode"
	// The ownerless notebooks and their audit events: the workspace's
	// admins', at the workspace level (M3 design 4).
	ActionListOwnerless   shared.Action = "notebook_ownerless.list"
	ActionTakeOver        shared.Action = "notebook_ownerless.take_over"
	ActionDeleteOwnerless shared.Action = "notebook_ownerless.delete"
	ActionListAudit       shared.Action = "notebook_audit.list"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{
		ActionList, ActionCreate, ActionRead, ActionUpdate, ActionDelete,
		ActionListMembers, ActionAddMember, ActionUpdateMember, ActionRemoveMember, ActionLeave, ActionChangeMode,
		ActionListOwnerless, ActionTakeOver, ActionDeleteOwnerless, ActionListAudit,
	}
}
