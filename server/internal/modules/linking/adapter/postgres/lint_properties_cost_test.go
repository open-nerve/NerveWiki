//go:build !race

package postgresadapter_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type lintPropertyCostFixture struct {
	c             *lintCostFixture
	ids           []uuid.UUID
	contents      []string
	sourceTarget  string
	firstPosition int32
	jsonBytes     int64
	necessaryJSON int64
	maxJSON       int64
}

func lintPropertyCostSeed(t *testing.T, foreign string) lintPropertyCostFixture {
	t.Helper()
	c := newLintCostFixture(t)
	f := lintPropertyCostFixture{c: c, firstPosition: 1003}
	rng := rand.New(rand.NewPCG(20261030, 6))
	trim := []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff}
	var whitespace strings.Builder
	whitespace.WriteString(strings.Repeat(" ", 257))
	for range 64 * 1024 {
		whitespace.WriteRune(trim[rng.IntN(len(trim))])
	}
	random := make([]byte, 384*1024)
	for i := range random {
		random[i] = byte(rng.Uint32())
	}
	large := base64.RawURLEncoding.EncodeToString(random)
	f.sourceTarget = large + "/x"
	// ASCII YAML escapes keep the serialized seed unambiguous; the exact
	// decoded Unicode value is asserted before any measured read.
	quote := strconv.QuoteToASCII
	for i, value := range []string{"x" + whitespace.String(), whitespace.String() + "x", whitespace.String(), "", "note"} {
		var content strings.Builder
		content.WriteString("---\n")
		if i == 4 {
			for k := range 1000 {
				fmt.Fprintf(&content, "unrelated%04d: x\n", k)
			}
		}
		if i != 3 {
			fmt.Fprintf(&content, "TYPE: %s\nsummary: %s\n", quote(value), quote(value))
		}
		fmt.Fprintf(&content, "other: %s\n", quote(large))
		if i == 4 {
			fmt.Fprintf(&content, "SoUrCeS: '[[%s]]'\nsources: later-is-not-the-first\n\"\": '[[empty-property]]'\n", f.sourceTarget)
		} else if i != 3 {
			// Contradict the first key, without using a duplicate YAML key.
			late := "late"
			if i < 2 {
				late = " "
			}
			fmt.Fprintf(&content, "type: %s\nSummary: %s\n", quote(late), quote(late))
		}
		content.WriteString("---\n")
		content.WriteString(large) // Real incompressible body, independently read below.
		if content.Len() > 5*1024*1024 {
			t.Fatal("necessary-property seed exceeds the actual content bound")
		}
		facts := c.page(t, uint64(i), fmt.Sprintf("property-%d", i), content.String(), nil, false)
		if !facts.FrontmatterValid {
			t.Fatalf("property fixture %d frontmatter is invalid", i)
		}
		if i != 3 {
			position := 0
			if i == 4 {
				position = 1000
			}
			for j, key := range []string{"TYPE", "summary"} {
				var parsed string
				if len(facts.Properties) <= position+j || facts.Properties[position+j].Key != key {
					t.Fatalf("property fixture %d lost ordered %s", i, key)
				}
				if err := json.Unmarshal(facts.Properties[position+j].Value, &parsed); err != nil || parsed != value {
					t.Fatalf("property fixture %d parser changed %s: bytes=%d want=%d err=%v", i, key, len(parsed), len(value), err)
				}
			}
		}
		f.ids = append(f.ids, lintCostID(uint64(i)))
		f.contents = append(f.contents, content.String())
	}
	// Only this necessary-property fixture needs real page_contents. The
	// linking index still came from the actual parser/Replace for these bytes.
	user, workspace := uuid.NewV7(), uuid.NewV7()
	at := lintCostTime(0)
	lintExec(t, c.f, `INSERT INTO users(id,email,password,display_name,created_at,updated_at) VALUES($1,'lint-cost@example.test','x','cost',$2,$2)`, user, at)
	lintExec(t, c.f, `INSERT INTO workspaces(id,slug,name,created_by_id,updated_by_id,created_at,updated_at) VALUES($1,'lint-cost','cost',$2,$2,$3,$3)`, workspace, user, at)
	lintExec(t, c.f, `INSERT INTO notebooks(id,workspace_id,name,created_by_id,updated_by_id,created_at,updated_at) VALUES($1,$2,'cost',$3,$3,$4,$4)`, c.f.eng, workspace, user, at)
	for i, id := range f.ids {
		lintExec(t, c.f, `INSERT INTO nodes(id,notebook_id,kind,name,name_key,sort_order,created_by_id,updated_by_id,created_at,updated_at) VALUES($1,$2,'page',$3,$3,$4,$5,$5,$6,$6)`, id, c.f.eng, fmt.Sprintf("property-%d", i), float64(i), user, at)
		lintExec(t, c.f, `INSERT INTO page_contents(node_id,content,revision,content_hash,byte_size,updated_by_id,updated_at) VALUES($1,$2,1,sha256(convert_to($2,'UTF8')),octet_length($2),$3,$4)`, id, f.contents[i], user, at)
	}
	c.foreign(t, foreign, 50000)
	var value string
	if err := c.f.pool.QueryRow(t.Context(), `SELECT value::text FROM page_properties WHERE source_id=$1 AND position=$2`, f.ids[4], f.firstPosition).Scan(&value); err != nil || value != `"[[`+f.sourceTarget+`]]"` {
		t.Fatalf("actual first sources at 1003: bytes=%d err=%v", len(value), err)
	}
	f.jsonBytes = int64(len(value))
	if err := c.f.pool.QueryRow(t.Context(), `SELECT sum(octet_length(value::text)),max(octet_length(value::text)) FROM page_properties
 WHERE (source_id=ANY($1::uuid[]) AND position IN(0,1)) OR (source_id=$2 AND position=$3)`, f.ids[1:3], f.ids[4], f.firstPosition).Scan(&f.necessaryJSON, &f.maxJSON); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"page_properties", "page_contents"} {
		var size int64
		if err := c.f.pool.QueryRow(t.Context(), `SELECT pg_relation_size(reltoastrelid) FROM pg_class WHERE oid=$1::regclass`, table).Scan(&size); err != nil || size <= 0 {
			t.Fatalf("%s lacks actual external TOAST: %d/%v", table, size, err)
		}
	}
	lintEvidence(t, "lint-property-cost-fixture", map[string]any{"foreign_prefix": foreign, "foreign_pages": 50000, "reader_pages": len(f.ids),
		"first_sources_position": f.firstPosition, "complete_sources_JSON_bytes": f.jsonBytes, "necessary_distinct_JSON_bytes": f.necessaryJSON,
		"largest_necessary_JSON_bytes": f.maxJSON, "body_bytes": len(large), "warm_seed": true})
	return f
}

func lintCostToastBlocks(ctx context.Context, db postgres.Querier, table string) (int64, error) {
	var n int64
	err := db.QueryRow(ctx, `SELECT coalesce(sum(pg_stat_get_xact_blocks_fetched(r.oid)),0)::bigint FROM (
 SELECT reltoastrelid AS oid FROM pg_class WHERE oid=$1::regclass
 UNION ALL SELECT indexrelid FROM pg_index WHERE indrelid=(SELECT reltoastrelid FROM pg_class WHERE oid=$1::regclass)) r`, table).Scan(&n)
	return n, err
}

func (f lintPropertyCostFixture) measure(t *testing.T, name string, propertyToast, bodyToast bool, run func(context.Context) error) {
	t.Helper()
	var samples []lintCostMetrics
	for sample := range 3 {
		var metrics lintCostMetrics
		var metricEvidence *lintCostMetrics
		var prop, body int64
		var physicalEvidence map[string]int64
		var statementCache *int64
		var work map[string]lintWork
		f.c.trace.begin()
		err := f.c.f.tx.WithinSnapshot(t.Context(), func(ctx context.Context) error {
			db := postgres.DB(ctx, f.c.f.pool)
			tx, ok := postgres.TxFrom(ctx)
			if !ok {
				return fmt.Errorf("necessary-value read is outside its actual snapshot")
			}
			before := lintRelationWork(t, tx)
			p0, err := lintCostToastBlocks(ctx, db, "page_properties")
			if err != nil {
				return err
			}
			b0, err := lintCostToastBlocks(ctx, db, "page_contents")
			if err != nil {
				return err
			}
			metrics, err = lintCostMeasure(t, func() error { return run(ctx) })
			metricEvidence = &metrics
			if err != nil {
				return err
			}
			p1, err := lintCostToastBlocks(ctx, db, "page_properties")
			if err != nil {
				return err
			}
			b1, err := lintCostToastBlocks(ctx, db, "page_contents")
			if err != nil {
				return err
			}
			prop, body = p1-p0, b1-b0
			physicalEvidence = map[string]int64{"property_TOAST_blocks": prop, "body_TOAST_blocks": body}
			work = lintWorkDelta(before, lintRelationWork(t, tx))
			var cached int64
			if err := db.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&cached); err != nil {
				return err
			}
			statementCache = &cached
			return nil
		})
		// The body-positive query deliberately is not a lint query.
		queries, modes, queryNS := f.c.trace.end()
		status := "success"
		if err != nil {
			status = "failed; unobserved metrics or physical deltas are null"
		}
		lintEvidence(t, "lint-property-cost-read", map[string]any{"case": t.Name(), "name": name, "sample": sample, "status": status, "metrics": metricEvidence, "error": fmt.Sprint(err),
			"physical_deltas": physicalEvidence, "work": work, "queries": queries, "query_modes": modes, "observed_lint_SQL_and_scan_wall_ns": queryNS,
			"named_lint_statements": statementCache,
			"scope":                 "only actual read/scan/JSON work inside caller snapshot; seed/physical-counter queries excluded from phase timer; Go process CPU/alloc/1ms sampled heap, not PG RSS"})
		if err != nil {
			t.Fatal(err)
		}
		if !bodyToast {
			lintCostDescribeExec(t, queries, modes)
		}
		if statementCache == nil || *statementCache != 0 {
			t.Fatalf("production read created named lint statements: %v", statementCache)
		}
		if (prop > 0) != propertyToast || (body > 0) != bodyToast {
			t.Fatalf("necessary/irrelevant physical reads %s: property/body=%d/%d, want positive=%t/%t", name, prop, body, propertyToast, bodyToast)
		}
		samples = append(samples, metrics)
	}
	lintCostMedian(t, "properties/"+name, name, samples)
}

func TestLintPropertiesReadOnlyNecessaryValues(t *testing.T) {
	if testing.Short() {
		t.Skip("necessary property and real backend TOAST cost measurements")
	}
	// Each A/B/A is a fresh INSERT database. None claims the documented
	// restored-MVCC page metadata boundary has been fixed.
	for part, hidden := range []string{"70000000", "08000000", "70000000"} {
		t.Run(fmt.Sprintf("fresh-%d-%s", part, hidden), func(t *testing.T) {
			f := lintPropertyCostSeed(t, hidden)
			for _, statistics := range []string{"no-statistics", "analyzed"} {
				t.Run(statistics, func(t *testing.T) {
					if statistics == "analyzed" {
						lintAnalyze(t, f.c.f)
					} else {
						lintAssertStatistics(t, f.c.f, false)
					}
					for _, rule := range lintPageRules(f.c.f.s)[2:] {
						t.Run(rule.name, func(t *testing.T) {
							f.measure(t, rule.name+"-prefix-and-unrelated", false, false, func(ctx context.Context) error {
								n, err := rule.count(ctx, app.PageRuleInput{NotebookID: f.c.f.eng, CandidateIDs: []uuid.UUID{f.ids[0], f.ids[3], f.ids[4]}})
								if err == nil && n != (domain.Count{Value: 1}) {
									return fmt.Errorf("first key/prefix/absent property count=%+v want only unrelated-only page", n)
								}
								return err
							})
							f.measure(t, rule.name+"-complete-needed-values", true, false, func(ctx context.Context) error {
								in := app.PageRuleInput{NotebookID: f.c.f.eng, CandidateIDs: f.ids}
								n, err := rule.count(ctx, in)
								if err != nil {
									return err
								}
								first, err := rule.items(ctx, in, nil, 101)
								if err != nil {
									return err
								}
								tail, err := rule.items(ctx, in, &f.ids[2], 101)
								if err == nil && (n != (domain.Count{Value: 2}) || !slices.Equal(first, f.ids[2:4]) || !slices.Equal(tail, f.ids[3:4])) {
									return fmt.Errorf("full whitespace/first-key count=%+v first=%v tail=%v; want two specified IDs", n, first, tail)
								}
								return err
							})
						})
					}
					f.measure(t, "first-sources-complete-JSON-and-slot", true, false, func(ctx context.Context) error {
						heads, err := f.c.reads.SourcePropertyHeads(ctx, f.c.f.eng, f.ids)
						if err != nil {
							return err
						}
						if len(heads) != 1 || heads[0].PageID != f.ids[4] || heads[0].Position != f.firstPosition || heads[0].ValueBytes != f.jsonBytes {
							return fmt.Errorf("1000 unrelated keys changed first source header: %+v", heads)
						}
						values, err := f.c.reads.SourceProperties(ctx, f.c.f.eng, []app.SourcePropertyPos{heads[0].SourcePropertyPos})
						if err != nil {
							return err
						}
						if len(values) != 1 || values[0].Key != "SoUrCeS" || string(values[0].JSON) != `"[[`+f.sourceTarget+`]]"` {
							return fmt.Errorf("first source full JSON changed: %d rows", len(values))
						}
						matcher, err := domain.NewSourceSlots(ctx, values[0].Key, values[0].JSON)
						if err != nil {
							return err
						}
						slots, err := f.c.reads.SourceSlotHeads(ctx, f.c.f.eng, []app.SourceSpec{{PageID: f.ids[4], Key: "SoUrCeS"}}, nil, 512)
						if err != nil {
							return err
						}
						if len(slots) != 1 || slots[0].PropertyKey != "SoUrCeS" || slots[0].TargetBytes != int64(len(f.sourceTarget)) {
							return fmt.Errorf("first source slot was not bounded/exact: %+v", slots)
						}
						edges, err := f.c.reads.SourceSlotEdges(ctx, f.c.f.eng, []app.EdgePos{slots[0].EdgePos})
						if err != nil {
							return err
						}
						if len(edges) != 1 || edges[0].Target != f.sourceTarget {
							return fmt.Errorf("full source target was not read")
						}
						if err := matcher.Observe(edges[0].PropertyKey, edges[0].Target, edges[0].ResolvedID, edges[0].ResolvedAsset); err != nil {
							return err
						}
						judgments, err := matcher.Finish(ctx)
						if err == nil && (len(judgments) != 1 || judgments[0].NotLink || judgments[0].FullText != f.sourceTarget) {
							return fmt.Errorf("complete JSON transformation changed its literal link judgment")
						}
						return err
					})
					f.measure(t, "actual-body-positive-control", false, true, func(ctx context.Context) error {
						var body string
						err := postgres.DB(ctx, f.c.f.pool).QueryRow(ctx, "SELECT content FROM page_contents WHERE node_id=$1", f.ids[4]).Scan(&body)
						if err == nil && body != f.contents[4] {
							return fmt.Errorf("actual page body positive control changed bytes")
						}
						return err
					})
					// Actual named generic execution is separate from production
					// DescribeExec above; no setting alone is called a generic plan.
					for _, rule := range []string{"MissingType", "MissingSummary"} {
						for _, phase := range []string{"Count", "First", "After"} {
							t.Run("actual-generic-"+rule+"-"+phase, func(t *testing.T) {
								_, evidence := lintExecuted(t, f.c.f, lintRuleStatement(t, rule, phase, f.c.f.eng, f.ids, nil, f.ids[2]), true)
								if evidence.GenericPlans < 1 || evidence.CustomPlans != 0 || evidence.Rows == 0 {
									t.Fatalf("missing real nonempty generic execution: %+v", evidence)
								}
							})
						}
					}
				})
			}
		})
	}
}
