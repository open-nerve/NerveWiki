package workspace_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// The notebook module's port on a real database (M3/P1 design 3.5).
func TestWorkspacesFindAndShare(t *testing.T) {
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
	alice, acme, gone := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	exec(`INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', $2, $2)`, alice, testNow())
	for slug, id := range map[string]uuid.UUID{"acme": acme, "gone": gone} {
		exec(`INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $2, $3, $3, $4, $4)`,
			id, slug, alice, testNow())
	}
	exec(`UPDATE workspaces SET deleted_at = $2 WHERE id = $1`, gone, testNow())
	ws := workspace.NewWorkspaces(pool)

	t.Run("FindBySlug", func(t *testing.T) {
		for _, tt := range []struct {
			slug string
			want uuid.UUID // zero: none
		}{
			{"acme", acme},
			{"gone", uuid.UUID{}},
			{"nope", uuid.UUID{}},
		} {
			got, ok, err := ws.FindBySlug(ctx, tt.slug)
			if err != nil || ok != (tt.want != uuid.UUID{}) || got != tt.want {
				t.Errorf("FindBySlug(%q) = %+v, %v, %v; want %+v", tt.slug, got, ok, err, tt.want)
			}
		}
	})

	t.Run("a slug no workspace could have reaches no query", func(t *testing.T) {
		closed, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 1})
		if err != nil {
			t.Fatal(err)
		}
		closed.Close()
		off := workspace.NewWorkspaces(closed)
		for _, slug := range []string{"", "Acme", "a\x00b", "a/b"} {
			if got, ok, err := off.FindBySlug(ctx, slug); ok || err != nil {
				t.Errorf("FindBySlug(%q) = %+v, %v, %v; want none, without the database", slug, got, ok, err)
			}
		}
		if _, _, err := off.FindBySlug(ctx, "acme"); err == nil {
			t.Error("FindBySlug(acme) on a closed pool = nil error: the control reaches no database either")
		}
	})

	t.Run("ShareByID", func(t *testing.T) {
		if _, err := ws.ShareByID(ctx, acme); err == nil {
			t.Error("ShareByID outside a transaction = nil error")
		}
		tx := postgres.NewTxManager(pool, 5*time.Second)
		for _, tt := range []struct {
			id   uuid.UUID
			want bool
		}{{acme, true}, {gone, false}, {uuid.NewV7(), false}} {
			if err := tx.WithinTx(ctx, func(ctx context.Context) error {
				ok, err := ws.ShareByID(ctx, tt.id)
				if err != nil || ok != tt.want {
					t.Errorf("ShareByID(%s) = %v, %v; want %v", tt.id, ok, err, tt.want)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		}
	})
}
