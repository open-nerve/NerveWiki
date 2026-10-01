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

// WorkspaceFinder finds a workspace, unlocked.
type WorkspaceFinder interface {
	// FindWorkspaceBySlug returns the workspace not deleted with slug;
	// ErrNotFound when there is none.
	FindWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error)
	// FindWorkspaceByID is FindWorkspaceBySlug by the workspace's id.
	FindWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error)
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

// WorkspaceSharer locks a workspace row FOR SHARE until the transaction
// ends: the invitations' writes take it, which run beside each other, while
// a change of the workspace or of its members waits for them, and they for
// it (M2/P3 design 3.3). Each returns ErrNotFound when no workspace not
// deleted matches, a deletion committed while it waited too.
type WorkspaceSharer interface {
	ShareWorkspaceBySlug(ctx context.Context, slug string) (domain.Workspace, error)
	ShareWorkspaceByID(ctx context.Context, id uuid.UUID) (domain.Workspace, error)
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
	// FindMembership returns userID's membership of workspaceID, and
	// whether it is active: an ended one too; ErrNotFound when there is
	// none.
	FindMembership(ctx context.Context, workspaceID, userID uuid.UUID) (domain.Member, bool, error)
}

// MemberUpdater changes memberships of workspaces the transaction has
// locked.
type MemberUpdater interface {
	// AddMember inserts m, added by by at m.CreatedAt.
	AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error
	// RestoreMember makes the ended membership id active again with role,
	// updated by by at now; when it was first created stays.
	RestoreMember(ctx context.Context, id uuid.UUID, role shared.WorkspaceRole, by uuid.UUID, now time.Time) error
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

// MemberProfiles reads accounts' profiles, unlocked: identity's directory,
// adapted by bootstrap.
type MemberProfiles interface {
	// MemberProfiles returns the profiles of the accounts userIDs, by id.
	MemberProfiles(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]Profile, error)
}

// AccountFinder finds an account by its address, unlocked: identity's
// directory.
type AccountFinder interface {
	// AccountIDByEmail returns the id of the account of email, a
	// normalized address, and whether there is one.
	AccountIDByEmail(ctx context.Context, email string) (uuid.UUID, bool, error)
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
	// ListPendingInvitations returns workspaceID's pending invitations,
	// newest first.
	ListPendingInvitations(ctx context.Context, workspaceID uuid.UUID) ([]domain.Invitation, error)
}

// InvitationUpdater changes the invitations of workspaces the transaction
// has locked. Lock order: an invitation's row after its workspace's, before
// any membership's (M2 design 8).
type InvitationUpdater interface {
	// LockPendingInvitation reads the pending invitation id again, locked
	// FOR UPDATE until the transaction ends; ErrNotFound when it is no
	// longer pending, an acceptance or a deletion committed while it
	// waited too.
	LockPendingInvitation(ctx context.Context, id uuid.UUID) (domain.Invitation, error)
	// CreateInvitation inserts inv, created by by at inv.CreatedAt:
	// domain.ErrAlreadyInvited when its workspace has a pending invitation
	// to its address.
	CreateInvitation(ctx context.Context, inv domain.Invitation, by uuid.UUID) error
	// DeleteInvitation deletes the invitation id softly at now, by by.
	DeleteInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error
	// AcceptInvitation marks the invitation id accepted, and deleted, at
	// now, by by.
	AcceptInvitation(ctx context.Context, id, by uuid.UUID, now time.Time) error
	// DeleteInvitationsTo deletes the pending invitations of workspaceIDs
	// to email softly at now, by by.
	DeleteInvitationsTo(ctx context.Context, workspaceIDs []uuid.UUID, email string, by uuid.UUID, now time.Time) error
	// DeleteInvitationsOf deletes every pending invitation of workspaceID
	// softly at now, by by.
	DeleteInvitationsOf(ctx context.Context, workspaceID, by uuid.UUID, now time.Time) error
}
