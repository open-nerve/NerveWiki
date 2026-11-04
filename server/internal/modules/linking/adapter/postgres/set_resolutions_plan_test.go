//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This is a PostgreSQL adapter fixture. Its committed app.Rebuild uses real
// parsing, Store, index lock and TxManager; cross-module notebook/content/path
// inputs are small fixtures. It is not the bootstrap Rebuild capacity test.
type setPlanInputs struct {
	notebook        uuid.UUID
	texts           map[uuid.UUID]string
	ids             []uuid.UUID
	locked, rekeyed bool
}

func (p *setPlanInputs) PageIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return slices.Clone(p.ids), nil
}
func (p *setPlanInputs) Content(_ context.Context, id uuid.UUID) (string, int, bool, error) {
	s, ok := p.texts[id]
	return s, 1, ok, nil
}
func (p *setPlanInputs) Rekey(context.Context, uuid.UUID) ([]app.Clash, error) {
	p.rekeyed = true
	return nil, nil
}
func (p *setPlanInputs) WorkspaceOf(_ context.Context, id uuid.UUID) (uuid.UUID, bool, error) {
	return lintID(252), id == p.notebook, nil
}
func (p *setPlanInputs) LockByID(_ context.Context, id uuid.UUID) (bool, error) {
	p.locked = id == p.notebook
	return p.locked, nil
}

type setPlanFixture struct {
	fixture
	before     []app.Link
	statistics map[string]int
	rebuilt    app.Rebuilt
}

func newSetPlanFixture(t *testing.T, state string, noise bool) setPlanFixture {
	t.Helper()
	f := newFixture(t)
	ingestPlanSetup(t, f)
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parser := markdownadapter.NewParser(md, markdown.NewBudget(8<<20, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))))
	ids := []uuid.UUID{{}, lintID(1), lintID(2), lintID(250)}
	texts := map[uuid.UUID]string{
		ids[0]: "---\nnote: plain\n---\n" + strings.Repeat("[[x]]\n", 4096),
		ids[1]: strings.Repeat("[[x]]\n", 4097),
		ids[2]: strings.Repeat("[[other]]\n", 10000), ids[3]: "",
	}
	counts := []int{4096, 4097, 10000, 0}
	for i, id := range ids {
		facts, err := parser.Facts(t.Context(), texts[id])
		if err != nil || len(facts.Links) != counts[i] {
			t.Fatalf("real parsed fixture %s links=%d err=%v", id, len(facts.Links), err)
		}
		if err := f.s.AddPage(t.Context(), app.Page{ID: id, NotebookID: f.eng, Revision: 1}, facts); err != nil {
			t.Fatal(err)
		}
	}
	if noise {
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
SELECT ('20000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,200000) k`, f.ops)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous)
SELECT node_id,0,1,notebook_id,'wikilink','foreign','foreign',false FROM indexed_pages WHERE notebook_id=$1`, f.ops)
	}
	var rebuilt app.Rebuilt
	switch state {
	case "adapter_no_statistics":
	case "adapter_analyzed", "adapter_app_rebuild_after_analyzed":
		if err := f.s.Analyze(t.Context()); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown fixture state %q", state)
	}
	if state == "adapter_app_rebuild_after_analyzed" {
		inputs := &setPlanInputs{notebook: f.eng, texts: texts, ids: ids}
		names := []string{"source0", "source1", "noise", "x"}
		pages := make(setIndexPages, len(ids))
		for i, id := range ids {
			pages[i] = domain.Node{ID: id, Path: []domain.Step{{ID: id, Key: names[i], Name: names[i]}}}
		}
		published := 0
		rebuild := app.Rebuild{Tx: f.tx, Index: app.Index{Store: f.s, Pages: pages, Publisher: setIndexPublisher(func(context.Context, app.LinksChanged) error { published++; return nil })}, Contents: inputs, Notebooks: inputs, Parser: parser}
		rebuilt, err = rebuild.Notebook(t.Context(), f.eng)
		if err != nil || rebuilt.Pages != 4 || rebuilt.Links != 18193 || rebuilt.Unresolved != 10000 || len(rebuilt.Clashes) != 0 || !inputs.locked || !inputs.rekeyed || published != 1 {
			t.Fatalf("real adapter app.Rebuild result=%+v err=%v locked=%t rekeyed=%t publish=%d", rebuilt, err, inputs.locked, inputs.rekeyed, published)
		}
		// Deliberately no VACUUM/ANALYZE: retain the committed rebuild's MVCC
		// versions and pre-rebuild statistics. This is not a pristine/cold state.
	}
	statistics := lintAssertStatistics(t, f, state != "adapter_no_statistics")
	before, err := f.s.Links(t.Context(), f.eng, domain.Reach{Sources: ids[:2]})
	if err != nil || len(before) != 8193 {
		t.Fatalf("real source rows=%d err=%v", len(before), err)
	}
	for _, l := range before {
		want := domain.Resolution{}
		if state == "adapter_app_rebuild_after_analyzed" {
			want.ID = lintID(250)
		}
		if l.Resolution != want {
			t.Fatalf("fixture baseline resolution %+v", l)
		}
	}
	return setPlanFixture{fixture: f, before: before, statistics: statistics, rebuilt: rebuilt}
}

// Copy the first and final real parameter groups before synchronous Exec
// returns and the bounded writer reuses them; never invent a hypothetical batch.
type setPlanCapture struct {
	base        setWriteTrace
	first, tail statement
	seen        int
	bad         string
}

func (tr *setPlanCapture) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(d.SQL, "-- name: SetResolutions ") {
		args := d.Args
		if len(args) > 0 {
			if _, ok := args[0].(pgx.QueryExecMode); ok {
				args = args[1:]
			}
		}
		tr.seen++
		if len(args) != 5 {
			tr.bad = "not five real arrays"
		} else if tr.seen == 1 || tr.seen == 33 {
			s := statement{sql: d.SQL}
			for _, arg := range args {
				switch v := arg.(type) {
				case []uuid.UUID:
					s.args = append(s.args, slices.Clone(v))
				case []int32:
					s.args = append(s.args, slices.Clone(v))
				case []bool:
					s.args = append(s.args, slices.Clone(v))
				default:
					tr.bad = fmt.Sprintf("unexpected real parameter %T", arg)
				}
			}
			if tr.seen == 1 {
				tr.first = s
			} else {
				tr.tail = s
			}
		}
	}
	return tr.base.TraceQueryStart(ctx, c, d)
}
func (tr *setPlanCapture) TraceQueryEnd(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryEndData) {
	tr.base.TraceQueryEnd(ctx, c, d)
}
func setPlanParameters(t *testing.T) (statement, statement) {
	t.Helper()
	f := newSetPlanFixture(t, "adapter_no_statistics", false)
	tr := &setPlanCapture{}
	pool := lintPool(t, f.fixture, tr, 1)
	store := postgresadapter.New(pool)
	manager := postgres.NewTxManager(pool, time.Second)
	desired := slices.Clone(f.before)
	for i := range desired {
		switch i % 4 {
		case 0:
			desired[i].Resolution = domain.Resolution{}
		case 1:
			desired[i].Resolution = domain.Resolution{ID: lintID(250)}
		case 2:
			desired[i].Resolution = domain.Resolution{ID: lintID(250), Ambiguous: true}
		case 3:
			desired[i].Resolution = domain.Resolution{ID: lintID(251), Asset: true}
		}
	}
	desired[len(desired)-1].Resolution = domain.Resolution{ID: lintID(251), Asset: true} // Tail changes every baseline state.
	cause := errors.New("captured actual complete Set; roll it back")
	var pid uint32
	err := manager.WithinTx(t.Context(), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		pid = tx.Conn().PgConn().PID()
		if err := store.Lock(ctx, f.eng); err != nil {
			return err
		}
		if err := store.SetResolutions(ctx, desired); err != nil {
			return err
		}
		return cause
	})
	if !errors.Is(err, cause) || tr.bad != "" || tr.seen != 33 || tr.first.sql != tr.tail.sql {
		t.Fatalf("real parameter capture=%d %s err=%v", tr.seen, tr.bad, err)
	}
	observer := setWriteFixture{fixture: f.fixture, trace: &tr.base}
	lengths := setWrite256Lengths(32, 1)
	for i, c := range observer.assertCalls(t, desired, lengths, pid) {
		if c.Err != nil || c.Rows != int64(lengths[i]) {
			t.Fatalf("capture real command=%+v", c)
		}
	}
	observer.assertRows(t, f.before)
	lintEvidence(t, "set-write-captured", map[string]any{"sql": tr.first.sql, "first_parameters": tr.first.args, "tail_parameters": tr.tail.args, "complete_calls": tr.base.snapshot()})
	return tr.first, tr.tail
}

type setPlanNode struct {
	Type      string        `json:"Node Type"`
	Relation  string        `json:"Relation Name"`
	Index     string        `json:"Index Name"`
	Rows      float64       `json:"Actual Rows"`
	Loops     float64       `json:"Actual Loops"`
	Removed   float64       `json:"Rows Removed by Filter"`
	Rechecked float64       `json:"Rows Removed by Index Recheck"`
	Plans     []setPlanNode `json:"Plans"`
}

func (n setPlanNode) reads() []setPlanNode {
	var out []setPlanNode
	if n.Relation == "page_links" && strings.Contains(n.Type, "Scan") {
		out = append(out, n)
	}
	for _, child := range n.Plans {
		out = append(out, child.reads()...)
	}
	return out
}

type setPlanEvidence struct {
	Case, State, Operation, Mode, SQL, ExecutedSQLSHA256 string
	ExecutedSQLBytes                                     int
	PID                                                  uint32
	ArgumentLengths                                      [5]int
	ParamOIDs                                            []uint32
	ParameterFormats                                     []int16
	ParametersBytes                                      []int
	ParameterEncodingAlloc, ExecutionGoAlloc             uint64
	WallNS                                               int64
	CommandTagRowsAffected                               *int64
	XactRowsUpdated                                      int64
	GenericPlans, CustomPlans                            int64
	Statistics                                           map[string]int
	Rebuilt                                              app.Rebuilt
	Settings                                             map[string]string
	PlanningWork, ExecutionWork                          map[string]lintWork
	Plan                                                 json.RawMessage
	Reads                                                []setPlanNode
}

// Exact observed SQL identity/mode/PID is checked by a narrow query tracer.
type setPlanExecutionTrace struct {
	sql   string
	calls int
	pid   uint32
	mode  pgx.QueryExecMode
}

func (tr *setPlanExecutionTrace) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if d.SQL == tr.sql {
		tr.calls++
		tr.pid = c.PgConn().PID()
		if len(d.Args) > 0 {
			tr.mode, _ = d.Args[0].(pgx.QueryExecMode)
		}
	}
	return ctx
}
func (*setPlanExecutionTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func setPlanPostState(t *testing.T, ctx context.Context, f setPlanFixture, s statement, changed bool) {
	t.Helper()
	want := slices.Clone(f.before)
	if changed {
		sources := s.args[0].([]uuid.UUID)
		starts := s.args[1].([]int32)
		targets := s.args[2].([]uuid.UUID)
		ambiguous := s.args[3].([]bool)
		assets := s.args[4].([]bool)
		at := 0
		for i := range want {
			if at < len(sources) && want[i].SourceID == sources[at] && want[i].Start == int(starts[at]) {
				want[i].Resolution = domain.Resolution{ID: targets[at], Ambiguous: ambiguous[at], Asset: assets[at]}
				at++
			}
		}
		if at != len(sources) {
			t.Fatalf("actual captured keys missing from fresh fixture: %d/%d", at, len(sources))
		}
	}
	got, err := f.s.Links(ctx, f.eng, domain.Reach{Sources: []uuid.UUID{{}, lintID(1)}})
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("whole real post-state got=%d want=%d err=%v", len(got), len(want), err)
	}
	var same, foreign, bad int
	err = postgres.DB(ctx, f.pool).QueryRow(ctx, `SELECT count(*) FILTER(WHERE notebook_id=$1),count(*) FILTER(WHERE notebook_id=$2),count(*) FILTER(WHERE resolved_id IS NOT NULL OR ambiguous OR resolved_asset)
FROM page_links WHERE source_id<>ALL($3::uuid[])`, f.eng, f.ops, []uuid.UUID{{}, lintID(1)}).Scan(&same, &foreign, &bad)
	if err != nil || same != 10000 || foreign != 200000 || bad != 0 {
		t.Fatalf("unrelated rows changed: same=%d foreign=%d bad=%d err=%v", same, foreign, bad, err)
	}
}

func setPlanRun(t *testing.T, state, mode, operation string, s statement) {
	t.Helper()
	// Each operation owns a new database. Direct Exec does not create dead
	// versions in the EXPLAIN fixture; only the named rebuild state has them.
	f := newSetPlanFixture(t, state, true)
	tr := &setPlanExecutionTrace{}
	pool := lintPool(t, f.fixture, tr, 1)
	f.s, f.tx = postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second)
	ev := setPlanEvidence{Case: t.Name(), State: state, Mode: mode, Operation: operation, SQL: s.sql, Statistics: f.statistics, Rebuilt: f.rebuilt, Settings: map[string]string{}}
	n := len(s.args[0].([]uuid.UUID))
	ev.ArgumentLengths = [5]int{n, len(s.args[1].([]int32)), len(s.args[2].([]uuid.UUID)), len(s.args[3].([]bool)), len(s.args[4].([]bool))}
	if ev.ArgumentLengths != [5]int{n, n, n, n, n} || (n != 256 && n != 1) {
		t.Fatalf("captured parameter shape=%v", ev.ArgumentLengths)
	}
	cause := errors.New("plan observation must roll back all original UPDATE effects")
	err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
		tx, _ := postgres.TxFrom(ctx)
		ev.PID = tx.Conn().PgConn().PID()
		if err := f.s.Lock(ctx, f.eng); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode="+mode); err != nil {
			return err
		}
		for _, name := range []string{"track_counts", "enable_seqscan", "enable_indexscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "plan_cache_mode"} {
			var value string
			if err := tx.QueryRow(ctx, "SHOW "+name).Scan(&value); err != nil {
				return err
			}
			ev.Settings[name] = value
			want := "on"
			if name == "plan_cache_mode" {
				want = mode
			}
			if value != want {
				return fmt.Errorf("planner setting %s=%s, want%s", name, value, want)
			}
		}
		desc, err := tx.Conn().PgConn().Prepare(ctx, "", s.sql, nil)
		if err != nil {
			return err
		}
		expectedOIDs := []uint32{pgtype.UUIDArrayOID, pgtype.Int4ArrayOID, pgtype.UUIDArrayOID, pgtype.BoolArrayOID, pgtype.BoolArrayOID}
		if !slices.Equal(desc.ParamOIDs, expectedOIDs) {
			return fmt.Errorf("actual described parameter OIDs=%v want%v", desc.ParamOIDs, expectedOIDs)
		}
		ev.ParamOIDs = slices.Clone(desc.ParamOIDs)
		ev.ParameterFormats = make([]int16, len(desc.ParamOIDs))
		for i := range ev.ParameterFormats {
			ev.ParameterFormats[i] = pgtype.BinaryFormatCode
		}
		if !slices.Equal(ev.ParameterFormats, []int16{1, 1, 1, 1, 1}) {
			return fmt.Errorf("parameter formats=%v, want five binary formats", ev.ParameterFormats)
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for i, arg := range s.args {
			encoded, err := tx.Conn().TypeMap().Encode(ev.ParamOIDs[i], ev.ParameterFormats[i], arg, nil)
			if err != nil {
				return err
			}
			ev.ParametersBytes = append(ev.ParametersBytes, len(encoded))
		}
		runtime.ReadMemStats(&after)
		ev.ParameterEncodingAlloc = after.TotalAlloc - before.TotalAlloc
		sql, args := s.sql, s.args
		if mode == "force_generic_plan" {
			if _, err := tx.Conn().Prepare(ctx, "set_write_plan", s.sql); err != nil {
				return err
			}
			defer func() { _ = tx.Conn().Deallocate(context.WithoutCancel(ctx), "set_write_plan") }()
			sql, args = lintExecute(t, "set_write_plan", s.args), nil
		}
		// Observe planner relation probes separately, without executing UPDATE.
		planningBefore := lintRelationWork(t, tx)
		var planning string
		if err := postgres.Planned(tx).QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&planning); err != nil {
			return err
		}
		ev.PlanningWork = lintWorkDelta(planningBefore, lintRelationWork(t, tx))
		var updatedBefore, updatedAfter int64
		if err := tx.QueryRow(ctx, "SELECT pg_stat_get_xact_tuples_updated('page_links'::regclass)").Scan(&updatedBefore); err != nil {
			return err
		}
		workBefore := lintRelationWork(t, tx)
		if operation == "explain" {
			sql = "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) " + sql
		}
		tr.sql = sql
		ev.ExecutedSQLBytes = len(sql)
		ev.ExecutedSQLSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(sql)))
		runtime.ReadMemStats(&before)
		started := time.Now()
		if operation == "exec" {
			tag, err := postgres.Planned(tx).Exec(ctx, sql, args...)
			if err != nil {
				return err
			}
			rows := tag.RowsAffected()
			ev.CommandTagRowsAffected = &rows
		} else {
			var raw string
			if err := postgres.Planned(tx).QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
				return err
			}
			ev.Plan = json.RawMessage(raw)
		}
		ev.WallNS = time.Since(started).Nanoseconds()
		runtime.ReadMemStats(&after)
		ev.ExecutionGoAlloc = after.TotalAlloc - before.TotalAlloc
		ev.ExecutionWork = lintWorkDelta(workBefore, lintRelationWork(t, tx))
		if err := tx.QueryRow(ctx, "SELECT pg_stat_get_xact_tuples_updated('page_links'::regclass)").Scan(&updatedAfter); err != nil {
			return err
		}
		ev.XactRowsUpdated = updatedAfter - updatedBefore
		if mode == "force_generic_plan" {
			if err := tx.QueryRow(ctx, "SELECT generic_plans,custom_plans FROM pg_prepared_statements WHERE name='set_write_plan'").Scan(&ev.GenericPlans, &ev.CustomPlans); err != nil {
				return err
			}
		}
		if operation == "explain" {
			var decoded []struct{ Plan setPlanNode }
			if err := json.Unmarshal(ev.Plan, &decoded); err != nil {
				return fmt.Errorf("actual UPDATE plan: %s: %w", ev.Plan, err)
			}
			if len(decoded) != 1 {
				return fmt.Errorf("actual UPDATE plan: %s: got %d plans, want 1", ev.Plan, len(decoded))
			}
			ev.Reads = decoded[0].Plan.reads()
		}
		lintEvidence(t, "set-write-plan", ev)
		if tr.calls != 1 || tr.pid != ev.PID || tr.mode != pgx.QueryExecModeDescribeExec {
			t.Errorf("actual SQL mode/PID identity: calls%d pid%d/%d mode%v", tr.calls, tr.pid, ev.PID, tr.mode)
		}
		if ev.XactRowsUpdated != int64(n) || operation == "exec" && (ev.CommandTagRowsAffected == nil || *ev.CommandTagRowsAffected != int64(n)) {
			t.Errorf("UPDATE count: tag=%v xact=%d expected%d", ev.CommandTagRowsAffected, ev.XactRowsUpdated, n)
		}
		if mode == "force_generic_plan" && (ev.GenericPlans < 1 || ev.CustomPlans != 0) {
			t.Errorf("not a real executed generic UPDATE: generic%d custom%d", ev.GenericPlans, ev.CustomPlans)
		}
		// Work may include the declared committed rebuild versions; four reads
		// per requested key is conservative for this fixture's insert/delete/
		// insert/resolve history. It cannot admit the 210000 unrelated rows.
		for _, rel := range []string{"page_links", "page_links_pkey"} {
			actual, planning := ev.ExecutionWork[rel], ev.PlanningWork[rel]
			if mode == "force_generic_plan" {
				planning = lintWork{}
			} // Prepared EXECUTE does not replan.
			returned := max(int64(0), actual.Returned-planning.Returned)
			fetched := max(int64(0), actual.Fetched-planning.Fetched)
			if returned > int64(4*n) || fetched > int64(4*n) || rel == "page_links" && actual.Scans > planning.Scans {
				t.Errorf("UPDATE work reached unrelated rows: %s actual=%+v planning=%+v limit%d", rel, actual, planning, 4*n)
			}
		}
		if operation == "explain" {
			var inspected float64
			for _, read := range ev.Reads {
				inspected += (read.Rows + read.Removed + read.Rechecked) * read.Loops
			}
			if len(ev.Reads) == 0 || inspected > float64(4*n) {
				t.Errorf("actual scan rows/loops reached noise: %+v, sum%g limit%d", ev.Reads, inspected, 4*n)
			}
		}
		setPlanPostState(t, ctx, f, s, true)
		return cause
	})
	if !errors.Is(err, cause) {
		t.Fatal(err)
	}
	setPlanPostState(t, t.Context(), f, s, false)
}

// Thirty-six leaves: three explicit adapter states, three planner modes,
// actual first256/final1 parameters of a complete 8193-row write, and isolated
// Exec/EXPLAIN databases.
func TestSetResolutionWritePlans(t *testing.T) {
	first, tail := setPlanParameters(t)
	for _, state := range []string{"adapter_no_statistics", "adapter_analyzed", "adapter_app_rebuild_after_analyzed"} {
		t.Run(state, func(t *testing.T) {
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					for _, shape := range []struct {
						name string
						s    statement
					}{{"first256", first}, {"tail1", tail}} {
						t.Run(shape.name, func(t *testing.T) {
							for _, operation := range []string{"exec", "explain"} {
								t.Run(operation, func(t *testing.T) { setPlanRun(t, state, mode, operation, shape.s) })
							}
						})
					}
				})
			}
		})
	}
}
