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
	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const trashChildrenParentIndex = "nodes_notebook_id_parent_id_idx"
const trashChildrenTargetPrefix = "75000000-0000-7000-8000-"

type trashChildrenRead struct {
	SQL           string
	Args          []any
	Mode          pgx.QueryExecMode
	ModeArguments int
	PID           uint32
	Rows          int64
	Err           error
}

type trashChildrenTrace struct{ reads []*trashChildrenRead }
type trashChildrenTraceKey struct{}

// These pools run serially on one connection. Keep independent argument
// snapshots: Store reuses its parent slice for successive levels.
func (tr *trashChildrenTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(d.SQL, "-- name: TrashChildrenOfAll :many\n") {
		return ctx
	}
	r := &trashChildrenRead{SQL: d.SQL, PID: conn.PgConn().PID()}
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
	return context.WithValue(ctx, trashChildrenTraceKey{}, r)
}

func (*trashChildrenTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if r, ok := ctx.Value(trashChildrenTraceKey{}).(*trashChildrenRead); ok {
		r.Rows, r.Err = d.CommandTag.RowsAffected(), d.Err
	}
}

func trashChildrenPool(t *testing.T, f fixture, mode string) (*postgresadapter.Store, *trashChildrenTrace, *pgxpool.Pool) {
	t.Helper()
	tr := &trashChildrenTrace{}
	cfg := f.pool.Config()
	cfg.MaxConns, cfg.ConnConfig.Tracer = 1, tr
	cfg.ConnConfig.RuntimeParams["plan_cache_mode"] = mode
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return postgresadapter.New(pool), tr, pool
}

// The tested child tuples are inserted already deleted, with no previous
// child versions or stale child index entries. Only two roots use the real
// deletion port. This is adapter node/current-content metadata, not the
// complete history/files or dead versions of a source HTTP deletion.
func TestTrashChildrenKeepTheirParentRanges(t *testing.T) {
	f := newFixture(t)
	tables := []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "asset_blobs"}
	for _, table := range tables {
		f.exec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	at := now().Add(time.Minute)
	root := f.page(t, f.eng, nil, "Target root", 0)
	other := f.page(t, f.eng, nil, "Unrelated root", 1)
	f.trashed(t, at, 10000, 0, 2, root.ID)
	f.trashed(t, at, 100001, 0, 2, other.ID)
	trashChildrenInsertDeleted(t, f, root.ID, 9999, trashChildrenTargetPrefix, "Target", at)
	trashChildrenInsertDeleted(t, f, other.ID, 100000, "76000000-0000-7000-8000-", "Unrelated", at)
	f.exec(t, `INSERT INTO page_contents (node_id,revision,content,content_hash,byte_size,updated_by_id,updated_at,deleted_at)
		SELECT id,3,'test',sha256(convert_to('test','UTF8')),4,$2,$3,$4
		FROM nodes WHERE notebook_id=$1 AND parent_id=$5`, f.eng, f.alice, now(), at, root.ID)
	root.UpdatedAt = at
	for _, stage := range []string{"no_statistics", "analyzed"} {
		// Never analyze the no_statistics branch or erase inherited pg_stats.
		if stage == "analyzed" {
			f.exec(t, "ANALYZE nodes,page_contents,page_revisions,changesets,changeset_items,asset_blobs")
		}
		stats := map[string]int{}
		for _, table := range tables {
			stats[table] = f.count(t, "SELECT count(*) FROM pg_stats WHERE schemaname='public' AND tablename=$1", table)
			if stage == "no_statistics" && stats[table] != 0 || stage == "analyzed" && (table == "nodes" || table == "page_contents") && stats[table] == 0 {
				t.Fatalf("%s %s pg_stats=%d", stage, table, stats[table])
			}
		}
		for _, mode := range []string{"force_generic_plan", "force_custom_plan"} {
			t.Run(stage+"/"+mode, func(t *testing.T) {
				s, tr, pool := trashChildrenPool(t, f, mode)
				p := gen.TrashChildrenOfAllParams{NotebookID: f.eng, Parents: []uuid.UUID{root.ID}, DeletedAt: at}
				// Validate the nonempty first parent before the long empty list:
				// old broad plans must fail on work, not a whole-suite timeout.
				first := trashChildrenRange(t, pool, p, mode, 9999, stats)
				leaves := make([]uuid.UUID, len(first))
				for i, row := range first {
					n := trashChildrenTargetNode(f, root.ID, 9999-i, 9999, at)
					want := trashChildrenExpectedRow(n, trashChildrenInt32(3), trashChildrenInt32(4))
					if !reflect.DeepEqual(row, want) {
						t.Fatalf("child%d = %+v want %+v", i, row, want)
					}
					leaves[i] = row.ID
				}
				p.Parents = leaves
				trashChildrenRange(t, pool, p, mode, 0, stats)
				if len(tr.reads) != 2 || tr.reads[0].PID == 0 || tr.reads[0].PID != tr.reads[1].PID {
					t.Fatal("direct reads did not use one actual backend")
				}
				pid := tr.reads[0].PID
				tr.reads = nil
				group, err := s.TrashGroup(t.Context(), f.eng, root.ID, at)
				if err != nil || len(group) != 10000 {
					t.Fatalf("TrashGroup nodes=%d err=%v want10000", len(group), err)
				}
				if !reflect.DeepEqual(group[0], domain.TrashMember{Node: root, Level: 1, Revision: 1}) {
					t.Fatalf("wrong root: %+v", group[0])
				}
				height := 1
				for i, member := range group[1:] {
					want := domain.TrashMember{Node: trashChildrenTargetNode(f, root.ID, 9999-i, 9999, at), Level: 2, Revision: 3, Bytes: 4}
					if !reflect.DeepEqual(member, want) {
						t.Fatalf("TrashGroup member%d = %+v want %+v", i+1, member, want)
					}
					if member.Level > height {
						height = member.Level
					}
				}
				if height != 2 || len(tr.reads) != 2 {
					t.Fatalf("height=%d child reads=%d want2/2", height, len(tr.reads))
				}
				for i, read := range tr.reads {
					parents := []uuid.UUID{root.ID}
					if i == 1 {
						parents = leaves
					}
					want := statementOf(t, func(q *gen.Queries) error {
						_, err := q.TrashChildrenOfAll(t.Context(), gen.TrashChildrenOfAllParams{NotebookID: f.eng, Parents: parents, DeletedAt: at})
						return err
					})
					if read.SQL != want.sql || !reflect.DeepEqual(read.Args, want.args) || read.Mode != pgx.QueryExecModeDescribeExec || read.ModeArguments != 1 || read.Rows != []int64{9999, 0}[i] || read.Err != nil || read.PID != pid {
						t.Fatalf("Store level%d did not use generated planned SQL/args/rows/backend: %+v", i+2, read)
					}
				}
				sourceEvidence(t, "trash-children-store", map[string]any{"stage": stage, "mode": mode, "reads": tr.reads, "members": len(group), "height": height})
			})
		}
	}
}

func trashChildrenInsertDeleted(t *testing.T, f fixture, parent uuid.UUID, n int, prefix, name string, at time.Time) {
	t.Helper()
	f.exec(t, `INSERT INTO nodes (id,notebook_id,parent_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at,deleted_at)
		SELECT ($1||lpad(to_hex(i),12,'0'))::uuid,$2,$3,'page',$4||' '||i,lower($4)||' '||i,$5::integer-i,$6,$6,$7,$8,$8
		FROM generate_series(1,$5::integer) i`, prefix, f.eng, parent, name, n, f.alice, now(), at)
}

func trashChildrenTargetNode(f fixture, parent uuid.UUID, i, total int, at time.Time) domain.Node {
	return domain.Node{ID: uuid.MustParse(fmt.Sprintf("%s%012x", trashChildrenTargetPrefix, i)), NotebookID: f.eng, ParentID: &parent, Kind: domain.KindPage, Name: fmt.Sprintf("Target %d", i), NameKey: fmt.Sprintf("target %d", i), SortOrder: float64(total - i), CreatedBy: f.alice, UpdatedBy: f.alice, CreatedAt: now(), UpdatedAt: at}
}

type trashChildrenWork struct{ Nodes, Contents map[string]sourceRelationWork }

// Keep content lookup work separate: sourceWork measures nodes only, and
// a content hash/seq scan or index lookup must not count as node entries.
func trashChildrenSnapshot(t *testing.T, tx pgx.Tx) trashChildrenWork {
	t.Helper()
	out := trashChildrenWork{Nodes: sourceWork(t, tx), Contents: map[string]sourceRelationWork{}}
	rows, err := tx.Query(t.Context(), `SELECT c.relname,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),
		pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid)
		FROM pg_class c WHERE c.oid='page_contents'::regclass OR c.oid IN (SELECT indexrelid FROM pg_index WHERE indrelid='page_contents'::regclass) ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var w sourceRelationWork
		if err := rows.Scan(&name, &w.Scans, &w.Returned, &w.Fetched, &w.Blocks, &w.Hits); err != nil {
			t.Fatal(err)
		}
		out.Contents[name] = w
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func trashChildrenDelta(a, b trashChildrenWork) trashChildrenWork {
	return trashChildrenWork{Nodes: sourceWorkDelta(a.Nodes, b.Nodes), Contents: sourceWorkDelta(a.Contents, b.Contents)}
}

func trashChildrenCheckWork(t *testing.T, label string, w trashChildrenWork, parents, want int) {
	t.Helper()
	// PostgreSQL can probe other indexes' endpoints while planning a join,
	// even in EXPLAIN without ANALYZE. Report those counters separately;
	// measure the parent's data access here, and independently reject any
	// other nodes access in the executed plan below.
	parent := w.Nodes[trashChildrenParentIndex]
	entries, scans, contentReturned := parent.Returned, parent.Scans, int64(0)
	heap := w.Nodes["nodes"]
	if entries != int64(want) || heap.Returned != 0 || heap.Scans != 0 || heap.Fetched > int64(want) {
		t.Fatalf("%s node work entries=%d heap=%+v want%d entries, no sequential/extra fetches", label, entries, heap, want)
	}
	if scans < 1 || scans > int64(parents) {
		t.Fatalf("%s node index probes=%d want1..%d", label, scans, parents)
	}
	for _, work := range w.Contents {
		contentReturned += work.Returned
	}
	// Every selected child has a real fresh content row. This positive
	// sample, and the exact nonempty node count above, reject all-zero stats.
	// Content work has no empty/upper bound here: a legitimate hash/seq
	// content-side build is separate from the primary node-range contract.
	if want > 0 && contentReturned < int64(want) {
		t.Fatalf("%s content work=%d want >=%d positive lookups/scan rows; details=%+v", label, contentReturned, want, w.Contents)
	}
}

func trashChildrenRange(t *testing.T, pool *pgxpool.Pool, p gen.TrashChildrenOfAllParams, mode string, want int, stats map[string]int) []gen.TrashChildrenOfAllRow {
	t.Helper()
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	settings := map[string]string{}
	for _, key := range []string{"plan_cache_mode", "enable_seqscan", "enable_indexscan", "enable_indexonlyscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "enable_sort", "track_counts"} {
		var value string
		if err := tx.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); err != nil {
			t.Fatal(err)
		}
		settings[key] = value
		expected := "on"
		if key == "plan_cache_mode" {
			expected = mode
		}
		if value != expected {
			t.Fatalf("actual backend %s=%s want%s", key, value, expected)
		}
	}
	s := statementOf(t, func(q *gen.Queries) error { _, err := q.TrashChildrenOfAll(ctx, p); return err })
	before := trashChildrenSnapshot(t, tx)
	rows, err := gen.New(postgres.Planned(tx)).TrashChildrenOfAll(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	direct := trashChildrenDelta(before, trashChildrenSnapshot(t, tx))
	const prepared = "trash_children_parent_work"
	if _, err := tx.Exec(ctx, "PREPARE "+prepared+" AS "+s.sql); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = tx.Exec(context.Background(), "DEALLOCATE "+prepared) }()
	execution := trashChildrenExecute(t, prepared, s.args)
	before = trashChildrenSnapshot(t, tx)
	var plan json.RawMessage
	if err := tx.QueryRow(ctx, "EXPLAIN(ANALYZE,BUFFERS,VERBOSE,FORMAT JSON) "+execution).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	explain := trashChildrenDelta(before, trashChildrenSnapshot(t, tx))
	var generic, custom int64
	if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name=$1", prepared).Scan(&generic, &custom); err != nil {
		t.Fatal(err)
	}
	sourceEvidence(t, "trash-children-range", map[string]any{"fixture": "fresh inserted deleted children/current contents; two soft-deleted roots; no complete HTTP history/files", "mode": mode, "settings": settings, "statistics": stats, "pid": tx.Conn().PgConn().PID(), "sql": s.sql, "arguments": s.args, "parents": len(p.Parents), "rows": len(rows), "generic": generic, "custom": custom, "direct_nodes": direct.Nodes, "direct_contents": direct.Contents, "explain_nodes": explain.Nodes, "explain_contents": explain.Contents, "plan": plan})
	if len(rows) != want {
		t.Fatalf("TrashChildrenOfAll rows=%d want%d", len(rows), want)
	}
	if mode == "force_generic_plan" && (generic != 1 || custom != 0) || mode == "force_custom_plan" && (custom != 1 || generic != 0) {
		t.Fatalf("actual prepared generic/custom=%d/%d in%s", generic, custom, mode)
	}
	trashChildrenCheckWork(t, "direct", direct, len(p.Parents), want)
	trashChildrenCheckWork(t, "explain", explain, len(p.Parents), want)
	trashChildrenCheckPlan(t, plan)
	return rows
}

// Accept genuine parent-key lookups by equality, ANY or closed range.
// Reject reading by time/notebook alone and filtering parents afterwards.
func trashChildrenCheckPlan(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var docs []struct{ Plan sourceExecutedPlan }
	if err := json.Unmarshal(raw, &docs); err != nil || len(docs) != 1 {
		t.Fatalf("decode actual trash plan: %v documents=%d", err, len(docs))
	}
	bound := func(n sourceExecutedPlan) bool {
		return n.IndexName == trashChildrenParentIndex && strings.Contains(n.IndexCond, "notebook_id =") && (strings.Contains(n.IndexCond, "parent_id =") || strings.Contains(n.IndexCond, "parent_id >=") && strings.Contains(n.IndexCond, "parent_id <="))
	}
	reads := 0
	var walk func(sourceExecutedPlan)
	walk = func(n sourceExecutedPlan) {
		if n.RelationName == "nodes" {
			reads++
			valid := (n.NodeType == "Index Scan" || n.NodeType == "Index Only Scan") && bound(n)
			if n.NodeType == "Bitmap Heap Scan" && len(n.Plans) == 1 {
				valid = n.Plans[0].NodeType == "Bitmap Index Scan" && bound(n.Plans[0])
			}
			if !valid {
				t.Fatalf("actual nodes access is not bounded by notebook and parent keys: %+v", n)
			}
		}
		for _, child := range n.Plans {
			walk(child)
		}
	}
	walk(docs[0].Plan)
	if reads != 1 {
		t.Fatalf("actual nodes accesses=%d want1", reads)
	}
}

// sourceProbeExecute does not accept timestamptz. Keep this small typed
// extension local instead of changing the existing source-query helpers.
func trashChildrenExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	parts := make([]string, len(args))
	for i, arg := range args {
		switch value := arg.(type) {
		case uuid.UUID:
			parts[i] = "'" + value.String() + "'::uuid"
		case []uuid.UUID:
			ids := make([]string, len(value))
			for j, id := range value {
				ids[j] = "'" + id.String() + "'"
			}
			parts[i] = "ARRAY[" + strings.Join(ids, ",") + "]::uuid[]"
		case time.Time:
			parts[i] = "'" + value.UTC().Format(time.RFC3339Nano) + "'::timestamptz"
		default:
			t.Fatalf("unexpected TrashChildrenOfAll argument %T", arg)
		}
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(parts, ",") + ")"
}
