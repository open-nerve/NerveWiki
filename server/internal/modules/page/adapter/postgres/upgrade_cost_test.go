//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// upgradeProtectedBound is the median of five reads the test below allows
// in each state: three times the larger of the medians of two local rounds
// of it, rounded up to 5 ms (05-lessons 1; the M11/P1 S1 review's fix 1,
// decision 236). The two rounds' medians, without statistics, analyzed,
// with the other upgrade's statistics alone, and planned for any
// arguments, were 7.5, 6.1, 6.3 and 5.1 ms, and 6.5, 5.8, 5.3 and 6.4 ms;
// three times 7.5 ms is 22.5 ms. CI's costs-2 took 6.0 to 8.6 ms. The
// plans come first: a parallel scan of the table whole took 19 ms here in
// the first three states, within the bound, and 564 ms planned for any
// arguments; a read by the upgrade's id alone, 40 to 73 ms and 572 ms.
const upgradeProtectedBound = 25 * time.Millisecond

// The deletion guardrail's read is a read by the primary key at any
// statistics (M11/P1 design 7.1's cost guard, decision 236): the delete of
// a subtree of MaxUnitNodes nodes, half of them of the upgrade's baseline
// of 100,000 rows, half created since, beside another upgrade's 100,000
// rows, read on one connection, which caches the statement as a server's
// does: without statistics; analyzed; with statistics of the other
// upgrade's rows alone, this one's written after them, as when a start's
// ANALYZE failed; and planned for any arguments, as a cached statement may
// be from its sixth run. In each, the statement's plans for its arguments
// and generic, as a cached statement gets either, read upgrade_nodes once,
// by upgrade_nodes_pkey, both of its columns in the index's condition, and
// no part of the table whole; the key's columns are pinned, as PG18 reads
// an index by a column not its first (05-lessons 4). Each read answers the
// subtree's half of the baseline, the median of five within
// upgradeProtectedBound; -v prints the medians.
func TestUpgradeProtectedReadsInTimeAtTheUnitsBound(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	var def string
	if err := f.pool.QueryRow(ctx, "SELECT pg_get_indexdef('upgrade_nodes_pkey'::regclass)").Scan(&def); err != nil {
		t.Fatalf("upgrade_nodes_pkey: %v", err)
	}
	if !strings.HasPrefix(def, "CREATE UNIQUE INDEX upgrade_nodes_pkey ON ") || !strings.HasSuffix(def, " USING btree (upgrade_id, node_id)") {
		t.Fatalf("upgrade_nodes_pkey is %q, want a unique btree of (upgrade_id, node_id)", def)
	}
	f.exec(t, "ALTER TABLE upgrade_nodes SET (autovacuum_enabled = false)")
	upgrade, other := uuid.NewV7(), uuid.NewV7()
	started := time.Now()
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT u.id, gen_random_uuid(), u.notebook_id, true, 'page', NULL, 'p' || g, g, 1, sha256(g::text::bytea), 1, 0,
			sha256(g::text::bytea)
		FROM (VALUES ($1::uuid, $2::uuid), ($3::uuid, $4::uuid)) u(id, notebook_id), generate_series(1, 100000) g`,
		upgrade, f.eng, other, f.ops)
	var ids []uuid.UUID
	rows, err := f.pool.Query(ctx, "SELECT node_id FROM upgrade_nodes WHERE upgrade_id = $1 ORDER BY node_id LIMIT $2", upgrade,
		domain.MaxUnitNodes/2)
	if err == nil {
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				break
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err == nil {
			err = rows.Err()
		}
	}
	if err != nil || len(ids) != domain.MaxUnitNodes/2 {
		t.Fatalf("the baseline's ids: %d, %v", len(ids), err)
	}
	for range domain.MaxUnitNodes / 2 {
		ids = append(ids, uuid.NewV7())
	}
	t.Logf("seeded in %s", time.Since(started))

	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := postgresadapter.New(pool)
	protected := statementOf(t, func(q *gen.Queries) error {
		_, err := q.UpgradeProtected(ctx, gen.UpgradeProtectedParams{UpgradeID: upgrade, NodeIds: ids})
		return err
	})
	read := func(stats string) {
		t.Helper()
		protectedPlans(t, pool, stats, protected)
		var took []time.Duration
		for range 5 {
			began := time.Now()
			got, err := s.Protected(ctx, upgrade, ids)
			took = append(took, time.Since(began))
			if err != nil || len(got) != domain.MaxUnitNodes/2 {
				t.Fatalf("Protected(%s) = %d ids, %v; want %d", stats, len(got), err, domain.MaxUnitNodes/2)
			}
		}
		slices.Sort(took)
		t.Logf("%s: the median of five reads of %d ids, %s", stats, len(ids), took[2])
		if took[2] > upgradeProtectedBound {
			t.Errorf("%s: the median of five reads of %d ids took %s, past %s", stats, len(ids), took[2], upgradeProtectedBound)
		}
	}
	read("without statistics")
	f.exec(t, "ANALYZE upgrade_nodes")
	read("analyzed")
	f.exec(t, "CREATE TABLE kept AS SELECT * FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
	f.exec(t, "DELETE FROM upgrade_nodes WHERE upgrade_id = $1", upgrade)
	f.exec(t, "ANALYZE upgrade_nodes")
	f.exec(t, "INSERT INTO upgrade_nodes SELECT * FROM kept")
	read("with statistics of the other upgrade's rows alone")
	if _, err := pool.Exec(ctx, "SET plan_cache_mode = force_generic_plan"); err != nil {
		t.Fatal(err)
	}
	read("planned for any arguments")
}

// protectedPlans fails t unless each plan of s on pool's connection, for
// its arguments and generic, as the cache of prepared statements makes
// them, reads upgrade_nodes once, by upgrade_nodes_pkey, upgrade_id and
// node_id each an equality of the index's condition, and scans no part of
// it whole.
func protectedPlans(t *testing.T, pool *pgxpool.Pool, stats string, s statement) {
	t.Helper()
	ctx := context.Background()
	const name = "upgrade_protected"
	for _, mode := range []string{"custom", "generic"} {
		var raw string
		var generic, custom int64
		err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode = force_"+mode+"_plan"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, "PREPARE "+name+" AS "+s.sql); err != nil {
				return err
			}
			results, err := tx.Conn().PgConn().Exec(ctx,
				"EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) "+sourceProbeExecute(t, name, s.args)).ReadAll()
			if err != nil {
				return err
			}
			if len(results) != 1 || len(results[0].Rows) != 1 {
				return fmt.Errorf("the plan: %d results", len(results))
			}
			raw = string(results[0].Rows[0][0])
			if err := tx.QueryRow(ctx, "SELECT generic_plans, custom_plans FROM pg_prepared_statements WHERE name = $1", name).
				Scan(&generic, &custom); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "DEALLOCATE "+name)
			return err
		})
		if err != nil {
			t.Fatalf("%s, %s: %v", stats, mode, err)
		}
		if mode == "custom" && (custom != 1 || generic != 0) || mode == "generic" && (generic != 1 || custom != 0) {
			t.Fatalf("%s, %s: the plan was generic %d times, custom %d", stats, mode, generic, custom)
		}
		var plans []struct {
			Plan planNode `json:"Plan"`
		}
		if err := json.Unmarshal([]byte(raw), &plans); err != nil || len(plans) != 1 {
			t.Fatalf("%s, %s: the plan %.2000s: %v", stats, mode, raw, err)
		}
		plan := plans[0].Plan
		if whole := plan.all(func(n *planNode) bool { return n.Relation == "upgrade_nodes" && n.Type == "Seq Scan" }); len(whole) != 0 {
			t.Errorf("%s, %s: scans upgrade_nodes whole:\n%s", stats, mode, planShape(&plan))
		}
		scans := plan.all(func(n *planNode) bool { return n.Relation == "upgrade_nodes" })
		if len(scans) != 1 || !reads("upgrade_nodes", "upgrade_nodes_pkey", "")(scans[0]) {
			t.Errorf("%s, %s: reads upgrade_nodes by other than upgrade_nodes_pkey:\n%s", stats, mode, planShape(&plan))
			continue
		}
		cond := scans[0].IndexCond
		for _, bitmap := range scans[0].all(func(n *planNode) bool { return n.Index == "upgrade_nodes_pkey" }) {
			cond += " " + bitmap.IndexCond
		}
		for _, column := range []string{"(upgrade_nodes.upgrade_id = ", "(upgrade_nodes.node_id = ANY "} {
			if !strings.Contains(cond, column) {
				t.Errorf("%s, %s: the index's condition has no %s…):\n%s", stats, mode, column, planShape(&plan))
			}
		}
	}
}

// planShape is each node of the plan below n, one a line, its conditions
// cut short: a custom plan's carry 10,000 ids.
func planShape(n *planNode) string {
	cut := func(s string) string {
		if len(s) > 160 {
			return s[:160] + "…"
		}
		return s
	}
	var b strings.Builder
	n.each(func(m *planNode) bool {
		fmt.Fprintf(&b, "%s on %s by %s; index condition %s; filter %s\n", m.Type, m.Relation, m.Index, cut(m.IndexCond), cut(m.Filter))
		return true
	})
	return b.String()
}

// The gate reads' guard's notebooks (decision 240, the P2 design review B's
// seed): eng of gateCostFolders folders at the root of gateCostPages pages
// each, the folder among them, 100,000 pages, its upgrade's baseline of
// each, a page's wanted count, gateCostChangesets changesets the upgrade
// wrote and gateCostPlain of eng's before it; beside them
// gateCostOthers notebooks of gateCostOtherPages pages each, each with
// another upgrade's baseline of them and gateCostChangesets changesets of
// it, the rows the reads should not read.
const (
	gateCostFolders     = 1000
	gateCostPages       = 100
	gateCostChangesets  = 1000
	gateCostPlain       = 100000
	gateCostOthers      = 9
	gateCostOtherPages  = 50000
	gateCostBaselineAll = gateCostFolders * gateCostPages
)

// The gate reads' plans (M11/P2 design 7.1, decisions 236, 240, 244): an
// upgrade's check reads its baseline (UpgradeGateBaseline), what it kept
// (UpgradePreserved) and how many changesets it wrote (UpgradeChangesets),
// of an upgrade of a notebook of 100,000 pages, beside nine notebooks of
// 50,000 pages with their upgrades' baselines and changesets.
// upgrade_nodes, nodes, page_contents and changesets never analyzed; then
// analyzed, as after a start; then upgrade_nodes and changesets with the
// other upgrades' statistics alone, this one's written after them, as when
// a start's ANALYZE failed. In each state, for their arguments and
// generic, as a cached statement gets either, each plan reads as
// rollbackReadProblems says: upgrade_nodes by upgrade_nodes_pkey's prefix,
// upgrade_id in its condition; nodes and page_contents by their keys on a
// Nested Loop's inner side, by the outer row's key; changesets by
// changesets_upgrade_id_idx alone; nothing read again whole for each row.
// UpgradePreserved's Sort of this upgrade's rows for its array_agg is no
// problem (design 3.9). A problem fails the test before the reads run.
// Then, on a connection of their own forced to each plan, each read
// answers the seed, three times; -v prints their medians, which no bound
// judges: the check's guard in the composition root does, the whole
// check's time (TestUpgradeCheckTakesItsTimeAtAnyStatistics).
func TestTheUpgradeGateReadsPlansAtAnyStatistics(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tables := append(rollbackCostTables(), "changesets")
	for _, table := range tables {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled = false)")
	}
	upgrade := uuid.NewV7()
	started := time.Now()
	f.exec(t, `WITH k AS MATERIALIZED (
			SELECT k, gen_random_uuid() AS folder FROM generate_series(0, $3::integer - 1) k
		), g AS MATERIALIZED (
			SELECT folder AS id, NULL::uuid AS parent, 'f' || k AS name, k AS i FROM k
			UNION ALL
			SELECT gen_random_uuid(), folder, 'f' || k || '-' || i, i FROM k, generate_series(1, $4::integer - 1) i
		), made AS (
			INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
				updated_at)
			SELECT id, $1, 'page', parent, name, name, i, $2, $2, now(), now() FROM g
		), contents AS (
			INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
			SELECT id, 'x', 1, sha256('x'), 1, $2, now() FROM g
		)
		INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT $5, id, $1, true, 'page', parent, name, i, 1, sha256('x'), 1, i % 7, sha256('x') FROM g`,
		f.eng, f.alice, gateCostFolders, gateCostPages, upgrade)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, upgrade_id)
		SELECT gen_random_uuid(), $1, 'edit', 'web', $2, now(), now(), CASE WHEN g <= $3::integer THEN $4::uuid END
		FROM generate_series(1, $3::integer + $5::integer) g`, f.eng, f.alice, gateCostChangesets, upgrade, gateCostPlain)
	f.exec(t, `INSERT INTO notebooks (id, workspace_id, name, created_by_id, updated_by_id, created_at, updated_at)
		SELECT gen_random_uuid(), b.workspace_id, 'Other ' || g, b.created_by_id, b.created_by_id, now(), now()
		FROM notebooks b, generate_series(1, $2::integer) g WHERE b.id = $1`, f.eng, gateCostOthers)
	f.exec(t, `INSERT INTO nodes (id, notebook_id, kind, parent_id, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
			updated_at)
		SELECT gen_random_uuid(), o.id, 'page', NULL, 'o' || k, 'o' || k, k, $2, $2, now(), now()
		FROM notebooks o, generate_series(1, $1::integer) k WHERE o.name LIKE 'Other %'`, gateCostOtherPages, f.alice)
	f.exec(t, `INSERT INTO page_contents (node_id, content, revision, content_hash, byte_size, updated_by_id, updated_at)
		SELECT id, 'x', 1, sha256('x'), 1, created_by_id, now() FROM nodes WHERE notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	// Each other notebook's upgrade's id is its id's hash, so that its
	// baseline and its changesets name the same upgrade.
	f.exec(t, `INSERT INTO upgrade_nodes (upgrade_id, node_id, notebook_id, baseline, kind, parent_id, name, sort_order, revision,
			content_hash, byte_size, wanted, accepted_hash)
		SELECT md5(n.notebook_id::text)::uuid, n.id, n.notebook_id, true, n.kind, n.parent_id, n.name, n.sort_order, c.revision,
			c.content_hash, c.byte_size, 0, c.content_hash
		FROM nodes n JOIN page_contents c ON c.node_id = n.id WHERE n.notebook_id IN (SELECT id FROM notebooks WHERE name LIKE 'Other %')`)
	f.exec(t, `INSERT INTO changesets (id, notebook_id, kind, client, created_by_id, created_at, updated_at, upgrade_id)
		SELECT gen_random_uuid(), o.id, 'edit', 'web', o.created_by_id, now(), now(), md5(o.id::text)::uuid
		FROM notebooks o, generate_series(1, $1::integer) g WHERE o.name LIKE 'Other %'`, gateCostChangesets)
	seeded := time.Since(started)

	statements := map[string]statement{
		"UpgradeGateBaseline": statementOf(t, func(q *gen.Queries) error { _, err := q.UpgradeGateBaseline(ctx, upgrade); return err }),
		"UpgradePreserved":    statementOf(t, func(q *gen.Queries) error { _, err := q.UpgradePreserved(ctx, upgrade); return err }),
		"UpgradeChangesets": statementOf(t, func(q *gen.Queries) error {
			_, err := q.UpgradeChangesets(ctx, &upgrade)
			return err
		}),
	}
	medians := map[string]any{}
	for _, statistics := range []string{"never analyzed", "analyzed", "the other upgrades' alone"} {
		switch statistics {
		case "never analyzed":
			for _, table := range tables {
				f.exec(t, "SELECT pg_clear_relation_stats('public', '"+table+"')")
				f.exec(t, `SELECT pg_clear_attribute_stats('public', '`+table+`', attname, false)
					FROM pg_attribute WHERE attrelid = '`+table+`'::regclass AND attnum > 0 AND NOT attisdropped`)
			}
		case "analyzed":
			f.exec(t, "ANALYZE upgrade_nodes, nodes, page_contents, changesets")
		case "the other upgrades' alone":
			for _, table := range []string{"upgrade_nodes", "changesets"} {
				f.exec(t, "CREATE TABLE kept_"+table+" AS SELECT * FROM "+table+" WHERE upgrade_id = $1", upgrade)
				f.exec(t, "DELETE FROM "+table+" WHERE upgrade_id = $1", upgrade)
				f.exec(t, "ANALYZE "+table)
				f.exec(t, "INSERT INTO "+table+" SELECT * FROM kept_"+table)
			}
		}
		var planned bool
		for name, s := range statements {
			for _, generic := range []bool{false, true} {
				var plan planNode
				var raw string
				if generic {
					plan, raw = f.genericPlanTree(t, s.sql)
				} else {
					plan, raw = f.explain(t, `EXPLAIN (FORMAT JSON, VERBOSE, COSTS OFF) `+s.sql, s.args, nil)
				}
				if problems := rollbackReadProblems(&plan); len(problems) > 0 {
					planned = true
					t.Errorf("%s, %s, generic %v: %s\n%s\n%.3000s", statistics, name, generic, strings.Join(problems, "; "),
						planShape(&plan), raw)
				}
			}
		}
		if planned {
			t.FailNow()
		}
		for _, mode := range []string{"custom", "generic"} {
			medians[statistics+", "+mode] = gateTimed(t, f, mode, upgrade)
		}
	}
	b, err := json.Marshal(map[string]any{"seed_ms": seeded.Milliseconds(), "medians": medians})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("upgrade gate reads cost %s", b)
}

// gateTimed are the medians of three of each gate read, in ms, on a
// connection of their own whose cached statements are planned as mode
// says, each answering as the seed has it.
func gateTimed(t *testing.T, f fixture, mode string, upgrade uuid.UUID) map[string]float64 {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config.DatabaseConfig{URL: f.pool.Config().ConnString(), MaxConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "SET plan_cache_mode = force_"+mode+"_plan"); err != nil {
		t.Fatal(err)
	}
	s := postgresadapter.New(pool)
	took := map[string][]time.Duration{}
	timed := func(name string, read func() error) {
		began := time.Now()
		err := read()
		took[name] = append(took[name], time.Since(began))
		if err != nil {
			t.Fatalf("%s: %s: %v", mode, name, err)
		}
	}
	for range 3 {
		timed("baseline_ms", func() error {
			rows, err := s.UpgradeGateBaseline(ctx, upgrade)
			if err == nil && len(rows) != gateCostBaselineAll {
				err = fmt.Errorf("%d rows, want %d", len(rows), gateCostBaselineAll)
			}
			return err
		})
		timed("preserved_ms", func() error {
			p, err := s.UpgradePreserved(ctx, upgrade)
			if err == nil && (p.Nodes != gateCostBaselineAll || p.NodesKept != p.Nodes || p.Pages != p.Nodes || p.PagesKept != p.Nodes ||
				len(p.NotKept) != 0) {
				err = fmt.Errorf("%+v, want every one of %d kept", p, gateCostBaselineAll)
			}
			return err
		})
		timed("changesets_ms", func() error {
			n, err := s.UpgradeChangesets(ctx, upgrade)
			if err == nil && n != gateCostChangesets {
				err = fmt.Errorf("%d, want %d", n, gateCostChangesets)
			}
			return err
		})
	}
	out := map[string]float64{}
	for name, d := range took {
		slices.Sort(d)
		out[name] = ms(d[1])
	}
	return out
}
