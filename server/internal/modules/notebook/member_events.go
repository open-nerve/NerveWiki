package notebook

import "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"

// WorkspaceMemberAdded is a workspace's new membership, field by field as
// the workspace module's addition tells it: bootstrap converts
// workspace.MembershipAddition.
type WorkspaceMemberAdded = app.WorkspaceMemberAdded

// WorkspaceRoleChanged is a workspace member's role changed, field by
// field as the workspace module's role change tells it: bootstrap converts
// workspace.MemberRoleChange.
type WorkspaceRoleChanged = app.WorkspaceRoleChanged

// WorkspaceMemberEvents is the module's registrant of the workspace
// module's addition and role change (M3/P2 design 3.6): it tells the
// visibility's subscribers of an account given or taken a default role.
type WorkspaceMemberEvents = app.WorkspaceMemberEvents

// NewWorkspaceMemberEvents builds the registrant, with the visibility's
// subscribers it calls in the workspace's transaction. It reads nothing.
func NewWorkspaceMemberEvents(subscribers []VisibilitySubscriber) WorkspaceMemberEvents {
	return app.WorkspaceMemberEvents{Subscribers: subscribers}
}
