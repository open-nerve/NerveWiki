package app

import (
	"context"
	"fmt"

	"github.com/open-nerve/NerveWiki/server/internal/modules/access/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Authorizer is shared.Authorizer over the rule table: it finds the
// action's rule, reads the facts of its level, and decides.
type Authorizer struct {
	memberships WorkspaceMemberships
}

// NewAuthorizer returns the Authorizer reading memberships.
func NewAuthorizer(memberships WorkspaceMemberships) *Authorizer {
	return &Authorizer{memberships: memberships}
}

// Authorize implements shared.Authorizer. It reads on every call: a
// membership removed or a role changed applies to the next request.
func (a *Authorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	rule, ok := domain.RuleFor(action)
	if !ok {
		return shared.Grant{}, fmt.Errorf("access: no rule for action %q", action)
	}
	role, active, err := a.memberships.RoleOf(ctx, t.WorkspaceID, actor.UserID)
	if err != nil {
		return shared.Grant{}, fmt.Errorf("access: read the workspace role: %w", err)
	}
	return domain.Decide(rule, domain.Facts{Workspace: domain.Membership{Active: active, Role: role}})
}
