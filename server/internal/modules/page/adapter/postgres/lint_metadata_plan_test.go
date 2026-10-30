//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

type lintMetadataTrace struct {
	mu    sync.Mutex
	names []string
}

func (trace *lintMetadataTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "-- name: ") {
		trace.mu.Lock()
		trace.names = append(trace.names, strings.Fields(data.SQL)[2])
		trace.mu.Unlock()
	}
	return ctx
}

func (*lintMetadataTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (trace *lintMetadataTrace) read() []string {
	trace.mu.Lock()
	defer trace.mu.Unlock()
	return slices.Clone(trace.names)
}

func lintTracedPool(t *testing.T, f fixture) (*pgxpool.Pool, *lintMetadataTrace) {
	t.Helper()
	trace := &lintMetadataTrace{}
	config := f.pool.Config()
	config.MaxConns, config.ConnConfig.Tracer = 1, trace
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, trace
}

func TestLintMetadataPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("page metadata plan evidence needs its own PostgreSQL database")
	}
	t.Run("cache-route", func(t *testing.T) {
		f := newFixture(t)
		root := f.page(t, f.eng, nil, "Raw", 0)
		child := f.page(t, f.eng, &root.ID, "Member", 0)
		pool, trace := lintTracedPool(t, f)
		store := postgresadapter.New(pool)
		for range 8 {
			got, err := page.NewLintNodes(pool).Nodes(t.Context(), f.eng)
			if err != nil || len(got) != 2 {
				t.Fatalf("real whole-book metadata=%d/%v; want two", len(got), err)
			}
			sources, err := page.NewSourceArea(pool).Nodes(t.Context(), f.eng, root.ID)
			if err != nil || len(sources) != 1 || sources[0].ID != child.ID {
				t.Fatalf("real source metadata=%v/%v", sources, err)
			}
			if _, err := store.FindNodeIn(t.Context(), f.eng, root.ID); err != nil {
				t.Fatal(err)
			}
		}
		cached := pgtest.CachedStatements(t, pool)
		if slices.Contains(cached, "LintNodeMetadata") || !slices.Contains(cached, "FindNodeIn") {
			t.Errorf("actual metadata must use Planned, cached FindNodeIn is positive control: %v", cached)
		}
		count := 0
		for _, name := range trace.read() {
			if name == "LintNodeMetadata" {
				count++
			}
		}
		if count != 16 {
			t.Errorf("metadata trace=%d; want both real reads on each of eight calls", count)
		}
		t.Logf("actual nonempty Store reads=%d; named cached statements=%v", count, cached)
	})
}

// Transaction-local counters observe table, index and TOAST work, including
// cached blocks. These fresh fixtures have no old target-row versions; counts
// are not asserted as a universal MVCC identity for production databases.
func lintMetadataWork(t *testing.T, db postgres.Querier) map[string]sourceRelationWork {
	t.Helper()
	rows, err := db.Query(t.Context(), `WITH relations AS (
        SELECT oid FROM pg_class WHERE oid IN ('nodes'::regclass,'page_contents'::regclass,'page_revisions'::regclass)
        UNION SELECT reltoastrelid FROM pg_class WHERE oid='page_contents'::regclass
    ), objects AS (
        SELECT oid FROM relations UNION SELECT indexrelid FROM pg_index WHERE indrelid IN (SELECT oid FROM relations)
    ) SELECT c.relname,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),
        pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid)
        FROM pg_class c JOIN objects o ON o.oid=c.oid ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]sourceRelationWork{}
	for rows.Next() {
		var name string
		var work sourceRelationWork
		if err := rows.Scan(&name, &work.Scans, &work.Returned, &work.Fetched, &work.Blocks, &work.Hits); err != nil {
			t.Fatal(err)
		}
		out[name] = work
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertLintMetadataWork(t *testing.T, work map[string]sourceRelationWork, nodes, pages int) {
	t.Helper()
	var nodeEntries, contentEntries int64
	for name, value := range work {
		switch {
		case name == "nodes" || name == "page_contents":
			if value.Scans != 0 || value.Returned != 0 {
				t.Errorf("whole relation scan instead of notebook/PK access: %s=%+v", name, value)
			}
		case strings.HasPrefix(name, "nodes_"):
			nodeEntries += value.Returned
		case strings.HasPrefix(name, "page_contents_"):
			contentEntries += value.Returned
		default:
			if value != (sourceRelationWork{}) {
				t.Errorf("unexpected revision/body TOAST work: %s=%+v", name, value)
			}
		}
	}
	if nodeEntries != int64(nodes) || contentEntries != int64(pages) {
		t.Errorf("physical index entries: nodes=%d contents=%d; want exactly %d/%d", nodeEntries, contentEntries, nodes, pages)
	}
}

type lintMetadataPlanEvidence struct {
	Statistics, Phase, Mode   string
	Returned                  int
	GenericPlans, CustomPlans int64
	Settings                  map[string]string
	TreeLevels                map[string]int64
	Work, ExplainWork         map[string]sourceRelationWork
	PlannedPairWork           map[string]sourceRelationWork
	Plan                      json.RawMessage
}

func lintMetadataLevels(t *testing.T, f fixture) map[string]int64 {
	t.Helper()
	f.exec(t, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	rows, err := f.pool.Query(t.Context(), `SELECT c.relname,(bt_metap(c.oid::regclass::text)).level
        FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_am a ON a.oid=c.relam
        WHERE i.indrelid IN ('nodes'::regclass,'page_contents'::regclass) AND a.amname='btree'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	levels := map[string]int64{}
	for rows.Next() {
		var name string
		var level int64
		if err := rows.Scan(&name, &level); err != nil {
			t.Fatal(err)
		}
		levels[name] = level
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return levels
}

// Bound physical blocks by target tuples and the observed tree height, not by
// foreign-table size. A point probe descends one tree; a range may visit fewer
// blocks. The extra root/leaf descent also covers initial index metadata reads.
func assertLintMetadataBlocks(t *testing.T, work map[string]sourceRelationWork, levels map[string]int64, nodes, pages int) {
	t.Helper()
	for name, value := range work {
		if value.Hits > value.Blocks {
			t.Errorf("cache hits exceed fetched blocks for %s: %+v", name, value)
		}
		limit := int64(0)
		switch name {
		case "nodes":
			limit = int64(nodes)
		case "page_contents":
			limit = int64(pages)
		default:
			if level, ok := levels[name]; ok {
				limit = (max(value.Scans, value.Returned) + 1) * (level + 2)
			}
		}
		if value.Blocks > limit {
			t.Errorf("%s blocks=%d exceed target/observed-height bound %d: %+v", name, value.Blocks, limit, value)
		}
	}
}

func lintMetadataPrepared(t *testing.T, f fixture, statistics, phase, mode string, count int, levels map[string]int64) {
	t.Helper()
	ctx := t.Context()
	statement := statementOf(t, func(q *gen.Queries) error { _, err := q.LintNodeMetadata(ctx, f.eng); return err })
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode=force_"+mode+"_plan"); err != nil {
		t.Fatal(err)
	}
	const prepared = "lint_metadata_physical"
	if _, err := tx.Exec(ctx, "PREPARE "+prepared+" AS "+statement.sql); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = tx.Exec(context.Background(), "DEALLOCATE "+prepared) }()
	execute := sourceExecute(t, prepared, statement.args)
	evidence := lintMetadataPlanEvidence{Statistics: statistics, Phase: phase, Mode: mode,
		Settings: map[string]string{}, TreeLevels: levels}
	for _, setting := range []string{"plan_cache_mode", "enable_seqscan", "enable_bitmapscan", "enable_indexscan",
		"enable_indexonlyscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "enable_sort", "track_counts"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatal(err)
		}
		evidence.Settings[setting] = value
		if setting != "plan_cache_mode" && value != "on" {
			t.Fatalf("normal planner and counts required: %s=%s", setting, value)
		}
	}
	before := lintMetadataWork(t, tx)
	rows, err := tx.Query(ctx, execute)
	if err != nil {
		t.Fatal(err)
	}
	columns := make([]string, len(rows.FieldDescriptions()))
	for i, field := range rows.FieldDescriptions() {
		columns[i] = field.Name
	}
	if !slices.Equal(columns, []string{"id", "parent_id", "kind", "name_key", "created_at", "page_bytes", "has_content", "linkable"}) {
		rows.Close()
		t.Fatalf("actual projection includes unexpected fields: %v", columns)
	}
	seen := map[uuid.UUID]bool{}
	badRows := 0
	for rows.Next() {
		var row gen.LintNodeMetadataRow
		if err := rows.Scan(&row.ID, &row.ParentID, &row.Kind, &row.NameKey, &row.CreatedAt, &row.PageBytes, &row.HasContent, &row.Linkable); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if seen[row.ID] || !strings.HasPrefix(row.ID.String(), "7a110000-0000-7000-0001-") || !row.HasContent || !row.Linkable {
			badRows++
		}
		seen[row.ID] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	evidence.Returned = len(seen)
	if badRows != 0 {
		t.Errorf("actual metadata included %d foreign/duplicate/incomplete rows", badRows)
	}
	if len(seen) != count {
		t.Errorf("metadata returned %d IDs; want %d target IDs", len(seen), count)
	}
	evidence.Work = sourceWorkDelta(before, lintMetadataWork(t, tx))
	assertLintMetadataWork(t, evidence.Work, count, count)
	assertLintMetadataBlocks(t, evidence.Work, levels, count, count)
	before = lintMetadataWork(t, tx)
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+execute).Scan(&evidence.Plan); err != nil {
		t.Fatal(err)
	}
	evidence.ExplainWork = sourceWorkDelta(before, lintMetadataWork(t, tx))
	assertLintMetadataWork(t, evidence.ExplainWork, count, count)
	assertLintMetadataBlocks(t, evidence.ExplainWork, levels, count, count)
	if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", prepared).
		Scan(&evidence.GenericPlans, &evidence.CustomPlans); err != nil {
		t.Fatal(err)
	}
	if mode == "generic" && (evidence.GenericPlans != 2 || evidence.CustomPlans != 0) ||
		mode == "custom" && (evidence.CustomPlans != 2 || evidence.GenericPlans != 0) {
		t.Errorf("actual %s PREPARE counters generic/custom=%d/%d", mode, evidence.GenericPlans, evidence.CustomPlans)
	}
	var plans []struct{ Plan sourceExecutedPlan }
	if err := json.Unmarshal(evidence.Plan, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("decode actual plan=%v, documents=%d", err, len(plans))
	}
	nodeRange, contentRange := false, false
	var inspect func(sourceExecutedPlan)
	inspect = func(plan sourceExecutedPlan) {
		if strings.HasPrefix(plan.IndexName, "nodes_") && strings.Contains(plan.IndexCond, "notebook_id") {
			nodeRange = true
		}
		if plan.RelationName == "page_contents" && plan.IndexName == "page_contents_pkey" &&
			strings.Contains(plan.IndexCond, "node_id") && strings.Contains(plan.IndexCond, "n.id") {
			contentRange = true
		}
		if plan.RowsRemovedFilter != 0 {
			t.Errorf("fresh fixture filtered rows outside its physical range: %+v", plan)
		}
		for _, child := range plan.Plans {
			inspect(child)
		}
	}
	inspect(plans[0].Plan)
	if !nodeRange || !contentRange {
		t.Errorf("actual plan lacks notebook/point-content IndexCond: nodes=%t contents=%t", nodeRange, contentRange)
	}
	// The actual port pair must retain the same bounded work under both modes.
	// This corpus has independent roots: the selected root has no descendants,
	// but SourceArea still validates its full metadata. cache-route separately
	// checks a nonempty SourceArea result; PD checks more than 10k descendants.
	err = postgres.NewTxManager(f.pool, 5*time.Second).WithinSnapshot(ctx, func(ctx context.Context) error {
		db := postgres.DB(ctx, f.pool)
		if _, err := db.Exec(ctx, "SET LOCAL plan_cache_mode=force_"+mode+"_plan"); err != nil {
			return err
		}
		before := lintMetadataWork(t, db)
		nodes, err := page.NewLintNodes(f.pool).Nodes(ctx, f.eng)
		if err != nil {
			return err
		}
		root := uuid.MustParse("7a110000-0000-7000-0001-000000000001")
		sources, err := page.NewSourceArea(f.pool).Nodes(ctx, f.eng, root)
		if err != nil {
			return err
		}
		if len(nodes) != count || len(sources) != 0 {
			t.Errorf("real Planned pair returned %d/%d; want %d/0", len(nodes), len(sources), count)
		}
		evidence.PlannedPairWork = sourceWorkDelta(before, lintMetadataWork(t, db))
		assertLintMetadataWork(t, evidence.PlannedPairWork, 2*count, 2*count)
		assertLintMetadataBlocks(t, evidence.PlannedPairWork, levels, 2*count, 2*count)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceEvidence(t, "lint-metadata-range", evidence)
}

func TestLintMetadataPhysicalRanges(t *testing.T) {
	if testing.Short() {
		t.Skip("page metadata physical evidence needs its own PostgreSQL database")
	}
	for _, analyzed := range []bool{false, true} {
		label := "no-statistics"
		if analyzed {
			label = "ANALYZE"
		}
		t.Run(label, func(t *testing.T) {
			for _, phase := range []string{"A-before", "B-hidden-grown", "A-restored"} {
				// Each phase starts from the pristine migrated template. Returning
				// to A by DELETE would retain dead versions and a different tree;
				// that is an MVCC diagnostic, not this fresh-fixture range proof.
				f := newFixture(t)
				for _, table := range []string{"nodes", "page_contents"} {
					f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
				}
				lintSeedPages(t, f, f.eng, nil, 1, 2000)
				lintSeedPages(t, f, f.ops, nil, 2, 50000)
				if phase == "B-hidden-grown" {
					lintSeedPages(t, f, f.ops, nil, 3, 50000)
				}
				if analyzed {
					f.exec(t, "ANALYZE nodes")
					f.exec(t, "ANALYZE page_contents")
				}
				for _, table := range []string{"nodes", "page_contents"} {
					stats := f.count(t, "SELECT count(*) FROM pg_stats WHERE tablename=$1", table)
					if (stats > 0) != analyzed {
						t.Fatalf("%s statistics=%d; analyzed=%t", table, stats, analyzed)
					}
				}
				levels := lintMetadataLevels(t, f)
				for _, mode := range []string{"custom", "generic"} {
					t.Run(phase+"/"+mode, func(t *testing.T) {
						lintMetadataPrepared(t, f, label, phase, mode, 2000, levels)
					})
				}
			}
		})
	}
}

// This is only the page metadata port's scope. It never constructs a fake
// presence map or claims to exercise the later production wanted consumer;
// that independent S49/BC obligation remains with bootstrap's real Execute.
func TestLintMetadataPresenceDoesNotReadAncestors(t *testing.T) {
	if testing.Short() {
		t.Skip("page metadata ancestor-work evidence needs its own PostgreSQL database")
	}
	f := newFixture(t)
	for depth := 1; depth <= 11; depth++ {
		f.exec(t, `INSERT INTO nodes(id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
            SELECT ('7a110000-0000-7000-0004-'||lpad(to_hex(branch*16+$1::int),12,'0'))::uuid,$2,
            CASE WHEN $1::int=1 THEN NULL ELSE ('7a110000-0000-7000-0004-'||lpad(to_hex(branch*16+$1::int-1),12,'0'))::uuid END,
            CASE WHEN $1::int=11 THEN 'asset' ELSE 'page' END,
            CASE WHEN $1::int=1 THEN 'Branch '||branch WHEN $1::int=10 THEN 'Shared' WHEN $1::int=11 THEN 'Shared.png' ELSE 'Level '||$1::int END,
            CASE WHEN $1::int=1 THEN 'branch '||branch WHEN $1::int=10 THEN 'shared' WHEN $1::int=11 THEN 'shared.png' ELSE 'level '||$1::int END,
            0,$3,$3,$4,$4 FROM generate_series(1,250) branch`, depth, f.eng, f.alice, now())
	}
	f.exec(t, `INSERT INTO page_contents(node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at)
        SELECT id,1,'',sha256(''::bytea),0,$2,$3 FROM nodes WHERE notebook_id=$1 AND kind='page'`, f.eng, f.alice, now())
	lintSeedPages(t, f, f.ops, nil, 5, 50000)
	f.exec(t, "ANALYZE nodes")
	f.exec(t, "ANALYZE page_contents")
	pool, trace := lintTracedPool(t, f)
	err := postgres.NewTxManager(pool, 5*time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
		db := postgres.DB(ctx, pool)
		before := lintMetadataWork(t, db)
		nodes, err := page.NewLintNodes(pool).Nodes(ctx, f.eng)
		if err != nil {
			return err
		}
		work := sourceWorkDelta(before, lintMetadataWork(t, db))
		assertLintMetadataWork(t, work, 2750, 2500)
		pageLeaves, assets := 0, 0
		for _, node := range nodes {
			if node.Kind == page.KindPage && node.NameKey == "shared" {
				pageLeaves++
			}
			if node.Kind == page.KindAsset && node.NameKey == "shared.png" && node.Linkable {
				assets++
			}
		}
		if len(nodes) != 2750 || pageLeaves != 250 || assets != 250 {
			t.Errorf("nonempty deep metadata=%d nodes/%d page leaves/%d linkable assets", len(nodes), pageLeaves, assets)
		}
		sourceEvidence(t, "lint-metadata-deep-port", work)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if names := trace.read(); !reflect.DeepEqual(names, []string{"LintNodeMetadata"}) {
		t.Errorf("compact port executed ancestor/display or extra queries: %v", names)
	}
}
