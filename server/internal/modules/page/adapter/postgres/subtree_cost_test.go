//go:build !race

package postgresadapter_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The race detector slows the reading of tens of thousands of rows past the
// bounds in time: make test-go runs these in a build without (v0.1 design
// 13.4 #8).

// readsOnOneConnection reads the subtree of notebook's node id eight times
// on one connection, as a busy instance does, and checks it has nodes nodes
// and height levels: pgx caches the statements, and from a statement's
// sixth run the server may plan it once for any arguments (M6 closeout
// FA4-M1). It returns the longest read.
func readsOnOneConnection(t *testing.T, f fixture, notebook, id uuid.UUID, nodes, height int) time.Duration {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	var longest time.Duration
	for range 8 {
		at := time.Now()
		sub, err := s.Subtree(ctx, notebook, id)
		took := time.Since(at)
		if err != nil || len(sub) != nodes || sub.Height() != height {
			t.Fatalf("Subtree: %d nodes of %d levels, %v; want %d of %d", len(sub), sub.Height(), err, nodes, height)
		}
		longest = max(longest, took)
	}
	return longest
}

// A subtree is read by each parent's children without statistics too (M6
// closeout FA-M1): before a notebook's nodes are analyzed (after an import,
// a restore), each level could read every node of the notebook by the index
// of titles, the square of their number: 30,000 nodes, a folder of 10,000
// of them, took 40 s; a level planned for any parents, once its statement
// was cached, compared each of them with the parents one by one: 100,000
// nodes, a folder of 50,000, 3.5 s (FA4-M1).
func TestSubtreeReadsWithoutStatisticsInTimeAsLongAsIt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, `ALTER TABLE nodes SET (autovacuum_enabled = false)`)
	folder := f.page(t, f.eng, nil, "Folder", 0)
	// 200 pages of 250 each beside the folder: with them the plan took the titles' index.
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, NULL, 'page', 'R' || i, 'r' || i, i, $2, $2, $3, $3 FROM generate_series(1, 200) i`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, n.id, 'page', 'S' || i, 's' || i, i, $2, $2, $3, $3
		FROM nodes n, generate_series(1, 250) i WHERE n.notebook_id = $1 AND n.name LIKE 'R%'`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, $4, 'page', 'C' || i, 'c' || i, i, $2, $2, $3, $3 FROM generate_series(1, 50000) i`,
		f.eng, f.alice, now(), folder.ID)
	var stats int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stats WHERE tablename = 'nodes'`).Scan(&stats); err != nil || stats != 0 {
		t.Fatalf("the nodes have statistics of %d columns, %v", stats, err)
	}
	if took := readsOnOneConnection(t, f, f.eng, folder.ID, 50_001, 2); took > time.Second {
		t.Errorf("the subtree of 50,001 nodes took %s", took)
	}
}

// The subtree of a folder that holds most of the nodes, the statistics
// saying so, reads in a time as long as it (M6 closeout FA2-I1: read step
// by step, each parent's children took a scan of the table, 19 s for a
// folder of 20,000).
func TestSubtreeOfAFolderOfMostNodesReadsInTimeAsLongAsIt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	folder := f.page(t, f.eng, nil, "Folder", 0)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, NULL, 'page', 'R' || i, 'r' || i, i, $2, $2, $3, $3 FROM generate_series(1, 10000) i`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, $4, 'page', 'C' || i, 'c' || i, i, $2, $2, $3, $3 FROM generate_series(1, 20000) i`,
		f.eng, f.alice, now(), folder.ID)
	f.exec(t, `ANALYZE nodes`)
	var stats int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stats WHERE tablename = 'nodes'`).Scan(&stats); err != nil || stats == 0 {
		t.Fatalf("the nodes have statistics of %d columns, %v", stats, err)
	}
	if took := readsOnOneConnection(t, f, f.eng, folder.ID, 20_001, 2); took > time.Second {
		t.Errorf("the subtree of 20,001 nodes took %s", took)
	}
}
