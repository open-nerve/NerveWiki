//go:build !race

package postgresadapter_test

import (
	"fmt"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// lintAreaPage is the n-th page of an area fixture's shape, prefix its first
// eight hex digits, so that shapes sort apart and in n's order.
func lintAreaPage(prefix string, n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("%s-0000-0000-0000-%012x", prefix, n))
}

// lintAreaPlanFixture is a notebook of 20,000 pages of three wanted links
// each that no batch passes, a chunk of 512 pages of 10 wanted links each
// and one page of 10,000 (MaxLinks) wanted links, with no statistics.
func lintAreaPlanFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	for _, shape := range []struct {
		prefix       string
		pages, links int
	}{{"10000000", 20000, 3}, {"30000000", 512, 10}, {"40000000", 1, 10000}} {
		lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($2||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,$3::integer) k`, f.eng, shape.prefix, shape.pages)
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous)
 SELECT ip.node_id,k,k+1,ip.notebook_id,'wikilink','x','x',false FROM indexed_pages ip CROSS JOIN generate_series(0,$2::integer-1) k
 WHERE ip.node_id::text LIKE $1||'%'`, shape.prefix, shape.links)
	}
	lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT node_id,0,notebook_id,'type','"note"'::jsonb FROM indexed_pages WHERE node_id::text LIKE '30000000%'`)
	lintAssertStatistics(t, f, false)
	return f
}

func lintAreaStatement(t *testing.T, pages []uuid.UUID, after app.EdgePos) statement {
	return statementOf(t, func(q *gen.Queries) error {
		_, err := q.LintAreaWantedEdgeHeads(t.Context(), gen.LintAreaWantedEdgeHeadsParams{PageIds: pages, AfterSource: after.SourceID, AfterStart: after.Start, Take: 512})
		return err
	})
}

// An upgrade's start's second pass reads an area chunk's wanted heads in
// batches past a cursor (M11/P1 design 3.13; decision 235): the chunk's pages
// drive page_links' primary key through the array, whose scan returns the
// rows in key order, so no plan sorts the chunk's edges and LIMIT stops the
// scan early, for 512 pages of 10 links and for one page of 10,000, with and
// without statistics, in a custom and in a generic plan. The whole set at
// once, joined and then sorted past a cursor, was quadratic (review B-I1).
func TestLintPlansOfTheAreaPass(t *testing.T) {
	if testing.Short() {
		t.Skip("plan and physical work fixture")
	}
	f := lintAreaPlanFixture(t)
	spread := make([]uuid.UUID, 512)
	for i := range spread {
		spread[i] = lintAreaPage("30000000", i+1)
	}
	dense := []uuid.UUID{lintAreaPage("40000000", 1)}
	start := app.EdgePos{Start: -1}
	parts := []struct {
		name  string
		pages []uuid.UUID
		after app.EdgePos
		rows  int
	}{
		{"spread/first", spread, start, 512},
		{"spread/middle", spread[255:], app.EdgePos{SourceID: spread[255], Start: 4}, 512},
		{"spread/tail", spread[511:], app.EdgePos{SourceID: spread[511], Start: 8}, 1},
		{"spread/past", spread[511:], app.EdgePos{SourceID: spread[511], Start: 9}, 0},
		{"dense/first", dense, start, 512},
		{"dense/middle", dense, app.EdgePos{SourceID: dense[0], Start: 5000}, 512},
		{"dense/tail", dense, app.EdgePos{SourceID: dense[0], Start: 9998}, 1},
	}
	for _, statistics := range []string{"no-statistics", "analyzed"} {
		if statistics == "analyzed" {
			lintAnalyze(t, f)
		}
		t.Run(statistics, func(t *testing.T) {
			for _, part := range parts {
				for _, generic := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/generic=%t", part.name, generic), func(t *testing.T) {
						plan, evidence := lintExecuted(t, f, lintAreaStatement(t, part.pages, part.after), generic)
						if evidence.Rows != part.rows {
							t.Errorf("rows %d, want %d", evidence.Rows, part.rows)
						}
						scans := plan.all(readsOf("page_links"))
						if len(scans) != 1 || scans[0].Index != "page_links_pkey" {
							t.Errorf("page_links is not read once through its primary key: %+v", plan)
						}
						if sorts := plan.all(func(n *planNode) bool { return n.Type == "Sort" || n.Type == "Incremental Sort" }); len(sorts) != 0 {
							t.Errorf("a batch sorts the chunk's edges: %+v", plan)
						}
						work := lintWorkWithoutPlannerProbes(t, evidence)
						for _, name := range []string{"page_links", "page_links_pkey"} {
							if w := work[name]; w.Returned > 1024 || w.Fetched > 1024 {
								t.Errorf("one 512-head batch read %s past its batch: %+v", name, w)
							}
						}
					})
				}
			}
		})
	}
}
