//go:build !race

package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// medianOf is the median time of eight runs of read on one connection, as
// a busy instance runs it: pgx caches the statements, and from a
// statement's sixth run the server may plan it once for any arguments.
func medianOf(t *testing.T, name string, read func() error) time.Duration {
	t.Helper()
	took := make([]time.Duration, 8)
	for i := range took {
		at := time.Now()
		if err := read(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		took[i] = time.Since(at)
	}
	slices.Sort(took)
	return (took[3] + took[4]) / 2
}

// The overview's two statements over the notebook whole, its counts and
// its last change, read a notebook of 100,000 nodes and 100,000
// changesets in no longer than its tree takes to read, before its
// statistics and after (M9/P1 design 3.17.3, v0.1 design 13.4 #8):
// open_notebook is one of an agent's first calls, and the tree's read
// is what the web's sidebar already pays.
func TestTheOverviewReadsInTimeAsLongAsTheTree(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"nodes", "changesets"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	f.nodes(t, f.eng, "N", 100_000)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, 'edit', 'web', $2, $3::timestamptz + k * interval '1 second', $3::timestamptz + k * interval '1 second'
		FROM generate_series(1, 100000) k`, f.eng, f.alice, now())
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	for _, stage := range []string{"without statistics", "with statistics"} {
		if stage == "with statistics" {
			f.exec(t, "ANALYZE nodes, changesets")
		}
		tree := medianOf(t, "ListNodes", func() error {
			nodes, err := s.ListNodes(ctx, f.eng)
			if err == nil && len(nodes) != 100_000 {
				t.Fatalf("ListNodes: %d nodes", len(nodes))
			}
			return err
		})
		counts := medianOf(t, "NotebookCounts", func() error {
			pages, _, err := s.NotebookCounts(ctx, f.eng)
			if err == nil && pages != 100_000 {
				t.Fatalf("NotebookCounts: %d pages", pages)
			}
			return err
		})
		last := medianOf(t, "NotebookLastChange", func() error {
			at, err := s.NotebookLastChange(ctx, f.eng)
			if err == nil && (at == nil || !at.Equal(now().Add(100_000*time.Second))) {
				t.Fatalf("NotebookLastChange: %v", at)
			}
			return err
		})
		t.Logf("%s: the tree %s, the counts %s, the last change %s", stage, tree, counts, last)
		if counts > tree || last > tree {
			t.Errorf("%s: the counts took %s, the last change %s, the tree %s", stage, counts, last, tree)
		}
	}
}
