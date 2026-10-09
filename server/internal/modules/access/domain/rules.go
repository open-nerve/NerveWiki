// Package domain holds the permission rules (v0.1 design 13.1, item 3):
// the rule table, keyed by the modules' action names, and the decision, a
// pure function of the facts the app layer reads.
package domain

import (
	"maps"
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Level is what a rule decides on: the workspace's (M2), or the
// notebook's (M3), each with facts of its own (v0.1 design 12.4).
type Level int

// The levels.
const (
	// LevelWorkspace: the caller's active membership of the target
	// workspace, and its role.
	LevelWorkspace Level = iota + 1
	// LevelNotebook: the caller's effective role in the target notebook,
	// which needs its active membership of the notebook's workspace.
	LevelNotebook
)

// Rule is an action's row: its level and the roles it allows, workspace
// roles at the workspace level and notebook roles at the notebook's.
type Rule struct {
	Level     Level
	Workspace []shared.WorkspaceRole
	Notebook  []shared.NotebookRole
}

// every is the three workspace roles.
func every() []shared.WorkspaceRole { return shared.WorkspaceRoles() }

// admins is the workspace's admins alone.
func admins() []shared.WorkspaceRole { return []shared.WorkspaceRole{shared.WorkspaceAdmin} }

// adminsAndMembers is the workspace's admins and members: not its guests.
func adminsAndMembers() []shared.WorkspaceRole {
	return []shared.WorkspaceRole{shared.WorkspaceAdmin, shared.WorkspaceMember}
}

// readers is the three notebook roles: each reads.
func readers() []shared.NotebookRole { return shared.NotebookRoles() }

// notebookAdmins is the notebook's admins alone.
func notebookAdmins() []shared.NotebookRole { return []shared.NotebookRole{shared.NotebookAdmin} }

// writers is the notebook's admins and editors: those who write its pages.
func writers() []shared.NotebookRole {
	return []shared.NotebookRole{shared.NotebookAdmin, shared.NotebookEditor}
}

// rules is the table: one row per action. An action without a row is
// allowed nothing. Each module's Actions() and the table's keys are the
// same set (bootstrap's actions test).
func rules() map[shared.Action]Rule {
	return map[shared.Action]Rule{
		"workspace.read":              {Level: LevelWorkspace, Workspace: every()},
		"workspace.update":            {Level: LevelWorkspace, Workspace: admins()},
		"workspace.delete":            {Level: LevelWorkspace, Workspace: admins()},
		"workspace.leave":             {Level: LevelWorkspace, Workspace: every()},
		"workspace_member.list":       {Level: LevelWorkspace, Workspace: every()},
		"workspace_member.update":     {Level: LevelWorkspace, Workspace: admins()},
		"workspace_member.remove":     {Level: LevelWorkspace, Workspace: admins()},
		"workspace_invitation.list":   {Level: LevelWorkspace, Workspace: admins()},
		"workspace_invitation.create": {Level: LevelWorkspace, Workspace: admins()},
		"workspace_invitation.delete": {Level: LevelWorkspace, Workspace: admins()},
		// The list is every member's: it shows the notebooks each sees.
		"notebook.list":   {Level: LevelWorkspace, Workspace: every()},
		"notebook.create": {Level: LevelWorkspace, Workspace: adminsAndMembers()},
		"notebook.read":   {Level: LevelNotebook, Notebook: readers()},
		"notebook.update": {Level: LevelNotebook, Notebook: notebookAdmins()},
		"notebook.delete": {Level: LevelNotebook, Notebook: notebookAdmins()},
		// Any role in a notebook sees its members, and may leave it.
		"notebook.leave":         {Level: LevelNotebook, Notebook: readers()},
		"notebook_member.list":   {Level: LevelNotebook, Notebook: readers()},
		"notebook_member.add":    {Level: LevelNotebook, Notebook: notebookAdmins()},
		"notebook_member.update": {Level: LevelNotebook, Notebook: notebookAdmins()},
		"notebook_member.remove": {Level: LevelNotebook, Notebook: notebookAdmins()},
		// A notebook without an admin is the workspace's admins' to take
		// over or delete; no notebook role reaches it.
		"notebook_ownerless.list":      {Level: LevelWorkspace, Workspace: admins()},
		"notebook_ownerless.take_over": {Level: LevelWorkspace, Workspace: admins()},
		"notebook_ownerless.delete":    {Level: LevelWorkspace, Workspace: admins()},
		"notebook_audit.list":          {Level: LevelWorkspace, Workspace: admins()},
		// A notebook's pages: any role reads them, its writers write them
		// (M4 design 5).
		"node.list":   {Level: LevelNotebook, Notebook: readers()},
		"page.read":   {Level: LevelNotebook, Notebook: readers()},
		"page.create": {Level: LevelNotebook, Notebook: writers()},
		"node.rename": {Level: LevelNotebook, Notebook: writers()},
		"node.move":   {Level: LevelNotebook, Notebook: writers()},
		"node.delete": {Level: LevelNotebook, Notebook: writers()},
		"page.write":  {Level: LevelNotebook, Notebook: writers()},
		"page.edit":   {Level: LevelNotebook, Notebook: writers()},
		// A task item's tick is a write of the content (M5/P6).
		"page.toggle_task": {Level: LevelNotebook, Notebook: writers()},
		// Its admins release a page's edit lock (M5 design 4.2).
		"page.release_edit_lock": {Level: LevelNotebook, Notebook: notebookAdmins()},

		// The link index's reads (M6/P5 design 2): whoever reads the notebook.
		"backlink.list":      {Level: LevelNotebook, Notebook: readers()},
		"page_property.read": {Level: LevelNotebook, Notebook: readers()},
		"tag.list":           {Level: LevelNotebook, Notebook: readers()},
		"tag.read":           {Level: LevelNotebook, Notebook: readers()},
		"link_target.list":   {Level: LevelNotebook, Notebook: readers()},
		// Where a page made for a link would go (M6/P6 design 2): the half
		// of a write, its writers'.
		"link_landing.read": {Level: LevelNotebook, Notebook: writers()},

		// A notebook's attachments (M7/P2 design 3.5–3.7): its writers
		// upload them, any role reads them; a rename, a move and a deletion
		// are the node's.
		"asset.upload": {Level: LevelNotebook, Notebook: writers()},
		"asset.read":   {Level: LevelNotebook, Notebook: readers()},

		// A notebook's imports and exports (M7/P5 design 3.14): an export
		// is a read, so any role starts one, reads its jobs and cancels
		// them; the use cases keep another's jobs to the notebook's admins.
		"transfer.export": {Level: LevelNotebook, Notebook: readers()},
		"transfer.read":   {Level: LevelNotebook, Notebook: readers()},
		"transfer.cancel": {Level: LevelNotebook, Notebook: readers()},
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
