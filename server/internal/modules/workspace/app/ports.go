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
	// transaction ends: identity.account_deactivated (403) when the account
	// is deactivated.
	ShareActiveAccount(ctx context.Context, id uuid.UUID) error
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
	// AddMember inserts m, added by by at now.
	AddMember(ctx context.Context, m domain.Member, by uuid.UUID, now time.Time) error
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
