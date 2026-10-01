package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's actions: each has a row in the access module's rule table
// (bootstrap's actions test).
const (
	ActionRead         shared.Action = "workspace.read"
	ActionUpdate       shared.Action = "workspace.update"
	ActionDelete       shared.Action = "workspace.delete"
	ActionLeave        shared.Action = "workspace.leave"
	ActionListMembers  shared.Action = "workspace_member.list"
	ActionUpdateMember shared.Action = "workspace_member.update"
	ActionRemoveMember shared.Action = "workspace_member.remove"

	ActionListInvitations  shared.Action = "workspace_invitation.list"
	ActionCreateInvitation shared.Action = "workspace_invitation.create"
	ActionDeleteInvitation shared.Action = "workspace_invitation.delete"
)

// Actions lists the module's actions.
func Actions() []shared.Action {
	return []shared.Action{
		ActionRead, ActionUpdate, ActionDelete, ActionLeave,
		ActionListMembers, ActionUpdateMember, ActionRemoveMember,
		ActionListInvitations, ActionCreateInvitation, ActionDeleteInvitation,
	}
}
