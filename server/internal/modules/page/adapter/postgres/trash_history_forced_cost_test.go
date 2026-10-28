//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type trashHistoryRead struct {
	SQL           string
	Args          []any
	Mode          pgx.QueryExecMode
	ModeArguments int
	PID           uint32
	Rows          int64
	Error         string
}

type trashHistoryTrace struct{ reads []*trashHistoryRead }
type trashHistoryTraceKey struct{}

// Each case is serial on one backend. Snapshot the IDs before any caller
// can reuse their slice; no file IO occurs inside the timed Store call.
func (tr *trashHistoryTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(d.SQL, "-- name: TrashHistory :one\n") {
		return ctx
	}
	r := &trashHistoryRead{SQL: d.SQL, PID: conn.PgConn().PID()}
	for _, arg := range d.Args {
		if mode, ok := arg.(pgx.QueryExecMode); ok {
			r.Mode, r.ModeArguments = mode, r.ModeArguments+1
			continue
		}
		if ids, ok := arg.([]uuid.UUID); ok {
			arg = slices.Clone(ids)
		}
		r.Args = append(r.Args, arg)
	}
	tr.reads = append(tr.reads, r)
	return context.WithValue(ctx, trashHistoryTraceKey{}, r)
}

func (*trashHistoryTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if r, ok := ctx.Value(trashHistoryTraceKey{}).(*trashHistoryRead); ok {
		r.Rows = d.CommandTag.RowsAffected()
		if d.Err != nil {
			r.Error = d.Err.Error()
		}
	}
}

type trashHistoryCostEvidence struct {
	Stage, Mode, SQL, Error string
	IDs                     []uuid.UUID
	Arguments               []any
	DeletedAt               time.Time
	Count, Want, Limit      int
	Elapsed                 time.Duration
	PID                     uint32
	MaxConns                int32
	Statistics              map[string]int
	Settings                map[string]string
	Reads                   []*trashHistoryRead
	Work                    map[string]sourceRelationWork
	PreparedTypes, Execute  string
	Before, After           [2]int64 // generic_plans, custom_plans of this named statement only
	Plan                    json.RawMessage
}

// This adds explicit forced modes to M8's existing one-second count bound.
// The fixture has real adapter metadata/history and real DeleteNodes MVCC,
// not complete source HTTP history, files or blobs. Count once per case;
// no one-second context cancels a slow old statement before it can finish.
func TestTrashHistoryAtItsBoundCountsInTimeWithForcedPlans(t *testing.T) {
	f, ids, at := trashHistoryBoundFixture(t)
	tables := []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"}
	for _, stage := range []string{"no_pg_stats", "explicitly_analyzed"} {
		if stage == "explicitly_analyzed" {
			f.exec(t, "ANALYZE nodes,page_contents,page_revisions,changesets,changeset_items,asset_blobs")
		}
		stats := map[string]int{}
		for _, table := range tables {
			stats[table] = f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename=$1", table)
			if stage == "no_pg_stats" && stats[table] != 0 || stage == "explicitly_analyzed" && table != "asset_blobs" && stats[table] == 0 {
				t.Fatalf("%s %s pg_stats=%d", stage, table, stats[table])
			}
		}
		for _, mode := range []string{"force_generic_plan", "force_custom_plan"} {
			t.Run(stage+"/"+mode, func(t *testing.T) {
				tr := &trashHistoryTrace{}
				s, pool := trashHistoryOneConnection(t, f, mode, tr)
				p := gen.TrashHistoryParams{Ids: ids, DeletedAt: at, Lim: 400001}
				statement := statementOf(t, func(q *gen.Queries) error { _, err := q.TrashHistory(t.Context(), p); return err })
				e := trashHistoryCostEvidence{Stage: stage, Mode: mode, SQL: statement.sql, IDs: ids, Arguments: statement.args,
					DeletedAt: at, Want: 400000, Limit: 400001, Statistics: stats, Settings: map[string]string{}, MaxConns: pool.Config().MaxConns}
				// Snapshot and counters use the very transaction/backend the real
				// Store uses, without including BEGIN/COMMIT or EXPLAIN in timing.
				err := postgres.NewTxManager(pool, 2*time.Second).WithinSnapshot(t.Context(), func(ctx context.Context) error {
					tx, ok := postgres.TxFrom(ctx)
					if !ok {
						return fmt.Errorf("Store count has no actual transaction")
					}
					e.PID = tx.Conn().PgConn().PID()
					for _, key := range []string{"plan_cache_mode", "track_counts", "enable_seqscan", "enable_indexscan", "enable_bitmapscan",
						"enable_indexonlyscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "enable_sort", "enable_incremental_sort", "enable_material", "jit", "statement_timeout"} {
						var value string
						if err := tx.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); err != nil {
							return err
						}
						e.Settings[key] = value
						want := "on"
						switch key {
						case "plan_cache_mode":
							want = mode
						case "jit":
							want = "off"
						case "statement_timeout":
							want = "0"
						}
						if value != want {
							return fmt.Errorf("actual backend %s=%s want%s", key, value, want)
						}
					}
					before, err := trashHistoryWork(ctx, tx)
					if err != nil {
						return err
					}
					started := time.Now()
					e.Count, err = s.TrashHistory(ctx, ids, at, 400001)
					e.Elapsed = time.Since(started)
					if err != nil {
						return err
					}
					after, err := trashHistoryWork(ctx, tx)
					if err != nil {
						return err
					}
					e.Work = sourceWorkDelta(before, after)
					prepare, execute, types, err := trashHistoryPrepared(statement)
					if err != nil {
						return err
					}
					e.PreparedTypes, e.Execute = types, execute
					if _, err = tx.Exec(ctx, prepare); err != nil {
						return err
					}
					counters := `SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name='trash_history_forced_evidence'`
					if err = tx.QueryRow(ctx, counters).Scan(&e.Before[0], &e.Before[1]); err != nil {
						return err
					}
					// No ANALYZE: this obtains a real named forced-mode plan and
					// counters without executing a second 400,000-row count. It
					// corroborates planning; it is not the unnamed Store's plan.
					if err = tx.QueryRow(ctx, "EXPLAIN(VERBOSE,FORMAT JSON) "+execute).Scan(&e.Plan); err != nil {
						return err
					}
					return tx.QueryRow(ctx, counters).Scan(&e.After[0], &e.After[1])
				})
				e.Reads = tr.reads
				if err != nil {
					e.Error = err.Error()
				}
				sourceEvidence(t, "trash-history-forced-cost", e)
				if err != nil {
					t.Fatal(err)
				}
				if e.Count != 400000 {
					t.Fatalf("real Store count=%d want400000", e.Count)
				}
				if e.MaxConns != 1 || e.PID == 0 || len(tr.reads) != 1 {
					t.Fatalf("actual backend PID=%d maxConns=%d reads=%d want one", e.PID, e.MaxConns, len(tr.reads))
				}
				r := tr.reads[0]
				if r.SQL != statement.sql || !reflect.DeepEqual(r.Args, statement.args) || r.Mode != pgx.QueryExecModeDescribeExec ||
					r.ModeArguments != 1 || r.PID != e.PID || r.Rows != 1 || r.Error != "" {
					t.Fatalf("real Store did not run generated Planned count on the checked backend: %+v", r)
				}
				want := [2]int64{1, 0}
				if mode == "force_custom_plan" {
					want = [2]int64{0, 1}
				}
				if e.Before != [2]int64{} || e.After != want {
					t.Fatalf("named plan counters %v->%v want[0 0]->%v", e.Before, e.After, want)
				}
				if e.Elapsed > time.Second {
					t.Errorf("complete real Store count of400000 rows %s/%s took%s; M8 bound is1s", stage, mode, e.Elapsed)
				}
			})
		}
	}
}

func trashHistoryBoundFixture(t *testing.T) (fixture, []uuid.UUID, time.Time) {
	t.Helper()
	if domain.MaxUnitPages != 100 || domain.MaxUnitNodes != 10000 || domain.MaxUnitHistory != 400000 {
		t.Fatal("review this literal 100-page/10000-node/400000-history boundary fixture after a domain bound change")
	}
	f := newFixture(t)
	for _, table := range []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"} {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	root := f.page(t, f.eng, nil, "Root", 0)
	pages := []uuid.UUID{root.ID}
	for i := range 99 {
		pages = append(pages, f.page(t, f.eng, &root.ID, fmt.Sprintf("Page %d", i), float64(i)).ID)
	}
	// Same history shape as the unchanged M8 builder: 2,000 shared
	// changesets, 2,000 revisions and matching items on each of100 pages.
	f.exec(t, `INSERT INTO changesets (id,notebook_id,kind,client,created_by_id,created_at,updated_at)
		SELECT md5($1::uuid::text||k)::uuid,$1,'edit','web',$2,$3,$3 FROM generate_series(1,2000) k`, f.eng, f.alice, now())
	f.exec(t, `INSERT INTO page_revisions (id,changeset_id,node_id,base_revision,revision,content,content_hash,byte_size,created_at,updated_at)
		SELECT gen_random_uuid(),md5($2::uuid::text||k)::uuid,p,NULLIF(k-1,0),k,'v'||k,
		sha256(convert_to('v'||k,'UTF8')),octet_length('v'||k),$3,$3
		FROM unnest($1::uuid[]) p,generate_series(1,2000) k`, pages, f.eng, now())
	f.exec(t, `UPDATE page_contents SET revision=2000 WHERE node_id=ANY($1)`, pages)
	f.exec(t, `INSERT INTO changeset_items (id,changeset_id,node_id,after_parent_id,after_name,after_sort_order,created_at,updated_at)
		SELECT gen_random_uuid(),r.changeset_id,n.id,n.parent_id,n.name,n.sort_order,n.created_at,n.created_at
		FROM page_revisions r JOIN nodes n ON n.id=r.node_id`)
	f.exec(t, `WITH p AS (SELECT id,ord-1 AS k FROM unnest($3::uuid[]) WITH ORDINALITY AS a(id,ord))
		INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at)
		SELECT gen_random_uuid(),$1,p.id,'asset','a'||g||'.png','a'||g||'.png',g,$2,$2,$4,$4
		FROM generate_series(0,9899) g JOIN p ON p.k=g%100`, f.eng, f.alice, pages, now())
	rows, err := f.pool.Query(t.Context(), `SELECT id FROM nodes WHERE notebook_id=$1 AND kind='asset' ORDER BY id`, f.eng)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil || len(assets) != 9900 {
		t.Fatalf("asset IDs=%d err=%v want9900", len(assets), err)
	}
	at := now().Add(time.Hour)
	f.trashed(t, at, 100, 9900, 2, slices.Concat(pages, assets)...)
	// Query order deliberately puts all9,900 empty-history asset IDs before
	// the100 pages. Deletion still received the real root first above.
	ids := slices.Concat(assets, pages)
	if got := f.count(t, "SELECT count(*) FROM nodes WHERE notebook_id=$1 AND deleted_at=$2", f.eng, at); got != 10000 {
		t.Fatalf("actual deleted nodes=%d want10000", got)
	}
	for _, table := range []string{"page_revisions", "changeset_items"} {
		if got := f.count(t, "SELECT count(*) FROM "+table+" WHERE deleted_at=$1", at); got != 200000 {
			t.Fatalf("fixture %s=%d want200000 real history rows", table, got)
		}
	}
	return f, ids, at
}

func trashHistoryWork(ctx context.Context, tx pgx.Tx) (map[string]sourceRelationWork, error) {
	rows, err := tx.Query(ctx, `WITH base AS (SELECT unnest(ARRAY['nodes'::regclass,'page_contents'::regclass,
		'page_revisions'::regclass,'changesets'::regclass,'changeset_items'::regclass,'asset_blobs'::regclass]) AS oid)
		SELECT c.relname,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),
		pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid)
		FROM pg_class c WHERE c.oid IN(SELECT oid FROM base)
		OR c.oid IN(SELECT indexrelid FROM pg_index WHERE indrelid IN(SELECT oid FROM base)) ORDER BY c.relname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]sourceRelationWork{}
	for rows.Next() {
		var name string
		var w sourceRelationWork
		if err := rows.Scan(&name, &w.Scans, &w.Returned, &w.Fetched, &w.Blocks, &w.Hits); err != nil {
			return nil, err
		}
		out[name] = w
	}
	return out, rows.Err()
}

// Derive types AND literal order from the actual generated arguments.
// Utility EXECUTE does not accept protocol parameters. Only these fixture
// types are accepted; no DSN, configuration or credential is serialized.
func trashHistoryPrepared(s statement) (string, string, string, error) {
	var types, values []string
	for _, arg := range s.args {
		switch v := arg.(type) {
		case []uuid.UUID:
			var ids []string
			for _, id := range v {
				ids = append(ids, "'"+id.String()+"'::uuid")
			}
			types, values = append(types, "uuid[]"), append(values, "ARRAY["+strings.Join(ids, ",")+"]::uuid[]")
		case time.Time:
			types, values = append(types, "timestamptz"), append(values, "'"+v.UTC().Format(time.RFC3339Nano)+"'::timestamptz")
		case int32:
			types, values = append(types, "integer"), append(values, fmt.Sprintf("%d::integer", v))
		default:
			return "", "", "", fmt.Errorf("unexpected generated TrashHistory argument %T", arg)
		}
	}
	name := pgx.Identifier{"trash_history_forced_evidence"}.Sanitize()
	typed := strings.Join(types, ",")
	return "PREPARE " + name + " (" + typed + ") AS " + s.sql, "EXECUTE " + name + " (" + strings.Join(values, ",") + ")", typed, nil
}
