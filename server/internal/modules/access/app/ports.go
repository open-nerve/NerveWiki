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
