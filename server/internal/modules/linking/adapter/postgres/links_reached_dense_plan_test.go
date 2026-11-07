//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const linksDenseSources = 400
const linksDenseAssets = 10000
const linksDenseRows = linksDenseSources * linksDenseAssets

// Query qualification only: real schema and parser-checked logical rows,
// with a pristine SQL seed. This does not recreate bootstrap's MVCC history,
// request, 99 ordinary writes, resolver, or unknown historical statistics.
type linksDenseFixture struct {
	*stablePlanFixture
	sources, incoming, assets []uuid.UUID
	names                     []string
}

func TestLinksReachedDenseReadPlans(t *testing.T) {
	if testing.Short() {
		t.Skip("native PostgreSQL complete four-million-row LinksReached qualification")
	}
	for _, state := range []string{"no_statistics", "analyzed"} {
		t.Run(state, func(t *testing.T) {
			f := newLinksDenseFixture(t, state)
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					var actual []map[string]stablePlanWork
					var replay []stablePlanEvidence
					rollback := errors.New("dense LinksReached observation rolls back")
					err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
						tx, ok := postgres.TxFrom(ctx)
						if !ok || postgres.DB(ctx, f.pool) != tx {
							t.Fatal("dense LinksReached lost its original transaction")
						}
						settings := stablePlanSettings(t, ctx, tx, mode)
						if err := f.s.Lock(ctx, f.eng); err != nil {
							return err
						}
						var xid string
						if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text").Scan(&xid); err != nil {
							return err
						}
						for _, full := range []bool{false, true} {
							reach := domain.Reach{Keys: []string{"aa.a", "zz.x"}, Targets: []uuid.UUID{f.assets[0]}, Sources: []uuid.UUID{f.assets[0]}}
							if full {
								reach = domain.Reach{Sources: slices.Clone(f.sources)} // Keys and Targets are typed nil.
							}
							// Keep fixture preparation and preceding result garbage out
							// of the measured call; no second 4M-row oracle is retained.
							runtime.GC()
							call, work := linksDenseActual(t, ctx, f, reach, full, mode, xid)
							actual = append(actual, work)
							runtime.GC() // Release the collected result before independent server replay.
							replay = append(replay, linksDenseReplay(t, ctx, f, call, full, mode, settings))
						}
						return rollback
					})
					terminal := f.trace.terminal()
					lintEvidence(t, "links-dense-rollback", map[string]any{"state": state, "mode": mode, "terminal": terminal, "original_cause": errors.Is(err, rollback)})
					if !errors.Is(err, rollback) || !slices.Equal(terminal, []string{"rollback"}) {
						t.Fatalf("dense rollback: %v terminal=%v", err, terminal)
					}
					linksDenseState(t, f, "after-rollback")
					if !reflect.DeepEqual(f.statistics, linksDenseStatistics(t, f)) {
						t.Fatal("dense statistics changed during the read-only leaf")
					}
					// Record both complete outputs, plans and the restored source
					// state before applying either physical-work guard.
					for i := range actual {
						linksDenseWork(t, actual[i], i == 1)
						linksDenseWork(t, replay[i].ExecutionWork, i == 1)
						linksDenseTree(t, replay[i].Plan, i == 1)
						if state == "analyzed" && mode == "auto" && i == 1 {
							linksDenseOrderedSourceWork(t, replay[i].Plan)
						}
					}
				})
			}
		})
	}
}

func newLinksDenseFixture(t *testing.T, state string) *linksDenseFixture {
	t.Helper()
	f := &linksDenseFixture{stablePlanFixture: &stablePlanFixture{fixture: newFixture(t), state: state}}
	f.eng, f.ops = stablePlanID(900001), stablePlanID(900002)
	ingestPlanSetup(t, f.fixture)
	for _, table := range []string{"page_tags", "page_aliases"} {
		lintExec(t, f.fixture, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	for i := range linksDenseSources {
		f.sources = append(f.sources, stablePlanID(uint64(100000+i)))
		f.incoming = append(f.incoming, stablePlanID(uint64(200000+i)))
	}
	for a := byte('a'); a <= 'z' && len(f.names) < linksDenseAssets; a++ {
		for b := byte('a'); b <= 'z' && len(f.names) < linksDenseAssets; b++ {
			for c := byte('a'); c <= 'z' && len(f.names) < linksDenseAssets; c++ {
				f.names = append(f.names, string([]byte{a, b, '.', c}))
			}
		}
	}
	for i := range linksDenseAssets {
		f.assets = append(f.assets, stablePlanID(uint64(300000+i)))
	}
	for _, ids := range [][]uuid.UUID{f.sources, f.incoming, f.assets} {
		if !slices.IsSortedFunc(ids, uuid.UUID.Compare) || len(slices.Compact(slices.Clone(ids))) != len(ids) {
			t.Fatal("dense deterministic IDs must be strictly sorted")
		}
	}
	linksDenseParser(t, f)
	heads := append(slices.Clone(f.sources), f.incoming...)
	lintExec(t, f.fixture, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
SELECT id,$1,1,2,true FROM unnest($2::uuid[]) p(id)`, f.eng, heads)
	// Only 400 source IDs and 10,000 parallel asset entries cross the wire.
	// No old/new Facts, per-row []any, or 4M-row expected slice is constructed.
	lintExec(t, f.fixture, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,
property_key,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,resolved_asset,aliases)
SELECT s.id,(8*a.ord)::int,(8*a.ord+4)::int,$1,'link',a.name,
NULL,NULL,NULL,a.name,NULL,a.id,false,true,false
FROM unnest($2::uuid[]) s(id)
CROSS JOIN unnest($3::text[],$4::uuid[]) WITH ORDINALITY a(name,id,ord)
ORDER BY s.id,a.ord`, f.eng, f.sources, f.names, f.assets)
	lintExec(t, f.fixture, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,
property_key,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,resolved_asset,aliases)
SELECT q.id,2,10,$1,'wikilink','raw/S'||lpad((q.ord-1)::text,3,'0'),
NULL,NULL,NULL,'s'||lpad((q.ord-1)::text,3,'0'),NULL,q.target,false,false,false
FROM unnest($2::uuid[],$3::uuid[]) WITH ORDINALITY q(id,target,ord)`, f.eng, f.incoming, f.sources)
	for _, table := range stablePlanTables() {
		lintExec(t, f.fixture, "VACUUM "+table) // Explicitly no ANALYZE here.
	}
	tr := &stablePlanTrace{}
	pool := lintPool(t, f.fixture, tr, 1)
	f.pool, f.s, f.tx, f.trace = pool, postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second), tr
	if state == "analyzed" {
		if err := f.s.Analyze(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	f.statistics = linksDenseStatistics(t, f)
	stablePlanCatalog(t, f.stablePlanFixture)
	linksDenseState(t, f, "seed")
	lintEvidence(t, "links-dense-fixture", map[string]any{"state": state, "qualification_only": true,
		"bootstrap_physical_history_reproduced": false, "sources": f.sources, "incoming": f.incoming,
		"asset_count": len(f.assets), "first_asset": f.assets[0], "last_asset": f.assets[len(f.assets)-1],
		"first_name": f.names[0], "last_name": f.names[len(f.names)-1], "dense_rows": linksDenseRows,
		"incoming_rows": linksDenseSources, "visible_versions_per_row": 1, "manual_vacuum": true,
		"small_reach_keys": []string{"aa.a", "zz.x"}, "small_non_nil_sources": []uuid.UUID{f.assets[0]},
		"small_arm_hits_key_alt_resolved_source": []int{400, 0, 400, 0}, "small_unique_rows": 400,
		"full_source_only_rows": linksDenseRows, "full_keys_targets_nil": true})
	return f
}

func linksDenseParser(t *testing.T, f *linksDenseFixture) {
	t.Helper()
	var body strings.Builder
	body.WriteString("S000\n")
	for _, name := range f.names {
		body.WriteString("[](" + name + ")")
	}
	text := body.String()
	facts := originFacts(t, text) // Existing real Markdown parser + PageFacts adapter.
	if len(text) != 80005 || len(facts.Links) != linksDenseAssets || !facts.FrontmatterValid ||
		len(facts.Tags) != 0 || len(facts.Properties) != 0 || len(facts.Aliases) != 0 {
		t.Fatal("dense body/parser dimensions differ from the independent literal contract")
	}
	for j, link := range facts.Links {
		// Arithmetic oracle is independent of the nested lexical seed loop,
		// SQL rows and parser output. Empty optional fields map to SQL NULL;
		// linksDenseState checks actual stored NULLs, not coalesced strings.
		name := string([]byte{byte('a' + j/676), byte('a' + j/26%26), '.', byte('a' + j%26)})
		want := domain.Link{Kind: "link", Target: name, Start: 8 + 8*j, End: 12 + 8*j}
		if link != want || f.names[j] != name || !slices.Equal(link.Keys(), []string{name}) || link.InFrontmatter() {
			t.Fatalf("dense parser literal row %d=%+v want=%+v", j, link, want)
		}
	}
	q := originFacts(t, "[[raw/S000]]")
	if len(q.Links) != 1 || q.Links[0] != (domain.Link{Kind: "wikilink", Target: "raw/S000", Start: 2, End: 10}) ||
		!slices.Equal(q.Links[0].Keys(), []string{"s000"}) {
		t.Fatalf("dense incoming parser=%+v", q)
	}
	lintEvidence(t, "links-dense-parser", map[string]any{"body_bytes": len(text), "body_sha": fmt.Sprintf("%x", sha256.Sum256([]byte(text))),
		"all_links_verified": len(facts.Links), "first": facts.Links[0], "last": facts.Links[len(facts.Links)-1],
		"literal_first_name": "aa.a", "literal_last_name": "ou.p", "alternate_keys": 0, "body_optional_fields_empty": true})
}

func linksDenseStatistics(t *testing.T, f *linksDenseFixture) map[string]int {
	t.Helper()
	// stablePlanStatistics requires nonempty analyzed data in all five
	// tables. This fixture deliberately leaves tags/properties/aliases empty.
	out := map[string]int{}
	for _, table := range stablePlanTables() {
		var disabled bool
		var count int
		err := f.pool.QueryRow(t.Context(), `SELECT coalesce(reloptions,'{}') @> ARRAY['autovacuum_enabled=false'],
(SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename=$1::text)
FROM pg_class WHERE oid=($1::text)::regclass`, table).Scan(&disabled, &count)
		wantStats := f.state == "analyzed" && (table == "indexed_pages" || table == "page_links")
		if err != nil || !disabled || (count > 0) != wantStats {
			t.Fatalf("dense statistics %s count=%d disabled=%t want_stats=%t err=%v", table, count, disabled, wantStats, err)
		}
		out[table] = count
	}
	return out
}

func linksDenseState(t *testing.T, f *linksDenseFixture, phase string) {
	t.Helper()
	// Validate every stored value against a compact independent relation.
	// PK(source_id,range_start), exact per-source counts and exact allowed
	// ranges prove completeness; this is not a checksum or sampled snapshot.
	rows, err := f.pool.Query(t.Context(), `WITH
s AS (SELECT id FROM unnest($2::uuid[]) p(id)),
q AS (SELECT id,target,ord FROM unnest($3::uuid[],$2::uuid[]) WITH ORDINALITY p(id,target,ord)),
a AS (SELECT name,id,ord FROM unnest($4::text[],$5::uuid[]) WITH ORDINALITY p(name,id,ord))
SELECT l.source_id,count(*),count(*) FILTER (WHERE (
l.notebook_id=$1 AND l.property_key IS NULL AND l.anchor IS NULL AND l.display IS NULL
AND l.target_alt_key IS NULL AND NOT l.ambiguous AND NOT l.aliases AND
CASE WHEN s.id IS NOT NULL THEN a.id IS NOT NULL AND l.kind='link' AND l.range_end=l.range_start+4
 AND l.target=a.name AND l.target_key=a.name AND l.resolved_id=a.id AND l.resolved_asset
WHEN q.id IS NOT NULL THEN l.kind='wikilink' AND l.range_start=2 AND l.range_end=10
 AND l.target='raw/S'||lpad((q.ord-1)::text,3,'0') AND l.target_key='s'||lpad((q.ord-1)::text,3,'0')
 AND l.resolved_id=q.target AND NOT l.resolved_asset ELSE false END) IS NOT TRUE),
count(*) FILTER (WHERE l.target_key=ANY(ARRAY['aa.a','zz.x'])),
count(*) FILTER (WHERE l.target_alt_key=ANY(ARRAY['aa.a','zz.x'])),
count(*) FILTER (WHERE l.resolved_id=$6),count(*) FILTER (WHERE l.source_id=$6)
FROM page_links l LEFT JOIN s ON s.id=l.source_id LEFT JOIN q ON q.id=l.source_id
LEFT JOIN a ON s.id IS NOT NULL AND l.range_start=8*a.ord
GROUP BY l.source_id ORDER BY l.source_id`, f.eng, f.sources, f.incoming, f.names, f.assets, f.assets[0])
	if err != nil {
		t.Fatal(err)
	}
	groups, total := 0, int64(0)
	var arms [4]int64
	for rows.Next() {
		var id uuid.UUID
		var count, bad int64
		var hits [4]int64
		if err := rows.Scan(&id, &count, &bad, &hits[0], &hits[1], &hits[2], &hits[3]); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		want := int64(1)
		if slices.Contains(f.sources, id) {
			want = linksDenseAssets
		} else if !slices.Contains(f.incoming, id) {
			rows.Close()
			t.Fatalf("dense unexpected stored source %s", id)
		}
		if count != want || bad != 0 {
			rows.Close()
			t.Fatalf("dense %s original source %s count=%d want=%d bad_complete_rows=%d", phase, id, count, want, bad)
		}
		groups++
		total += count
		for i := range arms {
			arms[i] += hits[i]
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil || groups != 800 || total != linksDenseRows+400 || arms != [4]int64{400, 0, 400, 0} {
		t.Fatalf("dense %s complete source state: groups=%d total=%d arms=%v err=%v", phase, groups, total, arms, err)
	}
	heads := append(slices.Clone(f.sources), f.incoming...)
	var count, bad int64
	err = f.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER (WHERE
(node_id=ANY($2::uuid[]) AND notebook_id=$1 AND revision=1 AND extractor=2 AND frontmatter_valid) IS NOT TRUE)
FROM indexed_pages`, f.eng, heads).Scan(&count, &bad)
	if err != nil || count != 800 || bad != 0 {
		t.Fatalf("dense %s original heads: count=%d bad=%d err=%v", phase, count, bad, err)
	}
	for _, table := range []string{"page_tags", "page_properties", "page_aliases"} {
		if err := f.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("dense %s original %s count=%d err=%v", phase, table, count, err)
		}
	}
	lintEvidence(t, "links-dense-source-state", map[string]any{"phase": phase, "state": f.state, "complete_rows_verified": total,
		"source_groups": groups, "heads": 800, "arm_hits_key_alt_resolved_source": arms, "all_five_tables_equal_to_formula": true,
		"stored_property_anchor_display_alt_are_sql_null": true})
}

// These wrappers only observe the original native transaction and Rows;
// query text, arguments, modes, Scan destinations and lifecycle are forwarded.
type linksDenseContext struct {
	context.Context
	base pgx.Tx
	spy  *linksDenseTx
}

func (c linksDenseContext) Value(key any) any {
	v := c.Context.Value(key)
	if tx, ok := v.(pgx.Tx); ok && tx == c.base {
		return c.spy
	}
	return v
}

type linksDenseTx struct {
	pgx.Tx
	rows    *linksDenseNativeRows
	queries int
}

func (s *linksDenseTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.queries++
	rows, err := s.Tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	s.rows = &linksDenseNativeRows{Rows: rows, conn: s.Conn(), d: stablePlanD{FieldBytes: make([]int64, 7), FieldNulls: make([]int, 7)}}
	for _, field := range rows.FieldDescriptions() {
		s.rows.d.OIDs = append(s.rows.d.OIDs, field.DataTypeOID)
		s.rows.d.Formats = append(s.rows.d.Formats, field.Format)
	}
	return s.rows, nil
}

type linksDenseNativeRows struct {
	pgx.Rows
	conn             *pgx.Conn
	d                stablePlanD
	scans, errChecks int
	badWidth         bool
}

func (r *linksDenseNativeRows) Next() bool {
	if !r.Rows.Next() {
		r.d.EOFs++
		return false
	}
	r.d.Rows++
	raw := r.RawValues()
	if len(raw) != 7 {
		r.badWidth = true
		return true
	}
	r.d.DataRowBytes += 35 // message header, column count, seven length fields
	for i, value := range raw {
		r.d.FieldBytes[i] += int64(len(value))
		r.d.DataRowBytes += int64(len(value))
		if value == nil {
			r.d.FieldNulls[i]++
		}
	}
	return true
}

func (r *linksDenseNativeRows) Scan(dest ...any) error {
	r.scans++
	r.badWidth = r.badWidth || len(dest) != 7
	return r.Rows.Scan(dest...)
}

func (r *linksDenseNativeRows) Err() error {
	r.errChecks++
	return r.Rows.Err()
}
func (r *linksDenseNativeRows) Close() {
	r.d.CloseCalls++
	r.Rows.Close()
	r.d.Closed = !r.conn.PgConn().IsBusy()
}

func linksDenseActual(t *testing.T, ctx context.Context, f *linksDenseFixture, reach domain.Reach, full bool, mode, xid string) (stablePlanCall, map[string]stablePlanWork) {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	spy := &linksDenseTx{Tx: tx}
	observed := linksDenseContext{Context: ctx, base: tx, spy: spy}
	if carried, ok := postgres.TxFrom(observed); !ok || carried != spy || postgres.DB(observed, f.pool) != spy || spy.Conn() != tx.Conn() {
		t.Fatal("dense observation changed the native transaction")
	}
	original := domain.Reach{Keys: slices.Clone(reach.Keys), Targets: slices.Clone(reach.Targets), Sources: slices.Clone(reach.Sources)}
	beforeChoice := stablePlanCounter(t, ctx, tx, gen.LinksReached)
	work := stablePlanRelationWork(t, ctx, tx)
	f.trace.begin()
	f.trace.mu.Lock()
	delete(f.trace.prepares, gen.LinksReached)
	f.trace.mu.Unlock()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	got, err := f.s.Links(observed, f.eng, reach)
	wall := time.Since(start)
	runtime.ReadMemStats(&after)
	calls := f.trace.end()
	if tx.Conn().PgConn().IsBusy() {
		t.Fatal("dense actual Store.Links left its native Rows busy")
	}
	delta := stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	afterChoice := stablePlanCounter(t, ctx, tx, gen.LinksReached)
	var afterXID string
	if err := tx.QueryRow(ctx, "SELECT pg_current_xact_id()::text").Scan(&afterXID); err != nil {
		t.Fatal(err)
	}
	f.trace.mu.Lock()
	prepareName, prepared := f.trace.prepares[gen.LinksReached]
	f.trace.mu.Unlock()
	wantRows := linksDenseSources
	if full {
		wantRows = linksDenseRows
	}
	var first, last app.Link
	if len(got) > 0 {
		first, last = got[0], got[len(got)-1]
	}
	var d *stablePlanD
	if spy.rows != nil {
		d = &spy.rows.d
	}
	lintEvidence(t, "links-dense-actual", map[string]any{"state": f.state, "mode": mode, "full_source_only": full,
		"scope": "actual complete Store.Links DescribeExec; no attached plan claimed", "calls": calls,
		"sql_sha": fmt.Sprintf("%x", sha256.Sum256([]byte(gen.LinksReached))), "work": delta, "rows": len(got), "cap": cap(got),
		"first": first, "last": last, "error": fmt.Sprint(err), "native_D": d, "wall_ns": wall.Nanoseconds(),
		"go_alloc": after.TotalAlloc - before.TotalAlloc, "go_heap_alloc_after": after.HeapAlloc,
		"retained_link_bytes": uint64(len(got)) * uint64(reflect.TypeFor[app.Link]().Size()),
		"tx_identity":         fmt.Sprintf("%p", tx), "xid_before": xid, "xid_after": afterXID, "tx_status": string(tx.Conn().PgConn().TxStatus()),
		"before_choice": beforeChoice, "after_choice": afterChoice, "prepare_observed": prepared, "prepare_name": prepareName})
	if err != nil || len(calls) != 1 || got == nil || len(got) != wantRows || cap(got) != wantRows || spy.rows == nil {
		t.Fatalf("dense actual output: rows=%d cap=%d want=%d calls=%d err=%v", len(got), cap(got), wantRows, len(calls), err)
	}
	for n, row := range got {
		i, j := n, 0
		if full {
			i, j = n/linksDenseAssets, n%linksDenseAssets
		}
		want := app.Link{SourceID: f.sources[i], Start: 8 + 8*j, Target: f.names[j], Resolution: domain.Resolution{ID: f.assets[j], Asset: true}}
		if row != want { // All seven values, exact (source_id, range_start) order.
			t.Fatalf("dense complete seven-value row %d=%+v want=%+v", n, row, want)
		}
	}
	c := calls[0]
	cfg := f.pool.Config().ConnConfig
	if c.Name != "LinksReached" || c.SQL != gen.LinksReached || c.Kind != ":many" || c.Error != "" ||
		c.PID != tx.Conn().PgConn().PID() || c.CommandTag != fmt.Sprintf("SELECT %d", wantRows) || c.RowsAffected != int64(wantRows) ||
		!slices.Equal(c.LeadingOptions, []string{fmt.Sprintf("leading=true mode=%v", pgx.QueryExecModeDescribeExec)}) ||
		!slices.Equal(c.ArgumentTypes, []string{"uuid.UUID", "[]string", "[]uuid.UUID", "[]uuid.UUID"}) ||
		!reflect.DeepEqual(c.Args, []any{f.eng, original.Keys, original.Targets, original.Sources}) || !reflect.DeepEqual(reach, original) ||
		c.DefaultMode != cfg.DefaultQueryExecMode || c.StatementCapacity != cfg.StatementCacheCapacity || c.DescriptionCapacity != cfg.DescriptionCacheCapacity ||
		!prepared || prepareName != "" || beforeChoice.Name != "" || afterChoice.Name != "" || afterXID != xid || tx.Conn().PgConn().TxStatus() != 'T' {
		t.Fatalf("dense actual binding/transaction/DescribeExec: %+v", c)
	}
	r := spy.rows
	if spy.queries != 1 || r.scans != wantRows || r.errChecks != 1 || r.badWidth || d.Rows != wantRows || d.EOFs != 1 || d.CloseCalls != 1 || !d.Closed ||
		!slices.Equal(d.OIDs, []uint32{pgtype.UUIDOID, pgtype.Int4OID, pgtype.TextOID, pgtype.UUIDOID, pgtype.BoolOID, pgtype.BoolOID, pgtype.BoolOID}) ||
		!slices.Equal(d.FieldNulls, []int{0, 0, 0, 0, 0, 0, 0}) || d.DataRowBytes != int64(wantRows)*78 {
		t.Fatalf("dense complete native Rows: queries=%d scans=%d err_checks=%d bad_width=%t D=%+v", spy.queries, r.scans, r.errChecks, r.badWidth, d)
	}
	for i, width := range []int64{16, 4, 4, 16, 1, 1, 1} {
		if d.FieldBytes[i] != int64(wantRows)*width || d.Formats[i] != tx.Conn().TypeMap().FormatCodeForOID(d.OIDs[i]) {
			t.Fatalf("dense native field %d: bytes=%d format=%d", i, d.FieldBytes[i], d.Formats[i])
		}
	}
	lintEvidence(t, "links-dense-values-verified", map[string]any{"full_source_only": full, "complete_seven_value_rows": len(got),
		"exact_order_and_capacity": true, "input_unchanged_including_nil": true, "scans": r.scans, "err_checks": r.errChecks, "D": d})
	return c, delta // got and its 4M app.Link values are not retained by evidence.
}

func linksDenseReplay(t *testing.T, ctx context.Context, f *linksDenseFixture, c stablePlanCall, full bool, mode string, settings map[string]string) stablePlanEvidence {
	t.Helper()
	tx, _ := postgres.TxFrom(ctx)
	desc, err := tx.Conn().PgConn().Prepare(ctx, "", c.SQL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(desc.ParamOIDs, []uint32{pgtype.UUIDOID, pgtype.TextArrayOID, pgtype.UUIDArrayOID, pgtype.UUIDArrayOID}) {
		t.Fatalf("dense four original parameter OIDs=%v", desc.ParamOIDs)
	}
	const name = "links_reached_dense_observation"
	if _, err := tx.Conn().Prepare(ctx, name, c.SQL); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Conn().Deallocate(ctx, name); err != nil {
			t.Fatal(err)
		}
	}()
	e := stablePlanEvidence{Case: t.Name(), Scope: "independent named captured-SQL replay; not an actual Store.Links attached plan",
		State: f.state, Mode: mode, Name: c.Name, SQL: c.SQL, SQLSHA: fmt.Sprintf("%x", sha256.Sum256([]byte(c.SQL))),
		ArgumentSHA: c.ArgumentSHA, PID: tx.Conn().PgConn().PID(), Statistics: f.statistics, Settings: settings,
		Encoding:            stablePlanEncoding{OIDs: slices.Clone(desc.ParamOIDs), InputWireFormats: "NOT_OBSERVED"},
		PlanningObservation: "separate EXPLAIN without ANALYZE; complete JSON retained",
		RawWorkAcceptance:   "source-only permits full scans, sorting and spill; no elapsed-time or zero-spill guard"}
	if full && f.state == "analyzed" && mode == "auto" {
		e.RawWorkAcceptance = "fixed analyzed/auto source-only regression: no full-result Sort or root temp I/O; no elapsed-time guard"
	}
	execute := stablePlanExecute(t, name, c.Args)
	beforeChoice := stablePlanCounter(t, ctx, tx, c.SQL)
	work := stablePlanRelationWork(t, ctx, tx)
	var raw string
	if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+execute).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	e.PlanningPlan = json.RawMessage(raw)
	e.PlanningWork = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	e.BeforeChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	lintEvidence(t, "links-dense-plan-only", map[string]any{"state": f.state, "mode": mode, "full_source_only": full,
		"scope": e.Scope, "call": c, "plan": e.PlanningPlan, "work": e.PlanningWork, "before_choice": beforeChoice, "after_choice": e.BeforeChoice})
	work = stablePlanRelationWork(t, ctx, tx)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+execute).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	e.WallNS = time.Since(start).Nanoseconds()
	runtime.ReadMemStats(&after)
	e.GoAlloc = after.TotalAlloc - before.TotalAlloc
	e.Plan = json.RawMessage(raw)
	e.ExecutionWork = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	e.AfterChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	lintEvidence(t, "links-dense-physical-replay", map[string]any{"full_source_only": full, "evidence": e})
	root := stablePlanTree(t, e.Plan)
	nodes := []map[string]any{}
	for _, n := range root.all() {
		nodes = append(nodes, map[string]any{"type": n.Type, "relation": n.Relation, "index": n.Index, "rows": n.Rows, "loops": n.Loops,
			"removed": n.Removed, "rechecked": n.Rechecked, "join_removed": n.JoinRemoved})
	}
	var plan []struct {
		Plan struct {
			Read    int64 `json:"Temp Read Blocks"`
			Written int64 `json:"Temp Written Blocks"`
		}
	}
	if err := json.Unmarshal(e.Plan, &plan); err != nil || len(plan) != 1 {
		t.Fatalf("dense temp observation: %v", err)
	}
	lintEvidence(t, "links-dense-replay-summary", map[string]any{"state": f.state, "mode": mode, "full_source_only": full, "scope": e.Scope,
		"actual_nodes": nodes, "root_temp_read_blocks": plan[0].Plan.Read, "root_temp_written_blocks": plan[0].Plan.Written,
		"wall_ns": e.WallNS, "go_alloc": e.GoAlloc, "rows_not_returned_to_go_by_explain": true})
	stablePlanCountDelta(t, beforeChoice, e.BeforeChoice, mode)
	stablePlanCountDelta(t, e.BeforeChoice, e.AfterChoice, mode)
	return e
}

func linksDenseWork(t *testing.T, work map[string]stablePlanWork, full bool) {
	t.Helper()
	var heap, sequential, index int64
	for relation, w := range work {
		if w.Inserted != 0 || w.Updated != 0 || w.Deleted != 0 {
			t.Errorf("dense read performed DML on %s: %+v", relation, w)
		}
		if w.Owner == "page_links" {
			heap += w.Fetched
			switch w.Kind {
			case "r":
				sequential += w.Returned
			case "i":
				index += w.Returned
			}
		}
	}
	// Pristine visible version ledger: key=400, alt=0, resolved=400,
	// source=0. Overlap may be visited in both arms, then emitted once.
	if !full && (heap+sequential > 800 || index > 800) {
		t.Errorf("DENSE_SMALL_REACH_WORK: heap=%d sequential=%d index=%d; four-arm visible allowance=400+0+400+0", heap, sequential, index)
	}
}

func linksDenseTree(t *testing.T, raw json.RawMessage, full bool) {
	t.Helper()
	root := stablePlanTree(t, raw)
	want := float64(linksDenseSources)
	if full {
		want = linksDenseRows
	}
	if root.Rows != want || root.Loops != 1 {
		t.Errorf("dense replay result rows=%v loops=%v want=%v/1", root.Rows, root.Loops, want)
	}
	var visited float64
	for _, node := range root.all() {
		if !node.RowsObserved || !node.LoopsObserved || (node.Relation != "" && node.Relation != "page_links") {
			t.Errorf("dense replay observation=%+v", node)
		}
		if node.Relation == "page_links" {
			visited += stablePlanRowsLower(node.Rows, node.Loops) + stablePlanRowsLower(node.Removed, node.Loops) + stablePlanRowsLower(node.Rechecked, node.Loops)
		}
	}
	if !full && visited > 800 {
		t.Errorf("DENSE_SMALL_REACH_PLAN: visible tuples lower bound=%v, allowance=400+0+400+0", visited)
	}
}

// Only the fixed analyzed/auto source-only leaf has an observed ordered, temp-free
// OR baseline. Other leaves keep their existing scan, sort and spill allowance.
func linksDenseOrderedSourceWork(t *testing.T, raw json.RawMessage) {
	t.Helper()
	root := stablePlanTree(t, raw)
	for _, node := range root.all() {
		// Actual Rows is per execution: repeated small sorts and incremental
		// sort groups are not a single full-result sort.
		if node.Type == "Sort" && node.Loops > 0 && node.Rows >= linksDenseRows {
			t.Errorf("DENSE_ORDERED_SOURCE_WORK: analyzed/auto source-only replay sorts the full result: rows=%v loops=%v", node.Rows, node.Loops)
		}
	}
	var document []struct {
		Plan struct {
			Read    *int64 `json:"Temp Read Blocks"`
			Written *int64 `json:"Temp Written Blocks"`
		}
	}
	if err := json.Unmarshal(raw, &document); err != nil || len(document) != 1 {
		t.Fatalf("DENSE_ORDERED_SOURCE_WORK: root temp observation: %v", err)
	}
	temp := document[0].Plan
	if temp.Read == nil || temp.Written == nil {
		t.Fatal("DENSE_ORDERED_SOURCE_WORK: root temp fields were not observed")
	}
	// Root totals already include descendants; do not add node temp counters.
	if *temp.Read > 0 || *temp.Written > 0 {
		t.Errorf("DENSE_ORDERED_SOURCE_WORK: analyzed/auto source-only replay temp read=%d written=%d blocks", *temp.Read, *temp.Written)
	}
}
