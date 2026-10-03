// Package app holds the event stream's hub, the opening of a stream and
// the publisher (M5 design 4.10).
package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Membership is a workspace an account is an active member of, and its
// role there.
type Membership struct {
	WorkspaceID uuid.UUID
	Role        shared.WorkspaceRole
}

// Visibility tells what an account sees, read without locks: bootstrap
// wires the workspace and notebook modules' ports to it (P2 design 3.7).
type Visibility interface {
	// WorkspacesOf lists the workspaces userID is an active member of.
	WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]Membership, error)
	// NotebooksIn lists the notebooks of the workspace userID, of role
	// there, may read.
	NotebooksIn(ctx context.Context, workspaceID, userID uuid.UUID, role shared.WorkspaceRole) ([]uuid.UUID, error)
}

// Notifier sends a payload when the transaction in ctx commits; outside
// one it is an error.
type Notifier interface {
	Notify(ctx context.Context, payload string) error
}
