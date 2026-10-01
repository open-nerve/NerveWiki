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
	notebooks   NotebookFacts
}

// NewAuthorizer returns the Authorizer reading memberships and notebooks.
func NewAuthorizer(memberships WorkspaceMemberships, notebooks NotebookFacts) *Authorizer {
	return &Authorizer{memberships: memberships, notebooks: notebooks}
}

// Authorize implements shared.Authorizer. It reads on every call: a
// membership removed or a role changed applies to the next request. At the
// notebook level it reads the notebook first: a notebook of another
// workspace than the target's is not there.
func (a *Authorizer) Authorize(ctx context.Context, actor shared.Actor, action shared.Action, t shared.Target) (shared.Grant, error) {
	rule, ok := domain.RuleFor(action)
	if !ok {
		return shared.Grant{}, fmt.Errorf("access: no rule for action %q", action)
	}
	var facts domain.Facts
	if rule.Level == domain.LevelNotebook {
		nb, err := a.notebooks.NotebookFacts(ctx, t.NotebookID, actor.UserID)
		if err != nil {
			return shared.Grant{}, fmt.Errorf("access: read the notebook: %w", err)
		}
		facts.Notebook = domain.Notebook{Found: nb.Found && nb.WorkspaceID == t.WorkspaceID, Access: nb.Access, Role: nb.Role}
	}
	role, active, err := a.memberships.RoleOf(ctx, t.WorkspaceID, actor.UserID)
	if err != nil {
		return shared.Grant{}, fmt.Errorf("access: read the workspace role: %w", err)
	}
	facts.Workspace = domain.Membership{Active: active, Role: role}
	return domain.Decide(rule, facts)
}
