// Package app holds the workspace module's use cases and the ports they
// need.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotFound is what a repository returns for a missing row.
var ErrNotFound = errors.New("not found")

// Clock tells the time: every business time comes from it.
type Clock interface {
	Now() time.Time
}

// Accounts is what identity offers the modules that give an account new
// access (v0.1 design 13.1, item 18): the first statement of such a
// transaction.
type Accounts interface {
	// ShareActiveAccount locks the account's row FOR SHARE until the
	// transaction ends and returns its address, read under the lock:
	// identity.account_deactivated (403) when the account is deactivated.
	ShareActiveAccount(ctx context.Context, id uuid.UUID) (email string, err error)
}

// Membership is a workspace and an account's role in it.
type Membership struct {
	Workspace domain.Workspace
	Role      shared.WorkspaceRole
}

// The repository's ports, each what its use cases need: the postgres
// adapter's Store implements them all.

// WorkspaceCreator writes a new workspace and its members.
type WorkspaceCreator interface {
	// CreateWorkspace inserts w, created by by at w.CreatedAt:
	// domain.ErrSlugTaken when a workspace not deleted has its slug.
	CreateWorkspace(ctx context.Context, w domain.Workspace, by uuid.UUID) error
	// AddMember inserts m, added by by at m.CreatedAt.
	AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error
}

// WorkspaceFinder finds a workspace by its slug.
type WorkspaceFinder interface {
	// FindWorkspaceBySlug returns the workspace not deleted with slug;
	// ErrNotFound when there is none.
	FindWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error)
}

// MembershipLister lists an account's workspaces.
type MembershipLister interface {
	// ListWorkspacesOf returns the workspaces not deleted of userID's
	// active memberships, with its role, by name, case-insensitively.
	ListWorkspacesOf(ctx context.Context, userID uuid.UUID) ([]Membership, error)
}

// SlugChecker tells whether a slug is taken.
type SlugChecker interface {
	// SlugTaken reports whether a workspace not deleted has slug.
	SlugTaken(ctx context.Context, slug string) (bool, error)
}

// WorkspaceLocker locks a workspace row FOR NO KEY UPDATE until the
// transaction ends: every change of a workspace and of its members takes
// it first, then decides (M2 design 8). Each returns ErrNotFound when no
// workspace not deleted matches, a deletion committed while it waited too.
type WorkspaceLocker interface {
	LockWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error)
	LockWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error)
}

// WorkspaceUpdater changes a workspace the transaction has locked.
type WorkspaceUpdater interface {
	// RenameWorkspace sets id's name, updated by by at now.
	RenameWorkspace(ctx context.Context, id uuid.UUID, name string, by uuid.UUID, now time.Time) error
	// DeleteWorkspace deletes id softly at now, by by.
	DeleteWorkspace(ctx context.Context, id, by uuid.UUID, now time.Time) error
}

// MemberFinder reads memberships.
type MemberFinder interface {
	// FindActiveMember returns the active membership id; ErrNotFound when
	// it does not exist, has ended or is deleted.
	FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error)
	// ListActiveMembers returns workspaceID's active memberships, by when
	// they joined.
	ListActiveMembers(ctx context.Context, workspaceID uuid.UUID) ([]domain.Member, error)
	// CountActiveAdmins counts workspaceID's active admins.
	CountActiveAdmins(ctx context.Context, workspaceID uuid.UUID) (int, error)
}

// MemberUpdater changes memberships of workspaces the transaction has
// locked.
type MemberUpdater interface {
	// UpdateMemberRole sets the membership id's role, updated by by at now.
	UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error
	// EndMemberships ends userID's active memberships of workspaceIDs at
	// now, by by.
	EndMemberships(ctx context.Context, userID uuid.UUID, workspaceIDs []uuid.UUID, by uuid.UUID, now time.Time) error
	// DeleteMembersOf deletes every membership of workspaceID softly, ended
	// ones too, at now, by by.
	DeleteMembersOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error
}

// Profile is what a member list shows of an account.
type Profile struct {
	DisplayName string
	Email       string
}

// MemberProfiles reads accounts' profiles, unlocked: identity's, adapted by
// bootstrap.
type MemberProfiles interface {
	// MemberProfiles returns the profiles of the accounts userIDs, by id.
	MemberProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]Profile, error)
}

// InvitationTokens makes and checks the invitations' tokens: the mac
// adapter's, whose key never reaches the app.
type InvitationTokens interface {
	// Token returns the token of the invitation id.
	Token(id uuid.UUID) string
	// Valid reports whether token is the invitation id's.
	Valid(id uuid.UUID, token string) bool
}

// InvitationFinder reads pending invitations, unlocked.
type InvitationFinder interface {
	// FindPendingInvitation returns the pending invitation id of a
	// workspace not deleted; ErrNotFound when there is none.
	FindPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error)
}
