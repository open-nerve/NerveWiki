package domain

import (
	"strings"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

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
	// ErrNoAdmin: a member or a guest would join a workspace that has no
	// active admin, its only one deactivated alone in it (rule three).
	ErrNoAdmin = shared.NewError(shared.KindConflict, "workspace.no_admin",
		"The workspace has no admin: an admin has to come back to it before anyone else can join.")
	// ErrAlreadyInvited: the address has a pending invitation to the
	// workspace (422).
	ErrAlreadyInvited = shared.Invalid(shared.FieldError{Field: "email", Code: shared.FieldDuplicate,
		Message: "has a pending invitation to this workspace"})
	// ErrAlreadyMember: the address is an active member's (422).
	ErrAlreadyMember = shared.Invalid(shared.FieldError{Field: "email", Code: shared.FieldNotAllowed,
		Message: "belongs to a member of this workspace"})
	// ErrInvitationNotFound: no pending invitation has the id, its token is
	// not the one sent, its workspace is deleted, or the caller cannot see
	// the workspace.
	ErrInvitationNotFound = shared.NewError(shared.KindNotFound, "workspace.invitation_not_found",
		"No such invitation: it may have been accepted or withdrawn.")
	// ErrInvitationEmailMismatch: the caller's address is not the one
	// invited.
	ErrInvitationEmailMismatch = shared.NewError(shared.KindForbidden, "workspace.invitation_email_mismatch",
		"The invitation was sent to another e-mail address: sign in with that one.")
)

// ErrSoleAdminOf refuses a deactivation by rule two: workspace.sole_admin
// (409), the code of ErrSoleAdmin, for another reason. Its detail names the
// workspaces by slug, for the administrator's command to print: whoever
// asks is the account itself or the server's administrator.
func ErrSoleAdminOf(slugs []string) *shared.Error {
	return shared.NewError(shared.KindConflict, ErrSoleAdmin.Code, "The account is the only admin of workspaces that have "+
		"other members ("+strings.Join(slugs, ", ")+"): make another member an admin of each first.")
}
