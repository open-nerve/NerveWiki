//go:build !race

package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/storage"
)

// What a revert costs, by the size of its changeset and of its notebook
// (M8/P3 design 3.11): the measurement that keeps the unit's bounds for a
// revert, run by hand,
//
//	go test -run '^$' -bench 'BenchmarkRevert' -benchtime 3x ./internal/bootstrap/
//
// and a test that holds a revert at the unit's bounds to its time. The
// bench is the trash's (trash_cost_test.go): the page module wired as
// serve wires it, its routes served in process.

// historyAtTheBound is how many versions, each with its item, each page
// of the full load takes so that a group of the unit's nodes, deleted, has
// about the bound's rows of history with its own items (M8/P2A review,
// A-M4).
const historyAtTheBound = (domain.MaxUnitHistory - 2*domain.MaxUnitNodes - 2*domain.MaxUnitPages) / (2 * domain.MaxUnitPages)

// newestChangeset is the notebook's newest changeset.
func (b trashBench) newestChangeset(tb testing.TB) uuid.UUID {
	tb.Helper()
	var id uuid.UUID
	if err := b.pool.QueryRow(context.Background(), `SELECT id FROM changesets WHERE notebook_id = $1
		ORDER BY created_at DESC, id DESC LIMIT 1`, b.nb).Scan(&id); err != nil {
		tb.Fatal(err)
	}
	return id
}

// imported makes the item's creation one changeset, an import's, as an
// import of 10,000 nodes writes it: the items and versions of the pages'
// creations merged into the attachments' changeset, the rest deleted.
func (b trashBench) imported(tb testing.TB) uuid.UUID {
	tb.Helper()
	ctx := context.Background()
	var c uuid.UUID
	if err := b.pool.QueryRow(ctx, `SELECT i.changeset_id FROM changeset_items i JOIN nodes n ON n.id = i.node_id
		WHERE n.notebook_id = $1 AND n.kind = 'asset' LIMIT 1`, b.nb).Scan(&c); err != nil {
		tb.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE changesets SET kind = 'import' WHERE id = $1`,
		`UPDATE changeset_items SET changeset_id = $1 WHERE changeset_id IN (SELECT id FROM changesets WHERE notebook_id = $2)`,
		`UPDATE page_revisions SET changeset_id = $1 WHERE changeset_id IN (SELECT id FROM changesets WHERE notebook_id = $2)`,
		`DELETE FROM changesets WHERE notebook_id = $2 AND id <> $1`,
	} {
		args := []any{c, b.nb}
		if !strings.Contains(sql, "$2") {
			args = args[:1]
		}
		if _, err := b.pool.Exec(ctx, sql, args...); err != nil {
			tb.Fatalf("%s: %v", sql, err)
		}
	}
	return c
}

// others gives the notebook n more nodes alive, through SQL: a page of
// each hundred at its root, the rest attachments under them, each with
// its content or its row.
func (b trashBench) others(tb testing.TB, n int) {
	tb.Helper()
	if n == 0 {
		return
	}
	ctx := context.Background()
	for _, x := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), $1, NULL, 'page', 'Other ' || g, 'other ' || g, 1000 + g, $2, $2, now(), now()
		FROM generate_series(1, $3::integer / 100) g`, []any{b.nb, b.alice, n}},
		{`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, '', 1, sha256(''), 0, $2, now() FROM nodes WHERE notebook_id = $1 AND name LIKE 'Other %'`, []any{b.nb, b.alice}},
		{`WITH p AS (SELECT id, row_number() OVER (ORDER BY id) - 1 AS k FROM nodes WHERE notebook_id = $1 AND name LIKE 'Other %')
		INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), $1, p.id, 'asset', 'o' || g || '.png', 'o' || g || '.png', g, $2, $2, now(), now()
		FROM generate_series(0, $3::integer - $3::integer / 100 - 1) g JOIN p ON p.k = g % ($3::integer / 100)`, []any{b.nb, b.alice, n}},
		{`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
		SELECT gen_random_uuid(), n.id, n.notebook_id, 'image/png', 3, sha256('abc'), n.created_by_id, n.created_at FROM nodes n
		WHERE n.notebook_id = $1 AND n.kind = 'asset' AND n.name LIKE 'o%'`, []any{b.nb}},
	} {
		if _, err := b.pool.Exec(ctx, x.sql, x.args...); err != nil {
			tb.Fatalf("%s: %v", x.sql, err)
		}
	}
}

// forget deletes the changesets ids with their items and versions: an
// iteration's, so that the history of the next is the same.
func (b trashBench) forget(tb testing.TB, ids ...uuid.UUID) {
	tb.Helper()
	ctx := context.Background()
	for _, sql := range []string{"DELETE FROM changeset_items WHERE changeset_id = ANY($1)",
		"DELETE FROM page_revisions WHERE changeset_id = ANY($1)", "DELETE FROM changesets WHERE id = ANY($1)"} {
		if _, err := b.pool.Exec(ctx, sql, ids); err != nil {
			tb.Fatalf("%s: %v", sql, err)
		}
	}
}

// generic has every connection of the pool plan its cached statements
// for any arguments, as a server's may from their sixth run.
func (b trashBench) generic(tb testing.TB) {
	tb.Helper()
	ctx := context.Background()
	var name string
	if err := b.pool.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil {
		tb.Fatal(err)
	}
	if _, err := b.pool.Exec(ctx, "ALTER DATABASE "+pgx.Identifier{name}.Sanitize()+" SET plan_cache_mode = force_generic_plan"); err != nil {
		tb.Fatal(err)
	}
	b.pool.Reset()
}

// planned reads, as a revert does, the state of the revert of the
// changeset c, then the notebook's tree, then plans it, and answers how
// long the reads of c took, which grow with it, and the tree's read and
// the plan, which grow with the notebook; a revert reads and plans twice,
// unlocked and in its unit.
func (b trashBench) planned(tb testing.TB, c uuid.UUID) (reads, tree, plan time.Duration) {
	tb.Helper()
	ctx := context.Background()
	store := postgresadapter.New(b.pool)
	at := time.Now()
	s := domain.RevertState{Groups: map[uuid.UUID][]domain.TrashMember{}, At: time.Now(), Retention: 1440 * time.Hour}
	var err error
	if s.Items, err = store.ChangesetItems(ctx, c); err != nil {
		tb.Fatal(err)
	}
	if s.Versions, err = store.ChangesetVersions(ctx, c); err != nil {
		tb.Fatal(err)
	}
	var ids []uuid.UUID
	for _, it := range s.Items {
		ids = append(ids, it.NodeID)
	}
	for _, v := range s.Versions {
		ids = append(ids, v.NodeID)
	}
	if s.Nodes, err = store.NodesNow(ctx, ids, c); err != nil {
		tb.Fatal(err)
	}
	for root, deleted := range domain.RevertGroups(s.Items, s.Nodes) {
		if s.Groups[root], err = store.TrashGroup(ctx, b.nb, root, deleted); err != nil {
			tb.Fatal(err)
		}
	}
	var pages []uuid.UUID
	for id, n := range s.Nodes {
		if n.Kind == domain.KindPage && n.DeletedAt == nil {
			pages = append(pages, id)
		}
	}
	if s.Contents, err = store.ContentsNow(ctx, pages); err != nil {
		tb.Fatal(err)
	}
	if s.Hashes, err = store.VersionHashes(ctx, domain.RevertRefs(s.Items, s.Versions, s.Groups)); err != nil {
		tb.Fatal(err)
	}
	reads, at = time.Since(at), time.Now()
	if s.Tree, err = store.ListNodes(ctx, b.nb); err != nil {
		tb.Fatal(err)
	}
	tree, at = time.Since(at), time.Now()
	p, err := domain.PlanRevert(s)
	plan = time.Since(at)
	if err != nil || p.Count > 0 || len(p.Missing) > 0 || len(p.Ops) == 0 {
		tb.Fatalf("the plan of %s = %d ops, %d conflicts, %d missing, %v; want ops alone", c, len(p.Ops), p.Count, len(p.Missing), err)
	}
	return reads, tree, plan
}

// BenchmarkRevert reverts, in a notebook of 0 or 100,000 nodes more, a
// changeset at the unit's bounds of each kind a revert does most with
// (M8/P3 design 3.11): a deletion of an item of the full load, 100 pages
// and 9,900 attachments, its history at the bound's 400,000 rows; a
// creation of 10,000 nodes, an import's. Beside them, the restore of the
// same deletion's item, which the revert of it is (M8/P2A's
// BenchmarkTrashRestoreHistory). Without statistics, then analyzed while
// the item is alive, then planned for any arguments. Each iteration a
// revert, or a restore, after the deletion, or followed by the revert of
// the revert, each untimed, after a vacuum; the iteration's changesets
// are then deleted, so that each finds the history at the bound. It
// reports the request; the parse of the contents in it; the unit, the
// request less the parse; the reads of the changeset, the tree's read and
// the plan, each once, as measured outside the request; and the rest, the
// unit less the tree's reads and plans, which grows with the changeset;
// each a mean in milliseconds. The bench's own reads and plan run just
// before the request, untimed: the revert's reads of the changeset (70 to
// 280 ms of its 3.3 s) find it in the cache, a warm request's numbers
// (M8/P3A review C N-4).
//
// Measured on an Apple M5 Max shared with other work (load 4–16), means
// of three, ms: the request, the unit and the rest, without statistics,
// analyzed, planned for any arguments (the first deletion's row at load
// 16, again at load 4):
//
//	deletion, 0 more nodes        4,754 / 4,521 / 4,480   3,553 / 3,320 / 3,270   3,611 / 3,378 / 3,326
//	  again                       3,294 / 3,084 / 3,041   3,329 / 3,119 / 3,076   3,773 / 3,562 / 3,507
//	deletion, 100,000 more nodes  3,572 / 3,364 / 3,090   3,536 / 3,328 / 3,039   3,783 / 3,575 / 3,284
//	its restore, 0 more nodes     3,370 / 3,159           3,302 / 3,092           3,881 / 3,670
//	its restore, 100,000 more     3,069 / 2,870           3,155 / 2,956           3,647 / 3,448
//	creation, 0 more nodes          447 /   447 /   420     389 /   389 /   360     405 /   405 /   375
//	creation, 100,000 more nodes    609 /   609 /   335     653 /   653 /   391     630 /   630 /   372
//
// The parse 199–233 ms; at 100,000 more nodes the tree's read 62–71 ms
// and the plan 61–79 ms, each twice a revert; the reads of the changeset
// 69–283 ms. The part that grows with the changeset is under 5 s at the
// full load, 3.0–3.5 s (4.5 s at load 16): a revert keeps the unit's
// bounds, MaxUnitNodes and MaxUnitHistory (M8/P3 design 3.11), and a
// revert of a deletion costs its restore's and a few hundred ms.
func BenchmarkRevert(b *testing.B) {
	for _, others := range []int{0, 100_000} {
		for _, kind := range []string{"deletion", "restore", "creation"} {
			bench := newTrashBench(b, 1<<20, 1<<30)
			root, contents := bench.item(b, benchLoads()[1], domain.MaxUnitNodes-domain.MaxUnitPages)
			var parse time.Duration
			var created uuid.UUID
			if kind == "creation" {
				created = bench.imported(b)
			} else {
				bench.versions(b, 1, historyAtTheBound)
				parse = bench.parsed(contents)
			}
			bench.others(b, others)
			for _, stats := range []string{"without statistics", "analyzed", "planned for any arguments"} {
				switch stats {
				case "analyzed":
					if _, err := bench.pool.Exec(context.Background(), "ANALYZE"); err != nil {
						b.Fatal(err)
					}
				case "planned for any arguments":
					bench.generic(b)
				}
				b.Run(fmt.Sprintf("%s/%d other nodes/%s", kind, others, stats), func(b *testing.B) {
					var request, reads, tree, plan time.Duration
					for range b.N {
						b.StopTimer()
						bench.vacuumed(b)
						c := created
						if kind != "creation" {
							bench.timed(b, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
							c = bench.newestChangeset(b)
							bench.vacuumed(b)
						}
						path := "/api/v0/nodes/" + root + "/restore"
						if kind != "restore" {
							r, tr, pl := bench.planned(b, c)
							reads, tree, plan = reads+r, tree+tr, plan+pl
							path = "/api/v0/changesets/" + c.String() + "/revert"
						}
						b.StartTimer()
						request += bench.timed(b, http.MethodPost, path, http.StatusOK)
						b.StopTimer()
						done := bench.newestChangeset(b)
						if kind == "creation" {
							bench.vacuumed(b)
							bench.timed(b, http.MethodPost, "/api/v0/changesets/"+done.String()+"/revert", http.StatusOK)
							bench.forget(b, done, bench.newestChangeset(b))
						} else {
							bench.forget(b, c, done)
						}
						b.StartTimer()
					}
					n := float64(b.N)
					ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) / n }
					unit := ms(request) - float64(parse)/float64(time.Millisecond)
					b.ReportMetric(ms(request), "request-ms")
					b.ReportMetric(float64(parse)/float64(time.Millisecond), "parse-ms")
					b.ReportMetric(unit, "unit-ms")
					b.ReportMetric(ms(reads), "reads-ms")
					b.ReportMetric(ms(tree), "tree-ms")
					b.ReportMetric(ms(plan), "plan-ms")
					b.ReportMetric(unit-2*(ms(tree)+ms(plan)), "rest-ms")
				})
			}
		}
	}
}

// tops makes n nodes of kind at the notebook's root, each a top, the
// creations of one changeset, an import's, as a flat vault's import
// writes it (M8/P3A review C I-2): pages each with its content and its
// version, or attachments each with its row.
func (b trashBench) tops(tb testing.TB, kind domain.Kind, n int) uuid.UUID {
	tb.Helper()
	ctx := context.Background()
	c := uuid.NewV7()
	type statement struct {
		sql  string
		args []any
	}
	stmts := []statement{
		{`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
		VALUES ($1, $2, 'import', 'web', $3, now(), now())`, []any{c, b.nb, b.alice}},
		{`INSERT INTO nodes (id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), $1, NULL, $4, 'top ' || g || CASE WHEN $4 = 'asset' THEN '.png' ELSE '' END,
			'top ' || g || CASE WHEN $4 = 'asset' THEN '.png' ELSE '' END, g, $2, $2, now(), now() FROM generate_series(1, $3::integer) g`,
			[]any{b.nb, b.alice, n, string(kind)}},
	}
	if kind == domain.KindPage {
		stmts = append(stmts,
			statement{`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT id, 'x', 1, sha256('x'), 1, $2, now() FROM nodes WHERE notebook_id = $1 AND name LIKE 'top %'`, []any{b.nb, b.alice}},
			statement{`INSERT INTO page_revisions (id, changeset_id, node_id, revision, content, content_hash, byte_size, created_at, updated_at)
			SELECT gen_random_uuid(), $2, id, 1, 'x', sha256('x'), 1, now(), now() FROM nodes WHERE notebook_id = $1 AND name LIKE 'top %'`,
				[]any{b.nb, c}},
			statement{`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, revision, created_at, updated_at)
			SELECT gen_random_uuid(), $2, id, parent_id, name, sort_order, 1, now(), now() FROM nodes WHERE notebook_id = $1 AND name LIKE 'top %'`,
				[]any{b.nb, c}})
	} else {
		stmts = append(stmts,
			statement{`INSERT INTO asset_blobs (id, node_id, notebook_id, mime, byte_size, sha256, created_by_id, created_at)
			SELECT gen_random_uuid(), id, notebook_id, 'image/png', 3, sha256('abc'), created_by_id, created_at FROM nodes
			WHERE notebook_id = $1 AND name LIKE 'top %'`, []any{b.nb}},
			statement{`INSERT INTO changeset_items (id, changeset_id, node_id, after_parent_id, after_name, after_sort_order, created_at, updated_at)
			SELECT gen_random_uuid(), $2, id, parent_id, name, sort_order, now(), now() FROM nodes WHERE notebook_id = $1 AND name LIKE 'top %'`,
				[]any{b.nb, c}})
	}
	for _, x := range stmts {
		if _, err := b.pool.Exec(ctx, x.sql, x.args...); err != nil {
			tb.Fatalf("%s: %v", x.sql, err)
		}
	}
	return c
}

// untrash brings every node of the notebook in the trash back, with what
// followed it, through SQL: the tops a revert of their creation deleted,
// whose revert of that revert would parse more pages than a unit does.
func (b trashBench) untrash(tb testing.TB) {
	tb.Helper()
	ctx := context.Background()
	for _, sql := range []string{
		`UPDATE page_contents SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)`,
		`UPDATE page_revisions SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)`,
		`UPDATE changeset_items SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND node_id IN (SELECT id FROM nodes WHERE notebook_id = $1)`,
		`UPDATE asset_blobs SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND notebook_id = $1`,
		`UPDATE nodes SET deleted_at = NULL, trash_root = false, trash_pages = NULL, trash_assets = NULL, trash_height = NULL
		WHERE notebook_id = $1 AND deleted_at IS NOT NULL`,
	} {
		if _, err := b.pool.Exec(ctx, sql, b.nb); err != nil {
			tb.Fatalf("%s: %v", sql, err)
		}
	}
}

// BenchmarkRevertOfManyTops reverts changesets of many tops, each a step
// of the round, a unit's operation (M8/P3A review C I-2), in a notebook of
// 0 or 100,000 nodes more: the creation of n pages at the root, a flat
// vault's import; of n attachments there; and the revert of the revert of
// that, which restores n trash items. Without statistics, then analyzed,
// then planned for any arguments, as BenchmarkRevert. Each iteration the
// revert, after a vacuum, timed; then, untimed, the tops put back through
// SQL and the revert's changeset deleted (for the restore, the revert
// before it, and both changesets deleted after). The request is measured
// as the server's, but the changeset and the tree were read just before
// it, untimed, by the bench's own plan, so the revert's reads find them in
// the cache. It reports the request, the reads of the changeset, the
// tree's read and the plan, and the rest, the request less the tree's
// reads and plans, twice, which grows with the steps (nothing is parsed);
// each a mean in milliseconds.
//
//	go test -run '^$' -bench 'BenchmarkRevertOfManyTops' -benchtime 3x ./internal/bootstrap/
//
// Measured on an Apple M5 Max shared with other work (load 4-16), means
// of three, ms: the request without statistics, analyzed, planned for any
// arguments, in a notebook of 0 and of 100,000 more nodes (the restores of
// 1,500 and 2,000 measured three times, the run shown the last):
//
//	                                 0 more nodes             100,000 more nodes
//	pages' creation, 1,000 tops      1,638 / 1,623 / 1,664    1,779 / 1,765 / 1,733
//	                 2,000 tops      3,802 / 3,304 / 2,914    3,386 / 3,467 / 2,902
//	                 3,000 tops      5,842 / 4,632 / 4,227    4,943 / 5,024 / 4,456
//	attachments' creation, 1,000     1,543 / 1,380 / 1,207    1,675 / 1,808 / 1,587
//	                 2,000           3,644 / 3,429 / 3,471    2,800 / 2,837 / 2,518
//	                 3,000           5,330 / 3,731 / 3,307    3,938 / 4,049 / 3,708
//	their restore, 1,000             1,709 / 1,872 / 1,944    1,987 / 2,516 / 2,203
//	                 1,500           2,606 / 2,973 / 3,226    2,996 / 4,054 / 3,549 (4,787 at most)
//	                 2,000           3,481 / 4,220 / 4,913    4,588 / 7,656 / 500 at 15 s (5,883, 6,509)
//	                 3,000           6,595 / 7,527 / 9,271    6,020 / 10,020 / 8,301
//
// At 100,000 more nodes the tree's read and the plan take about 60 ms
// each, twice; the rest is the request less 250 ms. MaxRevertSteps is
// 1,500. The time grows faster than the steps: each top's deletion reads
// its subtree, whose FindNodeIn and ChildrenOfAll go, without statistics,
// through the titles' partial index of the whole notebook (2.7 million
// index rows for 1,000 tops, 27 million for 3,000); DeleteNodes' stray
// count reads the time's deletions so far (a revert of 2,000 tops without
// it, 2.74 s against 2.90 s); a restore reads each group twice and its
// history once, about 0.45 ms a group a read. Reading them by the batch
// is M12's (its performance hand-off).
func BenchmarkRevertOfManyTops(b *testing.B) {
	for _, kind := range []string{"pages' creation", "attachments' creation", "attachments' restore"} {
		for _, n := range manyTops() {
			for _, others := range []int{0, 100_000} {
				b.Run(fmt.Sprintf("%s/%d tops/%d other nodes", kind, n, others), func(b *testing.B) {
					revertManyTops(b, kind, n, others)
				})
			}
		}
	}
}

// revertManyTops is BenchmarkRevertOfManyTops of a kind, n tops and
// others more nodes, at each statistics.
func revertManyTops(b *testing.B, kind string, n, others int) {
	bench := newTrashBenchOfSteps(b, 0, 0, 1<<20)
	c := bench.tops(b, map[bool]domain.Kind{true: domain.KindPage, false: domain.KindAsset}[kind == "pages' creation"], n)
	bench.others(b, others)
	for _, stats := range []string{"without statistics", "analyzed", "planned for any arguments"} {
		switch stats {
		case "analyzed":
			if _, err := bench.pool.Exec(context.Background(), "ANALYZE"); err != nil {
				b.Fatal(err)
			}
		case "planned for any arguments":
			bench.generic(b)
		}
		b.Run(stats, func(b *testing.B) {
			var request, reads, tree, plan time.Duration
			for range b.N {
				b.StopTimer()
				bench.vacuumed(b)
				target := c
				if kind == "attachments' restore" {
					bench.timed(b, http.MethodPost, "/api/v0/changesets/"+c.String()+"/revert", http.StatusOK)
					target = bench.newestChangeset(b)
					bench.vacuumed(b)
				}
				r, tr, pl := bench.planned(b, target)
				reads, tree, plan = reads+r, tree+tr, plan+pl
				b.StartTimer()
				request += bench.timed(b, http.MethodPost, "/api/v0/changesets/"+target.String()+"/revert", http.StatusOK)
				b.StopTimer()
				done := bench.newestChangeset(b)
				if kind == "attachments' restore" {
					bench.forget(b, target, done)
				} else {
					bench.untrash(b)
					bench.forget(b, done)
				}
				b.StartTimer()
			}
			runs := float64(b.N)
			ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) / runs }
			b.ReportMetric(ms(request), "request-ms")
			b.ReportMetric(ms(reads), "reads-ms")
			b.ReportMetric(ms(tree), "tree-ms")
			b.ReportMetric(ms(plan), "plan-ms")
			b.ReportMetric(ms(request)-2*(ms(tree)+ms(plan)), "rest-ms")
		})
	}
}

// manyTops are the counts of tops BenchmarkRevertOfManyTops reverts,
// NERVE_BENCH_TOPS's when it names them (comma-separated).
func manyTops() []int {
	out := []int{1000, 1500, 2000, 3000}
	if s := os.Getenv("NERVE_BENCH_TOPS"); s != "" {
		out = nil
		for f := range strings.SplitSeq(s, ",") {
			n, err := strconv.Atoi(f)
			if err != nil {
				panic(err)
			}
			out = append(out, n)
		}
	}
	return out
}

// BenchmarkPurgeOfTrimmedChangesets runs the purge once over 100,000
// changesets of a live notebook each of two items, one of a node long in
// the trash: half keep their other item, of a page alive, and are marked
// trimmed; half lose both, and the purge deletes them, trimmed and empty
// (M8/P3 design 3.3, 3.11): 6.6 s a run, in batches of 1,000, on the
// machine BenchmarkRevert was measured on.
func BenchmarkPurgeOfTrimmedChangesets(b *testing.B) {
	for range b.N {
		b.StopTimer()
		bench := newTrashBench(b, 0, 0)
		ctx := context.Background()
		long := time.Now().Add(-2000 * time.Hour)
		for _, x := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, 'page', 'Alive', 'alive', 0, $2, $2, now(), now())`, []any{bench.nb, bench.alice}},
			{`INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT id, '', 1, sha256(''), 0, $2, now() FROM nodes WHERE notebook_id = $1`, []any{bench.nb, bench.alice}},
			{`INSERT INTO nodes (id, notebook_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at, updated_at,
				deleted_at, trash_root, trash_pages, trash_assets, trash_height)
			SELECT gen_random_uuid(), $1, 'page', 'Gone ' || g, 'gone ' || g, g, $2, $2, $3, $3, $3, true, 1, 0, 1
			FROM generate_series(1, 2000) g`, []any{bench.nb, bench.alice, long}},
			{`INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at)
			SELECT md5($1::uuid::text || g)::uuid, $1, 'edit', 'web', $2, $3, $3 FROM generate_series(1, 100000) g`,
				[]any{bench.nb, bench.alice, long}},
			{`WITH gone AS (SELECT id, row_number() OVER (ORDER BY id) - 1 AS k FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL)
			INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), md5($1::uuid::text || g)::uuid, gone.id, 'Gone', 0, $2, $2, $2
			FROM generate_series(1, 100000) g JOIN gone ON gone.k = g % 1000`, []any{bench.nb, long}},
			{`WITH gone AS (SELECT id, row_number() OVER (ORDER BY id) - 1 AS k FROM nodes WHERE notebook_id = $1 AND deleted_at IS NOT NULL),
				alive AS (SELECT id FROM nodes WHERE notebook_id = $1 AND deleted_at IS NULL)
			INSERT INTO changeset_items (id, changeset_id, node_id, after_name, after_sort_order, created_at, updated_at, deleted_at)
			SELECT gen_random_uuid(), md5($1::uuid::text || g)::uuid, CASE WHEN g % 2 = 0 THEN alive.id ELSE gone.id END, 'Other', 0, $2, $2,
				CASE WHEN g % 2 = 0 THEN NULL ELSE $2::timestamptz END
			FROM generate_series(1, 100000) g JOIN gone ON gone.k = 1000 + g % 1000, alive`, []any{bench.nb, long}},
		} {
			if _, err := bench.pool.Exec(ctx, x.sql, x.args...); err != nil {
				b.Fatalf("%s: %v", x.sql, err)
			}
		}
		store, err := storage.OpenLocal(b.TempDir(), 0)
		if err != nil {
			b.Fatal(err)
		}
		all := purgers(bench.pool, postgres.NewTxManager(bench.pool, time.Minute), store, slog.New(slog.DiscardHandler))
		b.StartTimer()
		for _, p := range all {
			for {
				n, err := p.Purge(ctx, time.Now().Add(-1440*time.Hour), 1000)
				if err != nil {
					b.Fatalf("%s: %v", p.Table, err)
				}
				if n < 1000 {
					break
				}
			}
		}
		b.StopTimer()
		var left, trimmed int
		if err := bench.pool.QueryRow(ctx, "SELECT count(*), count(*) FILTER (WHERE trimmed) FROM changesets WHERE notebook_id = $1",
			bench.nb).Scan(&left, &trimmed); err != nil || left != 50000 || trimmed != 50000 {
			b.Fatalf("%d changesets left, %d trimmed, %v; want the half that keep an item, each trimmed", left, trimmed, err)
		}
	}
}

// A revert at the unit's bounds takes its time (M8/P3 design 3.11): the
// deletion of the full load's item at the bound of its nodes and of its
// history, as TestARestoreAtTheUnitsBoundTakesItsTime's, reverted in 10
// seconds, the time logged for CI's runs (make test-go runs it with -v).
// The deletion's changeset one item past the unit's nodes, its revert is
// page.revert_too_large before reading them. Deleted again, its history
// 420,000 rows with the items of the revert and of the deletion, 20,000
// past the bound, its revert is page.revert_too_large. Neither writes
// anything. The bench keeps the composition root's bounds, which the
// module hands the revert: one far from the domain's, or the two
// swapped, fails here.
func TestARevertAtTheUnitsBoundTakesItsTime(t *testing.T) {
	bench := newTrashBench(t, 0, 0)
	root, _ := bench.item(t, benchLoads()[1], domain.MaxUnitNodes-domain.MaxUnitPages)
	bench.versions(t, 1, historyAtTheBound)
	if status, answer := bench.serve(http.MethodPost, "/api/v0/notebooks/"+bench.nb.String()+"/pages", `{"parent_id":null,"title":"Extra"}`); status != http.StatusCreated {
		t.Fatalf("create Extra = %d %s", status, answer)
	}
	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
	c := bench.newestChangeset(t)
	rows := bench.history(t, root)
	if rows > domain.MaxUnitHistory {
		t.Fatalf("the item's history is %d rows, past the bound %d before it is topped up", rows, domain.MaxUnitHistory)
	}
	bench.toppedUp(t, root, domain.MaxUnitHistory-rows)
	if rows := bench.history(t, root); rows != domain.MaxUnitHistory {
		t.Fatalf("the item's history is %d rows, want the bound %d", rows, domain.MaxUnitHistory)
	}
	revert := "/api/v0/changesets/" + c.String() + "/revert"
	refused := func(what string) {
		t.Helper()
		if status, answer := bench.serve(http.MethodPost, revert, ""); status != http.StatusUnprocessableEntity ||
			!strings.Contains(answer, `"code":"page.revert_too_large"`) {
			t.Errorf("a revert %s = %d %s, want 422 page.revert_too_large", what, status, answer)
		}
		if n := count(t, bench.pool, "SELECT count(*) FROM changesets WHERE kind = 'revert'"); n > 1 {
			t.Errorf("%d reverts' changesets after a revert %s, want the first's at most", n, what)
		}
	}
	// Extra's creation in the deletion's changeset: an item past the bound.
	moved := func(from, to uuid.UUID) {
		t.Helper()
		if _, err := bench.pool.Exec(context.Background(), `UPDATE changeset_items SET changeset_id = $2 WHERE changeset_id = $1
			AND node_id = (SELECT id FROM nodes WHERE name = 'Extra')`, from, to); err != nil {
			t.Fatal(err)
		}
	}
	extra := bench.newestChangesetOf(t, "Extra")
	moved(extra, c)
	refused("of an item past the unit's nodes")
	moved(c, extra)

	took := bench.timed(t, http.MethodPost, revert, http.StatusOK)
	t.Logf("a revert of a deletion of %d nodes, %d rows of history, took %s", domain.MaxUnitNodes, domain.MaxUnitHistory, took)
	if took > 10*time.Second {
		t.Errorf("a revert of a deletion of %d nodes, %d rows of history, took %s, want 10 s at most", domain.MaxUnitNodes,
			domain.MaxUnitHistory, took)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL"); n != domain.MaxUnitNodes+1 {
		t.Errorf("%d nodes alive, want the %d back and Extra", n, domain.MaxUnitNodes)
	}
	checkPages(t, bench.pool)

	bench.timed(t, http.MethodDelete, "/api/v0/nodes/"+root, http.StatusNoContent)
	c = bench.newestChangeset(t)
	revert = "/api/v0/changesets/" + c.String() + "/revert"
	refused("past the history's bound")
	checkPages(t, bench.pool)
}

// A revert of many tops at the bound of its round's steps takes its time
// (M8/P3A review C I-2): the creation of MaxRevertSteps attachments at the
// notebook's root, a flat import's, reverted; then the revert of that
// revert, which restores as many trash items, the costliest step
// measured, in 10 seconds, the time logged for CI's runs (make test-go
// runs it with -v). One top more, its revert is page.revert_too_large,
// after the plan, writing nothing. The bench keeps the composition root's
// bound, which the module hands the revert.
func TestARevertOfManyTopsAtItsBoundTakesItsTime(t *testing.T) {
	bench := newTrashBench(t, 0, 0)
	c := bench.tops(t, domain.KindAsset, domain.MaxRevertSteps+1)
	revert := func(id uuid.UUID) string { return "/api/v0/changesets/" + id.String() + "/revert" }
	if status, answer := bench.serve(http.MethodPost, revert(c), ""); status != http.StatusUnprocessableEntity ||
		!strings.Contains(answer, `"code":"page.revert_too_large"`) {
		t.Errorf("a revert of %d tops = %d %s, want 422 page.revert_too_large", domain.MaxRevertSteps+1, status, answer)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM changesets WHERE kind = 'revert'"); n != 0 {
		t.Errorf("%d reverts' changesets after a revert past the steps, want none", n)
	}
	if _, err := bench.pool.Exec(context.Background(), `DELETE FROM changeset_items WHERE id = (SELECT id FROM changeset_items
		WHERE changeset_id = $1 ORDER BY node_id LIMIT 1)`, c); err != nil {
		t.Fatal(err)
	}
	bench.timed(t, http.MethodPost, revert(c), http.StatusOK)
	r := bench.newestChangeset(t)
	bench.vacuumed(t)
	took := bench.timed(t, http.MethodPost, revert(r), http.StatusOK)
	t.Logf("a revert restoring %d trash items took %s", domain.MaxRevertSteps, took)
	if took > 10*time.Second {
		t.Errorf("a revert restoring %d trash items took %s, want 10 s at most", domain.MaxRevertSteps, took)
	}
	if n := count(t, bench.pool, "SELECT count(*) FROM nodes WHERE deleted_at IS NULL"); n != domain.MaxRevertSteps+1 {
		t.Errorf("%d nodes alive, want the %d back and the one left out", n, domain.MaxRevertSteps)
	}
	checkPages(t, bench.pool)
}

// newestChangesetOf is the changeset of the page named name's creation.
func (b trashBench) newestChangesetOf(tb testing.TB, name string) uuid.UUID {
	tb.Helper()
	var id uuid.UUID
	if err := b.pool.QueryRow(context.Background(), `SELECT i.changeset_id FROM changeset_items i JOIN nodes n ON n.id = i.node_id
		WHERE n.notebook_id = $1 AND n.name = $2 AND i.before_name IS NULL`, b.nb, name).Scan(&id); err != nil {
		tb.Fatal(err)
	}
	return id
}
