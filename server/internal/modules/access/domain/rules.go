// Package domain holds the permission rules (v0.1 design 13.1, item 3):
// the rule table, keyed by the modules' action names, and the decision, a
// pure function of the facts the app layer reads.
package domain

import (
	"maps"
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Level is what a rule decides on. M2 has the workspace level; M3 adds the
// notebook's, with facts of its own, beside it (v0.1 design 12.4).
type Level int

// The levels.
const (
	// LevelWorkspace: the caller's active membership of the target
	// workspace, and its role.
	LevelWorkspace Level = iota + 1
)

// Rule is an action's row: its level and the roles it allows.
type Rule struct {
	Level     Level
	Workspace []shared.WorkspaceRole
}

// every is the three workspace roles.
func every() []shared.WorkspaceRole { return shared.WorkspaceRoles() }

// admins is the workspace's admins alone.
func admins() []shared.WorkspaceRole { return []shared.WorkspaceRole{shared.WorkspaceAdmin} }

// rules is the table: one row per action. An action without a row is
// allowed nothing. Each module's Actions() and the table's keys are the
// same set (bootstrap's actions test).
func rules() map[shared.Action]Rule {
	return map[shared.Action]Rule{
		"workspace.read":          {Level: LevelWorkspace, Workspace: every()},
		"workspace.update":        {Level: LevelWorkspace, Workspace: admins()},
		"workspace.delete":        {Level: LevelWorkspace, Workspace: admins()},
		"workspace.leave":         {Level: LevelWorkspace, Workspace: every()},
		"workspace_member.list":   {Level: LevelWorkspace, Workspace: every()},
		"workspace_member.update": {Level: LevelWorkspace, Workspace: admins()},
		"workspace_member.remove": {Level: LevelWorkspace, Workspace: admins()},
	}
}

// RuleFor returns action's rule, and whether it has one.
func RuleFor(action shared.Action) (Rule, bool) {
	r, ok := rules()[action]
	return r, ok
}

// RuleKeys returns the actions the table has a rule for, sorted.
func RuleKeys() []shared.Action {
	return slices.Sorted(maps.Keys(rules()))
}
