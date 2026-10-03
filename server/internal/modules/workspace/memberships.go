package workspace

import (
	"context"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Memberships reads an account's roles in workspaces: the facts of the
// access module's workspace level, and the workspaces of the event stream
// (M5 design 4.10), which bootstrap wires to it.
type Memberships interface {
	// RoleOf returns userID's role in workspaceID, and whether userID is an
	// active member of it, in the transaction ctx carries.
	RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (role shared.WorkspaceRole, ok bool, err error)
	// WorkspacesOf lists the workspaces not deleted that userID is an
	// active member of, with its role in each, unlocked: those RoleOf
	// answers ok for.
	WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]Membership, error)
}

// Membership is a workspace an account is an active member of, and its
// role there.
type Membership struct {
	WorkspaceID uuid.UUID
	Role        shared.WorkspaceRole
}

// NewMemberships returns Memberships over pool alone: bootstrap builds the
// access module, and the Authorizer the workspace module takes, before the
// module itself.
func NewMemberships(pool *pgxpool.Pool) Memberships {
	return memberships{store: postgresadapter.New(pool)}
}

type memberships struct {
	store *postgresadapter.Store
}

func (m memberships) RoleOf(ctx context.Context, workspaceID, userID uuid.UUID) (shared.WorkspaceRole, bool, error) {
	return m.store.RoleOf(ctx, workspaceID, userID)
}

func (m memberships) WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]Membership, error) {
	listed, err := m.store.ListWorkspacesOf(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("workspace: the workspaces of %s: %w", userID, err)
	}
	out := make([]Membership, len(listed))
	for i, l := range listed {
		out[i] = Membership{WorkspaceID: l.Workspace.ID, Role: l.Role}
	}
	return out, nil
}
