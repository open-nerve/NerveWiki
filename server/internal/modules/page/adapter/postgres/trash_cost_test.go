//go:build !race

package postgresadapter_test

import (
	"context"
	"testing"
	"time"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// trashOfManyItems is a notebook of 100,000 nodes, 10,000 of them trash
// items' roots, without statistics, as after a large import and its
// deletion (M8/P2 design 3.4): 100 folders, 900 pages in each, and under
// those pages the roots, deleted a second apart from now() back. It
// answers a store on one connection, which caches the statements, the
// notebook and the newest 100 roots' ids.
func trashOfManyItems(t *testing.T) (*postgresadapter.Store, uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	f.exec(t, `ALTER TABLE nodes SET (autovacuum_enabled = false)`)
	f.exec(t, `ALTER TABLE page_contents SET (autovacuum_enabled = false)`)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, NULL, 'page', 'F' || i, 'f' || i, i, $2, $2, $3, $3 FROM generate_series(1, 100) i`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at)
		SELECT gen_random_uuid(), $1, n.id, 'page', 'P' || i, 'p' || i, i, $2, $2, $3, $3
		FROM nodes n, generate_series(1, 900) i WHERE n.notebook_id = $1 AND n.parent_id IS NULL`,
		f.eng, f.alice, now())
	f.exec(t, `INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id,
		created_at, updated_at, deleted_at, trash_root, trash_pages, trash_assets, trash_height)
		SELECT gen_random_uuid(), $1, p.id, 'page', 'T' || p.n, 't' || p.n, 0, $2, $2, $3::timestamptz, $3::timestamptz,
			$3::timestamptz - p.n * interval '1 second',
			true, 1, 0, 1
		FROM (SELECT id, row_number() OVER (ORDER BY id) AS n FROM nodes WHERE notebook_id = $1 AND name LIKE 'P%') p
		WHERE p.n <= 10000`,
		f.eng, f.alice, now())
	var stats, total int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stats WHERE tablename = 'nodes'`).Scan(&stats); err != nil || stats != 0 {
		t.Fatalf("the nodes have statistics of %d columns, %v", stats, err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM nodes WHERE notebook_id = $1`, f.eng).Scan(&total); err != nil || total != 100_100 {
		t.Fatalf("the notebook holds %d nodes, %v", total, err)
	}
	rows, err := f.pool.Query(ctx, `SELECT id FROM nodes WHERE trash_root ORDER BY deleted_at DESC, id DESC LIMIT 100`)
	if err != nil {
		t.Fatal(err)
	}
	var roots []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, id)
	}
	if err := rows.Err(); err != nil || len(roots) != 100 {
		t.Fatalf("roots: %d, %v", len(roots), err)
	}
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), f.eng, roots
}

// A page of the trash reads a range of its index, whatever the notebook
// holds besides, eight times on one connection: from the newest, and from
// a cursor halfway down.
func TestListTrashReadsInTimeAsLongAsAPage(t *testing.T) {
	ctx := context.Background()
	s, notebook, _ := trashOfManyItems(t)
	halfway := domain.TrashCursor{At: now().Add(-5000 * time.Second), ID: uuid.Max()}
	for _, below := range []domain.TrashCursor{domain.TrashFrom(), halfway} {
		var longest time.Duration
		for range 8 {
			at := time.Now()
			page, err := s.ListTrash(ctx, notebook, now().Add(-24*time.Hour), below, 101)
			took := time.Since(at)
			if err != nil || len(page) != 101 {
				t.Fatalf("ListTrash(101) = %d, %v", len(page), err)
			}
			longest = max(longest, took)
		}
		if longest > 200*time.Millisecond {
			t.Errorf("a page of 101 items below %s took %s", below.At, longest)
		}
	}
}

// The lines of a page of the trash read each step up by its parent's key,
// whatever the notebook holds, eight times on one connection.
func TestTrashLinesReadInTimeAsLongAsAPage(t *testing.T) {
	ctx := context.Background()
	s, _, roots := trashOfManyItems(t)
	var longest time.Duration
	for range 8 {
		at := time.Now()
		lines, err := s.TrashLines(ctx, roots)
		took := time.Since(at)
		if err != nil || len(lines) != len(roots) || len(lines[roots[0]]) != 2 {
			t.Fatalf("TrashLines(100) = %d lines, %v; want 100 of 2 steps", len(lines), err)
		}
		longest = max(longest, took)
	}
	if longest > 200*time.Millisecond {
		t.Errorf("the lines of 100 items took %s", longest)
	}
}
