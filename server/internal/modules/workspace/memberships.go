package workspace

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Memberships reads an account's role in a workspace: the facts of the
// access module's workspace level, which bootstrap wires to it.
type Memberships interface {
	// RoleOf returns userID's role in workspaceID, and whether userID is an
	// active member of it, in the transaction ctx carries.
	RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (role shared.WorkspaceRole, ok bool, err error)
}

// NewMemberships returns Memberships over pool alone: bootstrap builds the
// access module, and the Authorizer the workspace module takes, before the
// module itself.
func NewMemberships(pool *pgxpool.Pool) Memberships {
	return postgresadapter.New(pool)
}
