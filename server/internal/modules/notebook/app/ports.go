// Package app holds the notebook module's use cases and the ports they
// need.
package app

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// ErrNotFound is what a repository returns for a missing row.
var ErrNotFound = errors.New("not found")

// Clock tells the time: every business time comes from it.
type Clock interface {
	Now() time.Time
}

// Workspaces is what the module reads of workspaces: the workspace
// module's, which bootstrap wires to it (M3/P1 design 3.5).
type Workspaces interface {
	// FindBySlug returns the id of the workspace not deleted with slug,
	// unlocked, and whether there is one.
	FindBySlug(ctx context.Context, slug string) (uuid.UUID, bool, error)
	// ShareByID locks the workspace not deleted with id FOR SHARE until the
	// transaction ends, and reports whether there is one.
	ShareByID(ctx context.Context, id uuid.UUID) (bool, error)
}

// WorkspaceMembers is what the module reads of a workspace's memberships:
// the workspace module's, which bootstrap wires to it (M3/P2 design 3.4).
type WorkspaceMembers interface {
	// RoleOf returns the role of userID's active membership of
	// workspaceID, and whether it has one, in the transaction ctx carries.
	RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error)
}

// Profile is what other accounts see of an account.
type Profile struct {
	DisplayName string
	Email       string
}

// MemberProfiles reads the accounts' profiles: identity's directory, which
// bootstrap converts.
type MemberProfiles interface {
	// MemberProfiles returns the profiles of the accounts ids, by id; an id
	// of no account is left out.
	MemberProfiles(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Profile, error)
}

// Listed is a notebook of a list as the repository reads it: with the
// caller's explicit role, "" for none, and its count of active members.
type Listed struct {
	Notebook    domain.Notebook
	Explicit    shared.NotebookRole
	MemberCount int
}

// The repository's ports, each what its use cases need: the postgres
// adapter's Store implements them all.

// NotebookCreator writes a new notebook and its members.
type NotebookCreator interface {
	// CreateNotebook inserts n, created by by at n.CreatedAt.
	CreateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error
	// AddMember inserts m, added by by at m.CreatedAt.
	AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error
}

// NotebookFinder reads a notebook, unlocked.
type NotebookFinder interface {
	// FindNotebook returns the notebook not deleted with id; ErrNotFound
	// when there is none.
	FindNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error)
	// CountMembers returns how many active members notebook id has.
	CountMembers(ctx context.Context, id uuid.UUID) (int, error)
}

// NotebookLister lists the notebooks a caller sees.
type NotebookLister interface {
	// ListNotebooks returns the notebooks not deleted of workspaceID that
	// userID is an active member of, and, when reached, those whose
	// workspace access is not none; by name, case-insensitively, then by
	// name and id.
	ListNotebooks(ctx context.Context, workspaceID, userID uuid.UUID, reached bool) ([]Listed, error)
}

// NotebookWriter changes a notebook under its lock.
type NotebookWriter interface {
	// LockNotebook returns the notebook not deleted with id, locked FOR NO
	// KEY UPDATE until the transaction ends; ErrNotFound when there is
	// none, a deletion committed while it waited too.
	LockNotebook(ctx context.Context, id uuid.UUID) (domain.Notebook, error)
	// UpdateNotebook writes n's name and access, by by at n.UpdatedAt.
	UpdateNotebook(ctx context.Context, n domain.Notebook, by uuid.UUID) error
	// DeleteNotebook soft-deletes notebook id and every member row of it,
	// by by at now.
	DeleteNotebook(ctx context.Context, id, by uuid.UUID, now time.Time) error
	// CountMembers is NotebookFinder's.
	CountMembers(ctx context.Context, id uuid.UUID) (int, error)
}

// MemberFinder reads memberships, unlocked or under the notebook's lock.
type MemberFinder interface {
	// ListMembers returns the notebook's active members, by when they
	// first joined, then by id.
	ListMembers(ctx context.Context, notebookID uuid.UUID) ([]domain.Member, error)
	// FindActiveMember returns the active membership with id; ErrNotFound
	// when there is none.
	FindActiveMember(ctx context.Context, id uuid.UUID) (domain.Member, error)
}

// MemberWriter changes a notebook's memberships under its lock.
type MemberWriter interface {
	// FindMemberOf returns userID's membership of the notebook, active or
	// ended; ErrNotFound when it never had one.
	FindMemberOf(ctx context.Context, notebookID, userID uuid.UUID) (domain.Member, error)
	// CountAdmins returns how many active admins the notebook has.
	CountAdmins(ctx context.Context, notebookID uuid.UUID) (int, error)
	// AddMember inserts m, added by by at m.CreatedAt.
	AddMember(ctx context.Context, m domain.Member, by uuid.UUID) error
	// UpdateMemberRole gives membership id role, by by at now.
	UpdateMemberRole(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, now time.Time) error
	// EndMember ends membership id, by by at now.
	EndMember(ctx context.Context, id, by uuid.UUID, now time.Time) error
	// RestoreMember makes the ended membership id active again with role,
	// by by at now; it keeps when the account first joined.
	RestoreMember(ctx context.Context, id uuid.UUID, role shared.NotebookRole, by uuid.UUID, now time.Time) error
}

// Fact is what an account has of a notebook, as the access module's
// notebook level reads it: whether it is there and not deleted, its
// workspace, its workspace access, and the role of the account's active
// membership of it, "" for none.
type Fact struct {
	Found       bool
	WorkspaceID uuid.UUID
	Access      shared.WorkspaceAccess
	Role        shared.NotebookRole
}

// NotebooksDeleter deletes a workspace's notebooks.
type NotebooksDeleter interface {
	// DeleteNotebooksOf soft-deletes the notebooks not deleted of
	// workspaceID and every member row of them, by by at at, and returns
	// their ids in order.
	DeleteNotebooksOf(ctx context.Context, workspaceID, by uuid.UUID, at time.Time) ([]uuid.UUID, error)
}
