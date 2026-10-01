package domain

import "github.com/open-nerve/NerveWiki/server/internal/shared"

// The module's problems (M2 design 5).
var (
	// ErrNotFound: no workspace has the slug, it is deleted, or the caller
	// is not an active member of it.
	ErrNotFound = shared.NewError(shared.KindNotFound, "workspace.not_found", "No such workspace.")
	// ErrCreationDisabled: workspace.creation_enabled is false; the
	// server's administrator creates them (nervewiki workspaces create).
	ErrCreationDisabled = shared.NewError(shared.KindForbidden, "workspace.creation_disabled",
		"Creating workspaces is disabled on this instance.")
	// ErrSlugTaken: a workspace not deleted has the slug.
	ErrSlugTaken = shared.NewError(shared.KindConflict, "workspace.slug_taken", "The slug is taken.")
	// ErrMemberNotFound: no active membership has the id, or the caller
	// cannot see its workspace.
	ErrMemberNotFound = shared.NewError(shared.KindNotFound, "workspace.member_not_found", "No such member.")
	// ErrOwnMembership: an admin changes or removes their own membership.
	// Since no admin can, the one who acts is still an admin after.
	ErrOwnMembership = shared.NewError(shared.KindConflict, "workspace.own_membership",
		"You cannot change or remove your own membership.")
	// ErrSoleAdmin: the workspace's only active admin would leave it.
	ErrSoleAdmin = shared.NewError(shared.KindConflict, "workspace.sole_admin",
		"The workspace's only admin cannot leave it: make another member an admin, or delete the workspace.")
)
