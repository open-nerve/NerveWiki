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
