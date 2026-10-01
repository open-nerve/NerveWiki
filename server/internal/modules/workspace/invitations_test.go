package workspace_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	macadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/mac"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The sign-up policy's check on a real database (M2/P3 design 3.6): only a
// pending invitation of a workspace not deleted, by its token, for the
// address it was sent to.
func TestInvitationCheckAdmits(t *testing.T) {
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
	invitation := func(slug, email string) uuid.UUID {
		t.Helper()
		ws, id := uuid.NewV7(), uuid.NewV7()
		exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $2, $3, $3, $4, $4)`,
			ws, slug, alice, testNow())
		exec(`INSERT INTO workspace_invitations (id, workspace_id, email, role, created_by_id, updated_by_id, created_at, updated_at)
			VALUES ($1, $2, $3, 'member', $4, $4, $5, $5)`, id, ws, email, alice, testNow())
		return id
	}
	pending := invitation("acme", "dana@corp.com")
	deleted := invitation("beta", "dana@corp.com")
	exec(`UPDATE workspace_invitations SET deleted_at = $2 WHERE id = $1`, deleted, testNow())
	ofDeleted := invitation("gone", "dana@corp.com")
	exec(`UPDATE workspaces SET deleted_at = $2 WHERE id = (SELECT workspace_id FROM workspace_invitations WHERE id = $1)`, ofDeleted, testNow())
	key := bytes.Repeat([]byte{7}, 32)
	token := macadapter.New(key).Token
	check := workspace.NewInvitationCheck(pool, key)

	for _, tt := range []struct {
		name         string
		id           uuid.UUID
		token, email string
		want         bool
	}{
		{"its token, its address", pending, token(pending), "dana@corp.com", true},
		{"another invitation's token", pending, token(deleted), "dana@corp.com", false},
		{"another key's token", pending, macadapter.New(bytes.Repeat([]byte{8}, 32)).Token(pending), "dana@corp.com", false},
		{"another address", pending, token(pending), "erin@corp.com", false},
		{"deleted", deleted, token(deleted), "dana@corp.com", false},
		{"of a deleted workspace", ofDeleted, token(ofDeleted), "dana@corp.com", false},
		{"no such invitation", uuid.NewV7(), "", "dana@corp.com", false},
	} {
		if got, err := check.Admits(ctx, tt.id, tt.token, tt.email); err != nil || got != tt.want {
			t.Errorf("Admits(%s) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	// A token checked first: a valid one reaches the database, whose
	// failure is the check's.
	pool.Close()
	if _, err := check.Admits(ctx, pending, token(pending), "dana@corp.com"); err == nil || errors.Is(err, context.Canceled) {
		t.Errorf("Admits() on a closed pool = %v, want its error", err)
	}
	if got, err := check.Admits(ctx, pending, token(deleted), "dana@corp.com"); err != nil || got {
		t.Errorf("Admits(a wrong token) on a closed pool = %v, %v; want false before any read", got, err)
	}
}
