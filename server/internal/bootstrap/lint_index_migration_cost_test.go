//go:build !race

package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
	"github.com/open-nerve/NerveWiki/server/migrations"
)

func lintCostExec(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func lintCostLog(t testing.TB, label string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("lint index cost %s %s", label, b)
}

func lintCostTables() []string {
	return append(sourceHTTPTables(), "indexed_pages", "page_links", "page_properties", "page_tags", "page_aliases", "page_search", "page_search_queue", "page_search_tags")
}

func lintCostSeedApp(t *testing.T) (string, sourceHTTPApp, string) {
	t.Helper()
	database := pgtest.NewDatabase(t)
	setup := connect(t, database)
	for _, table := range lintCostTables() {
		lintCostExec(t, setup, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	setup.Close()
	a := sourceHTTPStartApp(t, database, "auto", "")
	a.team.tokens["alice"] = registerAccount(t, a.team.contract, a.team.base, "alice@example.com").AccessToken
	if status, body := ask(t, a.team.contract, http.MethodPost, a.team.base+"/api/v0/workspaces", a.team.tokens["alice"], `{"name":"Acme","slug":"acme"}`); status != http.StatusCreated {
		t.Fatalf("seed workspace = %d %s", status, body)
	}
	return database, a, a.team.openNotebook(t, "alice", "Eng")
}

func lintCostStatistics(t *testing.T, pool *pgxpool.Pool, stage string) []sourceHTTPTableState {
	t.Helper()
	var observed []sourceHTTPTableState
	for _, table := range lintCostTables() {
		rows := count(t, pool, "SELECT count(*) FROM "+table)
		stats := count(t, pool, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename=$1", table)
		if stage == "no_statistics" && stats != 0 || stage != "no_statistics" && rows > 0 && stats == 0 {
			t.Fatalf("%s %s rows=%d statistics=%d", stage, table, rows, stats)
		}
		if n := count(t, pool, "SELECT count(*) FROM pg_class WHERE oid=$1::regclass AND 'autovacuum_enabled=false'=ANY(reloptions)", table); n != 1 {
			t.Fatalf("automatic analysis enabled on %s", table)
		}
		observed = append(observed, sourceHTTPTableState{Name: table, Rows: rows, Statistics: stats, AutovacuumDisabled: true})
	}
	return observed
}

type lintMigrationFootprint struct {
	WAL                                                string
	HeapBytes, AllIndexBytes, TotalBytes, SourcesBytes int64
}

func lintMigrationFootprintOf(t *testing.T, pool *pgxpool.Pool) lintMigrationFootprint {
	t.Helper()
	var f lintMigrationFootprint
	err := pool.QueryRow(t.Context(), `SELECT pg_current_wal_insert_lsn()::text,
 pg_relation_size('page_properties'),pg_indexes_size('page_properties'),pg_total_relation_size('page_properties'),
 COALESCE(pg_relation_size(to_regclass('page_properties_sources_idx')),0)`).
		Scan(&f.WAL, &f.HeapBytes, &f.AllIndexBytes, &f.TotalBytes, &f.SourcesBytes)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// Only the set of available files is bounded, never their SQL: each MapFS
// entry contains the actual embedded production file's unchanged bytes.
// This test-only ceiling keeps Up51 separate after production adds Up52.
func lintMigrationFS(t *testing.T, ceiling int64) fs.FS {
	t.Helper()
	original := migrations.FS()
	entries, err := fs.ReadDir(original, ".")
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{}
	found := false
	for _, entry := range entries {
		name := entry.Name()
		prefix, _, ok := strings.Cut(name, "_")
		if entry.IsDir() || !ok || !strings.HasSuffix(name, ".sql") {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if version > ceiling {
			continue
		}
		data, err := fs.ReadFile(original, name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = &fstest.MapFile{Data: data, Mode: 0444}
		if version == ceiling {
			found = true
			t.Logf("actual embedded migration ceiling=%d file=%s sha256=%x", ceiling, name, sha256.Sum256(data))
		}
	}
	if !found {
		t.Fatalf("no production migration %d", ceiling)
	}
	return files
}

func lintCostMigrator(t *testing.T, pool *pgxpool.Pool, ceiling int64) *postgres.Migrator {
	t.Helper()
	m, err := postgres.NewMigrator(pool, lintMigrationFS(t, ceiling))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func lintSourcesMigrator(t *testing.T, pool *pgxpool.Pool) *postgres.Migrator {
	t.Helper()
	return lintCostMigrator(t, pool, 51)
}

// Goose Down deletes its version row. Check both the production status API
// and the actual version table so Up51 cannot silently reapply or account
// for 52. The ordinary full template starts with both versions applied.
func lintMigrationState(t *testing.T, pool *pgxpool.Pool, sources, ranges bool) {
	t.Helper()
	m := lintCostMigrator(t, pool, 52)
	rows, err := m.Status(t.Context())
	if closeErr := m.Close(); err != nil || closeErr != nil {
		t.Fatalf("migration status = %v; close = %v", err, closeErr)
	}
	for _, wanted := range []struct {
		version int64
		applied bool
	}{{51, sources}, {52, ranges}} {
		i := slices.IndexFunc(rows, func(row postgres.MigrationStatus) bool { return row.Version == wanted.version })
		if i < 0 || rows[i].Applied != wanted.applied {
			t.Fatalf("migration %d status = %+v, want applied=%t", wanted.version, rows, wanted.applied)
		}
		want := 0
		if wanted.applied {
			want = 1
		}
		if n := count(t, pool, "SELECT count(*) FROM goose_db_version WHERE version_id=$1", wanted.version); n != want {
			t.Fatalf("migration %d has %d version rows, want %d", wanted.version, n, want)
		}
	}
	if n := count(t, pool, "SELECT count(*) FROM goose_db_version WHERE version_id>52 OR NOT is_applied"); n != 0 {
		t.Fatalf("unexpected later/unapplied migration history rows=%d", n)
	}
	lintCostLog(t, "migration-state", struct {
		Sources, Ranges bool
		History         []string
	}{sources, ranges, queryStrings(t, pool, "SELECT row_to_json(v)::text FROM goose_db_version v WHERE version_id>=51 ORDER BY id")})
}

func lintSourcesDown(t *testing.T, m *postgres.Migrator, pool *pgxpool.Pool) {
	t.Helper()
	r, err := m.Down(t.Context())
	if err != nil || r == nil || r.Version != 51 || r.Source != "00051_linking_lint_sources.sql" {
		t.Fatalf("Down = %+v, %v; want only sources migration 51", r, err)
	}
	if n := count(t, pool, "SELECT count(*) FROM pg_index WHERE indexrelid=to_regclass('page_properties_sources_idx')"); n != 0 {
		t.Fatal("Down retained sources index")
	}
}

func lintSourcesApplied(t *testing.T, rows []postgres.Migration, err error) {
	t.Helper()
	if err != nil || !slices.Equal(rows, []postgres.Migration{{Version: 51, Source: "00051_linking_lint_sources.sql"}}) {
		t.Fatalf("full production Up = %+v, %v; want only migration 51", rows, err)
	}
}

func lintSourcesDefinition(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var keys []string
	var attrs int
	var predicate string
	var valid, unique bool
	err := pool.QueryRow(t.Context(), `SELECT indnatts,indisvalid,indisunique,pg_get_expr(indpred,indrelid),
	 ARRAY(SELECT a.attname::text FROM unnest(indkey) WITH ORDINALITY k(attnum,ord)
	 JOIN pg_attribute a ON a.attrelid=indrelid AND a.attnum=k.attnum ORDER BY k.ord)
 FROM pg_index WHERE indexrelid=to_regclass('page_properties_sources_idx')`).Scan(&attrs, &valid, &unique, &predicate, &keys)
	if err != nil {
		t.Fatalf("required sources index: %v", err)
	}
	if attrs != 2 || !valid || unique || !slices.Equal(keys, []string{"source_id", "position"}) ||
		predicate != "(translate(key, 'SOURCE'::text, 'source'::text) = 'sources'::text)" {
		t.Fatalf("sources index attrs=%d valid=%t unique=%t keys=%v predicate=%s", attrs, valid, unique, keys, predicate)
	}
}

// Split recipes keep each full fixture process within the existing default
// Go timeout. Each invocation owns >=100k live pages and >=2M properties.
// The runner takes two process samples of each shape. Fresh insertion and
// invariant scans warm the fixture; these are not cold migration claims.
func TestLintSourcesMigrationCostsEmptySources(t *testing.T) { lintMigrationCost(t, false) }

func TestLintSourcesMigrationCostsDenseSources(t *testing.T) { lintMigrationCost(t, true) }

func lintMigrationCost(t *testing.T, dense bool) {
	t.Helper()
	if testing.Short() {
		t.Skip("full production Up uses 100k pages/2M properties, without race")
	}
	database, app, notebook := lintCostSeedApp(t)
	sourceIndexSettle(t, app.team.pool)
	app.stop()
	pool := connect(t, database)
	lintMigrationState(t, pool, true, true)
	ranges := lintCostMigrator(t, pool, 52)
	lintRangeDown(t, ranges, pool)
	if err := ranges.Close(); err != nil {
		t.Fatal(err)
	}
	m := lintSourcesMigrator(t, pool)
	lintSourcesDown(t, m, pool)
	lintMigrationState(t, pool, false, false)
	keys := make([]string, 20)
	var body strings.Builder
	body.WriteString("---\n")
	for i := range keys {
		keys[i] = fmt.Sprintf("unrelated%02d", i)
		if dense {
			key := []byte("sources")
			for j := range key {
				if i&(1<<j) != 0 {
					key[j] -= 'a' - 'A'
				}
			}
			keys[i] = string(key)
		}
		fmt.Fprintf(&body, "%s: x\n", keys[i])
	}
	body.WriteString("---\n")
	for batch := range 10 {
		pages := make([]sqlPage, 10000)
		for i := range pages {
			pages[i] = sqlPage{name: fmt.Sprintf("cost%06d", batch*10000+i), content: body.String()}
		}
		seedPagesSQL(t, pool, notebook, pages)
	}
	lintCostExec(t, pool, `INSERT INTO page_properties (source_id,position,notebook_id,key,value)
 SELECT n.id,k.position::int-1,n.notebook_id,k.key,'"x"'::jsonb
 FROM nodes n CROSS JOIN unnest($2::text[]) WITH ORDINALITY k(key,position)
 WHERE n.notebook_id=$1`, notebook, keys)
	if n := count(t, pool, "SELECT count(*) FROM nodes n JOIN page_contents c ON c.node_id=n.id WHERE n.notebook_id=$1 AND n.deleted_at IS NULL AND c.deleted_at IS NULL AND c.content=$2", notebook, body.String()); n != 100000 {
		t.Fatalf("migration live pages=%d want100000", n)
	}
	if n := count(t, pool, "SELECT count(*) FROM page_properties WHERE notebook_id=$1", notebook); n != 2000000 {
		t.Fatalf("migration properties=%d want2000000", n)
	}
	if n := count(t, pool, "SELECT count(*) FROM indexed_pages WHERE notebook_id=$1 AND extractor=2 AND revision=1", notebook); n != 100000 {
		t.Fatalf("migration literal-v2 current headers=%d want100000", n)
	}
	wantSources := 0
	if dense {
		wantSources = 2000000
	}
	if n := count(t, pool, "SELECT count(*) FROM page_properties WHERE translate(key,'SOURCE','source')='sources'"); n != wantSources {
		t.Fatalf("migration sources=%d want%d", n, wantSources)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	// One dedicated backend runs the real Up. Its pgx trace measures the
	// full CREATE INDEX SQL, and the caller measures Up through its commit.
	trace := &lintMigrationTrace{Index: "page_properties_sources_idx"}
	cfg := pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = trace
	measured, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer measured.Close()
	m = lintSourcesMigrator(t, measured)
	defer func() { _ = m.Close() }()
	before := lintMigrationFootprintOf(t, pool)
	progress := lintObserveMigration(t, pool, "page_properties")
	started := time.Now()
	ran, upErr := m.Up(t.Context())
	upElapsed := time.Since(started)
	observed := progress()
	after := lintMigrationFootprintOf(t, pool)
	lintSourcesApplied(t, ran, upErr)
	lintMigrationState(t, pool, true, false)
	lintSourcesDefinition(t, pool)
	if observed.Samples == 0 || observed.ShareSamples == 0 || observed.MaxBlocksTotal == 0 || observed.MaxBlocksDone == 0 {
		t.Fatalf("no real build/SHARE scan observation: %+v", observed)
	}
	var blockSize int64
	if err := pool.QueryRow(t.Context(), "SELECT current_setting('block_size')::bigint").Scan(&blockSize); err != nil {
		t.Fatal(err)
	}
	if observed.MaxBlocksTotal*blockSize != before.HeapBytes {
		t.Fatalf("observed scan covers %d blocks of %d bytes, heap=%d", observed.MaxBlocksTotal, blockSize, before.HeapBytes)
	}
	if before.HeapBytes != after.HeapBytes || before.SourcesBytes != 0 || after.SourcesBytes <= 0 {
		t.Fatal("sources migration rewrote heap or did not build an index")
	}
	if n := count(t, pool, "SELECT count(*) FROM page_properties WHERE translate(key,'SOURCE','source')='sources'"); n != wantSources {
		t.Fatal("migration changed source property rows")
	}
	trace.mu.Lock()
	build := trace.BuildNS
	trace.mu.Unlock()
	if build <= 0 || build > int64(upElapsed) {
		t.Fatalf("traced production build=%dns full Up=%s", build, upElapsed)
	}
	lintCostLog(t, "full-production-up", struct {
		Dense, WarmInput        bool
		Pages, Properties       int
		UpNS, BuildNS, WALBytes int64
		Before, After           lintMigrationFootprint
		Observed                lintMigrationObservation
	}{dense, true, 100000, 2000000, int64(upElapsed), build, sourceIndexWAL(t, pool, before.WAL, after.WAL), before, after, observed})
	// Separate deterministic interleaving: a test-only ddl_command_end
	// barrier holds the production Up after CREATE INDEX and before COMMIT.
	// The confirmed portion of its hold is reported separately, not the
	// whole advisory wait and never as natural build cost.
	lintSourcesDown(t, m, pool)
	lintMigrationLockInterleaving(t, m, pool, 51)
	lintMigrationState(t, pool, true, false)
	lintSourcesFirstKeyProbe(t, pool, notebook)
}

type lintMigrationTrace struct {
	mu      sync.Mutex
	Index   string
	BuildNS int64
}

type lintMigrationTraceKey struct{}

func (p *lintMigrationTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.Contains(q.SQL, "CREATE INDEX "+p.Index) {
		return context.WithValue(ctx, lintMigrationTraceKey{}, time.Now())
	}
	return ctx
}

func (p *lintMigrationTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if start, ok := ctx.Value(lintMigrationTraceKey{}).(time.Time); ok {
		p.mu.Lock()
		p.BuildNS += int64(time.Since(start))
		p.mu.Unlock()
	}
}

type lintMigrationObservation struct {
	Samples, ShareSamples         int
	MaxBlocksTotal, MaxBlocksDone int64
	ShareObservedNS               int64
	Phases                        []lintMigrationPhase
}

type lintMigrationPhase struct {
	Name                    string
	Samples                 int
	FirstNS, LastNS         int64
	BlocksTotal, BlocksDone int64
}

func lintObserveMigration(t *testing.T, pool *pgxpool.Pool, table string) func() lintMigrationObservation {
	t.Helper()
	stop := make(chan struct{})
	type result struct {
		seen lintMigrationObservation
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var seen lintMigrationObservation
		var first, last time.Time
		start := time.Now()
		for {
			var phase *string
			var total, blocks int64
			var shares int
			err := pool.QueryRow(t.Context(), `SELECT
 (SELECT phase FROM pg_stat_progress_create_index WHERE datid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND relid=$1::regclass LIMIT 1),
 COALESCE((SELECT max(blocks_total) FROM pg_stat_progress_create_index WHERE datid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND relid=$1::regclass),0),
 COALESCE((SELECT max(blocks_done) FROM pg_stat_progress_create_index WHERE datid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND relid=$1::regclass),0),
 (SELECT count(*) FROM pg_locks WHERE database=(SELECT oid FROM pg_database WHERE datname=current_database()) AND relation=$1::regclass AND mode='ShareLock' AND granted)`, table).Scan(&phase, &total, &blocks, &shares)
			if err != nil {
				done <- result{seen, err}
				return
			}
			seen.Samples++
			seen.MaxBlocksTotal = max(seen.MaxBlocksTotal, total)
			seen.MaxBlocksDone = max(seen.MaxBlocksDone, blocks)
			if phase != nil {
				i := slices.IndexFunc(seen.Phases, func(p lintMigrationPhase) bool { return p.Name == *phase })
				if i < 0 {
					i = len(seen.Phases)
					seen.Phases = append(seen.Phases, lintMigrationPhase{Name: *phase, FirstNS: int64(time.Since(start))})
				}
				p := &seen.Phases[i]
				p.Samples++
				p.LastNS = int64(time.Since(start))
				p.BlocksTotal = max(p.BlocksTotal, total)
				p.BlocksDone = max(p.BlocksDone, blocks)
			}
			if shares > 0 {
				seen.ShareSamples++
				last = time.Now()
				if first.IsZero() {
					first = last
				}
				seen.ShareObservedNS = int64(last.Sub(first))
			}
			select {
			case <-stop:
				done <- result{seen, nil}
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	return func() lintMigrationObservation {
		close(stop)
		r := <-done
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.seen
	}
}

func lintMigrationLockInterleaving(t *testing.T, m *postgres.Migrator, pool *pgxpool.Pool, version int64) {
	t.Helper()
	table, index := "page_properties", "page_properties_sources_idx"
	writeSQL := "UPDATE page_properties SET value=value WHERE source_id=(SELECT source_id FROM page_properties LIMIT 1) AND position=0"
	readSQL := "SELECT position FROM page_properties LIMIT 1"
	applied := lintSourcesApplied
	if version == 52 {
		table, index = "page_links", lintRangeIndexName
		writeSQL = "UPDATE page_links SET target=target WHERE (source_id,range_start)=(SELECT source_id,range_start FROM page_links LIMIT 1)"
		readSQL = "SELECT range_start FROM page_links LIMIT 1"
		applied = lintRangeApplied
	} else if version != 51 {
		t.Fatalf("no controlled interleaving for migration %d", version)
	}
	lock := version * 100
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(context.Background()) }()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_xact_lock($1::int,1)", lock); err != nil {
		t.Fatal(err)
	}
	lintCostExec(t, pool, fmt.Sprintf(`CREATE FUNCTION lint_cost_hold() RETURNS event_trigger LANGUAGE plpgsql AS $$
 BEGIN IF EXISTS(SELECT 1 FROM pg_event_trigger_ddl_commands() WHERE object_identity='public.%s')
 THEN PERFORM pg_advisory_xact_lock(%d,1); END IF; END $$;
 CREATE EVENT TRIGGER lint_cost_hold ON ddl_command_end WHEN TAG IN ('CREATE INDEX') EXECUTE FUNCTION lint_cost_hold()`, index, lock))
	type upResult struct {
		rows      []postgres.Migration
		err       error
		elapsedNS int64
	}
	up := make(chan upResult, 1)
	go func() {
		started := time.Now()
		rows, err := m.Up(ctx)
		up <- upResult{rows, err, int64(time.Since(started))}
	}()
	pgtest.WaitForAdvisoryLockWaits(t, pool, int32(lock), 1, 1, 15*time.Second)
	// Observation begins only after the database confirms this wait and
	// ends before releasing it. This is a lower bound on the injected hold.
	barrier := time.Now()
	if n := count(t, pool, `SELECT count(*) FROM pg_locks s JOIN pg_locks a USING(pid)
 WHERE s.database=(SELECT oid FROM pg_database WHERE datname=current_database())
 AND s.relation=$1::regclass AND s.mode='ShareLock' AND s.granted
 AND a.locktype='advisory' AND a.classid=$2 AND a.objid=1 AND NOT a.granted`, table, lock); n != 1 {
		t.Fatalf("production Up holds SHARE while waiting at commit barrier: %d owners", n)
	}
	type writeResult struct {
		rows, elapsedNS int64
		err             error
	}
	writer := make(chan writeResult, 1)
	go func() {
		started := time.Now()
		tag, err := pool.Exec(ctx, writeSQL)
		writer <- writeResult{tag.RowsAffected(), int64(time.Since(started)), err}
	}()
	pgtest.WaitForTableLockWaits(t, pool, table, 1, 10*time.Second)
	readStart := time.Now()
	var read int
	if err := pool.QueryRow(ctx, readSQL).Scan(&read); err != nil {
		t.Fatal(err)
	}
	readElapsed := time.Since(readStart)
	select {
	case result := <-writer:
		t.Fatalf("writer passed production Up SHARE before release: %+v", result)
	default:
	}
	holdElapsed := time.Since(barrier)
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result := <-up
	applied(t, result.rows, result.err)
	written := <-writer
	if written.err != nil || written.rows != 1 {
		t.Fatalf("writer after SHARE release = %+v, want one committed update", written)
	}
	lintCostExec(t, pool, "DROP EVENT TRIGGER lint_cost_hold; DROP FUNCTION lint_cost_hold()")
	lintCostLog(t, "controlled-lock-interleaving", struct {
		Version                                       int64
		UpNS, ObservedBarrierHoldNS, WriterNS, ReadNS int64
	}{version, result.elapsedNS, int64(holdElapsed), written.elapsedNS, int64(readElapsed)})
}

// A physical index diagnostic, not the separate production Planned-reader
// proof. Four pages contain 1000 unrelated keys each. Two matching keys on
// each nonempty page make first-position selection observable, including
// matching keys after the unrelated range. No planner setting is disabled.
func lintSourcesFirstKeyProbe(t *testing.T, pool *pgxpool.Pool, notebook string) {
	t.Helper()
	for _, position := range []int{0, 500, 1000, -1} {
		name := fmt.Sprintf("first-key-%d", position)
		var content strings.Builder
		content.WriteString("---\n")
		for i := range 1002 {
			key := fmt.Sprintf("unrelated%d", i)
			if i == position {
				key = "SoUrCeS"
			} else if i == 1001 && position >= 0 {
				key = "SOURCES"
			}
			fmt.Fprintf(&content, "%s: x\n", key)
		}
		content.WriteString("---\n")
		seedPagesSQL(t, pool, notebook, []sqlPage{{name: name, content: content.String()}})
		var source string
		if err := pool.QueryRow(t.Context(), "SELECT id::text FROM nodes WHERE notebook_id=$1 AND name=$2", notebook, name).Scan(&source); err != nil {
			t.Fatal(err)
		}
		lintCostExec(t, pool, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT $1,p,$2,CASE WHEN p=$3 THEN 'SoUrCeS' WHEN p=1001 AND $3>=0 THEN 'SOURCES' ELSE 'unrelated'||p END,'"x"'::jsonb
 FROM generate_series(0,1001) p`, source, notebook, position)
		var raw []byte
		query := "SELECT position FROM page_properties WHERE source_id=$1 AND translate(key,'SOURCE','source')='sources' ORDER BY position LIMIT 1"
		if err := pool.QueryRow(t.Context(), "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+query, source).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var plans []map[string]any
		if err := json.Unmarshal(raw, &plans); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"Index Name": "page_properties_sources_idx"`) || strings.Contains(string(raw), `"Seq Scan"`) {
			t.Fatalf("first-key diagnostic did not use required bounded source index: %s", raw)
		}
		rows, err := pool.Query(t.Context(), query, source)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[int])
		want := []int{}
		if position >= 0 {
			want = []int{position}
		}
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("first sources at %d = %v,%v want%v", position, got, err, want)
		}
		lintCostLog(t, "first-key-plan", struct {
			First int
			Plan  json.RawMessage
		}{position, raw})
	}
}
