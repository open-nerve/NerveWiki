//go:build !race

package bootstrap

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

// Fixed literals come from two independent successful calibration processes.
// Normal CI never derives a budget from the current run or an environment flag.
const virtualCostCalibration = false

type virtualCostMetric struct {
	WallNS, CPUNS                                                                   int64
	TotalAllocBytes, BaselineHeapBytes, SampledPeakBytes, SampledPeakIncrementBytes uint64
	SamplePeriodNS                                                                  int64
}

func virtualCostEvent(t *testing.T, kind, name, state string, data any) {
	t.Helper()
	phase := "guard"
	if virtualCostCalibration {
		phase = "calibration"
	}
	raw, err := json.Marshal(map[string]any{"Schema": "nervewiki.p4-s1.cost.v1", "Kind": kind, "Case": name, "Phase": phase, "State": state, "Data": data})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P4COST %s", raw)
}

// Go-process metrics include the real client/server and observer overhead, not
// PostgreSQL CPU/RSS. Sampled peak is a 1ms lower-bound observation. Setup,
// response oracles, JSON reserialization, ZIP parsing and EXPLAIN are outside.
func virtualCostMeasure(t *testing.T, run func() error) (virtualCostMetric, error) {
	return virtualCostMeasureFrom(t, run, true)
}

func virtualCostMeasureFrom(t *testing.T, run func() error, collect bool) (virtualCostMetric, error) {
	t.Helper()
	if collect {
		runtime.GC()
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	stop, done := make(chan struct{}), make(chan uint64, 1)
	go func() {
		peak := before.HeapAlloc
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			var now runtime.MemStats
			runtime.ReadMemStats(&now)
			peak = max(peak, now.HeapAlloc)
			select {
			case <-stop:
				done <- peak
				return
			case <-tick.C:
			}
		}
	}()
	cpu, at := originCPU(t), time.Now()
	err := run()
	elapsed := time.Since(at)
	cpu = originCPU(t) - cpu
	close(stop)
	peak := <-done
	runtime.ReadMemStats(&after)
	peak = max(peak, after.HeapAlloc)
	return virtualCostMetric{int64(elapsed), cpu, after.TotalAlloc - before.TotalAlloc, before.HeapAlloc, peak, peak - before.HeapAlloc, int64(time.Millisecond)}, err
}

type virtualCostStatement struct {
	Name, SQL string
	Args      []any
	Rows      int64
}
type virtualCostQuery struct {
	Name            string
	PID             uint32
	Snapshot        uint64
	ElapsedNS, Rows int64
	ArrayElements   []int
	Failed          bool
}
type virtualCostQueryKey struct{}
type virtualCostQueryStart struct {
	at        time.Time
	query     virtualCostQuery
	statement virtualCostStatement
}
type virtualCostTrace struct {
	mu         sync.Mutex
	active     bool
	queries    []virtualCostQuery
	statements []virtualCostStatement
}

func (c *virtualCostTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	active := c.active
	c.mu.Unlock()
	if !active || !strings.HasPrefix(d.SQL, "-- name: ") {
		return ctx
	}
	name := strings.Fields(d.SQL)[2]
	q := virtualCostQuery{Name: name, PID: conn.PgConn().PID()}
	q.Snapshot, _ = ctx.Value(lintSnapshotKey{}).(uint64)
	s := virtualCostStatement{Name: name, SQL: d.SQL}
	for _, a := range d.Args {
		if _, mode := a.(pgx.QueryExecMode); mode {
			continue
		}
		switch v := a.(type) {
		case []uuid.UUID:
			a = slices.Clone(v)
			q.ArrayElements = append(q.ArrayElements, len(v))
		case []string:
			a = slices.Clone(v)
			q.ArrayElements = append(q.ArrayElements, len(v))
		case []int32:
			a = slices.Clone(v)
			q.ArrayElements = append(q.ArrayElements, len(v))
		}
		s.Args = append(s.Args, a)
	}
	return context.WithValue(ctx, virtualCostQueryKey{}, virtualCostQueryStart{time.Now(), q, s})
}
func (c *virtualCostTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	s, ok := ctx.Value(virtualCostQueryKey{}).(virtualCostQueryStart)
	if !ok {
		return
	}
	s.query.ElapsedNS = int64(time.Since(s.at))
	s.query.Rows = d.CommandTag.RowsAffected()
	s.query.Failed = d.Err != nil
	s.statement.Rows = s.query.Rows
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = append(c.queries, s.query)
	c.statements = append(c.statements, s.statement)
}
func (c *virtualCostTrace) start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = true
	c.queries = nil
	c.statements = nil
}
func (c *virtualCostTrace) stop() ([]virtualCostQuery, []virtualCostStatement) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active = false
	return slices.Clone(c.queries), slices.Clone(c.statements)
}

type virtualCostFixture struct {
	*virtualExportLoad
	sql                 *virtualCostTrace
	ids                 []uuid.UUID
	bulk                []uuid.UUID
	foreign             string
	defaultMode         string
	measureContribution atomic.Bool
	contributions       chan virtualCostMetric
}

func virtualCostTables() []string {
	return []string{"nodes", "page_contents", "page_revisions", "changesets", "changeset_items", "indexed_pages", "page_properties", "page_links", "asset_blobs"}
}

func newVirtualCostFixture(t *testing.T) *virtualCostFixture {
	t.Helper()
	started := time.Now()
	tr := &virtualCostTrace{}
	f := &virtualCostFixture{sql: tr, contributions: make(chan virtualCostMetric, 4)}
	f.virtualExportLoad = newVirtualExportLoadConfigured(t, "CostWiki", func(c *config.Config) {
		if c.Server.RequestTimeout != 4*time.Second || c.Database.CommitTimeout != 2*time.Second || c.Database.MaxConns != 4 || c.Page.SearchIndexInterval != 0 {
			t.Fatalf("changed production request/commit/pool configuration: %+v %+v", c.Server, c.Database)
		}
	}, tr, func(d *transfer.Deps) {
		if len(d.Contributors) != 1 {
			t.Fatalf("actual virtual contributor count %d", len(d.Contributors))
		}
		actual := d.Contributors[0]
		d.Contributors = []transfer.ExportContributor{virtualContribution(func(ctx context.Context, scope transfer.ExportScope, sink transfer.ExportSink) error {
			if !f.measureContribution.Load() {
				return actual.Contribute(ctx, scope, sink)
			}
			metric, err := virtualCostMeasureFrom(t, func() error { return actual.Contribute(ctx, scope, sink) }, false)
			f.contributions <- metric
			return err
		})}
	})
	f.defaultMode = f.read.pool.Config().ConnConfig.DefaultQueryExecMode.String()
	for _, table := range virtualCostTables() {
		f.tm.originExec(t, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	parent := f.book.raw
	for i := range 10 {
		if i == 9 {
			parent = f.book.raw
		} // raw is already level one; retain ten legal source pages.
		parent = f.tm.createPageWith(t, "alice", f.book.id, parent, fmt.Sprintf("Source%d", i), fmt.Sprintf("real source payload %d", i))
	}
	f.tm.upload(t, "alice", f.book.id, f.book.raw, "pending.bin", "real pending asset")
	f.tm.upload(t, "alice", f.book.id, f.prototype, "attached.bin", "real attached asset")
	f.bulk = f.seedAfter(t, 98986, false, 0, 13)
	var body strings.Builder
	body.WriteString("---\ntype: z-emitter\nsummary: actual resolved inbound\n---\n[[Prototype]]\n")
	for i := range 199 {
		fmt.Fprintf(&body, "[[page%06d]]\n", i)
	}
	emitter := f.tm.createPageWith(t, "alice", f.book.id, "", "ZZEmitter", body.String())
	prototype := f.prototype
	f.prototype = emitter
	emitters := f.seedAfter(t, 1000, false, 98986, 99000)
	f.prototype = prototype
	f.ids = append([]uuid.UUID{uuid.MustParse(f.book.raw), uuid.MustParse(f.book.agents), uuid.MustParse(prototype), uuid.MustParse(emitter)}, f.bulk...)
	f.ids = append(f.ids, emitters...)
	slices.SortFunc(f.ids, uuid.UUID.Compare)
	if len(f.ids) != 99990 {
		t.Fatalf("independent eligible ID seed has %d pages", len(f.ids))
	}
	// Complete current/history/parser data in a larger *same-database* notebook
	// makes notebook/ID bounded queries encounter realistic physical indexes.
	other := &virtualExportLoad{tm: f.tm, book: f.tm.llmWiki(t, "ForeignCost")}
	other.prototype = f.tm.createPageWith(t, "alice", other.book.id, "", "Prototype", "---\ntype: hidden\nsummary: unrelated notebook\n---\nbody")
	other.seed(t, 199997, false)
	f.foreign = other.book.id
	if got := count(t, f.tm.pool, `SELECT count(*) FROM (SELECT resolved_id FROM page_links WHERE notebook_id=$1 AND resolved_id IS NOT NULL GROUP BY resolved_id HAVING count(DISTINCT source_id)=1001) q`, f.book.id); got != 200 {
		t.Fatalf("independent worst inbound seed has %d targets, want 200 with 1001 sources", got)
	}
	if got := count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id=$1`, f.book.id); got <= 1000 {
		t.Fatalf("real changeset seed=%d", got)
	}
	sourceIndexSettle(t, f.tm.pool)
	// testConfig disables the search worker, as in the existing HTTP cost
	// fixtures. Pending search projections are inert during virtual reads;
	// complete page content/history and current link/parser facts are present.
	virtualCostEvent(t, "seed", "virtual-files", "no_statistics", map[string]any{"ElapsedNS": int64(time.Since(started)), "Pages": 100000, "ForeignPages": 200000, "Assets": 2, "SourcePages": 10, "SourceDepth": 10, "EligibleIDs": len(f.ids), "InboundTargets": 200, "InboundSourcesPerTarget": 1001, "Changesets": count(t, f.tm.pool, `SELECT count(*) FROM changesets WHERE notebook_id=$1`, f.book.id), "Extractor": 2, "Statistics": f.statistics(t, false), "RequestTimeoutNS": int64(4 * time.Second), "CommitTimeoutNS": int64(2 * time.Second), "DefaultQueryExecMode": f.defaultMode, "SearchIndexIntervalNS": 0, "PendingSearchProjections": count(t, f.tm.pool, "SELECT count(*) FROM page_search_queue")})
	return f
}

func (f *virtualCostFixture) statistics(t *testing.T, analyzed bool) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range virtualCostTables() {
		var disabled bool
		if err := f.tm.pool.QueryRow(t.Context(), `SELECT coalesce(reloptions,'{}') @> ARRAY['autovacuum_enabled=false'] FROM pg_class WHERE oid=$1::regclass`, table).Scan(&disabled); err != nil || !disabled {
			t.Fatalf("actual automatic statistics %s: %t %v", table, disabled, err)
		}
		n := count(t, f.tm.pool, `SELECT count(*) FROM pg_stats WHERE schemaname=current_schema() AND tablename=$1`, table)
		out[table] = n
		if (n > 0) != analyzed {
			t.Fatalf("actual statistics %s=%d analyzed=%t", table, n, analyzed)
		}
	}
	return out
}

// Reading response bodies is delegated unchanged; a tee records bytes the SDK
// actually consumed. It neither pre-reads nor replaces/reformats the response.
type virtualWire struct {
	mu     sync.Mutex
	active bool
	bodies []*bytes.Buffer
}
type virtualWireBody struct {
	io.ReadCloser
	wire *virtualWire
	buf  *bytes.Buffer
}

func (b virtualWireBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.wire.mu.Lock()
	_, _ = b.buf.Write(p[:n])
	b.wire.mu.Unlock()
	return n, err
}
func (w *virtualWire) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		return res, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active && r.Method == http.MethodPost {
		b := &bytes.Buffer{}
		w.bodies = append(w.bodies, b)
		res.Body = virtualWireBody{res.Body, w, b}
	}
	return res, nil
}
func (w *virtualWire) start() { w.mu.Lock(); defer w.mu.Unlock(); w.active = true; w.bodies = nil }
func (w *virtualWire) stop() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.active = false
	out := make([][]byte, len(w.bodies))
	for i, b := range w.bodies {
		out[i] = bytes.Clone(b.Bytes())
	}
	return out
}

func (f *virtualCostFixture) client(t *testing.T) (*mcp.ClientSession, *virtualWire) {
	t.Helper()
	m := f.tm.agent(t, "alice", f.book.id)
	wire := &virtualWire{}
	client := mcp.NewClient(&mcp.Implementation{Name: "virtual-cost", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: f.tm.base + atNotebook(f.book.id), HTTPClient: &http.Client{Transport: withToken{m.pat, wire}, Timeout: 30 * time.Second}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, wire
}

func virtualCostCounts(queries []virtualCostQuery) map[string]int {
	out := map[string]int{}
	for _, q := range queries {
		out[q.Name]++
	}
	return out
}

func TestVirtualIndexOfAHundredThousandPagesTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("real 100k SDK/read/directory cost runs alone without race")
	}
	f := newVirtualCostFixture(t)
	client, wire := f.client(t)
	for _, state := range []string{"no_statistics", "analyzed"} {
		t.Run(state, func(t *testing.T) {
			if state == "analyzed" {
				for _, table := range virtualCostTables() {
					f.tm.originExec(t, "ANALYZE "+table)
				}
			}
			f.statistics(t, state == "analyzed")
			for _, name := range []string{"virtual-read-index", "virtual-read-log", "virtual-directory"} {
				t.Run(name, func(t *testing.T) {
					var samples [3]virtualCostMetric
					for ordinal := range samples {
						f.sql.start()
						wire.start()
						snapshots := f.read.next.Load()
						var result *mcp.CallToolResult
						var response []byte
						var status int
						metric, err := virtualCostMeasure(t, func() error {
							ctx, cancel := context.WithTimeout(t.Context(), 4*time.Second)
							defer cancel()
							if name == "virtual-directory" {
								var e error
								status, response, e = virtualCostHTTP(ctx, http.MethodGet, f.tm.base+"/api/v0/notebooks/"+f.book.id+"/index", f.tm.tokens["alice"], nil)
								return e
							}
							file := "index.md"
							if name == "virtual-read-log" {
								file = "log.md"
							}
							var e error
							result, e = client.CallTool(ctx, &mcp.CallToolParams{Name: "read", Arguments: map[string]any{"path": file}})
							return e
						})
						queries, statements := f.sql.stop()
						bodies := wire.stop()
						samples[ordinal] = metric
						counts := virtualCostCounts(queries)
						if err != nil {
							t.Fatalf("actual %s under original 4s request: %v", name, err)
						}
						output, actualWire, reserialized := 0, 0, 0
						if name == "virtual-directory" {
							output = len(response)
							actualWire = len(response)
							f.checkDirectory(t, status, response, statements)
							if f.read.next.Load() != snapshots {
								t.Fatal("ordinary directory acquired a virtual snapshot")
							}
						} else {
							if result == nil || result.IsError || len(result.Content) != 2 {
								t.Fatalf("actual SDK virtual read: %+v", result)
							}
							text := result.Content[1].(*mcp.TextContent).Text
							output = len(text)
							encoded, e := json.Marshal(result)
							if e != nil {
								t.Fatal(e)
							}
							reserialized = len(encoded)
							if len(bodies) != 1 || !json.Valid(bodies[0]) {
								t.Fatalf("actual SDK HTTP response bodies=%d", len(bodies))
							}
							actualWire = len(bodies[0])
							if output > 40<<10 || actualWire > 48<<10 || reserialized > 48<<10 || f.read.next.Load() != snapshots+1 || counts["IndexCatalogueHeads"] != 0 {
								t.Fatalf("actual virtual read bounds/snapshot/catalogue: output%d wire%d reserialized%d counts%v", output, actualWire, reserialized, counts)
							}
							if name == "virtual-read-index" {
								if counts["IndexPages"] != 1 || counts["InboundCounts"] != 1 || strings.Count(text, "; inbound: 1000") != 200 || !strings.Contains(text, "Showing 200 of 99990 pages.") {
									t.Fatalf("independent 200 selected pages/worst inbound oracle: counts%v text%.300s", counts, text)
								}
								for _, path := range []string{"Prototype.md", "page000000.md", "page000198.md"} {
									if !strings.Contains(text, "](<"+path+">)") {
										t.Fatalf("known selected path %s absent", path)
									}
								}
							} else if counts["ListChangesets"] != 1 || counts["IndexPages"] != 0 || counts["InboundCounts"] != 0 || !strings.Contains(text, "Showing 50 of the most recent 50 changesets.") {
								t.Fatalf("independent bounded log oracle: counts%v text%.300s", counts, text)
							}
						}
						if f.lintEvents.Load() != 0 {
							t.Fatal("virtual read or catalogue called lint")
						}
						virtualCostEvent(t, "sample", name, state, map[string]any{"Ordinal": ordinal + 1, "Metrics": metric, "OutputBytes": output, "ActualHTTPBodyBytes": actualWire, "ReserializedResultBytes": reserialized, "Queries": queries, "Counts": counts, "DefaultQueryExecMode": f.defaultMode, "Scope": "actual HTTP/SDK call to response consumption; setup/oracles/serialization/plan outside; Go process includes observer"})
						if ordinal == 2 {
							f.plans(t, name, state, statements)
						}
					}
					virtualCostGuard(t, name, state, samples)
				})
			}
			// The real existing MCP index consumer never asks for the REST catalogue.
			f.sql.start()
			answer, err := client.CallTool(t.Context(), &mcp.CallToolParams{Name: "index", Arguments: map[string]any{"limit": 100}})
			queries, _ := f.sql.stop()
			counts := virtualCostCounts(queries)
			if err != nil || answer.IsError || counts["IndexPages"] != 1 || counts["IndexCatalogueHeads"] != 0 {
				t.Fatalf("real MCP index changed catalogue opt-in: %v %+v %v", err, answer, counts)
			}
			data, _ := json.Marshal(answer.StructuredContent)
			var index struct {
				Pages []json.RawMessage `json:"pages"`
			}
			if err := json.Unmarshal(data, &index); err != nil || len(index.Pages) != 100 {
				t.Fatalf("real MCP index is empty/short: %d %v", len(index.Pages), err)
			}
			virtualCostEvent(t, "control", "mcp-index", state, map[string]any{"Items": 100, "CatalogueCalls": counts["IndexCatalogueHeads"], "Counts": counts})
		})
	}
}

func (f *virtualCostFixture) checkDirectory(t *testing.T, status int, raw []byte, statements []virtualCostStatement) {
	t.Helper()
	var body struct {
		Data []struct {
			PageID uuid.UUID `json:"page_id"`
		}
		Types []struct {
			Type  string
			Count int
		}
		TypesComplete bool `json:"types_complete"`
		Unfilterable  int  `json:"unfilterable_type_pages"`
		Mode          string
		Source        struct {
			Root    uuid.UUID `json:"root_id"`
			Sources int
		} `json:"source_area"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || status != 200 {
		t.Fatalf("actual REST catalogue: status%d %v %.300s", status, err, raw)
	}
	if body.Mode != "llm_wiki" || !body.TypesComplete || body.Unfilterable != 0 || len(body.Types) != 2 || body.Types[0].Type != "note" || body.Types[0].Count != 98987 || body.Types[1].Type != "z-emitter" || body.Types[1].Count != 1001 || body.Source.Root != uuid.MustParse(f.book.raw) || body.Source.Sources != 11 || len(body.Data) != 50 {
		t.Fatalf("real REST catalogue independent counts: %+v", body)
	}
	want := append([]uuid.UUID{uuid.MustParse(f.prototype)}, f.bulk[:49]...)
	var got []uuid.UUID
	for _, p := range body.Data {
		got = append(got, p.PageID)
	}
	if !slices.Equal(got, want) {
		t.Fatal("REST selected IDs differ from independent creation order")
	}
	var ids []uuid.UUID
	calls := 0
	for _, s := range statements {
		if s.Name == "IndexCatalogueHeads" {
			calls++
			batch := s.Args[len(s.Args)-1].([]uuid.UUID)
			if len(batch) > 512 {
				t.Fatal("REST catalogue sent more than 512 IDs")
			}
			ids = append(ids, batch...)
		}
	}
	if calls != 196 || !slices.Equal(ids, f.ids) {
		t.Fatalf("real REST catalogue did not drain complete source-excluded IDs: %d/%d", calls, len(ids))
	}
}

func TestVirtualExportOfAHundredThousandPagesTakesItsTime(t *testing.T) {
	if testing.Short() {
		t.Skip("real 100k export ZIP cost runs alone without race")
	}
	f := newVirtualCostFixture(t)
	for _, state := range []string{"no_statistics", "analyzed"} {
		t.Run(state, func(t *testing.T) {
			if state == "analyzed" {
				for _, table := range virtualCostTables() {
					f.tm.originExec(t, "ANALYZE "+table)
				}
			}
			f.statistics(t, state == "analyzed")
			var samples [3]virtualCostMetric
			var contributionSamples [3]virtualCostMetric
			for ordinal := range samples {
				f.sql.start()
				f.measureContribution.Store(true)
				snapshots := f.export.next.Load()
				var archive []byte
				metric, err := virtualCostMeasure(t, func() error { var e error; archive, e = f.exportArchive(t.Context()); return e })
				queries, statements := f.sql.stop()
				f.measureContribution.Store(false)
				samples[ordinal] = metric
				counts := virtualCostCounts(queries)
				if err != nil {
					t.Fatalf("actual export job/download: %v", err)
				}
				if len(f.contributions) != 1 {
					t.Fatalf("actual export contribution metrics=%d, want exactly one", len(f.contributions))
				}
				contributionSamples[ordinal] = <-f.contributions
				if counts["InboundCounts"] != 0 || counts["IndexCatalogueHeads"] != 0 || counts["IndexPages"] != 1 || counts["ListChangesets"] != 1 || f.export.next.Load() != snapshots+1 || f.lintEvents.Load() != 0 {
					t.Fatalf("actual export query/snapshot profile: %v", counts)
				}
				zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
				if err != nil {
					t.Fatal(err)
				}
				markdownFiles := 0
				contents := map[string]string{}
				for _, entry := range zr.File {
					if strings.HasSuffix(entry.Name, ".md") {
						markdownFiles++
					}
					if entry.Name == "CostWiki/index.md" || entry.Name == "CostWiki/log.md" || entry.Name == "CostWiki/.nerve/meta.json" || entry.Name == "CostWiki/Prototype.md" {
						r, e := entry.Open()
						if e != nil {
							t.Fatal(e)
						}
						b, e := io.ReadAll(r)
						_ = r.Close()
						if e != nil {
							t.Fatal(e)
						}
						contents[entry.Name] = string(b)
					}
				}
				index, log := contents["CostWiki/index.md"], contents["CostWiki/log.md"]
				if markdownFiles != 100002 || len(index) > 2<<20 || len(log) > 1<<20 || !strings.Contains(index, "Showing 10000 of 99990 pages.") || !strings.Contains(log, "Showing 1000 of the most recent 1000 changesets.") || !strings.Contains(index, "](<Prototype.md>)") || contents["CostWiki/Prototype.md"] != "---\ntype: note\nsummary: bounded fixture\n---\nbody" {
					t.Fatalf("actual 100k ZIP/content oracle: pages%d index%d log%d", markdownFiles, len(index), len(log))
				}
				_, _, contributed := metaPaths(t, contents["CostWiki/.nerve/meta.json"])
				if !slices.Equal(contributed, []string{"index.md", "log.md"}) {
					t.Fatalf("100k contributed metadata: %v", contributed)
				}
				virtualCostEvent(t, "sample", "virtual-export-whole", state, map[string]any{"Ordinal": ordinal + 1, "Metrics": metric, "OutputBytes": len(archive), "IndexBytes": len(index), "LogBytes": len(log), "RealMarkdownFiles": markdownFiles - 2, "Queries": queries, "Counts": counts, "Scope": "actual start job through downloaded ZIP body; setup and ZIP parsing/oracles/EXPLAIN outside"})
				virtualCostEvent(t, "sample", "virtual-export-contributor", state, map[string]any{"Ordinal": ordinal + 1, "Metrics": contributionSamples[ordinal], "OutputBytes": len(index) + len(log), "IndexBytes": len(index), "LogBytes": len(log), "Scope": "actual contributor inside the existing export RR with complete actual scope and actual ZIP sink; no extra GC; Go-process observation includes concurrent client/observer work"})
				if ordinal == 2 {
					f.plans(t, "virtual-export-whole", state, statements)
				}
			}
			virtualCostGuard(t, "virtual-export-whole", state, samples)
			virtualCostGuard(t, "virtual-export-contributor", state, contributionSamples)
		})
	}
}

func virtualCostHTTP(ctx context.Context, method, address, token string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	return res.StatusCode, b, err
}

func (f *virtualCostFixture) exportArchive(ctx context.Context) ([]byte, error) {
	status, raw, err := virtualCostHTTP(ctx, http.MethodPost, f.tm.base+"/api/v0/notebooks/"+f.book.id+"/exports", f.tm.tokens["alice"], []byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("start export: %w", err)
	}
	if status != http.StatusAccepted {
		return nil, fmt.Errorf("start export: status%d %s", status, raw)
	}
	var job transferJob
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, err
	}
	id := job.ID
	for deadline := time.Now().Add(jobWait); ; time.Sleep(50 * time.Millisecond) {
		status, raw, err = virtualCostHTTP(ctx, http.MethodGet, f.tm.base+"/api/v0/transfer-jobs/"+id, f.tm.tokens["alice"], nil)
		if err != nil {
			return nil, fmt.Errorf("poll export: %w", err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("poll export: status%d", status)
		}
		if err := json.Unmarshal(raw, &job); err != nil {
			return nil, err
		}
		if job.State == "succeeded" && job.Download != nil {
			break
		}
		if job.State != "running" && job.State != "queued" {
			return nil, fmt.Errorf("export failed: %s", raw)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("export did not finish within original job wait")
		}
	}
	status, raw, err = virtualCostHTTP(ctx, http.MethodGet, f.tm.base+job.Download.URL, "", nil)
	if err != nil {
		return nil, fmt.Errorf("download ZIP: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("download ZIP: status%d", status)
	}
	return raw, nil
}

func virtualCostGuard(t *testing.T, name, state string, samples [3]virtualCostMetric) {
	t.Helper()
	medians := map[string]uint64{}
	for _, metric := range []string{"WallNS", "CPUNS", "TotalAllocBytes", "SampledPeakIncrementBytes"} {
		var values [3]uint64
		for i, s := range samples {
			switch metric {
			case "WallNS":
				values[i] = uint64(s.WallNS)
			case "CPUNS":
				values[i] = uint64(s.CPUNS)
			case "TotalAllocBytes":
				values[i] = s.TotalAllocBytes
			default:
				values[i] = s.SampledPeakIncrementBytes
			}
		}
		slices.Sort(values[:])
		medians[metric] = values[1]
	}
	limits, known := map[string]map[string]uint64{
		"virtual-directory": {
			"WallNS":                    1865567625,
			"CPUNS":                     936228000,
			"TotalAllocBytes":           554644536,
			"SampledPeakIncrementBytes": 151983048,
		},
		"virtual-export-contributor": {
			"WallNS":                    2443183002,
			"CPUNS":                     2740134000,
			"TotalAllocBytes":           2879099568,
			"SampledPeakIncrementBytes": 335038608,
		},
		"virtual-export-whole": {
			"WallNS":                    5738227254,
			"CPUNS":                     6159636000,
			"TotalAllocBytes":           5306074272,
			"SampledPeakIncrementBytes": 528859608,
		},
		"virtual-read-index": {
			"WallNS":                    2591535876,
			"CPUNS":                     1625994000,
			"TotalAllocBytes":           1095755136,
			"SampledPeakIncrementBytes": 270810024,
		},
		"virtual-read-log": {
			"WallNS":                    68649624,
			"CPUNS":                     17772000,
			"TotalAllocBytes":           16709208,
			"SampledPeakIncrementBytes": 16709208,
		},
	}[name]
	if !known {
		t.Fatalf("no reviewed fixed cost limits for %s", name)
	}
	accepted := !virtualCostCalibration
	for metric, median := range medians {
		if median == 0 || median > limits[metric] {
			accepted = false
		}
	}
	virtualCostEvent(t, "guard", name, state, map[string]any{"Medians": medians, "Limits": limits, "Accepted": accepted, "CalibrationOnly": virtualCostCalibration})
	if !virtualCostCalibration && !accepted {
		t.Fatalf("%s costs exceed fixed limits: medians=%v limits=%v", name, medians, limits)
	}
}
