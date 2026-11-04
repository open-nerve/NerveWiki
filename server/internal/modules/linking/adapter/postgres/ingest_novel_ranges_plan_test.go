//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/markdown"
	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// This is a sparse-range adapter guard, not the bootstrap request's cost gate.
// A state gets its own database. The native entry runs each state separately.
func TestIngestNovelSourceRanges(t *testing.T) {
	for _, state := range []string{"no_statistics", "analyzed/pristine", "analyzed/app_rebuild"} {
		t.Run(state, func(t *testing.T) {
			f, q, want := novelRangeFixture(t, state)
			var got app.IngestCandidateEvidence
			if err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
				if err := f.s.Lock(ctx, f.eng); err != nil {
					return err
				}
				var err error
				got, err = postgresadapter.NewIngestReads(f.pool, 5<<20).NovelSources(ctx, q)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			var actual []novelRangeRow
			for _, r := range got.Candidates {
				actual = append(actual, novelRangeRow{"candidate", r.ID, r.Asset, r.Direct, r.FollowedUntouched, false, false})
			}
			for _, r := range got.Parents {
				actual = append(actual, novelRangeRow{"parent", r.ID, false, r.Direct, false, false, false})
			}
			novelRangeRows(t, actual, want)
			ids, assets, revisions := []uuid.UUID{}, []bool{}, []int32{}
			for _, c := range q.Candidates {
				ids = append(ids, c.ID)
				assets = append(assets, c.Asset)
				revisions = append(revisions, int32(c.Revision))
			}
			current, currentRevisions := []uuid.UUID{}, []int32{}
			for _, c := range q.CurrentSources {
				current = append(current, c.ID)
				currentRevisions = append(currentRevisions, int32(c.Revision))
			}
			s := statementOf(t, func(g *gen.Queries) error {
				_, err := g.IngestNovelSources(t.Context(), gen.IngestNovelSourcesParams{
					CandidateIds: ids, CandidateAssets: assets, CandidateRevisions: revisions,
					CurrentSourceIds: current, CurrentSourceRevisions: currentRevisions,
					NotebookID: f.eng, Extractor: int32(domain.Extractor), ParentIds: q.ParentIDs,
					EligibleEmitterIds: q.EligibleEmitters, UntouchedSourceIds: q.UntouchedSources,
					MaxSlotItems: 10000, MaxJsonBytes: app.MaxLintPayloadBytes(5 << 20), PossibleOldDirectIds: q.PossibleOldDirect,
				})
				return err
			})
			wantArgs := []any{ids, assets, revisions, current, currentRevisions, f.eng, int32(domain.Extractor), q.ParentIDs, q.EligibleEmitters, q.UntouchedSources, int32(10000), app.MaxLintPayloadBytes(5 << 20), q.PossibleOldDirect}
			if !reflect.DeepEqual(s.args, wantArgs) {
				t.Fatalf("actual generated 13-argument Novel binding differs: got %v want %v", s.args, wantArgs)
			}
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					var raw []novelRangeRow
					_, ev := lintExecutedMode(t, f, s, mode, func(rows pgx.Rows, _ int) {
						var r novelRangeRow
						if err := rows.Scan(&r.Kind, &r.ID, &r.Asset, &r.Direct, &r.Followed, &r.Invalid, &r.Proof); err != nil {
							t.Fatal(err)
						}
						raw = append(raw, r)
					})
					novelRangeRows(t, raw, want) // Complete ordered 403 rows, including both false flags on each.
					if ev.Rows != len(want) {
						t.Fatalf("drained rows %d != %d", ev.Rows, len(want))
					}
					novelRangePhysical(t, ev, state, lintRangeIndexes(t, f))
				})
			}
		})
	}
}

type novelRangeRow struct {
	Kind                                    string
	ID                                      uuid.UUID
	Asset, Direct, Followed, Invalid, Proof bool
}

func novelRangeID(n uint64) uuid.UUID   { id := lintID(n); id[0] = 0x68; return id }
func novelForeignID(n uint64) uuid.UUID { id := lintID(n); id[0] = 0x69; return id }

func novelRangeRows(t *testing.T, got, want []novelRangeRow) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("complete literal Novel output\ngot  %+v\nwant %+v", got, want)
	}
	var novel []uuid.UUID
	for _, r := range got {
		if r.Kind == "candidate" && !r.Direct && !r.Followed {
			novel = append(novel, r.ID)
		}
	}
	if !slices.Equal(novel, []uuid.UUID{novelRangeID(3)}) {
		t.Fatalf("only T is novel: %v", novel)
	}
}

// Facts and offsets are determined while constructing literal Markdown. No
// Parse/Extract/Novel implementation is used to generate any expected value.
func novelRangeSpec(id uuid.UUID, name string, targets []string, note bool) stablePlanSpec {
	s := stablePlanSpec{ID: id, Name: name}
	var b strings.Builder
	b.WriteString("---\nlabel: fixture\n")
	s.OldFacts = domain.Facts{FrontmatterValid: true, Properties: []domain.Property{{Key: "label", Value: []byte(`"fixture"`)}}}
	if note {
		b.WriteString("notes: '")
		start := b.Len() + 2
		b.WriteString("[[a.bin]]'\n")
		s.OldFacts.Links = append(s.OldFacts.Links, domain.Link{Kind: "wikilink", Property: "notes", Target: "a.bin", Start: start, End: start + 5, Quote: '\''})
		s.OldFacts.Properties = append(s.OldFacts.Properties, domain.Property{Key: "notes", Value: []byte(`"[[a.bin]]"`)})
	}
	b.WriteString("---\n")
	for _, target := range targets {
		start := b.Len() + 2
		b.WriteString("[[" + target + "]]\n")
		s.OldFacts.Links = append(s.OldFacts.Links, domain.Link{Kind: "wikilink", Target: target, Start: start, End: start + len(target)})
	}
	s.OldText = b.String()
	return s
}

func novelRangeFixture(t *testing.T, state string) (fixture, app.IngestCandidateQuery, []novelRangeRow) {
	t.Helper()
	f := newFixture(t)
	ingestPlanSetup(t, f)
	indexes := lintRangeIndexes(t, f)
	if definition := indexes["page_links_resolved_id_source_id_idx"]; !strings.Contains(definition, "btree (resolved_id, source_id, range_start)") || !strings.Contains(definition, "resolved_id IS NOT NULL") {
		t.Fatalf("reverse index differs from the global-range ledger: %q", definition)
	}
	lintEvidence(t, "novel-range-indexes", indexes)
	p := &stablePlanInputs{notebook: f.eng, nodes: map[uuid.UUID]domain.Node{}, byKey: map[string][]domain.Node{}}
	p.specs = append(p.specs, novelRangeSpec(novelRangeID(3), "t", nil, false), novelRangeSpec(novelRangeID(4), "s", []string{"a.bin"}, false), novelRangeSpec(novelRangeID(5), "u", nil, false))
	tail := make([]string, 512, 514)
	for i := range tail {
		tail[i] = "junk.bin"
	}
	tail = append(tail, "missing", "s")
	p.specs = append(p.specs, novelRangeSpec(novelRangeID(7), "qf", tail, true), novelRangeSpec(novelRangeID(8), "qb", []string{"b.bin"}, false))
	noise := make([]string, 128)
	for i := range noise {
		noise[i] = "junk.bin"
	}
	for i := range 400 {
		name := fmt.Sprintf("source-%03d", i)
		p.specs = append(p.specs, novelRangeSpec(novelRangeID(uint64(1000+i)), name, noise, false), novelRangeSpec(novelRangeID(uint64(2000+i)), fmt.Sprintf("emitter-%03d", i), []string{name}, false))
	}
	slices.SortFunc(p.specs, func(a, b stablePlanSpec) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	addNode := func(id uuid.UUID, name string, asset bool) {
		n := domain.Node{ID: id, Asset: asset, Path: []domain.Step{{ID: id, Key: name, Name: name}}}
		p.nodes[id], p.byKey[name] = n, []domain.Node{n}
	}
	for _, s := range p.specs {
		addNode(s.ID, s.Name, false)
	}
	addNode(novelRangeID(1), "a.bin", true)
	addNode(novelRangeID(2), "b.bin", true)
	addNode(novelRangeID(6), "junk.bin", true)
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	parser := markdownadapter.NewParser(md, markdown.NewBudget(8<<20, 2*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))))
	var wantLinks, resolved []app.Link
	for _, s := range p.specs {
		facts := stablePlanParsed(t, parser, s.OldText, s.OldFacts)
		if err := f.s.AddPage(t.Context(), app.Page{ID: s.ID, NotebookID: f.eng, Revision: 1}, facts); err != nil {
			t.Fatal(err)
		}
		for _, l := range s.OldFacts.Links {
			r := app.Link{SourceID: s.ID, Start: l.Start, Target: l.Target}
			if nodes := p.byKey[l.Target]; len(nodes) == 1 {
				r.Resolution = domain.Resolution{ID: nodes[0].ID, Asset: nodes[0].Asset}
			}
			wantLinks = append(wantLinks, r)
			if r.Resolution.ID != uuid.Nil() {
				resolved = append(resolved, r)
			}
		}
	}
	if err := f.s.SetResolutions(t.Context(), resolved); err != nil {
		t.Fatal(err)
	}
	novelRangeForeignSeed(t, f)
	// Remove seed-only dead versions without creating column statistics.
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "VACUUM "+table)
	}
	check := func() { novelRangeFixtureCheck(t, f, p, wantLinks) }
	check()
	if state != "no_statistics" {
		lintAnalyze(t, f)
	}
	if strings.HasSuffix(state, "/app_rebuild") {
		// Reuse the existing small input ports, but this fixture supplies all
		// 805 page bodies plus real flat attachment metadata. Revision is 1.
		trace := &stablePlanTrace{}
		pool := lintPool(t, f, trace, 6) // lintExecutedMode also uses pool reads while holding its transaction.
		f.pool, f.s, f.tx = pool, postgresadapter.New(pool), postgres.NewTxManager(pool, time.Second)
		trace.begin()
		r := app.Rebuild{Tx: f.tx, Index: app.Index{Store: f.s, Pages: p, Publisher: p}, Contents: p, Notebooks: p, Parser: parser}
		got, err := r.Notebook(t.Context(), f.eng)
		history := trace.end()
		want := app.Rebuilt{Pages: 805, Links: 52117, Unresolved: 1}
		if err != nil || !reflect.DeepEqual(got, want) || p.locked != 1 || p.rekeyed != 1 {
			t.Fatalf("real Rebuild = %+v lock=%d rekey=%d err=%v", got, p.locked, p.rekeyed, err)
		}
		if !slices.EqualFunc(p.events, []app.LinksChanged{{WorkspaceID: stablePlanID(900003), NotebookID: f.eng}}, func(a, b app.LinksChanged) bool { return reflect.DeepEqual(a, b) }) {
			t.Fatalf("complete Rebuild event: %+v", p.events)
		}
		written := map[string]int64{}
		for _, call := range history {
			if call.Error != "" {
				t.Fatalf("real Rebuild SQL %s: %s", call.Name, call.Error)
			}
			written[call.Name] += call.RowsAffected
		}
		if written["DeleteNotebooksLinks"] != 52117 || written["InsertLinks"] != 52117 || written["SetResolutions"] != 52116 || !slices.Equal(trace.terminal(), []string{"commit"}) {
			t.Fatalf("real Rebuild version history: rows=%v terminal=%v", written, trace.terminal())
		}
		lintEvidence(t, "novel-real-rebuild-history", history)
		check() // No post-Rebuild VACUUM, ANALYZE, manual Set, or row repair.
	}
	q := app.IngestCandidateQuery{NotebookID: f.eng, Candidates: []app.IngestSource{{ID: novelRangeID(1), Asset: true}, {ID: novelRangeID(2), Asset: true}, {ID: novelRangeID(3), Revision: 1}}, UntouchedSources: []uuid.UUID{novelRangeID(3), novelRangeID(4), novelRangeID(5)}, EligibleEmitters: []uuid.UUID{novelRangeID(7), novelRangeID(8)}}
	for _, n := range []uint64{3, 4, 5} {
		q.CurrentSources = append(q.CurrentSources, app.IngestRevision{ID: novelRangeID(n), Revision: 1})
	}
	want := []novelRangeRow{{"candidate", novelRangeID(1), true, false, true, false, false}, {"candidate", novelRangeID(2), true, true, false, false, false}, {"candidate", novelRangeID(3), false, false, false, false, false}}
	for i := range 400 {
		id := novelRangeID(uint64(1000 + i))
		q.CurrentSources = append(q.CurrentSources, app.IngestRevision{ID: id, Revision: 1})
		q.ParentIDs = append(q.ParentIDs, id)
		q.EligibleEmitters = append(q.EligibleEmitters, novelRangeID(uint64(2000+i)))
		want = append(want, novelRangeRow{"parent", id, false, true, false, false, false})
	}
	lintEvidence(t, "novel-sparse-fixture", map[string]any{"state": state, "current": 403, "untouched": 3, "eligible": 402, "pages": 805, "main_links": 52117, "foreign_links": 102408, "foreign_same_target": 8, "qf_complete_links": 515, "live_reverse_followed": 11, "live_reverse_wanted": 412, "live_sources": 916, "live_total": 1339, "scope": "real adapter and app Rebuild, not bootstrap Writer or whole request cost"})
	return f, q, want
}

func novelRangeForeignSeed(t *testing.T, f fixture) {
	t.Helper()
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('69000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,$2,true FROM generate_series(1000,1799) k`, f.ops, domain.Extractor)
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) VALUES($1,$2,1,$3,true)`, novelForeignID(2000), f.ops, domain.Extractor)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,ambiguous,resolved_asset)
 SELECT ip.node_id,k*16+2,k*16+13,$1,'wikilink','foreign.bin','foreign.bin',$2,false,true FROM indexed_pages ip CROSS JOIN generate_series(0,127) k WHERE ip.notebook_id=$1 AND ip.node_id<>$3`, f.ops, novelForeignID(1), novelForeignID(2000))
	// The reverse index has no notebook prefix. All eight entries are in
	// legitimate index ranges, even though notebook/asset/origin filters reject them.
	for i := range 8 {
		target := novelRangeID(1)
		if i >= 4 {
			target = novelRangeID(2)
		}
		var property any
		if i%4 >= 2 {
			property = "notes"
		}
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,resolved_id,ambiguous,resolved_asset) VALUES($1,$2,$3,$4,'wikilink',$5,'x','x',$6,false,$7)`, novelForeignID(2000), i, i+1, f.ops, property, target, i%2 == 0)
	}
}

func novelRangeFixtureCheck(t *testing.T, f fixture, p *stablePlanInputs, wantLinks []app.Link) {
	t.Helper()
	ids, err := p.PageIDs(t.Context(), f.eng)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.s.Links(t.Context(), f.eng, domain.Reach{Sources: ids})
	if err != nil {
		t.Fatal(err)
	}
	less := func(a, b app.Link) int {
		if n := strings.Compare(a.SourceID.String(), b.SourceID.String()); n != 0 {
			return n
		}
		return a.Start - b.Start
	}
	slices.SortFunc(got, less)
	expected := slices.Clone(wantLinks)
	slices.SortFunc(expected, less)
	if len(got) != 52117 || !slices.Equal(got, expected) {
		t.Fatalf("complete real Links before/after Rebuild differs: got=%d want=%d", len(got), len(expected))
	}
	var heads, bad, links, foreign, properties int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER(WHERE node_id<>ALL($2::uuid[]) OR revision<>1 OR extractor<>$3 OR NOT frontmatter_valid) FROM indexed_pages WHERE notebook_id=$1`, f.eng, ids, domain.Extractor).Scan(&heads, &bad); err != nil || heads != 805 || bad != 0 {
		t.Fatalf("literal source metadata: heads=%d bad=%d err=%v", heads, bad, err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FILTER(WHERE notebook_id=$1),count(*) FILTER(WHERE notebook_id=$2),count(*) FROM page_links`, f.eng, f.ops).Scan(&links, &foreign, &bad); err != nil || links != 52117 || foreign != 102408 || bad != 154525 {
		t.Fatalf("full two-book link counts=%d/%d/%d err=%v", links, foreign, bad, err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM page_properties WHERE notebook_id=$1`, f.eng).Scan(&properties); err != nil || properties != 806 {
		t.Fatalf("complete properties %d err=%v", properties, err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER(WHERE revision<>1 OR extractor<>$2 OR NOT frontmatter_valid OR NOT(node_id BETWEEN $3 AND $4 OR node_id=$5)) FROM indexed_pages WHERE notebook_id=$1`, f.ops, domain.Extractor, novelForeignID(1000), novelForeignID(1799), novelForeignID(2000)).Scan(&heads, &bad); err != nil || heads != 801 || bad != 0 {
		t.Fatalf("foreign head population=%d bad=%d err=%v", heads, bad, err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*),count(*) FILTER(WHERE source_id NOT BETWEEN $2 AND $3 OR range_start%16<>2 OR range_start<2 OR range_start>2034 OR range_end<>range_start+11 OR kind<>'wikilink' OR property_key IS NOT NULL OR target<>'foreign.bin' OR target_key IS DISTINCT FROM 'foreign.bin' OR target_alt_key IS NOT NULL OR anchor IS NOT NULL OR display IS NOT NULL OR resolved_id IS DISTINCT FROM $4 OR NOT resolved_asset OR ambiguous OR aliases) FROM page_links WHERE notebook_id=$1 AND source_id<>$5`, f.ops, novelForeignID(1000), novelForeignID(1799), novelForeignID(1), novelForeignID(2000)).Scan(&links, &bad); err != nil || links != 102400 || bad != 0 {
		t.Fatalf("full foreign noise population=%d bad=%d err=%v", links, bad, err)
	}
	rows := f.rows(t, `SELECT range_start,range_end,kind,coalesce(property_key,''),target,resolved_id::text,resolved_asset,ambiguous,aliases FROM page_links WHERE source_id=$1 ORDER BY range_start`, novelForeignID(2000))
	var want [][]string
	for i := range 8 {
		id, property := novelRangeID(1), ""
		if i >= 4 {
			id = novelRangeID(2)
		}
		if i%4 >= 2 {
			property = "notes"
		}
		want = append(want, []string{fmt.Sprint(i), fmt.Sprint(i + 1), "wikilink", property, "x", id.String(), fmt.Sprint(i%2 == 0), "false", "false"})
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("complete same-target foreign filter matrix\ngot %v\nwant %v", rows, want)
	}
}

// The independent ledger counts complete global resolved-ID ranges, then
// complete relevant source ranges, including rejected origins/unresolved rows.
// Tuple counters include work before SQL filters; plan averages alone do not.
func novelRangePhysical(t *testing.T, ev lintPlanEvidence, state string, indexes map[string]string) {
	t.Helper()
	rebuilt := strings.HasSuffix(state, "/app_rebuild")
	live := map[string]float64{"CTE followed_untouched": 11, "CTE relevant": 412, "CTE digested": 916, "CTE peers": 0}
	bound := int64(1339)
	if rebuilt {
		// Main reverse entries: old resolved + new resolved; foreign stays once.
		// Resolved source entries have at most three versions; the one
		// unresolved entry has two: 14 + 816 + 3*915 + 2 = 3577.
		bound = 3577
	}
	// Only source_id endpoint sampling through the full pkey gets a planning
	// allowance. Two probes is a strict guard for this fixed fixture/PG plan
	// family, not a global planner bound. Each endpoint has at most one live
	// version after seed VACUUM, or three versions after the real Rebuild.
	// Direction is not observed: do not assume one min plus one max probe.
	var endpointRows, endpointScans, endpointFetched int64
	if state != "no_statistics" {
		endpointScans, endpointFetched, endpointRows = 2, 2, 2
		if rebuilt {
			endpointRows = 6 // two probes * at most three versions per endpoint
		}
	}
	if (ev.Statistics["page_links"] > 0) != (state != "no_statistics") {
		t.Errorf("endpoint ledger statistics/state mismatch: state=%s statistics=%v", state, ev.Statistics)
	}
	if definition := indexes["page_links_pkey"]; !strings.Contains(definition, "btree (source_id, range_start)") || strings.Contains(definition, " WHERE ") {
		t.Errorf("endpoint ledger requires the full source-leading pkey: %q", definition)
	}
	for name, w := range ev.PlanningWork {
		if name != "page_links" && !strings.HasPrefix(name, "page_links_") {
			continue
		}
		var rows, scans, fetched int64
		if name == "page_links_pkey" {
			rows, scans, fetched = endpointRows, endpointScans, endpointFetched
		}
		if w.Returned > rows || w.Scans > scans || w.Fetched > fetched {
			t.Errorf("planning endpoint ledger: relation=%s work=%+v bounds returned/scans/fetched=%d/%d/%d", name, w, rows, scans, fetched)
		}
	}
	// The independent PlanningWork read is never subtracted from another
	// interval. Only analyzed auto/custom includes its own endpoint planning;
	// actual named generic EXECUTE and its EXPLAIN reuse the prepared plan.
	var extraRows, extraScans int64
	if ev.Mode == "auto" || ev.Mode == "force_custom_plan" {
		extraRows, extraScans = endpointRows, endpointScans
	}
	checkWork := func(label string, work map[string]lintWork, tupleLimit, scanLimit int64) {
		var indexRows, indexScans, nonPKRows, nonPKScans int64
		for name, w := range work {
			if strings.HasPrefix(name, "page_links_") {
				indexRows += w.Returned
				indexScans += w.Scans
				if name != "page_links_pkey" {
					nonPKRows += w.Returned
					nonPKScans += w.Scans
				}
			}
		}
		heap := work["page_links"]
		// PG's endpoint index_fetch_heap charges successful fetches to the
		// index counter. It does not grant extra heap tuple/sequence work.
		// Do not cap pkey total at endpointRows: valid execution may use it.
		if heap.Scans != 0 || heap.Returned != 0 || heap.Fetched > tupleLimit ||
			indexRows > tupleLimit+extraRows || indexScans > scanLimit+extraScans ||
			nonPKRows > tupleLimit || nonPKScans > scanLimit {
			t.Errorf("%s page_links work exceeds complete requested ranges: heap=%+v indexRows=%d indexScans=%d nonPKRows=%d nonPKScans=%d bounds=%d/%d pkey-endpoint-extra=%d/%d", label, heap, indexRows, indexScans, nonPKRows, nonPKScans, tupleLimit, scanLimit, extraRows, extraScans)
		}
	}
	// 2 asset probes + 404 distinct wanted pairs + 402 complete source probes.
	checkWork("actual full SQL", ev.ExecutionWork, bound, 808)
	checkWork("separate EXPLAIN execution", ev.ExplainWork, bound, 808)
	type node struct {
		Type      string  `json:"Node Type"`
		Relation  string  `json:"Relation Name"`
		Index     string  `json:"Index Name"`
		IndexCond string  `json:"Index Cond"`
		Subplan   string  `json:"Subplan Name"`
		Rows      float64 `json:"Actual Rows"`
		Loops     float64 `json:"Actual Loops"`
		Filtered  float64 `json:"Rows Removed by Filter"`
		Rechecked float64 `json:"Rows Removed by Index Recheck"`
		Plans     []node  `json:"Plans"`
	}
	var doc []struct {
		Plan node `json:"Plan"`
	}
	if err := json.Unmarshal(ev.Plan, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("actual plan: %v", err)
	}
	visible, bitmap, seen := map[string]float64{}, map[string]float64{}, map[string]int{}
	var visit func(node, string)
	visit = func(n node, scope string) {
		if strings.HasPrefix(n.Subplan, "CTE ") {
			scope = n.Subplan
			// These producers split the same complete logical ranges from
			// downstream joins. Keep the original aggregate work bounds.
			switch scope {
			case "CTE followed_ranges":
				scope = "CTE followed_untouched"
			case "CTE relevant_ranges":
				scope = "CTE relevant"
			case "CTE qualified":
				scope = "CTE digested"
			case "CTE peer_ranges":
				scope = "CTE peers"
			}
		}
		if n.Loops > 0 && (n.Relation == "page_links" || strings.HasPrefix(n.Index, "page_links_")) {
			allowed, ok := live[scope]
			if !ok {
				t.Errorf("unexpected executed page_links scope %q: %+v", scope, n)
			}
			if strings.HasSuffix(n.Type, "Seq Scan") {
				t.Errorf("whole page_links scan in %s", scope)
			}
			column := "resolved_id"
			if scope == "CTE digested" || scope == "CTE peers" {
				column = "source_id"
			}
			if n.Index != "" {
				definition := indexes[n.Index]
				prefix := strings.Contains(definition, "btree (resolved_id,")
				if column == "source_id" {
					prefix = strings.Contains(definition, "btree (source_id,") || strings.Contains(definition, "btree (notebook_id, source_id,") && strings.Contains(n.IndexCond, "notebook_id =")
				}
				if !prefix || !strings.Contains(n.IndexCond, column+" =") {
					t.Errorf("%s lacks a leading equality range on %s: index=%q cond=%q definition=%q", scope, column, n.Index, n.IndexCond, definition)
				}
			}
			// PostgreSQL rounds per-loop means. Subtract at most 0.5 per
			// reported component before comparing; count filter/recheck work too.
			lower := math.Max(0, n.Rows+n.Filtered+n.Rechecked-1.5) * n.Loops
			if n.Relation == "page_links" {
				visible[scope] += lower
				seen[scope]++
			} else if n.Type == "Bitmap Index Scan" {
				bitmap[scope] += math.Max(0, n.Rows-0.5) * n.Loops
			}
			if lower > allowed {
				t.Errorf("%s actual range read lower bound %.0f exceeds %.0f: %+v", scope, lower, allowed, n)
			}
		}
		for _, child := range n.Plans {
			visit(child, scope)
		}
	}
	visit(doc[0].Plan, "")
	for scope, allowed := range live {
		if allowed > 0 && seen[scope] == 0 {
			t.Errorf("no executed page_links range for %s", scope)
		}
		if visible[scope] > allowed || bitmap[scope] > allowed {
			t.Errorf("%s complete visible/filter work: heapLower=%.0f bitmapLower=%.0f bound=%.0f", scope, visible[scope], bitmap[scope], allowed)
		}
	}
	lintEvidence(t, "novel-range-ledger", map[string]any{"live": live, "tuple_bound_with_versions": bound, "logical_range_probes": 808, "actual_work": ev.ExecutionWork, "explain_work": ev.ExplainWork, "planning_work": ev.PlanningWork, "visible_lower": visible, "bitmap_lower": bitmap, "rebuilt": rebuilt, "note": "execution and EXPLAIN are distinct reads; generic is the helper's named EXECUTE, not a standalone raw-SQL label; blocks remain raw diagnostics"})
}
