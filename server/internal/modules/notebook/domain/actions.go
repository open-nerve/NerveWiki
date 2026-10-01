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
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{
		ActionList, ActionCreate, ActionRead, ActionUpdate, ActionDelete,
		ActionListMembers, ActionAddMember, ActionUpdateMember, ActionRemoveMember, ActionLeave,
	}
}
