//go:build !race

package postgresadapter_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func lintRulesPlanFixture(t *testing.T, hidden string, hiddenPages int) fixture {
	t.Helper()
	f := newFixture(t)
	lintExec(t, f, "CREATE EXTENSION IF NOT EXISTS pageinspect")
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		lintExec(t, f, "ALTER TABLE "+table+" SET (autovacuum_enabled=false)")
	}
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ('10000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,k%4<>0 FROM generate_series(1,2000) k`, f.eng)
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 SELECT ($2||'-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,$1,1,2,true FROM generate_series(1,$3::integer) k`, f.ops, hidden, hiddenPages)
	lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT node_id,k,notebook_id,CASE k WHEN 0 THEN 'type' WHEN 1 THEN 'summary' ELSE 'other' END,'" "'::jsonb
 FROM indexed_pages CROSS JOIN generate_series(0,2) k`)
	// A thousand unrelated keys on a real reader page must not replace either
	// partial first-property probe. This is pristine INSERT state, never UPDATE.
	lintExec(t, f, `INSERT INTO page_properties(source_id,position,notebook_id,key,value)
 SELECT $1,k,$2,'other'||k,'"x"'::jsonb FROM generate_series(3,1002) k`, lintReaderID(1), f.eng)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,ambiguous)
 SELECT node_id,0,1,notebook_id,'wikilink','self','self',node_id,false FROM indexed_pages`)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,property_key,target,target_key,resolved_id,resolved_asset,ambiguous)
 SELECT ('10000000-0000-0000-0000-'||lpad(to_hex(k),12,'0'))::uuid,1,2,$1,'wikilink',CASE WHEN k%2=0 THEN 'named' END,'asset.pdf','asset.pdf',
 ('10000000-0000-0000-0000-'||lpad(to_hex(3000+k),12,'0'))::uuid,true,false FROM generate_series(1,512) k`, f.eng)
	lintAssertStatistics(t, f, false)
	return f
}

// This is a test vector for the fixed SQL argument, deliberately independent
// of adapter's constant. The LQ tests cover every included/excluded codepoint.
const lintSQLTrim = "\t\n\v\f\r \u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

func lintRuleStatement(t *testing.T, rule, phase string, notebook uuid.UUID, ids, excluded []uuid.UUID, after uuid.UUID) statement {
	t.Helper()
	return statementOf(t, func(q *gen.Queries) error {
		switch rule + "/" + phase {
		case "Orphan/Count":
			_, err := q.LintOrphanCount(t.Context(), gen.LintOrphanCountParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, ExcludedEmitters: excluded})
			return err
		case "Orphan/First":
			_, err := q.LintOrphanItemsFirst(t.Context(), gen.LintOrphanItemsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, ExcludedEmitters: excluded, Take: 101})
			return err
		case "Orphan/After":
			_, err := q.LintOrphanItemsAfter(t.Context(), gen.LintOrphanItemsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, ExcludedEmitters: excluded, Take: 101, AfterID: after})
			return err
		case "Frontmatter/Count":
			_, err := q.LintFrontmatterCount(t.Context(), gen.LintFrontmatterCountParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids})
			return err
		case "Frontmatter/First":
			_, err := q.LintFrontmatterItemsFirst(t.Context(), gen.LintFrontmatterItemsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, Take: 101})
			return err
		case "Frontmatter/After":
			_, err := q.LintFrontmatterItemsAfter(t.Context(), gen.LintFrontmatterItemsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, Take: 101, AfterID: after})
			return err
		case "MissingType/Count":
			_, err := q.LintMissingTypeCount(t.Context(), gen.LintMissingTypeCountParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim})
			return err
		case "MissingType/First":
			_, err := q.LintMissingTypeItemsFirst(t.Context(), gen.LintMissingTypeItemsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim, Take: 101})
			return err
		case "MissingType/After":
			_, err := q.LintMissingTypeItemsAfter(t.Context(), gen.LintMissingTypeItemsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim, Take: 101, AfterID: after})
			return err
		case "MissingSummary/Count":
			_, err := q.LintMissingSummaryCount(t.Context(), gen.LintMissingSummaryCountParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim})
			return err
		case "MissingSummary/First":
			_, err := q.LintMissingSummaryItemsFirst(t.Context(), gen.LintMissingSummaryItemsFirstParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim, Take: 101})
			return err
		case "MissingSummary/After":
			_, err := q.LintMissingSummaryItemsAfter(t.Context(), gen.LintMissingSummaryItemsAfterParams{NotebookID: notebook, Extractor: domain.Extractor, CandidateIds: ids, TrimChars: lintSQLTrim, Take: 101, AfterID: after})
			return err
		default:
			t.Fatalf("unknown static rule statement %s/%s", rule, phase)
			return nil
		}
	})
}

func lintRulesPhysical(t *testing.T, plan planNode, ev lintPlanEvidence, rule string) {
	t.Helper()
	for _, table := range []string{"indexed_pages", "page_links", "page_properties"} {
		for _, n := range plan.all(readsOf(table)) {
			if n.Type == "Seq Scan" {
				t.Errorf("%s scanned whole %s: %+v", rule, table, n)
			}
		}
	}
	for name, w := range ev.ExecutionWork {
		bound := int64(6000)
		if strings.HasPrefix(name, "page_properties") {
			bound = 2000
		}
		if rule == "F" {
			bound = 1024
		}
		if w.Scans > bound || w.Returned > bound || w.Fetched > bound {
			t.Errorf("%s unrelated relation work %s=%+v > %d", rule, name, w, bound)
		}
	}
	// EXPLAIN's actual per-node rows × loops also covers bitmap index work;
	// backend-local counters above remain the physical tuple/heap/blocks oracle.
	type actualNode struct {
		Relation string       `json:"Relation Name"`
		Index    string       `json:"Index Name"`
		Rows     float64      `json:"Actual Rows"`
		Loops    float64      `json:"Actual Loops"`
		Plans    []actualNode `json:"Plans"`
	}
	var document []struct {
		Plan actualNode `json:"Plan"`
	}
	if err := json.Unmarshal(ev.Plan, &document); err != nil || len(document) != 1 {
		t.Fatalf("actual rule plan: %v", err)
	}
	var walk func(actualNode)
	walk = func(n actualNode) {
		if n.Relation != "" || n.Index != "" {
			bound := float64(6000)
			if strings.HasPrefix(n.Relation+n.Index, "page_properties") {
				bound = 2000
			}
			if rule == "F" {
				bound = 1024
			}
			if n.Rows*n.Loops > bound {
				t.Errorf("%s physical plan node %s/%s read %.0f × %.0f > %.0f", rule, n.Relation, n.Index, n.Rows, n.Loops, bound)
			}
		}
		for _, child := range n.Plans {
			walk(child)
		}
	}
	walk(document[0].Plan)
	if strings.HasPrefix(rule, "Missing") {
		key := "type"
		if rule == "MissingSummary" {
			key = "summary"
		}
		reads := plan.all(readsOf("page_properties"))
		if len(reads) != 1 || reads[0].Index != "page_properties_"+key+"_idx" || !strings.Contains(reads[0].IndexCond, "source_id") || len(plan.unlimited("page_properties")) != 0 {
			t.Errorf("%s lost its first-key partial probe: %+v", rule, reads)
		}
	}
	if rule == "F" || rule == "Orphan" {
		for _, n := range plan.all(readsOf("page_links")) {
			indexes, keyed := n.indexes("resolved_id")
			if len(indexes) != 1 || indexes[0] != "page_links_resolved_id_source_id_idx" || !keyed {
				t.Errorf("%s lost resolved target range: %+v", rule, n)
			}
		}
	}
}

func lintOrphanItemsWork(t *testing.T, ev lintPlanEvidence, wanted int) {
	t.Helper()
	// This fresh fixture has only self inbound links for every candidate.
	// All current reader heads therefore qualify. The requested page bounds
	// target PK checks plus at most one current-emitter PK check per item,
	// and one resolved-target anti probe per item; hidden rows add no work.
	// Keep this separate from the unchanged broad range/height guards.
	work := lintWorkWithoutPlannerProbes(t, ev)
	for _, relation := range []struct {
		name  string
		bound int64
	}{{"indexed_pages_pkey", 2 * int64(wanted)}, {"page_links_resolved_id_source_id_idx", int64(wanted)}} {
		w, ok := work[relation.name]
		if !ok {
			t.Fatalf("Orphan items have no actual work counter for %s", relation.name)
		}
		if w.Scans > relation.bound || w.Returned > relation.bound || w.Fetched > relation.bound {
			t.Errorf("Orphan items inspected candidates past the requested page: %s=%+v > %d for %d eligible items", relation.name, w, relation.bound, wanted)
		}
	}
}

func TestLintPlansRules(t *testing.T) {
	if testing.Short() {
		t.Skip("fresh rules matrix and complete parent-relation cost")
	}
	baseline := map[string]lintPlanEvidence{}
	for _, shape := range []struct {
		name, prefix string
		pages        int
	}{{"A1", "00000000", 50000}, {"B", "f0000000", 75000}, {"A2", "00000000", 50000}} {
		t.Run(shape.name, func(t *testing.T) {
			f := lintRulesPlanFixture(t, shape.prefix, shape.pages)
			ids := make([]uuid.UUID, 2000)
			for i := range ids {
				ids[i] = lintReaderID(uint64(i + 1))
			}
			compare := func(t *testing.T, key string, ev lintPlanEvidence) {
				if shape.name == "A1" {
					baseline[key] = ev
				} else {
					lintCompareHidden(t, baseline[key], ev)
				}
			}
			for _, stats := range []string{"no-statistics", "analyzed"} {
				if stats == "analyzed" {
					lintAnalyze(t, f)
				}
				t.Run(stats, func(t *testing.T) {
					for _, generic := range []bool{false, true} {
						t.Run(fmt.Sprintf("generic=%t", generic), func(t *testing.T) {
							for _, rule := range []string{"Orphan", "Frontmatter", "MissingType", "MissingSummary"} {
								t.Run(rule, func(t *testing.T) {
									for _, part := range []struct {
										name, phase string
										after       uint64
									}{{"count", "Count", 0}, {"first", "First", 0}, {"middle", "After", 1000}, {"tail", "After", 1990}, {"empty", "After", 2000}} {
										t.Run(part.name, func(t *testing.T) {
											stmt := lintRuleStatement(t, rule, part.phase, f.eng, ids, nil, lintReaderID(part.after))
											plan, ev := lintExecuted(t, f, stmt, generic)
											want := 101
											switch part.name {
											case "count":
												want = 1
											case "empty":
												want = 0
											case "tail":
												want = 10
												if rule == "Frontmatter" {
													want = 3
												} else if strings.HasPrefix(rule, "Missing") {
													want = 7
												}
											}
											if ev.Rows != want {
												t.Fatalf("real %s/%s rows=%d want %d", rule, part.name, ev.Rows, want)
											}
											lintRulesPhysical(t, plan, ev, rule)
											if rule == "Orphan" && part.phase != "Count" {
												lintOrphanItemsWork(t, ev, want)
											}
											compare(t, fmt.Sprintf("%s/%t/%s/%s", stats, generic, rule, part.name), ev)
										})
									}
								})
							}
							for _, n := range []int{0, 10000, 100000} {
								t.Run(fmt.Sprintf("F/parents=%d", n), func(t *testing.T) {
									parents := make([]uuid.UUID, n)
									for i := range parents {
										parents[i] = lintReaderID(uint64(i + 1))
									}
									assets := make([]uuid.UUID, 512)
									for i := range assets {
										assets[i] = lintReaderID(uint64(3001 + i))
									}
									stmt := statementOf(t, func(q *gen.Queries) error {
										_, err := q.LintPendingAssets(t.Context(), gen.LintPendingAssetsParams{NotebookID: f.eng, Extractor: domain.Extractor, ParentIds: parents, AssetIds: assets})
										return err
									})
									plan, ev := lintExecuted(t, f, stmt, generic)
									want := 256
									if n == 0 {
										want = 512
									}
									if ev.Rows != want {
										t.Fatalf("real F rows=%d want=%d", ev.Rows, want)
									}
									lintRulesPhysical(t, plan, ev, "F")
									if n > 0 {
										lintAssertMembership(t, ev, "parents", n)
									}
									compare(t, fmt.Sprintf("%s/%t/F/%d", stats, generic, n), ev)
								})
							}
						})
					}
				})
			}
		})
	}
}
