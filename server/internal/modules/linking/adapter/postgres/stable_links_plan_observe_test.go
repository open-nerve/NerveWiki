//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

const stablePlanInspect = "InspectPageLinksForRetention"
const stablePlanDelete = "DeleteMissingPageLinks"
const stablePlanWrite = "WritePageLinksRetainingEqualPrefiltered"

type stablePlanCall struct {
	Name, SQL, Kind, Error, ArgumentSHA, PreparedName string
	Args                                              []any
	ArgumentTypes, LeadingOptions                     []string
	PID                                               uint32
	DefaultMode                                       pgx.QueryExecMode
	StatementCapacity, DescriptionCapacity            int
	RowsAffected                                      int64
}
type stablePlanTrace struct {
	mu       sync.Mutex
	active   bool
	calls    []stablePlanCall
	ends     []string
	prepares map[string]string
}
type stablePlanTraceKey struct{}

func stablePlanCloneArg(arg any) (any, error) {
	switch v := arg.(type) {
	case uuid.UUID, string, int32, int64, bool, nil:
		return v, nil
	case []uuid.UUID:
		return slices.Clone(v), nil
	case []int32:
		return slices.Clone(v), nil
	case []string:
		return slices.Clone(v), nil
	case []bool:
		return slices.Clone(v), nil
	default:
		return nil, fmt.Errorf("uncaptured actual argument type %T", arg)
	}
}

func (tr *stablePlanTrace) TraceQueryStart(ctx context.Context, c *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	plain := strings.ToLower(strings.TrimSpace(d.SQL))
	if plain == "commit" || plain == "rollback" {
		tr.ends = append(tr.ends, plain)
	}
	if !tr.active {
		return ctx
	}
	fields := strings.Fields(strings.SplitN(d.SQL, "\n", 2)[0])
	if len(fields) != 4 || fields[0] != "--" || fields[1] != "name:" {
		return ctx
	}
	cfg := c.Config()
	call := stablePlanCall{Name: fields[2], Kind: fields[3], SQL: d.SQL, PID: c.PgConn().PID(), DefaultMode: cfg.DefaultQueryExecMode, StatementCapacity: cfg.StatementCacheCapacity, DescriptionCapacity: cfg.DescriptionCacheCapacity}
	leading := true
	for _, arg := range d.Args {
		switch option := arg.(type) {
		case pgx.QueryExecMode:
			call.LeadingOptions = append(call.LeadingOptions, fmt.Sprintf("leading=%t mode=%v", leading, option))
			continue
		case pgx.QueryRewriter:
			call.LeadingOptions = append(call.LeadingOptions, fmt.Sprintf("leading=%t rewriter=%T", leading, option))
			continue
		}
		leading = false
		copy, err := stablePlanCloneArg(arg)
		if err != nil {
			call.Error = err.Error()
		}
		call.Args = append(call.Args, copy)
		call.ArgumentTypes = append(call.ArgumentTypes, fmt.Sprintf("%T", arg))
	}
	encoded, err := json.Marshal(call.Args)
	if err != nil {
		call.Error = err.Error()
	}
	call.ArgumentSHA = fmt.Sprintf("%x", sha256.Sum256(encoded))
	i := len(tr.calls)
	tr.calls = append(tr.calls, call)
	return context.WithValue(ctx, stablePlanTraceKey{}, i)
}
func (tr *stablePlanTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	i, ok := ctx.Value(stablePlanTraceKey{}).(int)
	if !ok {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.calls[i].RowsAffected = d.CommandTag.RowsAffected()
	if d.Err != nil {
		tr.calls[i].Error = d.Err.Error()
	}
}
func (tr *stablePlanTrace) TracePrepareStart(ctx context.Context, _ *pgx.Conn, d pgx.TracePrepareStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.prepares == nil {
		tr.prepares = map[string]string{}
	}
	tr.prepares[d.SQL] = d.Name
	return ctx
}
func (*stablePlanTrace) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}
func (tr *stablePlanTrace) begin() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.active = true
	tr.calls = nil
	tr.ends = nil
}
func (tr *stablePlanTrace) end() []stablePlanCall {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.active = false
	return slices.Clone(tr.calls)
}
func (tr *stablePlanTrace) terminal() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.ends)
}
func (tr *stablePlanTrace) prepareName(sql string) string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.prepares[sql]
}

func stablePlanAssertCapture(t *testing.T, calls []stablePlanCall, pid uint32) {
	t.Helper()
	want := []string{stablePlanInspect, stablePlanDelete, "DeleteAliasesOf", "DeleteTagsOf", "DeletePropertiesOf", "DeleteIndexedPages", "InsertIndexedPage", stablePlanWrite, "InsertTags", "InsertProperties", "InsertAliases"}
	for _, name := range []string{stablePlanInspect, stablePlanDelete, stablePlanWrite} {
		if !slices.ContainsFunc(calls, func(c stablePlanCall) bool { return c.Name == name }) {
			t.Fatalf("stable physical capture: missing %s from real ReplacePage", name)
		}
	}
	var names []string
	for _, c := range calls {
		names = append(names, c.Name)
		if c.Error != "" || c.PID != pid || len(c.LeadingOptions) != 0 {
			t.Fatalf("stable actual query identity/mode/error: %+v", c)
		}
		if c.DefaultMode != pgx.QueryExecModeCacheStatement || c.StatementCapacity <= 0 {
			t.Fatalf("stable actual fixture is not the default named path: mode=%v statement_capacity=%d", c.DefaultMode, c.StatementCapacity)
		}
	}
	if !slices.Equal(names, want) {
		t.Fatalf("stable full actual replacement sequence=%v want=%v", names, want)
	}
}

// Bound to native sqlc v1.31.1 retention.sql.go SHA256 5fe9fcba927124f2...;
// runtime Describe still checks the original three/two/thirteen ordered OIDs.
// Classifications come from the fixture's literal remove/change/new branches,
// never from product equality or from the observed SQL parameters.
func stablePlanAssertParameters(t *testing.T, c stablePlanCall, s stablePlanSpec) {
	t.Helper()
	var starts, ends, missing []int32
	var kinds, properties, targets, anchors, displays, keys, alts []string
	var frontmatter, aliases []bool
	if s.M > 0 {
		// A nonempty full input owns all eleven arrays, even when every row
		// is retained and compaction leaves a non-nil empty view.
		starts, ends = make([]int32, 0, len(s.ToWrite)), make([]int32, 0, len(s.ToWrite))
		kinds, properties = make([]string, 0, len(s.ToWrite)), make([]string, 0, len(s.ToWrite))
		targets, anchors = make([]string, 0, len(s.ToWrite)), make([]string, 0, len(s.ToWrite))
		displays, keys, alts = make([]string, 0, len(s.ToWrite)), make([]string, 0, len(s.ToWrite)), make([]string, 0, len(s.ToWrite))
		frontmatter, aliases = make([]bool, 0, len(s.ToWrite)), make([]bool, 0, len(s.ToWrite))
	}
	for _, r := range s.Removed {
		missing = append(missing, r.Start)
	}
	for _, r := range s.ToWrite {
		starts = append(starts, r.Start)
		ends = append(ends, r.End)
		kinds = append(kinds, r.Kind)
		properties = append(properties, r.Property.String)
		frontmatter = append(frontmatter, r.Property.Valid)
		targets = append(targets, r.Target)
		anchors = append(anchors, r.Anchor.String)
		displays = append(displays, r.Display.String)
		keys = append(keys, r.Key.String)
		alts = append(alts, r.Alt.String)
		aliases = append(aliases, r.Aliases)
	}
	var want []any
	switch c.Name {
	case stablePlanInspect:
		want = []any{s.ID, stablePlanID(900001), int32(2)}
	case stablePlanDelete:
		want = []any{s.ID, missing}
	case stablePlanWrite:
		want = []any{s.ID, stablePlanID(900001), starts, ends, kinds, properties, frontmatter, targets, anchors, displays, keys, alts, aliases}
	default:
		return
	}
	if !reflect.DeepEqual(c.Args, want) {
		t.Fatalf("stable complete actual parameter values/order differ for %s: types=%v arg_count=%d expected=%d", c.Name, c.ArgumentTypes, len(c.Args), len(want))
	}
}

type stablePlanCount struct {
	Name            string
	Generic, Custom int64
	ParameterTypes  string
}

func stablePlanCounter(t *testing.T, ctx context.Context, tx pgx.Tx, sql string) stablePlanCount {
	t.Helper()
	rows, err := tx.Query(ctx, `SELECT name,generic_plans,custom_plans,parameter_types::text FROM pg_prepared_statements WHERE statement=$1 ORDER BY name`, sql)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out stablePlanCount
	n := 0
	for rows.Next() {
		if err := rows.Scan(&out.Name, &out.Generic, &out.Custom, &out.ParameterTypes); err != nil {
			t.Fatal(err)
		}
		n++
	}
	if err := rows.Err(); err != nil || n > 1 {
		t.Fatalf("stable exact SQL statement identity: matches=%d err=%v", n, err)
	}
	return out
}
func stablePlanCountDelta(t *testing.T, before, after stablePlanCount, mode string) {
	t.Helper()
	if after.Name == "" || (before.Name != "" && before.Name != after.Name) {
		t.Fatalf("stable named plan identity changed: before=%+v after=%+v", before, after)
	}
	g, c := after.Generic-before.Generic, after.Custom-before.Custom
	if g+c != 1 || g < 0 || c < 0 || (mode == "force_generic_plan" && g != 1) || (mode == "force_custom_plan" && c != 1) {
		t.Fatalf("stable actual plan choices: mode=%s before=%+v after=%+v", mode, before, after)
	}
}

type stablePlanWork struct {
	Owner, Kind                                                        string
	Scans, Returned, Fetched, Blocks, Hits, Inserted, Updated, Deleted int64
}

func stablePlanRelationWork(t *testing.T, ctx context.Context, tx pgx.Tx) map[string]stablePlanWork {
	t.Helper()
	rows, err := tx.Query(ctx, `WITH bases AS (
 SELECT oid,relname,reltoastrelid FROM pg_class WHERE oid=ANY(ARRAY['indexed_pages'::regclass,'page_links'::regclass,'page_tags'::regclass,'page_properties'::regclass,'page_aliases'::regclass])),
 tables AS (SELECT oid,relname::text AS owner FROM bases UNION ALL SELECT reltoastrelid,relname::text||'.toast' FROM bases WHERE reltoastrelid<>0),
 relations AS (SELECT oid,owner FROM tables UNION ALL SELECT i.indexrelid,t.owner FROM pg_index i JOIN tables t ON t.oid=i.indrelid)
 SELECT c.relname,r.owner,c.relkind::text,pg_stat_get_xact_numscans(c.oid),pg_stat_get_xact_tuples_returned(c.oid),pg_stat_get_xact_tuples_fetched(c.oid),pg_stat_get_xact_blocks_fetched(c.oid),pg_stat_get_xact_blocks_hit(c.oid),pg_stat_get_xact_tuples_inserted(c.oid),pg_stat_get_xact_tuples_updated(c.oid),pg_stat_get_xact_tuples_deleted(c.oid)
 FROM relations r JOIN pg_class c ON c.oid=r.oid ORDER BY c.relname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]stablePlanWork{}
	for rows.Next() {
		var name string
		var w stablePlanWork
		if err := rows.Scan(&name, &w.Owner, &w.Kind, &w.Scans, &w.Returned, &w.Fetched, &w.Blocks, &w.Hits, &w.Inserted, &w.Updated, &w.Deleted); err != nil {
			t.Fatal(err)
		}
		out[name] = w
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func stablePlanWorkDelta(before, after map[string]stablePlanWork) map[string]stablePlanWork {
	out := map[string]stablePlanWork{}
	for name, v := range after {
		b := before[name]
		out[name] = stablePlanWork{Owner: v.Owner, Kind: v.Kind, Scans: v.Scans - b.Scans, Returned: v.Returned - b.Returned, Fetched: v.Fetched - b.Fetched, Blocks: v.Blocks - b.Blocks, Hits: v.Hits - b.Hits, Inserted: v.Inserted - b.Inserted, Updated: v.Updated - b.Updated, Deleted: v.Deleted - b.Deleted}
	}
	return out
}

type stablePlanEncoding struct {
	OIDs              []uint32
	DiagnosticFormats []int16
	Bytes             []int
	SQLNull           []bool
	GoAlloc           uint64
	InputWireFormats  string
}

func stablePlanEncodingOf(t *testing.T, ctx context.Context, tx pgx.Tx, c stablePlanCall) stablePlanEncoding {
	t.Helper()
	desc, err := tx.Conn().PgConn().Prepare(ctx, "", c.SQL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(desc.ParamOIDs) != len(c.Args) {
		t.Fatalf("stable described parameter count %d/%d", len(desc.ParamOIDs), len(c.Args))
	}
	var want []uint32
	switch c.Name {
	case stablePlanInspect:
		want = []uint32{pgtype.UUIDOID, pgtype.UUIDOID, pgtype.Int4OID}
	case stablePlanDelete:
		want = []uint32{pgtype.UUIDOID, pgtype.Int4ArrayOID}
	case stablePlanWrite:
		want = []uint32{pgtype.UUIDOID, pgtype.UUIDOID, pgtype.Int4ArrayOID, pgtype.Int4ArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.TextArrayOID, pgtype.BoolArrayOID}
	default:
		t.Fatalf("unexpected encoding subject %s", c.Name)
	}
	if !slices.Equal(desc.ParamOIDs, want) {
		t.Fatalf("stable actual ordered OIDs=%v want=%v", desc.ParamOIDs, want)
	}
	out := stablePlanEncoding{OIDs: slices.Clone(desc.ParamOIDs), InputWireFormats: "NOT_OBSERVED: local TypeMap.Encode is not Bind-wire observation"}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i, arg := range c.Args {
		encoded, err := tx.Conn().TypeMap().Encode(desc.ParamOIDs[i], pgtype.BinaryFormatCode, arg, nil)
		if err != nil {
			t.Fatal(err)
		}
		out.DiagnosticFormats = append(out.DiagnosticFormats, pgtype.BinaryFormatCode)
		out.Bytes = append(out.Bytes, len(encoded))
		out.SQLNull = append(out.SQLNull, encoded == nil)
	}
	runtime.ReadMemStats(&after)
	out.GoAlloc = after.TotalAlloc - before.TotalAlloc
	return out
}

// Reuse the existing literal formatter, but preserve typed nil arrays as SQL
// NULL. Its ARRAY[] formatter alone would silently change captured nil inputs.
func stablePlanExecute(t *testing.T, name string, args []any) string {
	t.Helper()
	literals := make([]string, len(args))
	for i, arg := range args {
		var null string
		switch v := arg.(type) {
		case []int32:
			if v == nil {
				null = "NULL::integer[]"
			}
		case []string:
			if v == nil {
				null = "NULL::text[]"
			}
		case []bool:
			if v == nil {
				null = "NULL::boolean[]"
			}
		case []uuid.UUID:
			if v == nil {
				null = "NULL::uuid[]"
			}
		}
		if null != "" {
			literals[i] = null
			continue
		}
		one := lintExecute(t, "stable_parameter", []any{arg})
		value, ok := strings.CutPrefix(one, "EXECUTE \"stable_parameter\" (")
		if !ok || !strings.HasSuffix(value, ")") {
			t.Fatalf("existing literal formatter: %q", one)
		}
		literals[i] = strings.TrimSuffix(value, ")")
	}
	return "EXECUTE " + pgx.Identifier{name}.Sanitize() + " (" + strings.Join(literals, ",") + ")"
}

// Targets are derived by this observer from the complete thirteen-column
// stream. No UUID array is returned by Inspect. Bytes below are DataRow
// fields/frames, not an estimated array payload or a PostgreSQL CPU measure.
type stablePlanD struct {
	CanRetain                      bool
	Targets                        []uuid.UUID
	OIDs                           []uint32
	Formats                        []int16
	Rows, OldRows, Sentinels, EOFs int
	CloseCalls                     int
	Closed                         bool
	FieldBytes                     []int64
	FieldNulls                     []int
	DataRowBytes                   int64
}

func stablePlanReadD(t *testing.T, ctx context.Context, tx pgx.Tx, sql string, s stablePlanSpec, args ...any) (out stablePlanD) {
	t.Helper()
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		rows.Close()
		out.CloseCalls++
		out.Closed = !tx.Conn().PgConn().IsBusy()
		if !out.Closed {
			t.Fatal("stable complete Inspect did not release its actual connection")
		}
	}()
	wantOIDs := []uint32{pgtype.BoolOID, pgtype.Int4OID, pgtype.Int4OID, pgtype.UUIDOID,
		pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID, pgtype.TextOID,
		pgtype.TextOID, pgtype.TextOID, pgtype.BoolOID, pgtype.UUIDOID}
	fields := rows.FieldDescriptions()
	for _, f := range fields {
		out.OIDs = append(out.OIDs, f.DataTypeOID)
		out.Formats = append(out.Formats, f.Format)
		if f.Format != tx.Conn().TypeMap().FormatCodeForOID(f.DataTypeOID) {
			t.Fatalf("stable actual Inspect field format %s=%d", f.Name, f.Format)
		}
	}
	if !slices.Equal(out.OIDs, wantOIDs) {
		t.Fatalf("stable Inspect actual thirteen result OIDs=%v want=%v", out.OIDs, wantOIDs)
	}
	out.CanRetain = true
	out.FieldBytes, out.FieldNulls = make([]int64, 13), make([]int, 13)
	seen := map[uuid.UUID]bool{}
	for {
		if !rows.Next() {
			out.EOFs++
			break
		}
		raw := rows.RawValues()
		if len(raw) != 13 {
			t.Fatalf("stable Inspect DataRow fields=%d", len(raw))
		}
		out.Rows++
		out.DataRowBytes += 7 + 4*int64(len(raw)) // type, length, field count, field lengths
		for i, value := range raw {
			out.FieldBytes[i] += int64(len(value))
			out.DataRowBytes += int64(len(value))
			if value == nil {
				out.FieldNulls[i]++
			}
		}
		var allowed bool
		var start *int32
		r := stablePlanRow{Source: s.ID}
		if err := rows.Scan(&allowed, &start, &r.End, &r.Notebook, &r.Kind, &r.Property,
			&r.Target, &r.Anchor, &r.Display, &r.Key, &r.Alt, &r.Aliases, &r.Resolved); err != nil {
			t.Fatal(err)
		}
		out.CanRetain = out.CanRetain && allowed
		if start == nil {
			out.Sentinels++
			if len(s.Old) != 0 || !allowed || r != (stablePlanRow{Source: s.ID}) {
				t.Fatalf("stable empty Inspect sentinel=%+v allowed=%t old=%d", r, allowed, len(s.Old))
			}
			continue
		}
		r.Start = *start
		if out.OldRows >= len(s.Old) {
			t.Fatalf("stable Inspect returned an extra old row: %+v", r)
		}
		want := s.Old[out.OldRows]
		// These two derived columns are intentionally absent from Inspect.
		want.Ambiguous, want.Asset = false, false
		if !allowed || r != want {
			t.Fatalf("stable Inspect complete old row %d=%+v want=%+v allowed=%t", out.OldRows, r, want, allowed)
		}
		out.OldRows++
		if r.Resolved.Valid {
			seen[uuid.UUID(r.Resolved.Bytes)] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantSentinels := 0
	if len(s.Old) == 0 {
		wantSentinels = 1
	}
	if out.Rows != max(1, s.N) || out.OldRows != s.N || out.Sentinels != wantSentinels || out.EOFs != 1 {
		t.Fatalf("stable Inspect complete stream rows/old/sentinel/EOF=%d/%d/%d/%d want=%d/%d/%d/1", out.Rows, out.OldRows, out.Sentinels, out.EOFs, max(1, s.N), s.N, wantSentinels)
	}
	for id := range seen {
		out.Targets = append(out.Targets, id)
	}
	slices.SortFunc(out.Targets, uuid.UUID.Compare)
	return out
}

func stablePlanWantD(s stablePlanSpec) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	for _, r := range s.Old {
		if r.Resolved.Valid {
			seen[uuid.UUID(r.Resolved.Bytes)] = true
		}
	}
	out := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

type stablePlanNode struct {
	Type                        string           `json:"Node Type"`
	Relation                    string           `json:"Relation Name"`
	Index                       string           `json:"Index Name"`
	IndexCond                   string           `json:"Index Cond"`
	Parent                      string           `json:"Parent Relationship"`
	Subplan                     string           `json:"Subplan Name"`
	CTE                         string           `json:"CTE Name"`
	Rows                        float64          `json:"Actual Rows"`
	Loops                       float64          `json:"Actual Loops"`
	Removed                     float64          `json:"Rows Removed by Filter"`
	JoinRemoved                 float64          `json:"Rows Removed by Join Filter"`
	Rechecked                   float64          `json:"Rows Removed by Index Recheck"`
	Conflicts                   *float64         `json:"Conflicting Tuples"`
	Inserted                    *float64         `json:"Tuples Inserted"`
	ConflictRemoved             *float64         `json:"Rows Removed by Conflict Filter"`
	RowsObserved, LoopsObserved bool             `json:"-"`
	Plans                       []stablePlanNode `json:"Plans"`
}

func (n *stablePlanNode) UnmarshalJSON(raw []byte) error {
	type plain stablePlanNode
	var value plain
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	*n = stablePlanNode(value)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	_, n.RowsObserved = fields["Actual Rows"]
	_, n.LoopsObserved = fields["Actual Loops"]
	return nil
}
func (n stablePlanNode) all() []stablePlanNode {
	out := []stablePlanNode{n}
	for _, child := range n.Plans {
		out = append(out, child.all()...)
	}
	return out
}
func stablePlanTree(t *testing.T, raw json.RawMessage) stablePlanNode {
	t.Helper()
	var plans []struct{ Plan stablePlanNode }
	if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("stable JSON plan: %s %v", raw, err)
	}
	return plans[0].Plan
}

type stablePlanEvidence struct {
	Case, Scope, State, Mode, Operation, Name, SQL, ArgumentSHA string
	Source                                                      uuid.UUID
	PID                                                         uint32
	SQLSHA                                                      string
	Encoding                                                    stablePlanEncoding
	Statistics                                                  map[string]int
	Settings                                                    map[string]string
	BeforeChoice, AfterChoice                                   stablePlanCount
	PlanningWork, ExecutionWork                                 map[string]stablePlanWork
	PlanningPlan                                                json.RawMessage
	PlanningObservation                                         string
	Plan                                                        json.RawMessage
	D                                                           *stablePlanD
	RowsAffected                                                *int64
	WallNS                                                      int64
	GoAlloc                                                     uint64
	VisibleOld, VisibleNew, Stable, Changed, Added, Deleted     int
	VersionLedger                                               map[string]int
	RawWorkAcceptance                                           string
}

func stablePlanSettings(t *testing.T, ctx context.Context, tx pgx.Tx, mode string) map[string]string {
	t.Helper()
	if !slices.Contains([]string{"auto", "force_custom_plan", "force_generic_plan"}, mode) {
		t.Fatalf("invalid fixed mode %s", mode)
	}
	if _, err := tx.Exec(ctx, "SET LOCAL plan_cache_mode="+mode); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, key := range []string{"server_version", "plan_cache_mode", "track_counts", "enable_seqscan", "enable_indexscan", "enable_indexonlyscan", "enable_bitmapscan", "enable_nestloop", "enable_hashjoin", "enable_mergejoin", "jit", "work_mem"} {
		var value string
		if err := tx.QueryRow(ctx, "SHOW "+key).Scan(&value); err != nil {
			t.Fatal(err)
		}
		out[key] = value
		if (strings.HasPrefix(key, "enable_") || key == "track_counts") && value != "on" {
			t.Fatalf("stable normal planner %s=%s", key, value)
		}
		if key == "plan_cache_mode" && value != mode {
			t.Fatalf("stable actual plan mode=%s", value)
		}
		if key == "jit" && value != "off" {
			t.Fatalf("stable pool original jit setting=%s", value)
		}
	}
	return out
}

func stablePlanObserve(t *testing.T, ctx context.Context, f *stablePlanFixture, s stablePlanSpec, c stablePlanCall, mode, operation, preparedName string, observePlanning bool) stablePlanEvidence {
	t.Helper()
	tx, ok := postgres.TxFrom(ctx)
	if !ok {
		t.Fatal("stable observation outside original Tx")
	}
	stablePlanAssertParameters(t, c, s)
	e := stablePlanEvidence{Case: t.Name(), Source: s.ID, Scope: "captured_SQL_kernel_replay", State: f.state, Mode: mode, Operation: operation, Name: c.Name, SQL: c.SQL, SQLSHA: fmt.Sprintf("%x", sha256.Sum256([]byte(c.SQL))), ArgumentSHA: c.ArgumentSHA, PID: tx.Conn().PgConn().PID(), Statistics: f.statistics, VisibleOld: s.N, VisibleNew: s.M, Stable: s.S, Changed: s.C, Added: s.I, Deleted: s.Z}
	e.Settings = stablePlanSettings(t, ctx, tx, mode)
	e.Encoding = stablePlanEncodingOf(t, ctx, tx, c)
	known := 0
	for _, r := range s.Old {
		if r.Resolved.Valid {
			known++
		}
	}
	e.VersionLedger = map[string]int{"pristine_live_links": s.N, "rebuild_old_delete_existing_versions": 0, "rebuild_new_inserts": 0, "rebuild_resolution_updates": 0, "source_versions_created_since_pristine_upper": s.N, "prefix_deletes": 0, "visible_before_statement": s.N, "inspect_result_rows": max(1, s.N), "delete_input_keys": s.Z, "write_input_rows": s.C + s.I, "delete_selected_versions_upper": int(stablePlanSelectedVersions(s.Removed, f.state)), "write_selected_versions_upper": int(stablePlanSelectedVersions(s.ChangedOld, f.state))}
	if f.state == "app_rebuild_after_analyzed" {
		e.VersionLedger["rebuild_old_delete_existing_versions"] = s.N
		e.VersionLedger["rebuild_new_inserts"] = s.N
		e.VersionLedger["rebuild_resolution_updates"] = known
		e.VersionLedger["source_versions_created_since_pristine_upper"] = 2*s.N + known
	}
	if c.Name == stablePlanWrite {
		e.VersionLedger["prefix_deletes"] = s.Z
		e.VersionLedger["visible_before_statement"] = s.N - s.Z
	}
	e.RawWorkAcceptance = "REQUIRES_RAW_REVIEW: visible fetches and selected-key versions are hard guards; they are not universal buffer/temp/TOAST allowances; unexplained raw work still blocks physical acceptance"
	if preparedName == "" {
		t.Fatal("stable kernel has no captured actual Prepare name")
	}
	if _, err := tx.Conn().Prepare(ctx, preparedName, c.SQL); err != nil {
		t.Fatal(err)
	}
	execute := stablePlanExecute(t, preparedName, c.Args)
	e.PlanningObservation = "NOT_OBSERVED: named history preserves the thirteenth actual selection without an extra plan-only choice"
	if observePlanning {
		beforePlan := stablePlanRelationWork(t, ctx, tx)
		var planOnly string
		if err := tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+execute).Scan(&planOnly); err != nil {
			t.Fatal(err)
		}
		e.PlanningPlan = json.RawMessage(planOnly)
		e.PlanningObservation = "separate EXPLAIN without ANALYZE; its plan choice precedes BeforeChoice"
		e.PlanningWork = stablePlanWorkDelta(beforePlan, stablePlanRelationWork(t, ctx, tx))
	}
	e.BeforeChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	work := stablePlanRelationWork(t, ctx, tx)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	if operation == "explain" {
		var raw string
		if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) "+execute).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		e.Plan = json.RawMessage(raw)
	} else if c.Name == stablePlanInspect {
		d := stablePlanReadD(t, ctx, tx, c.SQL, s, c.Args...)
		e.D = &d
	} else {
		tag, err := tx.Exec(ctx, c.SQL, c.Args...)
		if err != nil {
			t.Fatal(err)
		}
		n := tag.RowsAffected()
		e.RowsAffected = &n
	}
	e.WallNS = time.Since(start).Nanoseconds()
	runtime.ReadMemStats(&after)
	e.GoAlloc = after.TotalAlloc - before.TotalAlloc
	e.ExecutionWork = stablePlanWorkDelta(work, stablePlanRelationWork(t, ctx, tx))
	e.AfterChoice = stablePlanCounter(t, ctx, tx, c.SQL)
	// Emit complete raw evidence even when a following physical assertion fails.
	lintEvidence(t, "stable-sql-stage", e)
	stablePlanCountDelta(t, e.BeforeChoice, e.AfterChoice, mode)
	if e.D != nil && (!e.D.CanRetain || !e.D.Closed || e.D.CloseCalls != 1 || !slices.Equal(e.D.Targets, stablePlanWantD(s))) {
		t.Fatalf("stable complete D=%v want=%v retain=%v", e.D.Targets, stablePlanWantD(s), e.D.CanRetain)
	}
	if e.RowsAffected != nil {
		want := s.Z
		if c.Name == stablePlanWrite {
			want = s.C + s.I
		}
		if *e.RowsAffected != int64(want) {
			t.Fatalf("stable affected %s=%d want=%d (not a read-work oracle)", c.Name, *e.RowsAffected, want)
		}
	}
	stablePlanCheckWork(t, s, e)
	if e.Plan != nil {
		stablePlanCheckTree(t, s, e, stablePlanTree(t, e.Plan))
	}
	return e
}

// EXPLAIN prints per-loop averages rounded to whole rows. An executed node
// with one loop is exact; with more, use the mathematical lower bound when
// rejecting excess work, not an invented tuple allowance or a rounded total.
func stablePlanRowsLower(rows, loops float64) float64 {
	if loops <= 1 {
		return rows * loops
	}
	return max(0, rows-0.5) * loops
}

func stablePlanCheckTree(t *testing.T, s stablePlanSpec, e stablePlanEvidence, root stablePlanNode) {
	t.Helper()
	limits := map[string]int{"eligibility": 1, "incoming": s.C + s.I}
	producers := map[string]stablePlanNode{}
	oldScans := 0
	for _, n := range root.all() {
		if !n.RowsObserved || !n.LoopsObserved {
			t.Fatalf("stable actual node rows/loops NOT_OBSERVED: %+v", n)
		}
		if name, ok := strings.CutPrefix(n.Subplan, "CTE "); ok {
			if _, bound := limits[name]; bound {
				if _, duplicate := producers[name]; duplicate {
					t.Fatalf("stable duplicate materialized producer %s", name)
				}
				producers[name] = n
			}
			if n.Loops > 0 && slices.Contains([]string{"old_links", "incoming_starts", "existing_links"}, name) {
				t.Fatalf("stable obsolete full-range producer executed: %s", name)
			}
		}
		if n.Loops == 0 {
			continue
		}
		if strings.Contains(n.Type, "Scan") && slices.Contains(stablePlanTables(), n.Relation) {
			column := "source_id"
			visible := s.N
			switch n.Relation {
			case "indexed_pages":
				column = "node_id"
				visible = 1
			case "page_tags":
				visible = len(s.OldFacts.Tags)
			case "page_properties":
				visible = len(s.OldFacts.Properties)
			case "page_aliases":
				visible = len(s.OldFacts.Aliases)
			case "page_links":
				switch e.Name {
				case stablePlanInspect:
					oldScans++
					if n.Loops != 1 || n.Rows != float64(s.N) {
						t.Fatalf("stable Inspect full old range rows/loops=%v/%v want=%d/1", n.Rows, n.Loops, s.N)
					}
				case stablePlanDelete:
					visible = s.Z
				case stablePlanWrite:
					t.Fatalf("stable Write read a full old-row producer: %+v", n)
				}
			}
			keyed := false
			for _, child := range n.all() {
				if child.Index != "" && strings.Contains(child.IndexCond, column) {
					if e.Name != stablePlanDelete || n.Relation != "page_links" || strings.Contains(child.IndexCond, "range_start") {
						keyed = true
					}
				}
			}
			work := stablePlanRowsLower(n.Rows, n.Loops) + stablePlanRowsLower(n.Removed, n.Loops) + stablePlanRowsLower(n.Rechecked, n.Loops)
			if !keyed || work > float64(visible) {
				t.Fatalf("stable source/key-bounded read %s node=%+v visible=%d work_lower=%v", e.Name, n, visible, work)
			}
		}
		if bound, ok := limits[n.CTE]; ok {
			if n.Loops > 1 || stablePlanRowsLower(n.Rows, n.Loops)+stablePlanRowsLower(n.Removed, n.Loops) > float64(bound) {
				t.Fatalf("stable CTE rescanned %s rows/filter/loops=%v/%v/%v bound=%d", n.CTE, n.Rows, n.Removed, n.Loops, bound)
			}
		}
		if stablePlanRowsLower(n.JoinRemoved, n.Loops) != 0 {
			t.Fatalf("stable exact-key stages performed pairwise filtering: %+v", n)
		}
	}
	var required []string
	switch e.Name {
	case stablePlanInspect:
		required = []string{"eligibility"}
		if root.Rows != float64(max(1, s.N)) || root.Loops != 1 || oldScans != 1 {
			t.Fatalf("stable Inspect N-or-empty-one root=%v/%v old_scans=%d want_rows=%d", root.Rows, root.Loops, oldScans, max(1, s.N))
		}
	case stablePlanWrite:
		required = []string{"incoming"}
	}
	for _, name := range required {
		n, ok := producers[name]
		if !ok {
			t.Fatalf("stable materialized producer %s NOT_OBSERVED", name)
		}
		if n.Loops != 1 || n.Rows != float64(limits[name]) {
			t.Fatalf("stable complete single producer %s rows/loops=%v/%v want=%d/1", name, n.Rows, n.Loops, limits[name])
		}
	}
	if e.Name == stablePlanDelete || e.Name == stablePlanWrite {
		if root.Type != "ModifyTable" {
			t.Fatalf("stable DML root=%s", root.Type)
		}
		outer := 0
		candidateRows := float64(0)
		for _, n := range root.Plans {
			if n.Parent == "Outer" {
				outer++
				candidateRows += n.Rows * n.Loops
			}
		}
		want := s.Z
		if e.Name == stablePlanWrite {
			want = s.C + s.I
		}
		if outer != 1 || candidateRows != float64(want) {
			t.Fatalf("stable exact DML input %s=%v outer=%d want=%d; retained rows must not enter conflict", e.Name, candidateRows, outer, want)
		}
	}
	if e.Name == stablePlanWrite {
		if root.Conflicts != nil && *root.Conflicts != float64(s.C) {
			t.Fatalf("stable actual conflicts=%v want=%d", *root.Conflicts, s.C)
		}
		if root.Inserted != nil && *root.Inserted != float64(s.I) {
			t.Fatalf("stable actual inserts=%v want=%d", *root.Inserted, s.I)
		}
		if root.ConflictRemoved != nil && *root.ConflictRemoved != 0 {
			t.Fatalf("stable facts reached unchanged conflict filter: %v", *root.ConflictRemoved)
		}
		// Missing zero fields stay absent in raw evidence. Nonzero conflict
		// instrumentation and the separate pg_stat probe counts are mandatory.
		if s.C > 0 && root.Conflicts == nil {
			t.Fatal("stable nonzero conflict instrumentation NOT_OBSERVED; bind actual server format")
		}
	}
}

// This fixture VACUUMs the pristine seed. Only the real rebuild creates extra
// versions before observation: delete the original, insert its replacement,
// and update each originally resolved row once. Apply that literal history
// only to the selected keys, never to all N when checking Delete or Write.
func stablePlanSelectedVersions(rows []stablePlanRow, state string) int64 {
	n := int64(len(rows))
	if state == "app_rebuild_after_analyzed" {
		n += int64(len(rows))
		for _, r := range rows {
			if r.Resolved.Valid {
				n++
			}
		}
	}
	return n
}

func stablePlanCheckWork(t *testing.T, s stablePlanSpec, e stablePlanEvidence) {
	t.Helper()
	// Heap fetch accounting belongs to the table for Bitmap Heap Scan and
	// the index for Index Scan / conflict checks. Sum those, not both copies
	// of a guessed result count. Keep all raw planning/buffer/TOAST evidence.
	var indexReturned, heapFetched int64
	for name, w := range e.ExecutionWork {
		if w.Scans < 0 || w.Returned < 0 || w.Fetched < 0 || w.Blocks < 0 || w.Hits < 0 || w.Inserted < 0 || w.Updated < 0 || w.Deleted < 0 {
			t.Fatalf("stable negative raw relation work %s=%+v", name, w)
		}
		if w.Owner != "page_links" {
			continue
		}
		heapFetched += w.Fetched
		if w.Kind == "i" {
			indexReturned += w.Returned
			if e.Name == stablePlanWrite && name != "page_links_pkey" && (w.Scans != 0 || w.Returned != 0 || w.Fetched != 0) {
				t.Fatalf("stable Write unexpected old-row index access %s=%+v", name, w)
			}
		}
	}
	links := e.ExecutionWork["page_links"]
	if links.Scans != 0 || links.Returned != 0 {
		t.Fatalf("stable sequential page_links access %s=%+v", e.Name, links)
	}
	visible := s.N
	versions := stablePlanSelectedVersions(s.Old, e.State)
	switch e.Name {
	case stablePlanDelete:
		visible, versions = s.Z, stablePlanSelectedVersions(s.Removed, e.State)
		if links.Deleted != int64(s.Z) {
			t.Fatalf("stable exact physical deletes=%d want=%d", links.Deleted, s.Z)
		}
	case stablePlanWrite:
		visible, versions = s.C, stablePlanSelectedVersions(s.ChangedOld, e.State)
		if links.Inserted != int64(s.I) || links.Updated != int64(s.C) {
			t.Fatalf("stable exact physical writes inserted/updated=%d/%d want=%d/%d", links.Inserted, links.Updated, s.I, s.C)
		}
		pk := e.ExecutionWork["page_links_pkey"]
		if pk.Scans != int64(s.C+s.I) || pk.Fetched != int64(s.C) {
			t.Fatalf("stable actual conflict probes/fetches=%d/%d want=%d/%d", pk.Scans, pk.Fetched, s.C+s.I, s.C)
		}
	}
	if heapFetched != int64(visible) || indexReturned < int64(visible) || indexReturned > versions {
		t.Fatalf("stable selected-key read work %s: heap_fetch=%d returned=%d want_visible=%d selected_versions_upper=%d", e.Name, heapFetched, indexReturned, visible, versions)
	}
}
