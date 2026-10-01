package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's problems (M3 design 5).
var (
	// ErrNotFound: no notebook has the id, it is deleted, or the caller has
	// no role in it.
	ErrNotFound = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
	// ErrWorkspaceNotFound: the workspace module's workspace.not_found, for
	// the operations that name a workspace: no workspace has the slug, it
	// is deleted, or the caller is not an active member of it.
	ErrWorkspaceNotFound = shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace.")
	// ErrMemberNotFound: no active membership has the id, or the caller has
	// no role in its notebook; for leaveNotebook, the caller is no active
	// member of the notebook, using it by its workspace access alone.
	ErrMemberNotFound = shared.NewError(shared.KindNotFound, "notebook.member_not_found", "No such member.")
	// ErrOwnMembership: an admin changes or removes their own membership.
	// Since no admin can, the one who acts is still an admin after.
	ErrOwnMembership = shared.NewError(shared.KindConflict, "notebook.own_membership",
		"You cannot change or remove your own membership.")
	// ErrSoleAdmin: the notebook's only active admin would leave it (rule
	// one).
	ErrSoleAdmin = shared.NewError(shared.KindConflict, "notebook.sole_admin",
		"The notebook's only admin cannot leave it: make another member an admin, or delete the notebook.")
)
