//go:build !race

package bootstrap

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pagepg "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

// Separate commands retain the original go test default10m timeout. These
// are warm-input observations, not a deployment limit or a new timing SLO.
func TestIngestMigrationCostsLongHistory(t *testing.T)  { ingestMigrationCost(t, 1) }
func TestIngestMigrationCostsTenHistories(t *testing.T) { ingestMigrationCost(t, 10) }

func ingestMigrationCost(t *testing.T, books int) {
	database := pgtest.NewEmptyDatabase(t)
	pool := connect(t, database)
	if n := count(t, pool, `SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace AND relname IN('changesets','goose_db_version')`); n != 0 {
		t.Fatal("target database was not empty")
	}
	m52 := lintCostMigrator(t, pool, 52)
	applied, err := m52.Up(t.Context())
	if err != nil || len(applied) != 52 || applied[len(applied)-1].Version != 52 {
		t.Fatalf("real empty-to52 Up: %+v %v", applied, err)
	}
	if err := m52.Close(); err != nil {
		t.Fatal(err)
	}
	f := ingestMigrationSeed(t, pool, books)
	tables := f.oldTables(t)
	old := f.image(t, tables)
	lintCostLog(t, "ingest-old52-columns-and-pk", tables)
	lintCostLog(t, "ingest-old-image-before-first-up", old)
	ingestMigrationState(t, pool, false)

	trace := &ingestMigrationTrace{}
	cfg := pool.Config()
	cfg.MaxConns = 1
	cfg.ConnConfig.Tracer = trace
	measured, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer measured.Close()
	conn, err := measured.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	pid := conn.Conn().PgConn().PID()
	conn.Release()
	m, err := postgres.NewMigrator(measured, lintMigrationFS(t, 53))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()

	before := ingestMigrationFootprintOf(t, pool)
	trace.reset()
	stop, observed := ingestMigrationObserve(t.Context(), pool, pid)
	observedDone := false
	defer func() {
		if !observedDone {
			close(stop)
			<-observed
		}
	}()
	lsn0 := ingestMigrationLSN(t, pool)
	started := time.Now()
	applied, err = m.Up(t.Context())
	wall := time.Since(started)
	lsn1 := ingestMigrationLSN(t, pool)
	close(stop)
	samples := <-observed
	observedDone = true
	ingestMigrationApplied(t, applied, err)
	if samples.Err != "" {
		t.Fatal(samples.Err)
	}
	events, txn := trace.check(t, pid)
	if txn > wall.Nanoseconds() {
		t.Fatal("transaction trace exceeds actual Up wall")
	}
	after := ingestMigrationFootprintOf(t, pool)
	lintCostLog(t, "ingest-natural-up53", struct {
		Books                                    int
		PID                                      uint32
		WallNS, TransactionNS, WALGeneratedBytes int64
		Events                                   []ingestMigrationEvent
		Samples                                  ingestMigrationSamples
		Before, After                            []ingestMigrationRelation
	}{books, pid, wall.Nanoseconds(), txn, sourceIndexWAL(t, pool, lsn0, lsn1), events, samples, before, after})
	ingestMigrationState(t, pool, true)
	f.sameImage(t, tables, old, "first-up53")
	checkPages(t, pool)

	f.writeMarks(t, books == 1)
	down, err := m.Down(t.Context())
	if err != nil || down == nil || *down != (postgres.Migration{Version: 53, Source: "00053_page_ingest.sql"}) {
		t.Fatalf("Down only53: %+v %v", down, err)
	}
	ingestMigrationState(t, pool, false)
	f.sameImage(t, tables, old, "down52")
	ingestMigrationLocks(t, pool, m, trace, pid, f.books[0].id)
	ingestMigrationState(t, pool, true)
	f.sameImage(t, tables, old, "second-up53")
	checkPages(t, pool)
}

func ingestMigrationApplied(t *testing.T, rows []postgres.Migration, err error) {
	t.Helper()
	if err != nil || !slices.Equal(rows, []postgres.Migration{{Version: 53, Source: "00053_page_ingest.sql"}}) {
		t.Fatalf("INGEST_MIGRATION_UP53: %+v %v", rows, err)
	}
}

func ingestMigrationState(t *testing.T, pool *pgxpool.Pool, applied bool) {
	t.Helper()
	want := 0
	if applied {
		want = 1
	}
	if n := count(t, pool, `SELECT count(*) FROM goose_db_version WHERE version_id=53 AND is_applied`); n != want {
		t.Fatalf("version53=%d want=%d", n, want)
	}
	if n := count(t, pool, `SELECT count(*) FROM goose_db_version WHERE version_id>53 OR NOT is_applied`); n != 0 {
		t.Fatalf("unexpected migration rows=%d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM pg_attribute WHERE attrelid='changesets'::regclass AND attname='ingest_count' AND NOT attisdropped`); n != want {
		t.Fatalf("ingest_count column=%d want=%d", n, want)
	}
	if n := count(t, pool, `SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace AND relname='changeset_sources'`); n != want {
		t.Fatalf("sources relation=%d want=%d", n, want)
	}
	if n := count(t, pool, `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relname IN('changesets_notebook_id_created_at_ingest_idx','changesets_notebook_id_created_by_id_created_at_ingest_idx') AND i.indisvalid AND i.indisready`); n != 2*want {
		t.Fatalf("valid partial indexes=%d want=%d", n, 2*want)
	}
	if applied {
		if n := count(t, pool, `SELECT count(*) FROM pg_attribute a JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid='changesets'::regclass AND a.attname='ingest_count' AND a.attnotnull AND a.atttypid='int4'::regtype AND pg_get_expr(d.adbin,d.adrelid)='0'`); n != 1 {
			t.Fatal("INGEST_MIGRATION_ZERO_DEFAULT: expected NOT NULL integer DEFAULT0")
		}
		if n := count(t, pool, `SELECT count(*) FROM changesets WHERE ingest_count IS DISTINCT FROM 0`); n != 0 {
			t.Fatalf("INGEST_MIGRATION_ZERO_DEFAULT: nonzero old rows=%d", n)
		}
		if n := count(t, pool, `SELECT count(*) FROM changeset_sources`); n != 0 {
			t.Fatalf("initial sources=%d", n)
		}
		// These are logical entries, not a claim that CREATE INDEX scanned no rows.
		for _, predicate := range []string{"ingest_count>0", "deleted_at IS NULL AND ingest_count>0"} {
			if n := count(t, pool, "SELECT count(*) FROM changesets WHERE "+predicate); n != 0 {
				t.Fatalf("initial partial entries=%d", n)
			}
		}
	}
}

type ingestMigrationIndex struct {
	Name, Definition string
	Bytes            int64
	Valid            bool
}
type ingestMigrationRelation struct {
	Name                                                                        string
	Rows, HeapBytes, ToastHeapBytes, ToastIndexBytes, AllIndexBytes, TotalBytes int64
	Indexes                                                                     []ingestMigrationIndex
}

// TotalBytes includes forks/TOAST/indexes and is never added to its parts.
func ingestMigrationFootprintOf(t *testing.T, pool *pgxpool.Pool) []ingestMigrationRelation {
	t.Helper()
	var out []ingestMigrationRelation
	for _, name := range []string{"changesets", "changeset_sources"} {
		var exists bool
		if err := pool.QueryRow(t.Context(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			continue
		}
		r := ingestMigrationRelation{Name: name}
		if err := pool.QueryRow(t.Context(), `SELECT pg_relation_size(c.oid),
 CASE WHEN c.reltoastrelid=0 THEN 0 ELSE pg_relation_size(c.reltoastrelid) END,
 CASE WHEN c.reltoastrelid=0 THEN 0 ELSE pg_indexes_size(c.reltoastrelid) END,
 pg_indexes_size(c.oid),pg_total_relation_size(c.oid) FROM pg_class c WHERE c.oid=$1::regclass`, name).
			Scan(&r.HeapBytes, &r.ToastHeapBytes, &r.ToastIndexBytes, &r.AllIndexBytes, &r.TotalBytes); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM "+pgx.Identifier{name}.Sanitize()).Scan(&r.Rows); err != nil {
			t.Fatal(err)
		}
		rows, err := pool.Query(t.Context(), `SELECT c.relname,pg_get_indexdef(c.oid),pg_relation_size(c.oid),i.indisvalid
 FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid=$1::regclass ORDER BY c.relname`, name)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var index ingestMigrationIndex
			if err := rows.Scan(&index.Name, &index.Definition, &index.Bytes, &index.Valid); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			r.Indexes = append(r.Indexes, index)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func ingestMigrationLSN(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var lsn string
	if err := pool.QueryRow(t.Context(), `SELECT pg_current_wal_insert_lsn()::text`).Scan(&lsn); err != nil {
		t.Fatal(err)
	}
	return lsn
}

type ingestMigrationEvent struct {
	Label          string
	PID            uint32
	StartNS, EndNS int64
	Err            string
}
type ingestMigrationTrace struct {
	mu     sync.Mutex
	origin time.Time
	events []ingestMigrationEvent
}
type ingestMigrationTraceKey struct{}

func (r *ingestMigrationTrace) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.origin = time.Now()
	r.events = nil
}
func (r *ingestMigrationTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sql := strings.ToLower(strings.Join(strings.Fields(data.SQL), " "))
	label := ""
	switch {
	case sql == "begin":
		label = "begin"
	case sql == "commit":
		label = "commit"
	case strings.Contains(sql, "alter table changesets add column ingest_count"):
		label = "alter"
	case strings.Contains(sql, "create table changeset_sources"):
		label = "sources"
	case strings.Contains(sql, "create index changesets_notebook_id_created_at_ingest_idx"):
		label = "index_notebook"
	case strings.Contains(sql, "create index changesets_notebook_id_created_by_id_created_at_ingest_idx"):
		label = "index_actor"
	case strings.Contains(sql, "insert into") && strings.Contains(sql, "goose_db_version"):
		label = "version"
	}
	if label == "" {
		return ctx
	}
	r.mu.Lock()
	e := &ingestMigrationEvent{Label: label, PID: conn.PgConn().PID(), StartNS: time.Since(r.origin).Nanoseconds()}
	r.mu.Unlock()
	return context.WithValue(ctx, ingestMigrationTraceKey{}, e)
}
func (r *ingestMigrationTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	e, _ := ctx.Value(ingestMigrationTraceKey{}).(*ingestMigrationEvent)
	if e == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e.EndNS = time.Since(r.origin).Nanoseconds()
	if data.Err != nil {
		e.Err = data.Err.Error()
	}
	r.events = append(r.events, *e)
}
func (r *ingestMigrationTrace) check(t *testing.T, pid uint32) ([]ingestMigrationEvent, int64) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	want := []string{"begin", "alter", "sources", "index_notebook", "index_actor", "version", "commit"}
	if len(r.events) != len(want) {
		t.Fatalf("migration transaction trace=%+v", r.events)
	}
	for i, e := range r.events {
		if e.Label != want[i] || e.PID != pid || e.Err != "" || e.EndNS < e.StartNS || (i > 0 && e.StartNS < r.events[i-1].EndNS) {
			t.Fatalf("migration transaction trace=%+v", r.events)
		}
	}
	return slices.Clone(r.events), r.events[len(r.events)-1].EndNS - r.events[0].StartNS
}

type ingestMigrationSamples struct {
	Samples, Waiting, HoldingAccessExclusive, IndexProgress                         int
	FirstWaitNS, LastWaitNS, FirstHeldNS, LastHeldNS, MaxBlocksDone, MaxBlocksTotal int64
	Phases                                                                          []string
	Err                                                                             string
}

// Sampling observes intervals only; a fast natural stage may have zero samples.
// The deterministic lock test below proves the lock independently.
func ingestMigrationObserve(ctx context.Context, pool *pgxpool.Pool, pid uint32) (chan struct{}, <-chan ingestMigrationSamples) {
	stop := make(chan struct{})
	done := make(chan ingestMigrationSamples, 1)
	go func() {
		var s ingestMigrationSamples
		origin := time.Now()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		defer func() { done <- s }()
		for {
			var waiting, held bool
			var phases []string
			var blocks, total int64
			err := pool.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='changesets'::regclass AND mode='AccessExclusiveLock' AND NOT granted),
 EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='changesets'::regclass AND mode='AccessExclusiveLock' AND granted),
 ARRAY(SELECT phase FROM pg_stat_progress_create_index WHERE pid=$1),
 COALESCE((SELECT max(blocks_done) FROM pg_stat_progress_create_index WHERE pid=$1),0),
 COALESCE((SELECT max(blocks_total) FROM pg_stat_progress_create_index WHERE pid=$1),0)`, pid).Scan(&waiting, &held, &phases, &blocks, &total)
			if err != nil {
				s.Err = err.Error()
				return
			}
			s.Samples++
			now := time.Since(origin).Nanoseconds()
			if waiting {
				if s.Waiting == 0 {
					s.FirstWaitNS = now
				}
				s.Waiting++
				s.LastWaitNS = now
			}
			if held {
				if s.HoldingAccessExclusive == 0 {
					s.FirstHeldNS = now
				}
				s.HoldingAccessExclusive++
				s.LastHeldNS = now
			}
			if len(phases) > 0 {
				s.IndexProgress++
			}
			for _, phase := range phases {
				if !slices.Contains(s.Phases, phase) {
					s.Phases = append(s.Phases, phase)
				}
			}
			s.MaxBlocksDone = max(s.MaxBlocksDone, blocks)
			s.MaxBlocksTotal = max(s.MaxBlocksTotal, total)
			select {
			case <-stop:
				return
			case <-ctx.Done():
				s.Err = ctx.Err().Error()
				return
			case <-ticker.C:
			}
		}
	}()
	return stop, done
}

func (f ingestMigrationFixture) writeMarks(t *testing.T, million bool) {
	t.Helper()
	pool, b := f.pool, f.books[0]
	ctx := t.Context()
	// Exact current source eligibility for these direct live page children:
	// nonempty correct metadata and hash, current head, and the live LLM root.
	rows, err := pool.Query(ctx, `SELECT n.id FROM nodes n JOIN page_contents c ON c.node_id=n.id
 JOIN indexed_pages i ON i.node_id=n.id JOIN notebooks b ON b.id=n.notebook_id
 JOIN nodes root ON root.id=b.source_root_id AND root.notebook_id=b.id
 WHERE n.notebook_id=$1 AND n.parent_id=$2 AND n.deleted_at IS NULL AND n.kind='page'
 AND c.deleted_at IS NULL AND c.content='source' AND c.byte_size=6 AND c.content_hash=sha256('source')
 AND n.source_hash=c.content_hash AND i.notebook_id=b.id AND i.revision=c.revision AND i.extractor=2
 AND b.mode='llm_wiki' AND b.deleted_at IS NULL AND root.id=$2 AND root.deleted_at IS NULL AND root.kind='page'
 ORDER BY n.id`, b.id, b.root)
	if err != nil {
		t.Fatal(err)
	}
	var sources []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if len(sources) > 0 && sources[len(sources)-1].Compare(id) >= 0 {
			rows.Close()
			t.Fatal("sources not strictly ordered")
		}
		sources = append(sources, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 10000 {
		t.Fatalf("INGEST_MIGRATION_REAL_SOURCES: %d", len(sources))
	}
	type cs struct {
		id uuid.UUID
		at time.Time
	}
	rows, err = pool.Query(ctx, `SELECT id,updated_at FROM changesets WHERE notebook_id=$1 AND kind='edit' AND deleted_at IS NULL
 AND NOT trimmed AND created_at<=$2 ORDER BY created_at DESC,id DESC LIMIT 200`, b.id, ingestMigrationAt().Add(100000*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var changes []cs
	for rows.Next() {
		var c cs
		if err := rows.Scan(&c.id, &c.at); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		changes = append(changes, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 200 {
		t.Fatalf("actual live edit changesets=%d", len(changes))
	}
	store := pagepg.New(pool)
	if million {
		before := ingestMigrationFootprintOf(t, pool)
		lsn0 := ingestMigrationLSN(t, pool)
		start := time.Now()
		for _, c := range changes[100:] {
			// The real ordinary path updates an old field to its exact old value.
			// PostgreSQL still writes a tuple/WAL; no old logical column changes.
			if err := store.TouchChangeset(ctx, c.id, c.at); err != nil {
				t.Fatal(err)
			}
		}
		wall := time.Since(start)
		lsn1 := ingestMigrationLSN(t, pool)
		ingestMigrationWriteReport(t, pool, "ordinary-touch-same-timestamp", 100, 0, wall, lsn0, lsn1, before)
		if n := count(t, pool, `SELECT count(*) FROM changeset_sources`); n != 0 {
			t.Fatalf("ordinary write made %d marks", n)
		}
	}
	calls := 100
	if !million {
		calls = 1
		sources = sources[:3]
	}
	before := ingestMigrationFootprintOf(t, pool)
	lsn0 := ingestMigrationLSN(t, pool)
	start := time.Now()
	for _, c := range changes[:calls] {
		got, err := store.Add(ctx, b.id, c.id, sources)
		if err != nil || got.Added != len(sources) || got.Count != len(sources) {
			t.Fatalf("INGEST_MIGRATION_ADD: %+v %v want=%d", got, err, len(sources))
		}
	}
	wall := time.Since(start)
	lsn1 := ingestMigrationLSN(t, pool)
	ingestMigrationWriteReport(t, pool, "actual-Store.Add", calls, calls*len(sources), wall, lsn0, lsn1, before)
	// Verify every persistent sequence and source, not a sample or count alone.
	for _, c := range changes[:calls] {
		rows, err := pool.Query(ctx, `SELECT seq,node_id FROM changeset_sources WHERE changeset_id=$1 ORDER BY seq`, c.id)
		if err != nil {
			t.Fatal(err)
		}
		i := 0
		for rows.Next() {
			var seq int
			var id uuid.UUID
			if err := rows.Scan(&seq, &id); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if i >= len(sources) || seq != i+1 || id != sources[i] {
				rows.Close()
				t.Fatalf("persistent mark %d: seq=%d source=%v", i, seq, id)
			}
			i++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if i != len(sources) || count(t, pool, `SELECT ingest_count FROM changesets WHERE id=$1`, c.id) != len(sources) {
			t.Fatalf("persistent parent/marks=%d want=%d", i, len(sources))
		}
	}
	if n := count(t, pool, `SELECT count(*) FROM changeset_sources`); n != calls*len(sources) {
		t.Fatalf("total marks=%d", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM changesets WHERE ingest_count>0`); n != calls {
		t.Fatalf("marked parents=%d", n)
	}
	checkPages(t, pool)
}

func ingestMigrationWriteReport(t *testing.T, pool *pgxpool.Pool, operation string, calls, marks int, wall time.Duration, lsn0, lsn1 string, before []ingestMigrationRelation) {
	t.Helper()
	wal := sourceIndexWAL(t, pool, lsn0, lsn1)
	if wal <= 0 {
		t.Fatal("real write generated no WAL")
	}
	lintCostLog(t, "ingest-write-growth", struct {
		Operation                 string
		Calls, Marks              int
		WallNS, WALGeneratedBytes int64
		Before, After             []ingestMigrationRelation
	}{operation, calls, marks, wall.Nanoseconds(), wal, before, ingestMigrationFootprintOf(t, pool)})
}

type ingestMigrationUpResult struct {
	Rows   []postgres.Migration
	Err    error
	WallNS int64
}
type ingestMigrationIOResult struct {
	ID           uuid.UUID
	Rows, WallNS int64
	Err          error
}

func ingestMigrationLocks(t *testing.T, pool *pgxpool.Pool, m *postgres.Migrator, trace *ingestMigrationTrace, pid uint32, book uuid.UUID) {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		t.Fatal("capacity recipe must retain the original go test timeout")
	}
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	var table, barrier pgx.Tx
	var reader, writer *pgxpool.Conn
	var up chan ingestMigrationUpResult
	var read, write chan ingestMigrationIOResult
	defer func() {
		cancel()
		if table != nil {
			_ = table.Rollback(context.Background())
		}
		if barrier != nil {
			_ = barrier.Rollback(context.Background())
		}
		if up != nil {
			<-up
		}
		if read != nil {
			<-read
		}
		if write != nil {
			<-write
		}
		if reader != nil {
			reader.Release()
		}
		if writer != nil {
			writer.Release()
		}
	}()
	var id uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM changesets WHERE notebook_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, book).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var err error
	table, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := table.Exec(ctx, `LOCK TABLE changesets IN ROW EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	tablePID := table.Conn().PgConn().PID()
	barrier, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(11353,1)`); err != nil {
		t.Fatal(err)
	}
	barrierPID := barrier.Conn().PgConn().PID()
	lintCostExec(t, pool, `CREATE FUNCTION ingest_migration_hold() RETURNS event_trigger LANGUAGE plpgsql AS $$
 BEGIN IF EXISTS(SELECT 1 FROM pg_event_trigger_ddl_commands()
 WHERE object_identity='public.changesets_notebook_id_created_by_id_created_at_ingest_idx')
 THEN PERFORM pg_advisory_xact_lock(11353,1); END IF; END $$;
 CREATE EVENT TRIGGER ingest_migration_hold ON ddl_command_end WHEN TAG IN ('CREATE INDEX') EXECUTE FUNCTION ingest_migration_hold()`)
	trace.reset()
	up = make(chan ingestMigrationUpResult, 1)
	go func() {
		start := time.Now()
		rows, err := m.Up(ctx)
		up <- ingestMigrationUpResult{rows, err, time.Since(start).Nanoseconds()}
	}()
	pgtest.WaitForTableLockWaits(t, pool, "changesets", 1, time.Until(deadline))
	ingestMigrationLock(t, pool, pid, "AccessExclusiveLock", false, tablePID)
	waitingAt := time.Now()
	if err := table.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	pgtest.WaitForAdvisoryLockWaits(t, pool, 11353, 1, 1, time.Until(deadline))
	if n := count(t, pool, `SELECT count(*) FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND classid=11353 AND objid=1 AND objsubid=2 AND NOT granted AND $2::integer=ANY(pg_blocking_pids(pid))`, pid, barrierPID); n != 1 {
		t.Fatalf("Up PID %d is not at final DDL barrier", pid)
	}
	ingestMigrationLock(t, pool, pid, "AccessExclusiveLock", true, 0)
	heldAt := time.Now()
	reader, err = pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	writer, err = pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	readPID, writePID := reader.Conn().PgConn().PID(), writer.Conn().PgConn().PID()
	if readPID == writePID || readPID == pid || writePID == pid || readPID == barrierPID || writePID == barrierPID {
		t.Fatal("reader/writer/Up/barrier must have independent PIDs")
	}
	read = make(chan ingestMigrationIOResult, 1)
	go func() {
		start := time.Now()
		var got uuid.UUID
		err := reader.QueryRow(ctx, `SELECT id FROM changesets WHERE id=$1`, id).Scan(&got)
		read <- ingestMigrationIOResult{ID: got, WallNS: time.Since(start).Nanoseconds(), Err: err}
	}()
	write = make(chan ingestMigrationIOResult, 1)
	go func() {
		start := time.Now()
		tag, err := writer.Exec(ctx, `UPDATE changesets SET updated_at=updated_at WHERE id=$1`, id)
		write <- ingestMigrationIOResult{Rows: tag.RowsAffected(), WallNS: time.Since(start).Nanoseconds(), Err: err}
	}()
	pgtest.WaitForTableLockWaits(t, pool, "changesets", 2, time.Until(deadline))
	ingestMigrationLock(t, pool, readPID, "AccessShareLock", false, pid)
	ingestMigrationLock(t, pool, writePID, "RowExclusiveLock", false, pid)
	select {
	case r := <-read:
		read = nil
		t.Fatalf("reader passed before barrier release: %+v", r)
	default:
	}
	select {
	case r := <-write:
		write = nil
		t.Fatalf("writer passed before barrier release: %+v", r)
	default:
	}
	confirmedHold := time.Since(heldAt)
	if err := barrier.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	upResult := <-up
	up = nil
	readResult := <-read
	read = nil
	writeResult := <-write
	write = nil
	ingestMigrationApplied(t, upResult.Rows, upResult.Err)
	if readResult.Err != nil || readResult.ID != id || writeResult.Err != nil || writeResult.Rows != 1 {
		t.Fatalf("postcommit reader=%+v writer=%+v", readResult, writeResult)
	}
	events, txn := trace.check(t, pid)
	lintCostLog(t, "ingest-injected-lock-proof", struct {
		UpPID, TableHolderPID, BarrierHolderPID, ReaderPID, WriterPID  uint32
		ReleaseTableToBarrierNS, ConfirmedBarrierHoldNS, TransactionNS int64
		Up                                                             ingestMigrationUpResult
		Reader, Writer                                                 ingestMigrationIOResult
		Events                                                         []ingestMigrationEvent
	}{pid, tablePID, barrierPID, readPID, writePID, heldAt.Sub(waitingAt).Nanoseconds(), confirmedHold.Nanoseconds(), txn, upResult, readResult, writeResult, events})
	lintCostExec(t, pool, `DROP EVENT TRIGGER ingest_migration_hold; DROP FUNCTION ingest_migration_hold()`)
}

func ingestMigrationLock(t *testing.T, pool *pgxpool.Pool, pid uint32, mode string, granted bool, blocker uint32) {
	t.Helper()
	if n := count(t, pool, `SELECT count(*) FROM pg_locks WHERE pid=$1 AND locktype='relation'
 AND relation='changesets'::regclass AND mode=$2 AND granted=$3
 AND ($4::integer=0 OR $4::integer=ANY(pg_blocking_pids(pid)))`, pid, mode, granted, blocker); n != 1 {
		t.Fatalf("INGEST_MIGRATION_ACCESS_EXCLUSIVE: pid=%d mode=%s granted=%t blocker=%d matches=%d", pid, mode, granted, blocker, n)
	}
}
