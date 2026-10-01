package bootstrap

import (
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
)

// deactivationRegistrants are the modules that take part in an account's
// deactivation: the vetoers that may refuse it and the subscribers that
// follow it (M1 design 8). serve and the command line both take them from
// here, so that a registrant a module adds reaches the self-service
// deactivation and the administrator's alike (M1/P4 design 3.6). M1 has
// none: the first ones come with the workspaces (M2).
func deactivationRegistrants() ([]identity.DeactivationVetoer, []identity.DeactivationSubscriber) {
	return nil, nil
}

// workspaceExtensions are the registrants of the workspace module's
// extension points (M2 design 8): the vetoers that may refuse a membership
// end, the subscribers that follow one, and those that follow a workspace's
// deletion.
type workspaceExtensions struct {
	endVetoers          []workspace.MembershipEndVetoer
	endSubscribers      []workspace.MembershipEndSubscriber
	deletionSubscribers []workspace.WorkspaceDeletionSubscriber
}

// workspaceRegistrants are the modules that take part in the workspace
// module's membership ends and deletions. M2 has none: the notebooks' come
// with M3.
func workspaceRegistrants() workspaceExtensions {
	return workspaceExtensions{}
}
