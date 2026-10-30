//go:build !race

package postgresadapter_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

type lintCostExpected struct {
	key, target string
	pages       int64
	cut         bool
	position    domain.Position
	kind        domain.NodeKind
	bytes       int64
}

func lintCostWanted(raw, canonical string, pages int64) lintCostExpected {
	hash := lintHashText(canonical)
	return lintCostExpected{key: "w:" + hash, target: raw[:min(256, len(raw))], cut: len(raw) > 256, pages: pages,
		position: domain.Position{Pages: pages, Prefix: []byte(canonical[:min(256, len(canonical))]), Hash: hash}}
}

func lintCostPage(rule domain.Rule, n uint64) lintCostExpected {
	id := lintCostID(n)
	return lintCostExpected{key: "p:" + id.String(), position: domain.Position{ID: &id}}
}

func lintCostSortWanted(items []lintCostExpected) {
	slices.SortFunc(items, func(a, b lintCostExpected) int {
		if a.pages > b.pages {
			return -1
		}
		if a.pages < b.pages {
			return 1
		}
		if n := strings.Compare(string(a.position.Prefix), string(b.position.Prefix)); n != 0 {
			return n
		}
		return strings.Compare(a.position.Hash, b.position.Hash)
	})
}

func lintCostAssert(t *testing.T, rule domain.Rule, got []domain.Item, more bool, count domain.Count, all []lintCostExpected, offset, limit int) {
	t.Helper()
	wantCount := domain.Count{Value: int64(min(len(all), 10000)), Capped: len(all) >= 10000}
	want := all[offset:min(len(all), offset+limit)]
	if count != wantCount || len(got) != len(want) || more != (offset+len(want) < len(all)) {
		t.Fatalf("independent %s count/page offset=%d: count=%+v len=%d more=%t; want %+v len=%d more=%t", rule, offset, count, len(got), more, wantCount, len(want), offset+len(want) < len(all))
	}
	for i, item := range got {
		w := want[i]
		if item.Rule != rule || item.ItemKey != w.key || item.Cursor == "" ||
			rule == domain.Wanted && (item.Target != w.target || item.TargetCut != w.cut || item.Pages != w.pages) {
			t.Fatalf("independent %s item %d=%+v; want key=%s target=%q pages=%d cut=%t", rule, offset+i, item, w.key, w.target, w.pages, w.cut)
		}
		if rule == domain.UndigestedSources && (item.NodeID == nil || *item.NodeID != *w.position.ID || !item.CreatedAt.Equal(w.position.CreatedAt) || item.Bytes != w.bytes || item.Kind != w.kind) {
			t.Fatalf("pending metadata was not real fixture metadata: %+v", item)
		}
	}
}

func (c *lintCostFixture) sample(t *testing.T, query app.RuleQuery, expected map[domain.Rule][]lintCostExpected, offset, limit int) lintCostMetrics {
	t.Helper()
	c.trace.begin()
	c.reads.payloads = map[string]lintCostPayload{}
	var prepared *app.Prepared
	var timings app.PreparationTimings
	results := map[domain.Rule]struct {
		count domain.Count
		items []domain.Item
		more  bool
	}{}
	var statementCache *int64
	var prepareWall *int64
	measurement, err := lintCostMeasure(t, func() error {
		return c.f.tx.WithinSnapshot(t.Context(), func(ctx context.Context) error {
			var err error
			started := time.Now()
			prepared, err = c.rules.Prepare(ctx, c.in, query)
			elapsed := time.Since(started).Nanoseconds()
			prepareWall = &elapsed
			if err != nil {
				return err
			}
			timings = prepared.Timings()
			for rule := range expected {
				count, err := prepared.RuleCount(ctx, rule)
				if err != nil {
					return err
				}
				items, more, err := prepared.RuleItems(ctx, rule)
				if err != nil {
					return err
				}
				results[rule] = struct {
					count domain.Count
					items []domain.Item
					more  bool
				}{count, items, more}
			}
			var cached int64
			if err := postgres.DB(ctx, c.f.pool).QueryRow(ctx, `SELECT count(*) FROM pg_prepared_statements WHERE statement LIKE '-- name: Lint%'`).Scan(&cached); err != nil {
				return err
			}
			statementCache = &cached
			return nil
		})
	})
	queries, modes, queryNS := c.trace.end()
	status := "success"
	var timingEvidence any
	var unassignedPrepare any
	if err != nil {
		status = "failed; prepared statistics unavailable"
	} else {
		timingEvidence = timings
		accounted := time.Duration(0)
		for _, timing := range timings.Rules {
			accounted += timing.Duration
		}
		if timings.SharedSources != nil {
			accounted += *timings.SharedSources
		}
		unassignedPrepare = *prepareWall - accounted.Nanoseconds()
	}
	lintEvidence(t, "lint-cost-rule", map[string]any{"case": t.Name(), "status": status, "error": fmt.Sprint(err), "metrics": measurement,
		"observed_lint_SQL_and_scan_wall_ns": queryNS, "other_observed_wall_ns": measurement.WallNS - queryNS,
		"queries": queries, "query_modes": modes, "payloads": c.reads.payloads, "preparation_timings": timingEvidence,
		"observed_Prepare_wall_ns": prepareWall, "Prepare_heads_scope_projection_and_unassigned_ns": unassignedPrepare,
		"named_lint_statements": statementCache, "source_ids": len(c.in.SourceIDs), "metadata_nodes": len(c.in.Nodes),
		"scope": "actual linking Prepare plus result reads and snapshot lifecycle; observed lint SQL spans include scan; other wall also includes snapshot and statement-cache diagnostic SQL, result reads and Go work, not pure Go or CPU; Go process metrics include observation overhead, exclude seed and PostgreSQL RSS"})
	if err != nil {
		t.Fatal(err)
	}
	lintCostDescribeExec(t, queries, modes)
	_, invalid := expected[domain.InvalidSources]
	_, pending := expected[domain.UndigestedSources]
	if statementCache == nil || *statementCache != 0 || prepared.StalePages() != c.stale || len(timings.Rules) != len(expected) || (timings.SharedSources != nil) != (invalid || pending) {
		t.Fatalf("actual consumer cache/stale/timings=%v/%d/%+v; want cache0/%d/%d rules and shared=%t", statementCache, prepared.StalePages(), timings, c.stale, len(expected), invalid || pending)
	}
	for rule, want := range expected {
		got := results[rule]
		lintCostAssert(t, rule, got.items, got.more, got.count, want, offset, limit)
	}
	for name, payload := range c.reads.payloads {
		if payload.MaxRows > app.LintReadRows || payload.MaxBatch > app.LintReadTargetBytes && payload.MaxBatch != payload.MaxItem {
			t.Fatalf("ordinary exact payload %s exceeded 8MiB: %+v", name, payload)
		}
	}
	return measurement
}

func lintCostRulePages(t *testing.T, c *lintCostFixture, group string, rule domain.Rule, want []lintCostExpected) {
	t.Helper()
	if len(want) == 0 {
		t.Fatal("cost rule requires a nonempty independent oracle")
	}
	for _, page := range []struct {
		name   string
		offset int
	}{{"first", 0}, {"middle", len(want) / 2}, {"tail", max(0, len(want)-2)}, {"empty", len(want)}} {
		t.Run(page.name, func(t *testing.T) {
			query := app.RuleQuery{Rule: &rule, Limit: 2}
			if page.offset > 0 {
				position := want[page.offset-1].position
				query.After = &position
			}
			var samples []lintCostMetrics
			for range 3 {
				samples = append(samples, c.sample(t, query, map[domain.Rule][]lintCostExpected{rule: want}, page.offset, 2))
			}
			lintCostMedian(t, group, string(rule)+"/"+page.name, samples)
			c.trace.encoding(t)
		})
	}
}

func lintCostEight(t *testing.T) (*lintCostFixture, map[domain.Rule][]lintCostExpected) {
	t.Helper()
	c := newLintCostFixture(t)
	root := lintCostID(0)
	c.in.EffectiveMode, c.in.AreaRootID = domain.ModeLLM, &root
	complete := "---\ntype: note\nsummary: ready\n---\nbody"
	c.page(t, 0, "raw", complete, nil, false)
	c.page(t, 1, "agents", complete, nil, false)
	c.bulk(t, 100000, 100000, "source", &root, true)
	c.bulk(t, 1000, 12000, "[[stale-wanted]]", nil, false)
	lintExec(t, c.f, `UPDATE indexed_pages SET extractor=1 WHERE notebook_id=$1 AND node_id BETWEEN $2 AND $3`, c.f.eng, lintCostID(1000), lintCostID(12999))
	c.stale = 12000
	c.bulk(t, 20000, 12000, "", nil, false)
	for i := uint64(0); i < 12000; i++ {
		parent := lintCostID(20000 + i)
		c.in.Nodes = append(c.in.Nodes, app.CompactNode{ID: lintCostID(300000 + i), ParentID: &parent, Kind: domain.NodeAsset, NameKey: "child.bin", CreatedAt: lintCostTime(300000 + i)})
	}
	want := map[domain.Rule][]lintCostExpected{}
	for n := uint64(20000); n < 32000; n++ {
		want[domain.MissingType] = append(want[domain.MissingType], lintCostPage(domain.MissingType, n))
		want[domain.MissingSummary] = append(want[domain.MissingSummary], lintCostPage(domain.MissingSummary, n))
	}
	for n := uint64(100000); n < 200000; n++ {
		id := lintCostID(n)
		want[domain.UndigestedSources] = append(want[domain.UndigestedSources], lintCostExpected{key: "n:" + id.String(), position: domain.Position{ID: &id, CreatedAt: lintCostTime(n)}, kind: domain.NodePage, bytes: 6})
	}
	c.page(t, 400100, "target", complete, nil, false)
	for i := uint64(0); i < 7; i++ {
		n := 400000 + i
		content := fmt.Sprintf("---\ntype: ' '\nsummary: ' '\nsources: ['[[need-source]]', {x: 1}]\n\"\": '[[raw/page-100000]]'\n---\n[[Need%d]] [[ambig]]", i)
		facts := c.page(t, n, fmt.Sprintf("wiki-%d", i), content, nil, false)
		for _, link := range facts.Links {
			switch link.Target {
			case "ambig":
				c.f.resolve(t, lintCostID(n), domain.Resolution{ID: lintCostID(400100), Ambiguous: true}, link.Start)
			case "raw/page-100000":
				c.f.resolve(t, lintCostID(n), domain.Resolution{ID: lintCostID(100000)}, link.Start)
			}
		}
		want[domain.Wanted] = append(want[domain.Wanted], lintCostWanted(fmt.Sprintf("Need%d", i), fmt.Sprintf("need%d", i), 1))
		for _, rule := range []domain.Rule{domain.Orphans, domain.MissingType, domain.MissingSummary} {
			want[rule] = append(want[rule], lintCostPage(rule, n))
		}
		id := lintCostID(n)
		want[domain.Ambiguous] = append(want[domain.Ambiguous], lintCostExpected{key: "a:" + id.String() + ":" + lintHashText("ambig"), position: domain.Position{ID: &id, Hash: lintHashText("ambig")}})
		var invalid []lintCostExpected
		for _, raw := range []struct {
			text string
			not  bool
		}{{"need-source", false}, {`{"x":1}`, true}} {
			key := lintSourceIdentity(id, raw.text, raw.not)
			invalid = append(invalid, lintCostExpected{key: key, position: domain.Position{ID: &id, Hash: key[len(key)-64:]}})
		}
		slices.SortFunc(invalid, func(a, b lintCostExpected) int { return strings.Compare(a.key, b.key) })
		want[domain.InvalidSources] = append(want[domain.InvalidSources], invalid...)
		c.page(t, 400200+i, fmt.Sprintf("bad-%d", i), "---\na: [\n---\n", nil, false)
		want[domain.InvalidFrontmatter] = append(want[domain.InvalidFrontmatter], lintCostPage(domain.InvalidFrontmatter, 400200+i))
		c.page(t, 400300+i, fmt.Sprintf("orphan-%d", i), complete, nil, false)
		asset := lintCostID(200000 + i)
		c.in.Nodes = append(c.in.Nodes, app.CompactNode{ID: asset, ParentID: &root, Kind: domain.NodeAsset, NameKey: fmt.Sprintf("asset-%d.pdf", i), Linkable: true, CreatedAt: lintCostTime(200000 + i)})
		c.in.SourceIDs = append(c.in.SourceIDs, asset)
		c.sizes.values[asset] = 12 + int64(i)
		want[domain.UndigestedSources] = append(want[domain.UndigestedSources], lintCostExpected{key: "n:" + asset.String(), position: domain.Position{ID: &asset, CreatedAt: lintCostTime(200000 + i)}, kind: domain.NodeAsset, bytes: 12 + int64(i)})
	}
	for _, start := range []uint64{400200, 400300} {
		for n := start; n < start+7; n++ {
			want[domain.Orphans] = append(want[domain.Orphans], lintCostPage(domain.Orphans, n))
		}
	}
	want[domain.Wanted] = append(want[domain.Wanted], lintCostWanted("need-source", "need-source", 7))
	lintCostSortWanted(want[domain.Wanted])
	c.foreign(t, "70000000", 50000)
	var body, empty int
	if err := c.f.pool.QueryRow(t.Context(), `SELECT count(*) FILTER(WHERE property_key IS NULL),count(*) FILTER(WHERE property_key='') FROM page_links WHERE source_id=$1`, lintCostID(400000)).Scan(&body, &empty); err != nil || body != 2 || empty != 1 {
		t.Fatalf("real parsed origin witness: body=%d empty-property=%d err=%v", body, empty, err)
	}
	lintEvidence(t, "lint-cost-eight-fixture", map[string]any{"source_IDs": len(c.in.SourceIDs), "stale": c.stale, "zero_folders_before_cap": 12000, "foreign_pages": 50000,
		"expected": map[string]int{"wanted": 8, "orphans": 21, "ambiguous": 7, "invalid_frontmatter": 7, "missing_type": 12007, "missing_summary": 12007, "undigested_sources": 100007, "invalid_sources": 14}})
	return c, want
}

func TestLintRulesOfAHundredThousandPagesTakeTheirTime(t *testing.T) {
	if testing.Short() {
		t.Skip("100k-page linking consumer costs run alone without race")
	}
	t.Run("eight-rules-and-100k-sources", func(t *testing.T) {
		c, want := lintCostEight(t)
		for _, statistics := range []string{"no-statistics", "analyzed"} {
			t.Run(statistics, func(t *testing.T) {
				if statistics == "analyzed" {
					lintAnalyze(t, c.f)
				} else {
					lintAssertStatistics(t, c.f, false)
				}
				var summary []lintCostMetrics
				for range 3 {
					summary = append(summary, c.sample(t, app.RuleQuery{}, want, 0, 5))
				}
				lintCostMedian(t, "rules/eight/summary", "linking-summary-not-wiki-execute", summary)
				for _, rule := range domain.ApplicableRules(domain.ModeLLM) {
					t.Run(string(rule), func(t *testing.T) { lintCostRulePages(t, c, "rules/eight/"+string(rule), rule, want[rule]) })
				}
			})
		}
	})
	for _, shape := range []string{"hot-many-spellings-kinds", "unique-long-targets", "relative-deep-folder", "same-last-key-presence", "one-page-10000-edges", "100k-excluded-and-outside"} {
		t.Run(shape, func(t *testing.T) {
			c, want := lintCostWantedShape(t, shape)
			lintAnalyze(t, c.f)
			rule := domain.Wanted
			var samples []lintCostMetrics
			for range 3 {
				samples = append(samples, c.sample(t, app.RuleQuery{Rule: &rule, Limit: 5}, map[domain.Rule][]lintCostExpected{rule: want}, 0, 5))
			}
			lintCostMedian(t, "rules/wanted/"+shape, shape, samples)
			c.trace.encoding(t)
			if shape == "unique-long-targets" {
				lintCostRulePages(t, c, "rules/wanted/"+shape, rule, want)
			}
		})
	}
}

func lintCostWantedShape(t *testing.T, shape string) (*lintCostFixture, []lintCostExpected) {
	t.Helper()
	c := newLintCostFixture(t)
	var want []lintCostExpected
	switch shape {
	case "hot-many-spellings-kinds":
		c.bulk(t, 100000, 100000, "[[HOT]] [[hot.md]] [[/hot]] [other](Other) ![[Other]] ![asset](Image.png)", nil, false)
		want = []lintCostExpected{lintCostWanted("HOT", "hot", 100000), lintCostWanted("Other", "other", 100000)}
	case "unique-long-targets":
		c.bulk(t, 100000, 100000, "", nil, false)
		for group := range 10 {
			var content strings.Builder
			for j := range 10000 {
				raw := strings.Repeat("longsegment/", 23) + fmt.Sprintf("target-%06d", group*10000+j)
				fmt.Fprintf(&content, "[[%s]]\n", raw)
				want = append(want, lintCostWanted(raw, raw, 1))
			}
			if content.Len() > 5*1024*1024 {
				t.Fatal("long-target prototype exceeds the real page limit")
			}
			c.page(t, uint64(500000+group), fmt.Sprintf("long-%d", group), content.String(), nil, false)
		}
	case "relative-deep-folder":
		var parent *uuid.UUID
		for n := uint64(1); n <= 9; n++ {
			c.page(t, n, fmt.Sprintf("d%d", n), "", parent, false)
			id := lintCostID(n)
			parent = &id
		}
		c.bulk(t, 100000, 100000, "[[../Need]] [[./Child]]", parent, false)
		want = []lintCostExpected{lintCostWanted("../Need", "d1/d2/d3/d4/d5/d6/d7/d8/need", 100000), lintCostWanted("./Child", "d1/d2/d3/d4/d5/d6/d7/d8/d9/child", 100000)}
	case "same-last-key-presence":
		var parent *uuid.UUID
		for n := uint64(1); n <= 7; n++ {
			c.page(t, n, fmt.Sprintf("d%d", n), "", parent, false)
			id := lintCostID(n)
			parent = &id
		}
		c.bulk(t, 100000, 100000, "", nil, false)
		for i := 7; i < len(c.in.Nodes); i += 2 {
			c.in.Nodes[i].ParentID = parent
			c.in.Nodes[i].NameKey = fmt.Sprintf("folder-%d", i)
			id := c.in.Nodes[i].ID
			c.in.Nodes[i+1].ParentID, c.in.Nodes[i+1].NameKey = &id, "last.bin"
		}
		for n := uint64(0); n < 50000; n++ {
			id := lintCostID(100000 + n*2)
			c.in.Nodes = append(c.in.Nodes, app.CompactNode{ID: lintCostID(600000 + n), ParentID: &id, Kind: domain.NodeAsset, NameKey: "last.bin", Linkable: true, CreatedAt: lintCostTime(600000 + n)})
		}
		c.page(t, 500000, "emitter", "[[last.bin]] [[ghost/last.bin.md]] ![image](ghost/last.bin.md)", parent, false)
		want = []lintCostExpected{lintCostWanted("ghost/last.bin.md", "ghost/last.bin", 1)}
	case "one-page-10000-edges":
		c.bulk(t, 100000, 100000, "", nil, false)
		c.page(t, 500000, "emitter", strings.Repeat("[[Same]]\n", 10000), nil, false)
		want = []lintCostExpected{lintCostWanted("Same", "same", 1)}
	case "100k-excluded-and-outside":
		root := lintCostID(0)
		c.in.EffectiveMode, c.in.AreaRootID = domain.ModeLLM, &root
		c.page(t, 0, "raw", "", nil, false)
		c.bulk(t, 100000, 100000, "[[inside-only]]", &root, true)
		c.bulk(t, 500000, 20000, "[[Outside]] [[outside.md]]", nil, false)
		want = []lintCostExpected{lintCostWanted("Outside", "outside", 20000)}
	default:
		t.Fatalf("unknown cost shape %q", shape)
	}
	c.foreign(t, "70000000", 50000)
	lintCostSortWanted(want)
	return c, want
}
