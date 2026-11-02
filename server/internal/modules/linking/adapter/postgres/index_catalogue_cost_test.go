//go:build !race

package postgresadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// Fixed literals come from two independent successful calibration processes.
// Normal CI never derives a budget from the current run or an environment flag.
const indexCatalogueCalibration = false

type catalogueNotebook struct{}

func (catalogueNotebook) StateOf(context.Context, uuid.UUID) (app.IndexNotebookState, bool, error) {
	return app.IndexNotebookState{Mode: "standard"}, true, nil
}

// Capture the actual Store call, including every batch's complete UUID array.
// This is an observer; SQL, parameters and pgx execution mode are unchanged.
type catalogueCostTrace struct {
	previous   pgx.QueryTracer
	mu         sync.Mutex
	active     bool
	statements []statement
	modes      map[string]int
}

func (c *catalogueCostTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if c.previous != nil {
		ctx = c.previous.TraceQueryStart(ctx, conn, d)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active && strings.HasPrefix(d.SQL, "-- name: IndexCatalogueHeads ") {
		args := make([]any, 0, len(d.Args))
		for _, a := range d.Args {
			if mode, ok := a.(pgx.QueryExecMode); ok {
				c.modes[mode.String()]++
				continue
			}
			if ids, ok := a.([]uuid.UUID); ok {
				a = slices.Clone(ids)
			}
			args = append(args, a)
		}
		c.statements = append(c.statements, statement{sql: d.SQL, args: args})
	}
	return ctx
}
func (c *catalogueCostTrace) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryEndData) {
	if c.previous != nil {
		c.previous.TraceQueryEnd(ctx, conn, d)
	}
}
func (c *catalogueCostTrace) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = true
	c.statements = nil
	c.modes = map[string]int{}
}
func (c *catalogueCostTrace) stop() []statement {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	return slices.Clone(c.statements)
}

func catalogueCostEvent(t *testing.T, kind, state string, value any) {
	t.Helper()
	phase := "guard"
	if indexCatalogueCalibration {
		phase = "calibration"
	}
	raw, err := json.Marshal(map[string]any{"Schema": "nervewiki.p4-s1.cost.v1", "Kind": kind, "Case": "index-catalogue", "Phase": phase, "State": state, "Data": value})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P4COST %s", raw)
}

// The adapter guard measures the real ListIndex/capped catalogue and SQL. As in
// the existing index guard, authentication and the final page-path projection
// are supplied fakes; the separate bootstrap guards exercise real HTTP/SDK.
func TestIndexCatalogueHasBoundedWorkAndOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("real 100k catalogue cost runs alone without race")
	}
	f := newFixture(t)
	trace := &catalogueCostTrace{}
	cfg := f.pool.Config()
	trace.previous, cfg.ConnConfig.Tracer = cfg.ConnConfig.Tracer, trace
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	f.pool, f.s = pool, postgresadapter.New(pool)
	ids := catalogueCostSeed(t, f)
	uc := app.ListIndex{Access: app.Access{Notebooks: guardAccess{uuid.NewV7()}, Auth: guardAccess{}}, Reads: f.s,
		Catalogue: f.s, Notebooks: catalogueNotebook{}, Paths: guardPaths{}}
	ctx := shared.WithActor(t.Context(), shared.Actor{UserID: uuid.NewV7(), SessionID: uuid.NewV7()})
	for _, state := range []string{"no_statistics", "analyzed"} {
		t.Run(state, func(t *testing.T) {
			if state == "analyzed" {
				lintAnalyze(t, f)
			}
			stats := lintAssertStatistics(t, f, state == "analyzed")
			var samples [3]lintCostMetrics
			var captured []statement
			for i := range samples {
				conn, e := f.pool.Acquire(t.Context())
				if e != nil {
					t.Fatal(e)
				}
				prepared := func() [2]int64 {
					var v [2]int64
					if e := conn.QueryRow(t.Context(), `SELECT coalesce(sum(custom_plans),0)::bigint,coalesce(sum(generic_plans),0)::bigint FROM pg_prepared_statements WHERE statement LIKE '-- name: IndexCatalogueHeads %'`).Scan(&v[0], &v[1]); e != nil {
						t.Fatal(e)
					}
					return v
				}
				beforePlans := prepared()
				trace.start()
				var out app.IndexPage
				metric, err := lintCostMeasure(t, func() error {
					var e error
					out, e = uc.Execute(postgres.OnConn(ctx, conn), f.eng, app.IndexQuery{Catalogue: true})
					return e
				})
				captured = trace.stop()
				afterPlans := prepared()
				conn.Release()
				if afterPlans[0]+afterPlans[1]-beforePlans[0]-beforePlans[1] != 196 {
					t.Fatalf("actual cached catalogue executions: before%v after%v", beforePlans, afterPlans)
				}
				samples[i] = metric
				if err != nil {
					t.Fatalf("actual catalogue: %v", err)
				}
				var got []uuid.UUID
				for _, s := range captured {
					batch, ok := s.args[len(s.args)-1].([]uuid.UUID)
					if !ok || len(batch) == 0 || len(batch) > 512 {
						t.Fatalf("actual catalogue batch: %+v", s.args)
					}
					got = append(got, batch...)
				}
				if len(captured) != 196 || !slices.Equal(got, ids) {
					t.Fatalf("catalogue did not drain the complete 100k IDs exactly once: batches=%d ids=%d", len(captured), len(got))
				}
				if len(out.Types) != 15 || out.TypesComplete || out.UnfilterableTypePages != 1 || len(out.Items) != 50 || out.Next == "" {
					t.Fatalf("bounded nonempty catalogue = %+v", out)
				}
				for j, typ := range out.Types {
					if typ.Type != catalogueCostType(j+1) || typ.Count != 1 {
						t.Fatalf("catalogue independent distinct oracle at %d: %+v", j, typ)
					}
				}
				encoded, err := json.Marshal(out.Types)
				if err != nil || len(encoded) > 32<<10 || len(encoded) < 30<<10 {
					t.Fatalf("catalogue JSON bound: %d %v", len(encoded), err)
				}
				catalogueCostEvent(t, "sample", state, map[string]any{"Ordinal": i + 1, "Metrics": metric, "OutputBytes": len(encoded), "Rows": len(got), "Batches": len(captured), "Statistics": stats, "DefaultQueryExecMode": cfg.ConnConfig.DefaultQueryExecMode.String(), "ExplicitOverrides": trace.modes, "PreparedBefore": beforePlans, "PreparedAfter": afterPlans, "Execution": "production Store.queries on one observed pool connection; seed/validation may warm buffers"})
			}
			catalogueCostParameters(t, f, captured)
			// The first and final finite batches cover ordinary distinct values,
			// the no-type 1000-property pages and both full-value whitespace paths.
			for _, batch := range []struct {
				name string
				s    statement
			}{{"first", captured[0]}, {"tail", captured[len(captured)-1]}} {
				for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
					t.Run(batch.name+"/"+mode, func(t *testing.T) {
						plan, ev := lintExecutedMode(t, f, batch.s, mode, nil)
						want := len(batch.s.args[len(batch.s.args)-1].([]uuid.UUID))
						if ev.Rows != want {
							t.Fatalf("actual catalogue plan returned %d, want %d", ev.Rows, want)
						}
						indexes, keyed := plan.indexes("source_id")
						if !slices.Contains(indexes, "page_properties_type_idx") {
							t.Fatalf("catalogue lost first-type partial index: %+v keyed=%t", indexes, keyed)
						}
						for _, name := range []string{"indexed_pages", "page_properties", "indexed_pages_pkey", "page_properties_type_idx"} {
							w := ev.ExecutionWork[name]
							if w.Returned > int64(want+4) || w.Fetched > int64(want+4) {
								t.Fatalf("catalogue read unrelated notebook or extra properties through %s: %+v", name, w)
							}
						}
						if ev.ExecutionWork["indexed_pages_pkey"].Fetched == 0 || ev.ExecutionWork["page_properties_type_idx"].Fetched == 0 {
							t.Fatal("catalogue physical counters did not observe real primary/first-type probes")
						}
						var toast int64
						for name, w := range ev.ExecutionWork {
							if strings.HasPrefix(name, "pg_toast") {
								toast += w.Fetched
							}
						}
						if batch.name == "tail" && toast == 0 {
							t.Fatal("catalogue full-whitespace branch did not read actual TOAST values")
						}
						catalogueCostEvent(t, "plan", state, ev)
					})
				}
			}
			catalogueCostGuard(t, state, samples)
		})
	}
}

func catalogueCostSeed(t *testing.T, f fixture) []uuid.UUID {
	t.Helper()
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_properties", "page_links"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parse := func(body string) domain.Facts {
		facts, e := markdownadapter.PageFacts(md.Parse([]byte(body)).Facts())
		if e != nil || !facts.FrontmatterValid {
			t.Fatalf("real catalogue parser: %v", e)
		}
		return facts
	}
	var noType strings.Builder
	noType.WriteString("---\n")
	for i := range 1000 {
		fmt.Fprintf(&noType, "key%d: x\n", i)
	}
	noType.WriteString("---\nbody")
	none := parse(noType.String())
	blank := parse("---\ntype: '" + strings.Repeat(" ", 5_000_000) + "'\n---\nbody")
	late := parse("---\ntype: '" + strings.Repeat(" ", 4_999_999) + "X'\n---\nbody")
	ids := make([]uuid.UUID, 100000)
	for first := 0; first < len(ids); first += 1000 {
		var pages, props [][]any
		for i := first; i < first+1000; i++ {
			id := lintReaderID(uint64(i + 1))
			ids[i] = id
			facts := none
			switch {
			case i < 99980:
				quoted, _ := json.Marshal(catalogueCostType(i + 1))
				facts = parse(fmt.Sprintf("---\ntype: %s\nsummary: current parser\na: one\nb: two\nc: three\n---\nbody", quoted))
			case i == 99998:
				facts = blank
			case i == 99999:
				facts = late
			}
			pages = append(pages, []any{id, f.eng, int32(1), int32(2), true})
			for pos, p := range facts.Properties {
				props = append(props, []any{id, int32(pos), f.eng, p.Key, p.Value})
			}
		}
		for _, batch := range []struct {
			table string
			cols  []string
			rows  [][]any
		}{{"indexed_pages", []string{"node_id", "notebook_id", "revision", "extractor", "frontmatter_valid"}, pages}, {"page_properties", []string{"source_id", "position", "notebook_id", "key", "value"}, props}} {
			if n, e := f.pool.CopyFrom(t.Context(), pgx.Identifier{batch.table}, batch.cols, pgx.CopyFromRows(batch.rows)); e != nil || n != int64(len(batch.rows)) {
				t.Fatalf("real parsed catalogue seed %s: %d %v", batch.table, n, e)
			}
		}
	}
	// A larger unrelated notebook shares the same physical indexes. Its facts
	// are copied from a real parser/store prototype, never query answers.
	foreign := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	f.replace(t, app.Page{ID: foreign, NotebookID: f.ops, Revision: 1}, originFacts(t, "---\ntype: hidden\nsummary: other notebook\n---\n[[missing]]"))
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT ('30000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(2,200000) k`, f.ops)
	lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value) SELECT i.node_id,p.position,i.notebook_id,p.key,p.value FROM indexed_pages i CROSS JOIN page_properties p WHERE i.notebook_id=$1 AND i.node_id<>$2 AND p.source_id=$2`, f.ops, foreign)
	if got := f.rows(t, `SELECT notebook_id::text,count(*)::text,min(extractor)::text,min(revision)::text FROM indexed_pages GROUP BY notebook_id ORDER BY notebook_id`); len(got) != 2 {
		t.Fatalf("catalogue seed notebooks: %v", got)
	}
	counts := f.rows(t, `SELECT (SELECT count(*) FROM indexed_pages WHERE notebook_id=$1)::text,(SELECT count(*) FROM indexed_pages WHERE notebook_id=$2)::text`, f.eng, f.ops)
	if len(counts) != 1 || !slices.Equal(counts[0], []string{"100000", "200000"}) {
		t.Fatalf("catalogue seed counts: %v", counts)
	}
	catalogueCostEvent(t, "seed", "no_statistics", map[string]any{"Pages": 100000, "ForeignPages": 200000, "DistinctFilterableTypes": 99980, "FilterableTypeBytes": 1024, "ExpectedTypesAfterJSONCap": 15, "NoTypeThousandPropertyPages": 18, "FullValueBytes": 5000000, "BlankFullValues": 1, "LateNonblankFullValues": 1, "Extractor": 2, "Statistics": lintAssertStatistics(t, f, false), "Scope": "adapter index facts parsed from real Markdown; not bootstrap page/content/history storage"})
	return ids
}

// 1024 legal bytes, all distinct, with JSON escaping doubling 1018 of them.
// Fifteen complete menu entries fit 32KiB; the independently known sixteenth
// does not. The two 5MB whitespace values exercise a different SQL slow path.
func catalogueCostType(n int) string { return fmt.Sprintf("%06d", n) + strings.Repeat(`\"`, 509) }

func catalogueCostParameters(t *testing.T, f fixture, statements []statement) {
	t.Helper()
	conn, err := f.pool.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	desc, err := conn.Conn().PgConn().Prepare(t.Context(), "", statements[0].sql, nil)
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	var bytesByBatch [][]int
	hash := sha256.New()
	for _, s := range statements {
		var sizes []int
		for i, arg := range s.args {
			b, e := conn.Conn().TypeMap().Encode(desc.ParamOIDs[i], pgtype.BinaryFormatCode, arg, nil)
			if e != nil {
				t.Fatal(e)
			}
			sizes = append(sizes, len(b))
			_, _ = hash.Write(b)
		}
		bytesByBatch = append(bytesByBatch, sizes)
	}
	runtime.ReadMemStats(&after)
	catalogueCostEvent(t, "parameters", "outside_algorithm", map[string]any{"SQL": statements[0].sql, "OIDs": desc.ParamOIDs, "AllBatchParameterBytes": bytesByBatch, "AllEncodedParametersSHA256": fmt.Sprintf("%x", hash.Sum(nil)), "EncodingTotalAllocBytes": after.TotalAlloc - before.TotalAlloc, "Rows": 100000, "Batches": len(statements)})
}

func catalogueCostGuard(t *testing.T, state string, samples [3]lintCostMetrics) {
	t.Helper()
	values := map[string][3]uint64{"WallNS": {}, "CPUNS": {}, "TotalAllocBytes": {}, "SampledPeakIncrementBytes": {}}
	for name, v := range values {
		for i, s := range samples {
			switch name {
			case "WallNS":
				v[i] = uint64(s.WallNS)
			case "CPUNS":
				v[i] = uint64(s.GoCPUTimeNS)
			case "TotalAllocBytes":
				v[i] = s.TotalAlloc
			default:
				v[i] = s.SampledHeapGrowth
			}
		}
		slices.Sort(v[:])
		values[name] = v
	}
	medians := map[string]uint64{}
	for n, v := range values {
		medians[n] = v[1]
	}
	limits := map[string]uint64{
		"WallNS":                    4527240378,
		"CPUNS":                     1668177000,
		"TotalAllocBytes":           699248952,
		"SampledPeakIncrementBytes": 141228192,
	}
	accepted := !indexCatalogueCalibration
	for metric, median := range medians {
		if median == 0 || median > limits[metric] {
			accepted = false
		}
	}
	catalogueCostEvent(t, "guard", state, map[string]any{"Medians": medians, "Limits": limits, "Accepted": accepted, "CalibrationOnly": indexCatalogueCalibration})
	if !indexCatalogueCalibration && !accepted {
		t.Fatalf("catalogue costs exceed fixed limits: medians=%v limits=%v", medians, limits)
	}
}
