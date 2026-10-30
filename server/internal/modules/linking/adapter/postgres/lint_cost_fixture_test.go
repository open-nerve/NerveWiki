//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// Measurements below belong to this linking test process. The two external
// ports remain explicit fixture facts; bootstrap measures the actual modules.
type lintCostQuery struct {
	Calls, Rows, WallNS int64
	ArrayElements       int64
	MaxArrayElements    int
}

type lintCostArray struct {
	Name, Argument string
	IDs            []uuid.UUID
	Calls          int
}

type lintCostTrace struct {
	mu      sync.Mutex
	active  bool
	queries map[string]lintCostQuery
	arrays  map[string]*lintCostArray
	modes   map[string]int
}

type lintCostTraceKey struct{}
type lintCostQueryStart struct {
	name string
	at   time.Time
}

func (tr *lintCostTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if !tr.active || !strings.HasPrefix(data.SQL, "-- name: Lint") {
		return ctx
	}
	name := strings.Fields(data.SQL)[2]
	q := tr.queries[name]
	for i, arg := range data.Args {
		if mode, ok := arg.(pgx.QueryExecMode); ok {
			tr.modes[mode.String()]++
		}
		if ids, ok := arg.([]uuid.UUID); ok {
			q.ArrayElements += int64(len(ids))
			q.MaxArrayElements = max(q.MaxArrayElements, len(ids))
			key := fmt.Sprintf("%s/arg%d/len%d", name, i, len(ids))
			if tr.arrays[key] == nil {
				// One read-only representative is retained per argument/length.
				// No extra encoding or clone occurs in the timed query interval.
				tr.arrays[key] = &lintCostArray{Name: name, Argument: key, IDs: ids}
			}
			tr.arrays[key].Calls++
		}
	}
	tr.queries[name] = q
	return context.WithValue(ctx, lintCostTraceKey{}, lintCostQueryStart{name, time.Now()})
}

func (tr *lintCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(lintCostTraceKey{}).(lintCostQueryStart)
	if !ok {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	q := tr.queries[start.name]
	q.Calls++
	q.Rows += data.CommandTag.RowsAffected()
	q.WallNS += time.Since(start.at).Nanoseconds()
	tr.queries[start.name] = q
}

func (tr *lintCostTrace) begin() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.active = true
	tr.queries = map[string]lintCostQuery{}
	tr.arrays = map[string]*lintCostArray{}
	tr.modes = map[string]int{}
}

func (tr *lintCostTrace) end() (map[string]lintCostQuery, map[string]int, int64) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.active = false
	var elapsed int64
	for _, q := range tr.queries {
		elapsed += q.WallNS
	}
	return tr.queries, tr.modes, elapsed
}

func lintCostDescribeExec(t *testing.T, queries map[string]lintCostQuery, modes map[string]int) {
	t.Helper()
	for mode, count := range modes {
		if mode != pgx.QueryExecModeDescribeExec.String() || count == 0 {
			t.Fatalf("real production query mode %q=%d, want DescribeExec", mode, count)
		}
	}
	if len(queries) == 0 || len(modes) == 0 {
		t.Fatal("cost interval observed no actual production lint queries")
	}
}

func (tr *lintCostTrace) encoding(t *testing.T) {
	t.Helper()
	keys := make([]string, 0, len(tr.arrays))
	for key := range tr.arrays {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	m := pgtype.NewMap()
	for _, key := range keys {
		a := tr.arrays[key]
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		raw, err := m.Encode(pgtype.UUIDArrayOID, pgtype.BinaryFormatCode, a.IDs, nil)
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		lintEvidence(t, "lint-cost-parameter-encoding", map[string]any{
			"argument": a.Argument, "calls": a.Calls, "elements": len(a.IDs), "encoded_bytes": len(raw),
			"observed_calls_times_encoded_bytes": int64(a.Calls) * int64(len(raw)),
			"diagnostic_single_encoding_wall_ns": elapsed.Nanoseconds(), "diagnostic_single_encoding_alloc": after.TotalAlloc - before.TotalAlloc,
			"scope": "one real pgx binary encoding of the actual representative UUID argument, outside Prepare metrics; multiplied bytes describe observed equal-length arguments, not protocol wire bytes or repeated allocation measurements",
		})
	}
}

type lintCostPayload struct{ Calls, Rows, Bytes, MaxBatch, MaxItem, MaxRows int64 }

// This wrapper only observes completed production slices. It never supplies
// a row, changes an argument or starts a second read.
type lintCostReads struct {
	app.LintReadStore
	payloads map[string]lintCostPayload
}

func (r *lintCostReads) record(name string, sizes []int) {
	p := r.payloads[name]
	p.Calls++
	p.Rows += int64(len(sizes))
	p.MaxRows = max(p.MaxRows, int64(len(sizes)))
	var bytes int64
	for _, n := range sizes {
		bytes += int64(n)
		p.MaxItem = max(p.MaxItem, int64(n))
	}
	p.Bytes += bytes
	p.MaxBatch = max(p.MaxBatch, bytes)
	r.payloads[name] = p
}

func (r *lintCostReads) Edges(ctx context.Context, nb uuid.UUID, positions []app.EdgePos) ([]app.LintEdge, error) {
	rows, err := r.LintReadStore.Edges(ctx, nb, positions)
	sizes := make([]int, len(rows))
	for i, row := range rows {
		sizes[i] = len(row.Target)
	}
	r.record("edges", sizes)
	return rows, err
}

func (r *lintCostReads) SourceProperties(ctx context.Context, nb uuid.UUID, positions []app.SourcePropertyPos) ([]app.SourceProperty, error) {
	rows, err := r.LintReadStore.SourceProperties(ctx, nb, positions)
	sizes := make([]int, len(rows))
	for i, row := range rows {
		sizes[i] = len(row.Key) + len(row.JSON)
	}
	r.record("sources-JSON", sizes)
	return rows, err
}

func (r *lintCostReads) SourceSlotEdges(ctx context.Context, nb uuid.UUID, positions []app.EdgePos) ([]app.SourceSlotEdge, error) {
	rows, err := r.LintReadStore.SourceSlotEdges(ctx, nb, positions)
	sizes := make([]int, len(rows))
	for i, row := range rows {
		sizes[i] = len(row.Target)
		if row.PropertyKey != nil {
			sizes[i] += len(*row.PropertyKey)
		}
	}
	r.record("source-slot-targets", sizes)
	return rows, err
}

type lintCostMetrics struct {
	WallNS, GoCPUTimeNS                      int64
	TotalAlloc, HeapAtStart, SampledHeapPeak uint64
	SampledHeapGrowth, HeapAtEnd             uint64
	SamplingNS                               int64
}

func lintCostCPU(t *testing.T) int64 {
	t.Helper()
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		t.Fatal(err)
	}
	return r.Utime.Nano() + r.Stime.Nano()
}

func lintCostMeasure(t *testing.T, run func() error) (lintCostMetrics, error) {
	t.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	stop, stopped := make(chan struct{}), make(chan uint64, 1)
	go func() {
		peak := before.HeapAlloc
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			case <-stop:
				stopped <- peak
				return
			}
		}
	}()
	cpu, at := lintCostCPU(t), time.Now()
	err := run()
	elapsed := time.Since(at)
	cpu = lintCostCPU(t) - cpu
	close(stop)
	peak := <-stopped
	runtime.ReadMemStats(&after)
	peak = max(peak, after.HeapAlloc)
	return lintCostMetrics{WallNS: elapsed.Nanoseconds(), GoCPUTimeNS: cpu, TotalAlloc: after.TotalAlloc - before.TotalAlloc,
		HeapAtStart: before.HeapAlloc, SampledHeapPeak: peak, SampledHeapGrowth: peak - before.HeapAlloc,
		HeapAtEnd: after.HeapAlloc, SamplingNS: int64(time.Millisecond)}, err
}

type lintCostFixture struct {
	f     fixture
	in    app.ReadInput
	sizes *lintConsumerSizes
	rules *app.Rules
	reads *lintCostReads
	trace *lintCostTrace
	stale int64
}

func newLintCostFixture(t *testing.T) *lintCostFixture {
	t.Helper()
	f := newFixture(t)
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	tr := &lintCostTrace{}
	pool := lintPool(t, f, tr, 1)
	f.tx = postgres.NewTxManager(pool, time.Second)
	reads := &lintCostReads{LintReadStore: postgresadapter.NewLintReads(pool, 5*1024*1024), payloads: map[string]lintCostPayload{}}
	sizes := &lintConsumerSizes{notebook: f.eng, values: map[uuid.UUID]int64{}}
	return &lintCostFixture{f: f, in: app.ReadInput{NotebookID: f.eng, EffectiveMode: domain.ModeStandard}, sizes: sizes,
		rules: app.NewRules(app.NewLintReads(reads, 5*1024*1024), postgresadapter.New(pool), sizes), reads: reads, trace: tr}
}

func lintCostID(n uint64) uuid.UUID { id := lintID(n); id[0] = 0x10; return id }

func lintCostTime(n uint64) time.Time { return time.Unix(1700000000+int64(n), 0).UTC() }

func (c *lintCostFixture) page(t *testing.T, n uint64, key, content string, parent *uuid.UUID, source bool) domain.Facts {
	t.Helper()
	facts := originFacts(t, content)
	c.f.replace(t, app.Page{ID: lintCostID(n), NotebookID: c.f.eng, Revision: 1}, facts)
	c.in.Nodes = append(c.in.Nodes, app.CompactNode{ID: lintCostID(n), ParentID: parent, Kind: domain.NodePage,
		NameKey: key, PageBytes: int64(len(content)), CreatedAt: lintCostTime(n)})
	if source {
		c.in.SourceIDs = append(c.in.SourceIDs, lintCostID(n))
	}
	return facts
}

// bulk copies an actual parser/Replace prototype's complete relevant facts.
// It changes only the copied source ID. Per-source metadata is independent.
func (c *lintCostFixture) bulk(t *testing.T, start uint64, count int, content string, parent *uuid.UUID, source bool) domain.Facts {
	t.Helper()
	if count < 1 {
		t.Fatal("empty cost seed")
	}
	facts := c.page(t, start, fmt.Sprintf("page-%d", start), content, parent, source)
	if len(facts.Tags) != 0 || len(facts.Aliases) != 0 {
		t.Fatal("bulk seed must not silently omit tags or aliases")
	}
	if count == 1 {
		return facts
	}
	const generated = `('10000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid`
	args := []any{c.f.eng, lintCostID(start), int64(start + 1), int64(start) + int64(count) - 1}
	lintExec(t, c.f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT `+generated+`,p.notebook_id,p.revision,p.extractor,p.frontmatter_valid
 FROM generate_series($3::bigint,$4::bigint) k CROSS JOIN indexed_pages p WHERE p.node_id=$2 AND p.notebook_id=$1`, args...)
	lintExec(t, c.f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT `+generated+`,p.position,p.notebook_id,p.key,p.value FROM generate_series($3::bigint,$4::bigint) k CROSS JOIN page_properties p
 WHERE p.source_id=$2 AND p.notebook_id=$1`, args...)
	lintExec(t, c.f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,aliases,resolved_asset)
 SELECT `+generated+`,p.range_start,p.range_end,p.notebook_id,p.kind,p.property_key,p.target,p.anchor,p.display,p.target_key,p.target_alt_key,p.resolved_id,p.ambiguous,p.aliases,p.resolved_asset
 FROM generate_series($3::bigint,$4::bigint) k CROSS JOIN page_links p WHERE p.source_id=$2 AND p.notebook_id=$1`, args...)
	for n := start + 1; n < start+uint64(count); n++ {
		c.in.Nodes = append(c.in.Nodes, app.CompactNode{ID: lintCostID(n), ParentID: parent, Kind: domain.NodePage,
			NameKey: fmt.Sprintf("page-%d", n), PageBytes: int64(len(content)), CreatedAt: lintCostTime(n)})
		if source {
			c.in.SourceIDs = append(c.in.SourceIDs, lintCostID(n))
		}
	}
	var pages, properties, links int
	if err := c.f.pool.QueryRow(t.Context(), `SELECT
 (SELECT count(*) FROM indexed_pages WHERE notebook_id=$1 AND node_id BETWEEN $2 AND $3 AND revision=1 AND extractor=2),
 (SELECT count(*) FROM page_properties WHERE notebook_id=$1 AND source_id BETWEEN $2 AND $3),
 (SELECT count(*) FROM page_links WHERE notebook_id=$1 AND source_id BETWEEN $2 AND $3)`,
		c.f.eng, lintCostID(start), lintCostID(start+uint64(count)-1)).Scan(&pages, &properties, &links); err != nil {
		t.Fatal(err)
	}
	if pages != count || properties != count*len(facts.Properties) || links != count*len(facts.Links) {
		t.Fatalf("actual production-fact copies: pages/properties/links=%d/%d/%d want %d/%d/%d", pages, properties, links, count, count*len(facts.Properties), count*len(facts.Links))
	}
	var changed int
	if err := c.f.pool.QueryRow(t.Context(), `SELECT
 (SELECT count(*) FROM indexed_pages c CROSS JOIN indexed_pages p
  WHERE p.node_id=$2 AND c.notebook_id=$1 AND c.node_id BETWEEN $2 AND $3
  AND (c.notebook_id,c.revision,c.extractor,c.frontmatter_valid) IS DISTINCT FROM (p.notebook_id,p.revision,p.extractor,p.frontmatter_valid)) +
 (SELECT count(*) FROM page_properties c JOIN page_properties p ON p.source_id=$2 AND p.position=c.position
  WHERE c.notebook_id=$1 AND c.source_id BETWEEN $2 AND $3 AND (c.notebook_id,c.key,c.value) IS DISTINCT FROM (p.notebook_id,p.key,p.value)) +
 (SELECT count(*) FROM page_links c JOIN page_links p ON p.source_id=$2 AND p.range_start=c.range_start
  WHERE c.notebook_id=$1 AND c.source_id BETWEEN $2 AND $3 AND
  (c.notebook_id,c.range_end,c.kind,c.property_key,c.target,c.anchor,c.display,c.target_key,c.target_alt_key,c.resolved_id,c.ambiguous,c.aliases,c.resolved_asset)
  IS DISTINCT FROM
  (p.notebook_id,p.range_end,p.kind,p.property_key,p.target,p.anchor,p.display,p.target_key,p.target_alt_key,p.resolved_id,p.ambiguous,p.aliases,p.resolved_asset))`,
		c.f.eng, lintCostID(start), lintCostID(start+uint64(count)-1)).Scan(&changed); err != nil || changed != 0 {
		t.Fatalf("copied production facts changed outside source ID: rows=%d err=%v", changed, err)
	}
	return facts
}

func (c *lintCostFixture) foreign(t *testing.T, prefix string, count int) {
	t.Helper()
	prototype := uuid.MustParse(prefix + "-0000-0000-0000-000000000001")
	facts := originFacts(t, "---\ntype: note\nsummary: hidden\nsources: '[[hidden]]'\n---\n[[hidden]]")
	c.f.replace(t, app.Page{ID: prototype, NotebookID: c.f.ops, Revision: 1}, facts)
	lintExec(t, c.f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($2||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(2,$3::int) k`, c.f.ops, prefix, count)
	lintExec(t, c.f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,anchor,display,target_key,target_alt_key,resolved_id,ambiguous,aliases,resolved_asset)
 SELECT i.node_id,p.range_start,p.range_end,i.notebook_id,p.kind,p.property_key,p.target,p.anchor,p.display,p.target_key,p.target_alt_key,p.resolved_id,p.ambiguous,p.aliases,p.resolved_asset
 FROM indexed_pages i CROSS JOIN page_links p WHERE i.notebook_id=$1 AND i.node_id<>$2 AND p.source_id=$2`, c.f.ops, prototype)
	lintExec(t, c.f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT i.node_id,p.position,i.notebook_id,p.key,p.value FROM indexed_pages i CROSS JOIN page_properties p
 WHERE i.notebook_id=$1 AND i.node_id<>$2 AND p.source_id=$2`, c.f.ops, prototype)
}

func lintCostMedian(t *testing.T, group, name string, samples []lintCostMetrics) {
	t.Helper()
	if len(samples) < 3 {
		t.Fatal("cost calibration needs at least three actual samples")
	}
	walls := make([]int64, len(samples))
	cpus := make([]int64, len(samples))
	alloc := make([]uint64, len(samples))
	heaps := make([]uint64, len(samples))
	for i, sample := range samples {
		walls[i], cpus[i] = sample.WallNS, sample.GoCPUTimeNS
		alloc[i], heaps[i] = sample.TotalAlloc, sample.SampledHeapGrowth
	}
	slices.Sort(walls)
	slices.Sort(cpus)
	slices.Sort(alloc)
	slices.Sort(heaps)
	median, memory := walls[len(walls)/2], alloc[len(alloc)/2]
	cpu, heap := cpus[len(cpus)/2], heaps[len(heaps)/2]
	limit, known := lintCostLimitsFor(group)
	var fixedLimits any
	if known {
		fixedLimits = limit
	}
	encoded, err := json.Marshal(samples)
	if err != nil {
		t.Fatal(err)
	}
	flavor := map[string]any{"case": t.Name(), "name": name, "samples": json.RawMessage(encoded), "median_wall_ns": median, "median_total_alloc": memory,
		"median_times_three_wall_ns": median * 3, "median_times_three_alloc": memory * 3,
		"median_go_cpu_ns": cpu, "median_sampled_heap_growth": heap, "budget_group": group, "fixed_limits": fixedLimits,
		"scope": "warm local linking consumer with fixed local group limits; not production or PostgreSQL RSS guarantees; same-input final double rounds/slowest CI are coordinated separately"}
	lintEvidence(t, "lint-cost-calibration", flavor)
	if !known {
		t.Fatalf("cost group %q has no fixed limits", group)
	}
	if time.Duration(median) > limit.Wall || time.Duration(cpu) > limit.GoCPU || memory > limit.TotalAlloc || heap > limit.HeapGrowth {
		t.Errorf("%s cost medians exceed fixed limits: wall=%s CPU=%s alloc=%d sampled-heap-growth=%d; limits=%+v", group, time.Duration(median), time.Duration(cpu), memory, heap, limit)
	}
}

// Fixed from clean 14–17 calibration on input 0c6c1619a2d8ecb3fc0266aaaa09e30b196416e5e07b4f4ca07379174cfdd869.
// Each group takes the largest case median of either round, times three,
// rounded up to 1ms / 64KiB. The 112 cases and all raw samples remain in
// the external calibration-proposal; no limit is derived from this run.
type lintCostLimits struct {
	Wall, GoCPU            time.Duration
	TotalAlloc, HeapGrowth uint64
}

func lintCostLimitsFor(group string) (lintCostLimits, bool) {
	limits := map[string]lintCostLimits{
		"properties/actual-body-positive-control":           {Wall: 7 * time.Millisecond, GoCPU: 4 * time.Millisecond, TotalAlloc: 11_141_120, HeapGrowth: 11_141_120},
		"properties/first-sources-complete-JSON-and-slot":   {Wall: 18 * time.Millisecond, GoCPU: 10 * time.Millisecond, TotalAlloc: 20_643_840, HeapGrowth: 20_643_840},
		"properties/missing_summary-complete-needed-values": {Wall: 37 * time.Millisecond, GoCPU: 6 * time.Millisecond, TotalAlloc: 65_536, HeapGrowth: 65_536},
		"properties/missing_summary-prefix-and-unrelated":   {Wall: 2 * time.Millisecond, GoCPU: 1 * time.Millisecond, TotalAlloc: 65_536, HeapGrowth: 65_536},
		"properties/missing_type-complete-needed-values":    {Wall: 36 * time.Millisecond, GoCPU: 5 * time.Millisecond, TotalAlloc: 65_536, HeapGrowth: 65_536},
		"properties/missing_type-prefix-and-unrelated":      {Wall: 3 * time.Millisecond, GoCPU: 3 * time.Millisecond, TotalAlloc: 65_536, HeapGrowth: 65_536},
		"rules/eight/ambiguous":                             {Wall: 932 * time.Millisecond, GoCPU: 964 * time.Millisecond, TotalAlloc: 598_474_752, HeapGrowth: 503_709_696},
		"rules/eight/invalid_frontmatter":                   {Wall: 816 * time.Millisecond, GoCPU: 850 * time.Millisecond, TotalAlloc: 509_739_008, HeapGrowth: 429_654_016},
		"rules/eight/invalid_sources":                       {Wall: 1525 * time.Millisecond, GoCPU: 898 * time.Millisecond, TotalAlloc: 505_872_384, HeapGrowth: 425_721_856},
		"rules/eight/missing_summary":                       {Wall: 860 * time.Millisecond, GoCPU: 861 * time.Millisecond, TotalAlloc: 509_739_008, HeapGrowth: 429_588_480},
		"rules/eight/missing_type":                          {Wall: 855 * time.Millisecond, GoCPU: 853 * time.Millisecond, TotalAlloc: 509_739_008, HeapGrowth: 429_457_408},
		"rules/eight/orphans":                               {Wall: 937 * time.Millisecond, GoCPU: 936 * time.Millisecond, TotalAlloc: 593_690_624, HeapGrowth: 497_942_528},
		"rules/eight/summary":                               {Wall: 2364 * time.Millisecond, GoCPU: 1403 * time.Millisecond, TotalAlloc: 937_754_624, HeapGrowth: 568_590_336},
		"rules/eight/undigested_sources":                    {Wall: 1608 * time.Millisecond, GoCPU: 1060 * time.Millisecond, TotalAlloc: 611_057_664, HeapGrowth: 510_590_976},
		"rules/eight/wanted":                                {Wall: 983 * time.Millisecond, GoCPU: 941 * time.Millisecond, TotalAlloc: 598_540_288, HeapGrowth: 503_119_872},
		"rules/wanted/100k-excluded-and-outside":            {Wall: 8846 * time.Millisecond, GoCPU: 4270 * time.Millisecond, TotalAlloc: 4_467_982_336, HeapGrowth: 479_133_696},
		"rules/wanted/hot-many-spellings-kinds":             {Wall: 9799 * time.Millisecond, GoCPU: 5081 * time.Millisecond, TotalAlloc: 2_881_748_992, HeapGrowth: 250_281_984},
		"rules/wanted/one-page-10000-edges":                 {Wall: 615 * time.Millisecond, GoCPU: 593 * time.Millisecond, TotalAlloc: 338_558_976, HeapGrowth: 232_914_944},
		"rules/wanted/relative-deep-folder":                 {Wall: 4144 * time.Millisecond, GoCPU: 2770 * time.Millisecond, TotalAlloc: 1_433_468_928, HeapGrowth: 277_020_672},
		"rules/wanted/same-last-key-presence":               {Wall: 720 * time.Millisecond, GoCPU: 820 * time.Millisecond, TotalAlloc: 392_495_104, HeapGrowth: 316_407_808},
		"rules/wanted/unique-long-targets":                  {Wall: 4534 * time.Millisecond, GoCPU: 4401 * time.Millisecond, TotalAlloc: 3_677_421_568, HeapGrowth: 1_120_010_240},
	}
	limit, ok := limits[group]
	return limit, ok
}
