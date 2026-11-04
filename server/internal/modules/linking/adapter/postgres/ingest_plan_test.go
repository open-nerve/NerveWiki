//go:build !race

package postgresadapter_test

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Physical work uses the existing non-race plan instrumentation: actual
// original SQL with parameters in auto/custom mode, explicit PREPARE/EXECUTE
// in generic mode, pg_stat_xact, encoding, and a separate ANALYZE/BUFFERS EXPLAIN. It is not a timed HTTP cost acceptance.
func TestIngestPlans(t *testing.T) {
	t.Run("indexed_heads", testIngestHeadPlans)
	t.Run("body_heads", testIngestBodyPlans)
	t.Run("private_old_f", testIngestOldBodyPlans)
	t.Run("first_json_once", testIngestJSONPlans)
	t.Run("maximum_json", func(t *testing.T) { testIngestJSONPlansSized(t, true) })
}
func ingestPlanSetup(t *testing.T, f fixture) {
	t.Helper()
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
}
func testIngestHeadPlans(t *testing.T) {
	f := newFixture(t)
	ingestPlanSetup(t, f)
	first := lintReaderID(1)
	f.replace(t, app.Page{ID: first, NotebookID: f.eng, Revision: 1}, originFacts(t, "---\nsources: '[[seed]]'\n---\n[[body]]"))
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('10000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,CASE WHEN k=257 THEN 1 ELSE 2 END,true FROM generate_series(2,100000) k`, f.eng)
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('20000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,3,2,false FROM generate_series(1,200000) k`, f.ops)
	missing, foreign := lintReaderID(100001), lintReaderID(1)
	foreign[0] = 0x20
	mixed, tail := make([]uuid.UUID, 512), make([]uuid.UUID, 512)
	for i := range 512 {
		tail[i] = lintReaderID(uint64(99489 + i))
		if i < 510 {
			mixed[i] = lintReaderID(uint64(i + 1))
		}
	}
	mixed[510], mixed[511] = missing, foreign
	for _, stats := range []string{"no-statistics", "analyzed"} {
		if stats == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(stats, func(t *testing.T) {
			for _, shape := range []struct {
				name    string
				ids     []uuid.UUID
				present int
			}{{"mixed", mixed, 510}, {"tail", tail, 512}} {
				t.Run(shape.name, func(t *testing.T) {
					revisions := make([]int32, 512)
					proof := make([]app.IngestRevision, 512)
					for i, id := range shape.ids {
						revisions[i] = 7
						proof[i] = app.IngestRevision{ID: id, Revision: 7}
					}
					real, err := postgresadapter.NewIngestReads(f.pool, 5<<20).IndexedHeads(t.Context(), f.eng, proof)
					if err != nil || len(real) != 512 {
						t.Fatalf("real bounded adapter heads=%d err=%v", len(real), err)
					}
					for i, h := range real {
						present := i < shape.present
						if h.ID != shape.ids[i] || h.PageRevision != 7 || h.Present != present || present && h.IndexedRevision != 1 || !present && (h.IndexedRevision != 0 || h.Extractor != 0 || h.FrontmatterValid) {
							t.Fatalf("exact nullable/current revision head %d: %+v", i, h)
						}
						if present && (h.Extractor != 2 && h.ID != lintReaderID(257) || !h.FrontmatterValid) {
							t.Fatalf("literal extractor or FM head lost: %+v", h)
						}
						if h.ID == lintReaderID(257) && h.Extractor != 1 {
							t.Fatalf("literal v1 became current: %+v", h)
						}
					}
					s := statementOf(t, func(q *gen.Queries) error {
						_, err := q.IngestIndexedHeads(t.Context(), gen.IngestIndexedHeadsParams{NotebookID: f.eng, PageIds: shape.ids, PageRevisions: revisions})
						return err
					})
					for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
						t.Run(mode, func(t *testing.T) {
							plan, ev := lintExecutedMode(t, f, s, mode, func(rows pgx.Rows, i int) {
								var id uuid.UUID
								var pageRev, rev, extractor int
								var present, fm bool
								if err := rows.Scan(&id, &pageRev, &present, &rev, &extractor, &fm); err != nil {
									t.Fatal(err)
								}
								want := real[i]
								if id != want.ID || pageRev != want.PageRevision || rev != want.IndexedRevision || extractor != want.Extractor || present != want.Present || fm != want.FrontmatterValid {
									t.Fatalf("prepared head %d differs from exact hand oracle: id=%s page=%d indexed=%d extractor=%d present=%t fm=%t", i, id, pageRev, rev, extractor, present, fm)
								}
							})
							scans := plan.all(readsOf("indexed_pages"))
							if ev.Rows != 512 || len(scans) != 1 || scans[0].Index != "indexed_pages_pkey" || !strings.Contains(scans[0].IndexCond, "node_id") || len(plan.unlimited("indexed_pages")) != 0 {
								t.Fatalf("512 requested heads did not drive bounded PK probes: rows=%d scans=%+v", ev.Rows, scans)
							}
							w := ev.ExecutionWork["indexed_pages_pkey"]
							if w.Scans != 512 || w.Returned > 512 || w.Fetched > 512 {
								t.Fatalf("head work depends on unrelated100k/200k rows: %+v", w)
							}
							table := ev.ExecutionWork["indexed_pages"]
							if table.Scans != 0 || table.Returned > 512 || table.Fetched > 512 {
								t.Fatalf("whole indexed_pages scan: %+v", table)
							}
						})
					}
				})
			}
		})
	}
}
func testIngestJSONPlans(t *testing.T) { testIngestJSONPlansSized(t, false) }
func testIngestJSONPlansSized(t *testing.T, large bool) {
	f := newFixture(t)
	ingestPlanSetup(t, f)
	const total = 512
	pageID := lintReaderID(900000)
	ids := make([]uuid.UUID, total)
	names := map[string]uuid.UUID{}
	var content strings.Builder
	content.WriteString("---\nsources:\n")
	for i := range total {
		ids[i] = lintReaderID(uint64(i + 1))
		names[fmt.Sprintf("A%03d", i)] = ids[i]
		if i%4 == 2 {
			fmt.Fprintf(&content, "  - 'plain %03d'\n", i)
		} else {
			fmt.Fprintf(&content, "  - '[[A%03d]]'\n", i)
		}
	}
	// Every 4k+1 slot has two real peers with the same path and same target.
	// They must not become a unique slot after target-based deduplication.
	for i := 1; i < total; i += 4 {
		fmt.Fprintf(&content, "'sources.%d': '[[A%03d]]'\n", i, i)
	}
	content.WriteString("Sources: ['[[A002]]']\n---\n") // later matching key is not the first
	body := content.String()
	if large {
		// Keep the real 512-slot/513-peer grammar and hand-written novel set.
		// Only a non-link string grows; deterministic high-entropy legal YAML
		// reaches the actual 5MiB page bound without a highly compressible TOAST value.
		const marker = "'plain 002'"
		const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		rng := rand.New(rand.NewPCG(20261103, 512))
		padding := make([]byte, (5<<20)-len(body))
		for i := range padding {
			padding[i] = alphabet[rng.Uint32()&63]
		}
		if strings.Count(body, marker) != 1 {
			t.Fatal("maximum JSON padding marker is not unique")
		}
		body = strings.Replace(body, marker, "'plain 002"+string(padding)+"'", 1)
		if len(body) != 5<<20 {
			t.Fatalf("maximum real page bytes%d", len(body))
		}
	}
	facts := originFacts(t, body)
	f.replace(t, app.Page{ID: pageID, NotebookID: f.eng, Revision: 1}, facts)
	if large {
		var logical, stored int
		if err := f.pool.QueryRow(t.Context(), "SELECT octet_length(value::text),pg_column_size(value) FROM page_properties WHERE source_id=$1 AND key='sources'", pageID).Scan(&logical, &stored); err != nil {
			t.Fatal(err)
		}
		if logical < (5<<20)-(64<<10) || stored < (4<<20) {
			t.Fatalf("maximum JSON/TOAST fixture logical%d stored%d", logical, stored)
		}
		t.Logf("INGESTJSON page_bytes=%d source_json_bytes=%d stored_bytes=%d slots=512 real_peers=513 hand_novel=256", len(body), logical, stored)
	}
	var resolved []app.Link
	for _, l := range facts.Links {
		id, ok := names[l.Target]
		if !ok {
			t.Fatalf("unexpected parser target %q", l.Target)
		}
		resolved = append(resolved, app.Link{SourceID: pageID, Start: l.Start, Resolution: domain.Resolution{ID: id, Asset: true}})
	}
	if len(resolved) != 513 {
		t.Fatalf("real parser peers=%d want513", len(resolved))
	}
	if err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error { return f.s.SetResolutions(ctx, resolved) }); err != nil {
		t.Fatal(err)
	}
	// Irrelevant same-notebook and foreign-notebook current indexes make a
	// whole relation scan distinguishable from one bounded first-JSON read.
	// The active page above still comes from the real parser and resolutions.
	for _, noise := range []struct {
		prefix string
		nb     uuid.UUID
		count  int
	}{{"30000000", f.eng, 100000}, {"40000000", f.ops, 200000}} {
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($1||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$2,1,2,true FROM generate_series(1,$3::integer) k`, noise.prefix, noise.nb, noise.count)
		lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT node_id,0,notebook_id,'sources','"unrelated plain value"'::jsonb FROM indexed_pages WHERE notebook_id=$1 AND node_id<>$2`, noise.nb, pageID)
	}
	q := app.IngestCandidateQuery{NotebookID: f.eng, EligibleEmitters: []uuid.UUID{pageID}}
	for _, id := range ids {
		q.Candidates = append(q.Candidates, app.IngestSource{ID: id, Asset: true})
	}
	check := func(t *testing.T, e app.IngestCandidateEvidence) {
		t.Helper()
		if len(e.Candidates) != total || len(e.Parents) != 0 {
			t.Fatalf("candidate rows=%d parents=%d", len(e.Candidates), len(e.Parents))
		}
		var novel []uuid.UUID
		for i, c := range e.Candidates {
			want := i%4 == 0 || i%4 == 3
			if c.ID != ids[i] || !c.Asset || c.Direct != want || c.FollowedUntouched {
				t.Fatalf("hand D/novel oracle row%d=%+v wantdirect=%t", i, c, want)
			}
			if !c.Direct {
				novel = append(novel, c.ID)
			}
		}
		var expected []uuid.UUID
		for i, id := range ids {
			if i%4 == 1 || i%4 == 2 {
				expected = append(expected, id)
			}
		}
		if !slices.Equal(novel, expected) || len(novel) != 256 {
			t.Fatalf("hand novel set=%v expected=%v", novel, expected)
		}
	}
	for _, stats := range []string{"no-statistics", "analyzed"} {
		if stats == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(stats, func(t *testing.T) {
			var got app.IngestCandidateEvidence
			if err := f.tx.WithinTx(t.Context(), func(ctx context.Context) error {
				var err error
				got, err = postgresadapter.NewIngestReads(f.pool, 5<<20).NovelSources(ctx, q)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			check(t, got)
			assets := make([]bool, total)
			revs := make([]int32, total)
			for i := range assets {
				assets[i] = true
			}
			s := statementOf(t, func(q *gen.Queries) error {
				_, err := q.IngestNovelSources(t.Context(), gen.IngestNovelSourcesParams{NotebookID: f.eng, Extractor: 2, CandidateIds: ids, CandidateAssets: assets, CandidateRevisions: revs, EligibleEmitterIds: []uuid.UUID{pageID}, MaxSlotItems: 10000, MaxJsonBytes: app.MaxLintPayloadBytes(5 << 20)})
				return err
			})
			for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
				t.Run(mode, func(t *testing.T) {
					actual := app.IngestCandidateEvidence{}
					plan, ev := lintExecutedMode(t, f, s, mode, func(rows pgx.Rows, _ int) {
						var kind string
						var c app.IngestCandidateFact
						var invalid, proof bool
						if err := rows.Scan(&kind, &c.ID, &c.Asset, &c.Direct, &c.FollowedUntouched, &invalid, &proof); err != nil {
							t.Fatal(err)
						}
						if kind != "candidate" || invalid || proof {
							t.Fatalf("prepared malformed row: kind%s flags%t/%t", kind, invalid, proof)
						}
						actual.Candidates = append(actual.Candidates, c)
					})
					check(t, actual)
					ingestAssertJSONPeerWork(t, ev)
					scans := plan.all(readsOf("page_properties"))
					work := ev.ExecutionWork["page_properties_sources_idx"]
					if ev.Rows != total || len(scans) != 1 || scans[0].Index != "page_properties_sources_idx" || len(plan.unlimited("page_properties")) != 0 || work.Scans != 1 || work.Returned != 1 || work.Fetched > 1 {
						t.Fatalf("first JSON was not read once for all512 candidates and all peers: rows=%d work=%+v scans=%+v", ev.Rows, work, scans)
					}
				})
			}
		})
	}
}

// This fixture has one slot-bearing source, 512 slots and 513 real property
// links. Keep the late Sources peer in the complete range, even though only
// 512 edges join the first JSON's slots. The independent plan observes visible
// tuple work; relation/index counters (including dead versions) remain separate.
func ingestAssertJSONPeerWork(t *testing.T, ev lintPlanEvidence) {
	t.Helper()
	const slots, peers = 512, 513
	const linear = slots + peers
	all := stablePlanTree(t, ev.Plan).all()
	lower := func(value, loops float64) float64 {
		if loops <= 1 {
			return value * loops
		}
		// EXPLAIN rounds per-loop means. A strict lower bound must allow
		// half a tuple for each independently reported component.
		return math.Max(0, value-0.5) * loops
	}
	for _, want := range []struct {
		name     string
		rows     float64
		optional bool
	}{{"CTE peer_ranges", peers, false}, {"CTE peers", slots, false}, {"CTE peer_pairs", 0, true}} {
		var producers []stablePlanNode
		for _, n := range all {
			if n.Subplan == want.name {
				producers = append(producers, n)
			}
		}
		// A future pairing fence must not hide repeated work from this
		// guard. Its absence is valid for the candidate03 baseline.
		if want.optional && len(producers) == 0 {
			continue
		}
		if len(producers) != 1 {
			t.Fatalf("complete peer producer %s: got %d want1", want.name, len(producers))
		}
		producer := producers[0]
		if !want.optional && (!producer.RowsObserved || !producer.LoopsObserved || producer.Rows != want.rows || producer.Loops != 1) {
			t.Errorf("complete peer producer %s: rows%g loops%g want%g/1", want.name, producer.Rows, producer.Loops, want.rows)
		}
		sourceReads := 0
		for _, n := range producer.all() {
			if !n.RowsObserved || !n.LoopsObserved {
				t.Errorf("peer plan lacks actual rows/loops: scope%s type%s", want.name, n.Type)
				continue
			}
			if n.Relation == "page_links" {
				sourceReads++
				if want.name != "CTE peer_ranges" || n.Rows != peers || n.Loops != 1 || n.Removed != 0 || n.Rechecked != 0 {
					t.Errorf("complete peer source read: scope%s type%s rows%g loops%g filtered%g rechecked%g want513/1/0/0", want.name, n.Type, n.Rows, n.Loops, n.Removed, n.Rechecked)
				}
			}
			if n.Type == "Bitmap Index Scan" {
				// Bitmap TIDs can include SetResolutions' dead versions. Do
				// not compare them with the 513 visible peers; still reject
				// repeated construction of the one source's bitmap.
				if n.Loops > 1 {
					t.Errorf("peer bitmap range repeated: scope%s index%s loops%g", want.name, n.Index, n.Loops)
				}
				continue
			}
			work := lower(n.Rows, n.Loops) + lower(n.Removed, n.Loops) + lower(n.Rechecked, n.Loops) + lower(n.JoinRemoved, n.Loops)
			// Each node gets the two complete input cardinalities, not a
			// slots*peers allowance. This catches repeated CTE/Materialize
			// reads and rejected join pairs without choosing a join method.
			if work > linear || n.Loops > linear {
				t.Errorf("peer work exceeds slots+peers=%d: scope%s type%s cte%s rows%g loops%g filter%g recheck%g join_filter%g lower%g", linear, want.name, n.Type, n.CTE, n.Rows, n.Loops, n.Removed, n.Rechecked, n.JoinRemoved, work)
			}
		}
		wantReads := 0
		if want.name == "CTE peer_ranges" {
			wantReads = 1
		}
		if sourceReads != wantReads {
			t.Errorf("peer base reads: scope%s got%d want%d", want.name, sourceReads, wantReads)
		}
	}
}
