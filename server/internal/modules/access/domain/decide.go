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

// Notebook is what the caller has of a notebook: whether it is there, not
// deleted, in the target's workspace; how open it is to the workspace; and
// the role of the caller's active membership of it, "" for none.
type Notebook struct {
	Found  bool
	Access shared.WorkspaceAccess
	Role   shared.NotebookRole
}

// Facts are what a decision reads: the workspace's for both levels, the
// notebook's for the notebook level.
type Facts struct {
	Workspace Membership
	Notebook  Notebook
}

// Decide applies rule to the facts: a caller who is no active member does
// not see the workspace (shared.ErrNotVisible); a member whose role the rule
// does not list may not act (403 forbidden). A role outside the three,
// which the database's CHECK keeps out, is in no rule's set.
func Decide(rule Rule, f Facts) (shared.Grant, error) {
	switch rule.Level {
	case LevelWorkspace:
		return decideWorkspace(rule, f.Workspace)
	case LevelNotebook:
		return decideNotebook(rule, f)
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

// decideNotebook decides at the notebook level (M3/P1 design 3.4): the
// caller sees a notebook of a workspace it is an active member of, with an
// effective role in it; with none, the notebook is not visible, to the
// workspace's admins too (v0.1 design 3.4). A role the rule does not list
// may not act.
func decideNotebook(rule Rule, f Facts) (shared.Grant, error) {
	if !f.Workspace.Active || !f.Notebook.Found {
		return shared.Grant{}, shared.ErrNotVisible
	}
	role := shared.EffectiveNotebookRole(f.Notebook.Role, f.Notebook.Access, f.Workspace.Role)
	switch {
	case role == "":
		return shared.Grant{}, shared.ErrNotVisible
	case !slices.Contains(rule.Notebook, role):
		return shared.Grant{}, shared.Forbidden()
	}
	return shared.Grant{WorkspaceRole: f.Workspace.Role, NotebookRole: role}, nil
}
