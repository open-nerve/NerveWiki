package workspace_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// An account's memberships are the workspaces not deleted it is an active
// member of, guests included, each with its role there, its slug and its
// name, by name: not one it left, nor a deleted one (M9/P1 design 3.17.1).
func TestMembershipsListTheWorkspacesOfAnAccount(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	alice := uuid.NewV7()
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', $2, $2)`, alice, testNow())
	ids := map[string]uuid.UUID{}
	for _, w := range []struct {
		slug, name string
		role       shared.WorkspaceRole
		ended      bool
	}{
		{"zeta", "Acme Labs", shared.WorkspaceGuest, false},
		{"acme", "Beta 研发", shared.WorkspaceAdmin, false},
		{"left", "Left", shared.WorkspaceMember, true},
		{"gone", "Gone", shared.WorkspaceMember, false},
		{"other", "Other", "", false},
	} {
		ids[w.slug] = uuid.NewV7()
		exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $4, $5, $5)`,
			ids[w.slug], w.slug, w.name, alice, testNow())
		if w.role != "" {
			exec(`INSERT INTO workspace_members (id, workspace_id, user_id, role, created_by_id, updated_by_id, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $3, $3, $5, $5)`, uuid.NewV7(), ids[w.slug], alice, string(w.role), testNow())
		}
		if w.ended {
			exec(`UPDATE workspace_members SET ended_at = $2 WHERE workspace_id = $1`, ids[w.slug], testNow())
		}
	}
	exec(`UPDATE workspaces SET deleted_at = $2 WHERE id = $1`, ids["gone"], testNow())
	got, err := workspace.NewMemberships(pool).WorkspacesOf(ctx, alice)
	want := []workspace.Membership{
		{WorkspaceID: ids["zeta"], Slug: "zeta", Name: "Acme Labs", Role: shared.WorkspaceGuest},
		{WorkspaceID: ids["acme"], Slug: "acme", Name: "Beta 研发", Role: shared.WorkspaceAdmin},
	}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("WorkspacesOf = %+v, %v; want %+v", got, err, want)
	}
}
