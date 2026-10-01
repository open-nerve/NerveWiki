// Package app decides permissions on the facts its ports read.
package app

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// WorkspaceMemberships reads the facts of the workspace level. The
// workspace module implements it (workspace.NewMemberships), in the
// transaction ctx carries.
type WorkspaceMemberships interface {
	// RoleOf returns userID's role in workspaceID, and whether userID is an
	// active member of it: a membership not ended, in a workspace not
	// deleted.
	RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (role shared.WorkspaceRole, ok bool, err error)
}

// NotebookFacts reads the facts of the notebook level. The notebook module
// implements it (notebook.NewFacts), in the transaction ctx carries.
type NotebookFacts interface {
	// NotebookFacts returns what userID has of notebookID: whether it is
	// there and not deleted, its workspace, how open it is to it, and the
	// role of userID's active membership of it, "" for none.
	NotebookFacts(ctx context.Context, notebookID, userID uuid.UUID) (NotebookFact, error)
}

// NotebookFact is what NotebookFacts reads.
type NotebookFact struct {
	Found       bool
	WorkspaceID uuid.UUID
	Access      shared.WorkspaceAccess
	Role        shared.NotebookRole
}
