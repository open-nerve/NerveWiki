package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The module's extension points (M2 design 8). The registrants are built
// from the pool alone and composed in bootstrap's registrants.go; their
// statements reach the transaction through the context.

// EndCause is why memberships end.
type EndCause string

// The causes.
const (
	EndRemoved     EndCause = "removed"
	EndLeft        EndCause = "left"
	EndDeactivated EndCause = "deactivated"
)

// MembershipEnd is an account's memberships of workspaces ending: one
// workspace when it is removed or leaves, every one when it is deactivated.
type MembershipEnd struct {
	UserID       uuid.UUID
	WorkspaceIDs []uuid.UUID
	Cause        EndCause
	By           uuid.UUID // who ends them: an admin removes, the account leaves or is deactivated
	At           time.Time
}

// MembershipEndVetoer may refuse a membership end, with the workspaces'
// rows locked and before any write: the *shared.Error it returns rolls the
// whole transaction back and is the answer.
type MembershipEndVetoer interface {
	VetoMembershipEnd(ctx context.Context, e MembershipEnd) error
}

// MembershipEndSubscriber follows a membership end, after the memberships
// were written and in their transaction: an error rolls everything back.
type MembershipEndSubscriber interface {
	MembershipEnded(ctx context.Context, e MembershipEnd) error
}

// WorkspaceDeletion is a workspace being deleted. A subscriber deletes its
// own rows at At, so the cleanup removes them with the workspace.
type WorkspaceDeletion struct {
	WorkspaceID uuid.UUID
	By          uuid.UUID
	At          time.Time
}

// WorkspaceDeletionSubscriber follows a deletion, after the workspace and
// its members were deleted and in their transaction: an error rolls
// everything back. A deletion has no vetoer (M2 design 4).
type WorkspaceDeletionSubscriber interface {
	WorkspaceDeleted(ctx context.Context, d WorkspaceDeletion) error
}

// MembershipRestore is an ended membership active again, the same row with
// a new role: by an accepted invitation, or (M2/P4) an admin's
// reactivation.
type MembershipRestore struct {
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	Role        shared.WorkspaceRole
	By          uuid.UUID // who restores it: the account accepting, or the admin
	At          time.Time
}

// MembershipRestoreSubscriber follows a restore, after the membership was
// written and in its transaction: an error rolls everything back.
type MembershipRestoreSubscriber interface {
	MembershipRestored(ctx context.Context, r MembershipRestore) error
}
