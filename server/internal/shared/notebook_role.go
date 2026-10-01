package shared

// NotebookRole is a member's role in a notebook (v0.1 design 3.3), as the
// API and the database spell it. It is not a WorkspaceRole: both have an
// admin. Rules compare roles by set; which of two is higher only the order
// below says (EffectiveNotebookRole), never the values.
type NotebookRole string

// The three notebook roles.
const (
	NotebookAdmin  NotebookRole = "admin"
	NotebookEditor NotebookRole = "editor"
	NotebookReader NotebookRole = "reader"
)

// NotebookRoles are the three roles.
func NotebookRoles() []NotebookRole {
	return []NotebookRole{NotebookAdmin, NotebookEditor, NotebookReader}
}

// WorkspaceAccess is how open a notebook is to its workspace: the role it
// gives the workspace's members and admins who are not its members.
type WorkspaceAccess string

// The three degrees.
const (
	AccessNone   WorkspaceAccess = "none"
	AccessViewer WorkspaceAccess = "viewer"
	AccessEditor WorkspaceAccess = "editor"
)

// WorkspaceAccesses are the three degrees.
func WorkspaceAccesses() []WorkspaceAccess {
	return []WorkspaceAccess{AccessNone, AccessViewer, AccessEditor}
}

// EffectiveNotebookRole is the role a caller has in a notebook: the higher
// of explicit, the role of its membership ("" for none), and the role the
// notebook's access gives the caller's role in the workspace. That default
// goes to the workspace's admins and members, not its guests: a guest sees
// a notebook only as its member. "" is no role: the notebook is not
// visible. The access module's decision and the notebook module's list both
// take it from here, so the two agree (M3/P1 design 3.3).
func EffectiveNotebookRole(explicit NotebookRole, access WorkspaceAccess, workspace WorkspaceRole) NotebookRole {
	var byDefault NotebookRole
	if ReachedByAccess(workspace) {
		switch access {
		case AccessViewer:
			byDefault = NotebookReader
		case AccessEditor:
			byDefault = NotebookEditor
		}
	}
	if notebookRank(byDefault) > notebookRank(explicit) {
		return byDefault
	}
	if notebookRank(explicit) == 0 {
		return ""
	}
	return explicit
}

// ReachedByAccess reports whether a notebook's workspace access gives a
// caller of workspace role its default role: an admin's or a member's, not
// a guest's. The notebook list filters by it.
func ReachedByAccess(workspace WorkspaceRole) bool {
	return workspace == WorkspaceAdmin || workspace == WorkspaceMember
}

// notebookRank orders the roles: reader below editor below admin. A value
// outside the three ranks with none.
func notebookRank(r NotebookRole) int {
	switch r {
	case NotebookReader:
		return 1
	case NotebookEditor:
		return 2
	case NotebookAdmin:
		return 3
	}
	return 0
}
