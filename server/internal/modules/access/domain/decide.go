package domain

import (
	"fmt"
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Membership is the caller's membership of a workspace: whether it is
// active, in a workspace not deleted, and its role.
type Membership struct {
	Active bool
	Role   shared.WorkspaceRole
}

// Facts are what a decision reads.
type Facts struct {
	Workspace Membership
}

// Decide applies rule to the facts: a caller who is no active member does
// not see the workspace (shared.ErrNotVisible); a member whose role the rule
// does not list may not act (403 forbidden). A role outside the three,
// which the database's CHECK keeps out, is in no rule's set.
func Decide(rule Rule, f Facts) (shared.Grant, error) {
	switch rule.Level {
	case LevelWorkspace:
		return decideWorkspace(rule, f.Workspace)
	}
	return shared.Grant{}, fmt.Errorf("access: a rule of unknown level %d", rule.Level)
}

func decideWorkspace(rule Rule, m Membership) (shared.Grant, error) {
	switch {
	case !m.Active:
		return shared.Grant{}, shared.ErrNotVisible
	case !slices.Contains(rule.Workspace, m.Role):
		return shared.Grant{}, shared.Forbidden()
	}
	return shared.Grant{WorkspaceRole: m.Role}, nil
}
