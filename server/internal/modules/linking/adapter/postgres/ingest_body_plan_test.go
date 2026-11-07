//go:build !race

package postgresadapter_test

import (
	"context"
	"slices"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The legacy constants below are exact generated statements saved before the
// approved predicate move. Both versions run against one database, parameters,
// hand-written DTO sequence and unchanged planner settings. This measures the
// actual head probes; a Memoize label alone is not a successful comparison.
func testIngestBodyPlans(t *testing.T) {
	f := newFixture(t)
	ingestPlanSetup(t, f)
	zero, late := uuid.Nil(), lintReaderID(100)
	var expected []app.BodyReference
	// Deliberately seed index facts rather than claiming a markdown parser test.
	// Each accepted body row has three interleaved rejected peers: named FM,
	// empty-key FM, unresolved body. All four body kinds and asset flags occur.
	source := func(id, nb uuid.UUID, n int) []app.BodyReference {
		facts := domain.Facts{FrontmatterValid: true, Properties: []domain.Property{{Key: "plain", Value: []byte(`"body plan fixture"`)}}}
		var resolved []app.Link
		var want []app.BodyReference
		for i := range n {
			target := lintReaderID(uint64(10000 + i))
			asset := i%2 == 0
			for j := range 4 {
				property := ""
				if j == 1 {
					property = "named"
				}
				if j == 2 {
					property = "empty-later"
				}
				start := 4*i + j
				facts.Links = append(facts.Links, domain.Link{Kind: []string{"wikilink", "embed", "link", "image"}[i%4], Property: property, Target: "T", Start: start, End: start + 1})
				if j != 3 {
					resolved = append(resolved, app.Link{SourceID: id, Start: start, Resolution: domain.Resolution{ID: target, Asset: asset}})
				}
			}
			expectedTarget := target
			if i == 0 {
				expectedTarget = uuid.Nil()
			}
			want = append(want, app.BodyReference{EdgePos: app.EdgePos{SourceID: id, Start: int32(4 * i)}, ResolvedID: expectedTarget, ResolvedAsset: asset})
		}
		f.replace(t, app.Page{ID: id, NotebookID: nb, Revision: 1}, facts)
		if err := f.s.SetResolutions(t.Context(), resolved); err != nil {
			t.Fatal(err)
		}
		// Public SetResolutions treats zero as absent. This explicit SQL boundary
		// installs a non-NULL zero UUID, whose read DTO must remain literal zero.
		lintExec(t, f, "UPDATE page_links SET resolved_id=$2::uuid WHERE source_id=$1 AND range_start=0", id, uuid.Nil())
		lintExec(t, f, "UPDATE page_links SET property_key='' WHERE source_id=$1 AND range_start%4=2", id)
		return want
	}
	expected = append(expected, source(zero, f.eng, 1024)...)
	expected = append(expected, source(late, f.eng, 5)...)
	stale, missing, foreignHead, foreignRows, unrequested := lintReaderID(10), lintReaderID(20), lintReaderID(30), lintReaderID(40), lintReaderID(50)
	source(stale, f.eng, 1)
	lintExec(t, f, "UPDATE indexed_pages SET extractor=1 WHERE node_id=$1", stale)
	source(missing, f.eng, 1)
	lintExec(t, f, "DELETE FROM indexed_pages WHERE node_id=$1", missing)
	source(foreignHead, f.ops, 1)
	lintExec(t, f, "UPDATE page_links SET notebook_id=$2 WHERE source_id=$1", foreignHead, f.eng)
	source(foreignRows, f.eng, 1)
	lintExec(t, f, "UPDATE page_links SET notebook_id=$2 WHERE source_id=$1", foreignRows, f.ops)
	source(unrequested, f.eng, 1)
	// Noise stays outside the requested first/last closed range in two books.
	for _, noise := range []struct {
		prefix string
		nb     uuid.UUID
		n      int
	}{{"50000000", f.eng, 50000}, {"60000000", f.ops, 75000}} {
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($1||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$2,1,2,true FROM generate_series(1,$3::integer) k`, noise.prefix, noise.nb, noise.n)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT node_id,j,j+1,notebook_id,'link','hidden','hidden',node_id,true,false FROM indexed_pages CROSS JOIN generate_series(0,1) j WHERE notebook_id=$1 AND node_id >= ($2||'-0000-0000-0000-000000000000')::uuid`, noise.nb, noise.prefix)
	}
	// SetResolutions and the deliberately damaged origin/head rows above
	// create old tuple/index versions. Keep this range-work fixture pristine
	// without collecting statistics: analyzed remains a separate later state.
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "VACUUM "+table)
	}
	emitters := []uuid.UUID{late, foreignHead, zero, missing, stale, foreignRows, zero}
	reader := postgresadapter.NewLintReads(f.pool, 5<<20)
	for _, set := range []struct {
		name  string
		ids   []uuid.UUID
		want  []app.BodyReference
		calls int
	}{{"all-unsorted-duplicate", emitters, expected, 4}, {"exact1024", []uuid.UUID{zero}, expected[:1024], 3}, {"empty", nil, nil, 1}, {"no-match", []uuid.UUID{missing}, nil, 1}} {
		t.Run(set.name, func(t *testing.T) {
			var after *app.EdgePos
			var got []app.BodyReference
			calls := 0
			for {
				rows, err := reader.BodyReferences(t.Context(), f.eng, set.ids, after, 512)
				calls++
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) > 512 {
					t.Fatalf("body batch rows%d", len(rows))
				}
				got = append(got, rows...)
				if len(rows) == 0 {
					break
				}
				last := rows[len(rows)-1].EdgePos
				after = &last
			}
			if calls != set.calls || !slices.Equal(got, set.want) {
				t.Fatalf("complete independent body DTOs/cursor: calls%d want%d rows%d want%d", calls, set.calls, len(got), len(set.want))
			}
		})
	}
	testIngestBoundBodyDTOs(t, f, emitters, expected, zero, missing)
	for _, stats := range []string{"no-statistics", "analyzed"} {
		if stats == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(stats, func(t *testing.T) {
			for _, part := range []struct {
				name  string
				after *app.EdgePos
				want  []app.BodyReference
			}{{"first", nil, expected[:512]}, {"after-same-source", &expected[511].EdgePos, expected[512:1024]}, {"cross-source", &expected[1023].EdgePos, expected[1024:]}, {"eof", &expected[len(expected)-1].EdgePos, nil}} {
				t.Run(part.name, func(t *testing.T) {
					current := statementOf(t, func(q *gen.Queries) error {
						if part.after == nil {
							_, err := q.LintBodyReferencesFirst(t.Context(), gen.LintBodyReferencesFirstParams{NotebookID: f.eng, Extractor: 2, WikiPageIds: emitters, FirstSource: zero, LastSource: late, Take: 512})
							return err
						}
						_, err := q.LintBodyReferencesAfter(t.Context(), gen.LintBodyReferencesAfterParams{NotebookID: f.eng, Extractor: 2, WikiPageIds: emitters, FirstSource: zero, LastSource: late, AfterSource: part.after.SourceID, AfterStart: part.after.Start, Take: 512})
						return err
					})
					for _, mode := range []string{"auto", "force_custom_plan", "force_generic_plan"} {
						t.Run(mode, func(t *testing.T) {
							legacy := statement{sql: ingestBodyLegacyFirst, args: []any{f.eng, int32(2), int32(512), emitters}}
							if part.after != nil {
								legacy = statement{sql: ingestBodyLegacyAfter, args: []any{f.eng, int32(2), part.after.SourceID, part.after.Start, int32(512), emitters}}
							}
							var evidence [2]lintPlanEvidence
							for i, version := range []struct {
								name string
								s    statement
							}{{"before", legacy}, {"after", current}} {
								t.Run(version.name, func(t *testing.T) {
									var actual []app.BodyReference
									plan, ev := lintExecutedMode(t, f, version.s, mode, func(rows pgx.Rows, _ int) {
										var row app.BodyReference
										if err := rows.Scan(&row.SourceID, &row.Start, &row.ResolvedID, &row.ResolvedAsset); err != nil {
											t.Fatal(err)
										}
										actual = append(actual, row)
									})
									evidence[i] = ev
									if !slices.Equal(actual, part.want) {
										t.Fatalf("body %s full DTOs differ: actual%+v expected%+v", version.name, actual, part.want)
									}
									lintAssertRangeRows(t, plan, ev, part.after != nil, true, 4*512+32)
									// The saved05 reference independently failed the old oracle because
									// its min/max bounds reread emitters. Never weaken the current oracle.
									if i == 1 && len(part.want) > 0 {
										lintAssertMembership(t, ev, "emitters", len(emitters))
									}
									if ev.Rows != len(part.want) {
										t.Fatalf("body result rows%d want%d", ev.Rows, len(part.want))
									}
								})
							}
							before, after := evidence[0].ExecutionWork["indexed_pages_pkey"], evidence[1].ExecutionWork["indexed_pages_pkey"]
							lintEvidence(t, "ingest-body-head-comparison", map[string]any{"part": part.name, "before": before, "after": after, "rows": len(part.want), "fewer_actual_probes": after.Scans < before.Scans})
							if after.Scans > before.Scans || after.Returned > before.Returned || after.Fetched > before.Fetched {
								t.Fatalf("moved predicates increased actual head probes: before%+v after%+v", before, after)
							}
							// Probe reduction is measured, not assumed. The original
							// physical bound and the no-work-increase assertion above
							// remain mandatory even when this mode chooses no reuse.
						})
					}
				})
			}
		})
	}
}

// The existing ordinary-pool traversal above remains unchanged. These extra
// public calls reuse its independent 1029/1024 DTO oracle on a hot bound Conn.
func testIngestBoundBodyDTOs(t *testing.T, f fixture, emitters []uuid.UUID, expected []app.BodyReference, zero, missing uuid.UUID) {
	t.Helper()
	for _, route := range []string{"onconn", "tx"} {
		t.Run("bound-body-dtos/"+route, func(t *testing.T) {
			tr := &lintTrace{}
			pool := lintBodyModePool(t, f, tr, 0, 8)
			conn, err := pool.Acquire(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()
			pid := conn.Conn().PgConn().PID()
			reader := postgresadapter.NewLintReads(pool, 5<<20)
			run := func(ctx context.Context) error {
				for _, prime := range []struct {
					name  string
					after *app.EdgePos
					want  []app.BodyReference
				}{{"LintBodyReferencesFirst", nil, expected[:512]}, {"LintBodyReferencesAfter", &expected[511].EdgePos, expected[512:1024]}} {
					before := len(tr.prepareObservations())
					rows, err := reader.BodyReferences(ctx, f.eng, emitters, prime.after, 512)
					if err != nil || !slices.Equal(rows, prime.want) {
						t.Fatalf("bound prime independent DTOs: rows=%d err=%v", len(rows), err)
					}
					calls := tr.observations()
					lintAssertMode(t, calls[len(calls)-1], prime.name, pgx.QueryExecModeCacheDescribe, pid, 0, 8)
					lintAssertPrepares(t, tr.prepareObservations()[before:], prime.name, pid, 1)
				}
				for pass := range 2 {
					for _, set := range []struct {
						name  string
						ids   []uuid.UUID
						want  []app.BodyReference
						calls int
					}{{"all-unsorted-duplicate", emitters, expected, 4}, {"exact1024", []uuid.UUID{zero}, expected[:1024], 3}, {"empty", nil, nil, 1}, {"no-match", []uuid.UUID{missing}, nil, 1}} {
						var after *app.EdgePos
						var got []app.BodyReference
						calls := 0
						for {
							before, prepared := len(tr.observations()), len(tr.prepareObservations())
							rows, err := reader.BodyReferences(ctx, f.eng, set.ids, after, 512)
							calls++
							if err != nil || len(rows) > 512 {
								t.Fatalf("bound hot batch %s/pass%d: rows=%d err=%v", set.name, pass, len(rows), err)
							}
							observed := tr.observations()[before:]
							if len(set.ids) == 0 {
								if len(observed) != 0 || len(tr.prepareObservations()) != prepared {
									t.Fatal("empty Body emitters queried")
								}
							} else {
								name := "LintBodyReferencesFirst"
								if after != nil {
									name = "LintBodyReferencesAfter"
								}
								if len(observed) != 1 {
									t.Fatalf("bound hot Body calls: %+v", observed)
								}
								lintAssertMode(t, observed[0], name, pgx.QueryExecModeCacheDescribe, pid, 0, 8)
								lintAssertPrepares(t, tr.prepareObservations()[prepared:], name, pid, 0)
							}
							got = append(got, rows...)
							if len(rows) == 0 {
								break
							}
							last := rows[len(rows)-1].EdgePos
							after = &last
						}
						if calls != set.calls || !slices.Equal(got, set.want) {
							t.Fatalf("bound complete independent DTOs %s/pass%d: calls=%d/%d rows=%d/%d", set.name, pass, calls, set.calls, len(got), len(set.want))
						}
					}
				}
				lintNoNamedStatements(t, ctx, pool)
				t.Logf("bound hot independent Body1029/1024+EOF route=%s PID=%d actual queries=%d unnamed descriptions=%d; original pool/physical oracles retained", route, pid, len(tr.observations()), len(tr.prepareObservations()))
				return nil
			}
			ctx := postgres.OnConn(t.Context(), conn)
			if route == "tx" {
				err = lintBodyTxManager(pool).WithinTx(ctx, run)
			} else {
				err = run(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

const ingestBodyLegacyFirst = `-- name: LintBodyReferencesFirst :many
WITH emitters AS MATERIALIZED (SELECT unnest($4::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=$1
   AND ip.extractor=$2
   AND l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=$1
 AND l.source_id >= (SELECT id FROM emitters ORDER BY id LIMIT 1)
 AND l.source_id <= (SELECT id FROM emitters ORDER BY id DESC LIMIT 1)
ORDER BY l.source_id,l.range_start LIMIT (SELECT $3::integer)
`

const ingestBodyLegacyAfter = `-- name: LintBodyReferencesAfter :many
WITH emitters AS MATERIALIZED (SELECT unnest($6::uuid[]) AS id)
SELECT l.source_id,l.range_start,l.resolved_id,l.resolved_asset
FROM page_links l
CROSS JOIN LATERAL (
 SELECT ip.node_id FROM indexed_pages ip
 WHERE ip.node_id=l.source_id AND ip.notebook_id=$1
   AND ip.extractor=$2
   AND l.source_id IN (SELECT id FROM emitters)
   AND l.property_key IS NULL AND l.resolved_id IS NOT NULL
 LIMIT 1
) current_page
WHERE l.notebook_id=$1
 AND l.source_id >= (SELECT id FROM emitters ORDER BY id LIMIT 1)
 AND l.source_id <= (SELECT id FROM emitters ORDER BY id DESC LIMIT 1)
 AND (l.source_id,l.range_start)>($3::uuid,$4::integer)
ORDER BY l.source_id,l.range_start LIMIT (SELECT $5::integer)
`
