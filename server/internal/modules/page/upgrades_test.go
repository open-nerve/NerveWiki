package page_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Gates 1 and 2 of an upgrade's check (M11/P2 design 3.9): the root's
// AGENTS, a page whose body past its frontmatter and a byte order mark,
// trimmed, is not empty; an attachment titled AGENTS is none; and whether
// the proposed source root is a live page of the notebook, each read in
// the snapshot.
func TestTheUpgradeRoots(t *testing.T) {
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	alice, acme := uuid.NewV7(), uuid.NewV7()
	exec := func(t *testing.T, sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(t, "INSERT INTO users (id, email, password, display_name, created_at, updated_at) VALUES ($1, 'alice@corp.com', 'x', 'x', now(), now())", alice)
	exec(t, "INSERT INTO workspaces (id, slug, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, 'acme', 'Acme', $2, $2, now(), now())",
		acme, alice)
	type node struct {
		kind, name, content string
		gone                bool
	}
	// notebook writes a notebook of its own holding nodes at its root and
	// answers its id and theirs, by name.
	notebook := func(t *testing.T, nodes ...node) (uuid.UUID, map[string]uuid.UUID) {
		t.Helper()
		nb, ids := uuid.NewV7(), map[string]uuid.UUID{}
		exec(t, "INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $4, now(), now())",
			nb, acme, nb.String(), alice)
		for i, n := range nodes {
			id := uuid.NewV7()
			ids[n.name] = id
			exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
				created_at, updated_at, deleted_at) VALUES ($1, $2, NULL, $3, $4, $5, $6, $7, $7, now(), now(), CASE WHEN $8 THEN now() END)`,
				id, nb, n.kind, n.name, shared.TitleKey(n.name), i, alice, n.gone)
			if n.kind == "page" {
				hash := sha256.Sum256([]byte(n.content))
				exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at, deleted_at)
					VALUES ($1, $2, 1, $3, $4, $5, now(), CASE WHEN $6 THEN now() END)`, id, n.content, hash[:], len(n.content), alice, n.gone)
			}
		}
		return nb, ids
	}
	upgrades := page.NewUpgrades(pool)
	roots := func(t *testing.T, nb, root uuid.UUID) page.UpgradeRoots {
		t.Helper()
		var out page.UpgradeRoots
		if err := postgres.NewTxManager(pool, 5*time.Second).WithinSnapshot(ctx, func(ctx context.Context) error {
			var err error
			out, err = upgrades.Roots(ctx, nb, root)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	raw := node{"page", "raw", "", false}
	for _, tc := range []struct {
		name   string
		agents node
		body   bool
	}{
		{"a body", node{"page", "AGENTS", "---\ntype: guide\n---\nRead the sources.\n", false}, true},
		{"a body without frontmatter", node{"page", "AGENTS", "\ufeffRead the sources.\n", false}, true},
		{"only frontmatter", node{"page", "AGENTS", "---\ntype: guide\nsummary: The rules.\n---\n", false}, false},
		{"only a byte order mark and white space", node{"page", "AGENTS", "---\ntype: guide\n---\n\ufeff \n\t\n", false}, false},
		{"empty", node{"page", "AGENTS", "", false}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nb, ids := notebook(t, tc.agents, raw)
			got := roots(t, nb, ids["raw"])
			if got.AgentsID == nil || *got.AgentsID != ids["AGENTS"] || got.AgentsBody != tc.body || !got.RootPage {
				t.Errorf("roots = %+v, want AGENTS %s, its body %t, the root a page", got, ids["AGENTS"], tc.body)
			}
		})
	}
	t.Run("an attachment titled AGENTS", func(t *testing.T) {
		nb, ids := notebook(t, node{"asset", "AGENTS", "", false}, raw)
		if got := roots(t, nb, ids["raw"]); got.AgentsID != nil || got.AgentsBody || !got.RootPage {
			t.Errorf("roots = %+v, want no AGENTS", got)
		}
	})
	t.Run("none", func(t *testing.T) {
		nb, ids := notebook(t, raw)
		if got := roots(t, nb, ids["raw"]); got.AgentsID != nil || got.AgentsBody || !got.RootPage {
			t.Errorf("roots = %+v, want no AGENTS", got)
		}
	})
	t.Run("a deleted AGENTS", func(t *testing.T) {
		nb, ids := notebook(t, node{"page", "AGENTS", "Rules.\n", true}, raw)
		if got := roots(t, nb, ids["raw"]); got.AgentsID != nil || got.AgentsBody {
			t.Errorf("roots = %+v, want no AGENTS", got)
		}
	})
	t.Run("the source root", func(t *testing.T) {
		nb, ids := notebook(t, node{"page", "raw", "", true}, node{"asset", "x.png", "", false})
		other, theirs := notebook(t, raw)
		for name, root := range map[string]uuid.UUID{"deleted": ids["raw"], "an attachment": ids["x.png"],
			"another notebook's page": theirs["raw"], "none": uuid.NewV7()} {
			if got := roots(t, nb, root); got.RootPage {
				t.Errorf("%s: roots = %+v, want no source root", name, got)
			}
		}
		if got := roots(t, other, theirs["raw"]); !got.RootPage {
			t.Errorf("a live page: roots = %+v, want the source root", got)
		}
	})
}
