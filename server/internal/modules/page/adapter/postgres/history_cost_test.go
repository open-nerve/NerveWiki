//go:build !race

package postgresadapter_test

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// longLog is a notebook of a long history, without statistics (M8/P3
// design 3.11): 100,000 changesets a second apart, of three actors, from
// the web, the API and two MCP clients, one in a thousand from the CLI; a
// page of 20,000 versions and 20,000 items, each in a changeset of its
// own; a changeset of 50,000 items, one of the newest 100; 50,000 pages,
// an item of each in three of five changesets, and a version of 10,000 of
// them. Its store is on one connection, which caches the statements as a
// server's do.
type longLog struct {
	s        *postgresadapter.Store
	pool     *pgxpool.Pool
	notebook uuid.UUID
	bob      uuid.UUID
	page     uuid.UUID   // the page of a long history
	large    uuid.UUID   // the changeset of 50,000 items
	newest   []uuid.UUID // the newest 100 changesets, large among them
	pages    []uuid.UUID // the 50,000 pages, by id
	analyze  func()
}

func newLongLog(t *testing.T) longLog {
	t.Helper()
	ctx := context.Background()
	f := newFixture(t)
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changeset_items", "changesets"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	bob, carol := f.user(t, "bob@corp.com"), f.user(t, "carol@corp.com")
	page := f.page(t, f.eng, nil, "Long", 0)
	pages := f.nodes(t, f.eng, "N", 50_000)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '', 1, sha256(''), 0, $2, $3 FROM nodes WHERE notebook_id = $1 AND id <> $4`, f.eng, f.alice, now(), page.ID)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		SELECT uuidv7(), $1, 'edit', CASE WHEN k % 1000 = 0 THEN 'cli' ELSE (ARRAY['web', 'api', 'mcp:a', 'mcp:b'])[1 + k % 4] END,
			($2::uuid[])[1 + k % 3], $3::timestamptz + k * interval '1 second', $3::timestamptz + k * interval '1 second'
		FROM generate_series(1, 100000) k`, f.eng, []uuid.UUID{f.alice, bob, carol}, now())
	const numbered = `(SELECT id, row_number() OVER (ORDER BY created_at) AS k FROM changesets WHERE notebook_id = $1) c`
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, base_revision, revision, content, content_hash, byte_size,
			created_at, updated_at)
		SELECT uuidv7(), c.id, $2, NULLIF(c.k / 5, 0), c.k / 5 + 1, '', sha256(''), 0, $3, $3 FROM `+numbered+` WHERE c.k % 5 = 1`,
		f.eng, page.ID, now())
	f.exec(t, `UPDATE page_contents SET revision = 20000 WHERE node_id = $1`, page.ID)
	f.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, before_name, before_sort_order, after_name, after_sort_order,
			created_at, updated_at)
		SELECT uuidv7(), c.id, $2, 'Long', c.k - 1, 'Long', c.k, $3, $3 FROM `+numbered+` WHERE c.k % 5 = 2`, f.eng, page.ID, now())
	f.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at)
		SELECT uuidv7(), c.id, ($2::uuid[])[1 + c.k % 50000], 'N', 0, $3, $3 FROM `+numbered+`
		WHERE c.k % 5 IN (0, 3, 4) AND c.k <> 99950`, f.eng, pages, now())
	f.exec(t, `INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
		SELECT uuidv7(), c.id, ($2::uuid[])[c.k / 5 + 1], 1, '', sha256(''), 0, $3, $3 FROM `+numbered+`
		WHERE c.k % 5 = 3 AND c.k < 50000`, f.eng, pages, now())
	var large uuid.UUID
	if err := f.pool.QueryRow(ctx, `SELECT c.id FROM `+numbered+` WHERE c.k = 99950`, f.eng).Scan(&large); err != nil {
		t.Fatal(err)
	}
	f.exec(t, `INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at)
		SELECT uuidv7(), $1, n.id, n.name, n.sort_order, $3, $3 FROM nodes n WHERE n.id = ANY($2)`, large, pages, now())
	var stats, items int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_stats WHERE tablename IN ('changesets', 'changeset_items')`).Scan(&stats); err != nil ||
		stats != 0 {
		t.Fatalf("the log has statistics of %d columns, %v", stats, err)
	}
	if items = f.count(t, `SELECT count(*) FROM changeset_items`); items != 50_000+20_000+59_999 {
		t.Fatalf("the log holds %d items", items)
	}
	rows, err := f.pool.Query(ctx, `SELECT id FROM changesets WHERE notebook_id = $1 ORDER BY created_at DESC, id DESC LIMIT 100`, f.eng)
	if err != nil {
		t.Fatal(err)
	}
	newest, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return longLog{s: postgresadapter.New(pool), pool: pool, notebook: f.eng, bob: bob, page: page.ID, large: large, newest: newest,
		pages: pages, analyze: func() { f.exec(t, "ANALYZE") }}
}

// at runs each of the reads eight times without statistics, then with
// them, then planned for any arguments, as a cached statement may be from
// its sixth run, and answers each one's longest by its name and the stage.
func (l longLog) at(t *testing.T, reads []logRead) map[[2]string]time.Duration {
	t.Helper()
	out := map[[2]string]time.Duration{}
	for _, stage := range []string{"without statistics", "analyzed", "planned for any arguments"} {
		switch stage {
		case "analyzed":
			l.analyze()
		case "planned for any arguments":
			if _, err := l.pool.Exec(context.Background(), "SET plan_cache_mode = force_generic_plan"); err != nil {
				t.Fatal(err)
			}
		}
		for _, r := range reads {
			var longest time.Duration
			for range 8 {
				at := time.Now()
				err := r.read()
				took := time.Since(at)
				if err != nil {
					t.Fatalf("%s %s: %v", r.name, stage, err)
				}
				longest = max(longest, took)
			}
			out[[2]string{r.name, stage}] = longest
			t.Logf("%s %s: %s", r.name, stage, longest)
		}
	}
	return out
}

// logRead is a read of the long log, which checks what it reads.
type logRead struct {
	name string
	read func() error
}

// expect is the read's error, or what it got when that is not what it
// wants.
func expect(err error, ok bool, got any) error {
	if err == nil && !ok {
		return fmt.Errorf("got %v", got)
	}
	return err
}

// The log reads a page of 101 changesets, the facts and first changes of
// a page of 100, a large changeset's moves and changes from deep within
// it in time, whatever the notebook holds besides (M8/P3 design 3.11):
// each in 200 ms, eight times on one connection, without statistics, with
// them, and planned for any arguments. The rare client's page reads the
// notebook's whole index, the worst of its filters: 5 to 8 ms here; the
// long page's sorts its 40,000 changesets, 31 to 43 ms; the rest took
// under 27 ms, most under 1. The long page's, joined as the planner
// pleased, took 7 s a page without statistics.
func TestTheLogReadsInTimeAsLongAsAPage(t *testing.T) {
	ctx := context.Background()
	l := newLongLog(t)
	page := func(f domain.LogFilter, below domain.ChangesetCursor) func() error {
		return func() error {
			got, err := l.s.ListChangesets(ctx, l.notebook, f, below, 101)
			return expect(err, len(got) == 101 || f.Client == domain.ClientCLI && len(got) == 100, len(got))
		}
	}
	middle := domain.ChangesetCursor{At: now().Add(50_000 * time.Second), ID: uuid.Max()}
	reads := []logRead{
		{"the first page", page(domain.LogFilter{}, domain.ChangesetFrom())},
		{"a page from the middle", page(domain.LogFilter{}, middle)},
		{"a page of one actor's", page(domain.LogFilter{Actor: &l.bob}, middle)},
		{"a page of the rare client's", page(domain.LogFilter{Client: domain.ClientCLI}, domain.ChangesetFrom())},
		{"a page of the MCP clients'", page(domain.LogFilter{AllMCP: true}, middle)},
		{"a page of the long page's", page(domain.LogFilter{PageID: &l.page}, middle)},
		{"the facts of a page", func() error {
			got, err := l.s.ChangesetFacts(ctx, l.newest)
			return expect(err, len(got) == 100 && got[l.large].Nodes == 1000, got[l.large])
		}},
		{"the first changes of a page", func() error {
			got, err := l.s.FirstChanges(ctx, l.newest, 3)
			return expect(err, len(got) == 100 && len(got[l.large]) == 3, len(got))
		}},
		{"the large changeset's moves", func() error {
			got, err := l.s.ChangesetMoves(ctx, l.large)
			return expect(err, got == domain.ChangesetMoves{}, got)
		}},
		{"deep in the large changeset's changes", func() error {
			got, err := l.s.ListChanges(ctx, l.large, l.pages[40_000], 101)
			return expect(err, len(got) == 101 && got[0].NodeID == l.pages[40_001], len(got))
		}},
	}
	for key, took := range l.at(t, reads) {
		if took > 200*time.Millisecond {
			t.Errorf("%s %s took %s", key[0], key[1], took)
		}
	}
}

// A revert's reads of arrays read the unit's bound of 10,000 nodes,
// contents and versions in time, whatever plan the server keeps for them
// (M8/P3 design 3.11): 4 to 20 ms here, a plan for any arguments no
// slower than one with them, so they are pgx's cached statements; a
// second each, a CI's runner slower. The nodes are read alive, and as
// many trash items' roots, each deleted by a revert, whose deletions it
// reads (M8/P3A review A-1): 23 to 26 ms.
func TestTheRevertsArrayReadsInTimeAtTheUnitsBound(t *testing.T) {
	ctx := context.Background()
	l := newLongLog(t)
	ids, refs := l.pages[:domain.MaxUnitNodes], make([]domain.VersionRef, domain.MaxUnitNodes)
	for i, id := range ids {
		refs[i] = domain.VersionRef{NodeID: id, Revision: 1}
	}
	roots := l.pages[domain.MaxUnitNodes : 2*domain.MaxUnitNodes]
	// Each root deleted at the time of a changeset of the long page's
	// versions, which has no items, made a revert holding its deletion.
	for _, sql := range []string{
		`WITH r AS (SELECT id, row_number() OVER (ORDER BY id) AS k FROM unnest($1::uuid[]) id),
			c AS (SELECT id, created_at, row_number() OVER (ORDER BY created_at) AS k FROM changesets
				WHERE notebook_id = $2 AND id NOT IN (SELECT changeset_id FROM changeset_items))
		UPDATE nodes SET deleted_at = c.created_at, trash_root = true, trash_pages = 1, trash_assets = 0, trash_height = 1
		FROM r JOIN c ON c.k = r.k WHERE nodes.id = r.id`,
		`UPDATE changesets c SET kind = 'revert', reverts_id = c.id FROM nodes n
		WHERE n.id = ANY($1::uuid[]) AND n.notebook_id = $2 AND c.notebook_id = $2 AND c.created_at = n.deleted_at`,
		`INSERT INTO changeset_items (id, changeset_id, node_id, before_name, before_sort_order, created_at, updated_at, deleted_at)
		SELECT uuidv7(), c.id, n.id, n.name, n.sort_order, n.deleted_at, n.deleted_at, n.deleted_at FROM nodes n
		JOIN changesets c ON c.notebook_id = n.notebook_id AND c.created_at = n.deleted_at WHERE n.id = ANY($1::uuid[]) AND n.notebook_id = $2`,
	} {
		if _, err := l.pool.Exec(ctx, sql, roots, l.notebook); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	reads := []logRead{
		{"NodesNow", func() error {
			got, err := l.s.NodesNow(ctx, ids)
			return expect(err, len(got) == len(ids), len(got))
		}},
		{"NodesNow of roots a revert deleted", func() error {
			got, err := l.s.NodesNow(ctx, roots)
			n := 0
			for _, r := range got {
				if r.DeletedByRevert {
					n++
				}
			}
			return expect(err, len(got) == len(roots) && n == len(roots), n)
		}},
		{"ContentsNow", func() error {
			got, err := l.s.ContentsNow(ctx, ids)
			return expect(err, len(got) == len(ids), len(got))
		}},
		{"VersionHashes", func() error {
			got, err := l.s.VersionHashes(ctx, refs)
			return expect(err, len(got) == len(refs), len(got))
		}},
	}
	for key, took := range l.at(t, reads) {
		if took > time.Second {
			t.Errorf("%s %s took %s", key[0], key[1], took)
		}
	}
}
