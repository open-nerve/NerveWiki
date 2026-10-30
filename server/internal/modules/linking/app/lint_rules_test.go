package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

func lintEdge(source uint64, start int32, raw, kind string) app.LintEdge {
	return app.LintEdge{EdgePos: app.EdgePos{SourceID: lintID(source), Start: start},
		Kind: kind, Target: raw, TargetKey: lintPtr(strings.ToLower(raw[strings.LastIndex(raw, "/")+1:]))}
}
func lintAnswer(t *testing.T, p *app.Prepared, r domain.Rule, count int64, capped bool) ([]domain.Item, bool) {
	t.Helper()
	got, err := p.RuleCount(t.Context(), r)
	if err != nil || got.Value != count || got.Capped != capped {
		t.Fatalf("%s count = %+v, %v; want %d capped=%t", r, got, err, count, capped)
	}
	items, more, err := p.RuleItems(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Rule != r || item.ItemKey == "" || item.Cursor == "" {
			t.Fatalf("%s returned an incomplete item: %+v", r, item)
		}
	}
	return items, more
}

func TestLintPrepareWanted(t *testing.T) {
	// Counting raw spellings, resetting seen at a batch, or choosing a
	// representative before the kind filter changes these literal answers.
	t.Run("union-and-qualified-representative", func(t *testing.T) {
		f, in := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1), lintNode(21, nil, "q", domain.NodePage, 1))
		f.edges = []app.LintEdge{lintEdge(20, 0, "GONE.PDF", "image"),
			lintEdge(20, 1, "Gone.pdf", "wikilink"), lintEdge(20, 2, "/GONE.PDF", "link"),
			lintEdge(21, 0, "gone.pdf", "wikilink")}
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted)})
		if err != nil {
			t.Fatal(err)
		}
		items, more := lintAnswer(t, p, domain.Wanted, 1, false)
		if more || len(items) != 1 || items[0].Target != "Gone.pdf" || items[0].Pages != 2 || items[0].TargetCut {
			t.Fatalf("wanted = %+v, more=%t; want one Gone.pdf representative on two pages", items, more)
		}
	})
	t.Run("same-source-across-byte-and-row-cuts", func(t *testing.T) {
		f, in := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1), lintNode(21, nil, "q", domain.NodePage, 1))
		raw := strings.Repeat("a", 32*1024) + "/x"
		for i := range 600 {
			f.edges = append(f.edges, lintEdge(20, int32(i), raw, "wikilink"))
		}
		f.edges = append(f.edges, lintEdge(21, 0, raw, "link"))
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted)})
		if err != nil {
			t.Fatal(err)
		}
		items, _ := lintAnswer(t, p, domain.Wanted, 1, false)
		if len(items) != 1 || items[0].Pages != 2 || !items[0].TargetCut || len(items[0].Target) != 256 {
			t.Fatalf("cross-cut union = %+v, want two pages and the preserved cut representative", items)
		}
		if len(f.valueSizes) < 3 {
			t.Fatalf("fixture crossed only %d value batches", len(f.valueSizes))
		}
		for _, n := range f.valueSizes {
			if n > 8<<20 {
				t.Fatalf("ordinary exact batch materialized %d bytes", n)
			}
		}
	})
	t.Run("cap-does-not-end-pagination", func(t *testing.T) {
		f, in := lintFixture(lintNode(20, nil, "p", domain.NodePage, 1), lintNode(21, nil, "q", domain.NodePage, 1))
		for i := range 10001 {
			source, start := uint64(20), int32(i)
			if i == 10000 {
				source, start = 21, 0
			}
			f.edges = append(f.edges, lintEdge(source, start, fmt.Sprintf("t%05d", i), "wikilink"))
		}
		key := "t09999"
		hash := sha256.Sum256([]byte(key))
		after := domain.Position{Pages: 1, Prefix: []byte(key), Hash: fmt.Sprintf("%x", hash)}
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted), After: &after, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		items, more := lintAnswer(t, p, domain.Wanted, 10000, true)
		if more || len(items) != 1 || items[0].Target != "t10000" || items[0].Pages != 1 {
			t.Fatalf("after the count cap = %+v more=%t, want t10000", items, more)
		}
	})
}

func TestLintPrepareScopeBeforeCap(t *testing.T) {
	root := lintID(10)
	nodes := []app.CompactNode{lintNode(10, nil, "raw", domain.NodePage, 0),
		lintNode(11, nil, "agents", domain.NodePage, 1), lintNode(12, nil, "stale", domain.NodePage, 1)}
	var sources, wanted []uuid.UUID
	for i := uint64(100); i < 10101; i++ {
		nodes = append(nodes, lintNode(i, &root, "source", domain.NodePage, 1))
		sources = append(sources, lintID(i))
	}
	for i := uint64(20001); i <= 20007; i++ {
		nodes = append(nodes, lintNode(i, nil, "wiki", domain.NodePage, 0))
		wanted = append(wanted, lintID(i))
	}
	f, in := lintFixture(nodes...)
	f.heads[2].Extractor = 1
	in.EffectiveMode, in.AreaRootID, in.SourceIDs = domain.ModeLLM, &root, sources
	f.sqlIDs[domain.Orphans] = append(slices.Clone(sources), wanted...)
	f.candidates[domain.Orphans] = wanted
	p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Orphans), Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	items, more := lintAnswer(t, p, domain.Orphans, 7, false)
	if len(items) != 3 || !more || items[0].PageID == nil || *items[0].PageID != wanted[0] || *items[2].PageID != wanted[2] || p.StalePages() != 1 {
		t.Fatalf("cap-before-filter or wrong scope: items=%+v more=%t stale=%d", items, more, p.StalePages())
	}
	cursor, err := domain.DecodeLintCursor(items[2].Cursor)
	if err != nil {
		t.Fatal(err)
	}
	after, err := cursor.PositionFor(domain.CursorBinding{NotebookID: f.nb, Mode: domain.ModeLLM, AreaRootID: &root}, domain.Orphans)
	if err != nil {
		t.Fatal(err)
	}
	p, err = f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Orphans), After: &after, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	items, more = lintAnswer(t, p, domain.Orphans, 7, false)
	if len(items) != 3 || !more || *items[0].PageID != wanted[3] || *items[2].PageID != wanted[5] {
		t.Fatalf("second page = %+v more=%t, want ids 20004..20006", items, more)
	}
}

// This independent fixture distinguishes D, F, and reporting eligibility:
// root/AGENTS still report, empty wiki emitters count, stale/empty parents do
// not swallow assets, and a direct asset reference does not digest its page.
func lintSourceFixture() (*lintData, app.ReadInput) {
	root := lintID(10)
	nodes := []app.CompactNode{
		lintNode(10, nil, "raw", domain.NodePage, 0), lintNode(11, nil, "agents", domain.NodePage, 1),
		lintNode(20, nil, "wiki", domain.NodePage, 4), lintNode(21, nil, "empty-wiki", domain.NodePage, 0),
		lintNode(30, &root, "source-a", domain.NodePage, 9), lintNode(31, &root, "empty-source", domain.NodePage, 0),
		lintNode(32, &root, "stale-source", domain.NodePage, 7), lintNode(33, &root, "source-b", domain.NodePage, 8),
		lintNode(34, &root, "source-c", domain.NodePage, 6),
		lintNode(40, lintPtr(lintID(30)), "a.pdf", domain.NodeAsset, 0),
		lintNode(41, lintPtr(lintID(33)), "b.pdf", domain.NodeAsset, 0),
		lintNode(42, &root, "c.pdf", domain.NodeAsset, 0), lintNode(43, &root, "d.pdf", domain.NodeAsset, 0),
	}
	f, in := lintFixture(nodes...)
	in.EffectiveMode, in.AreaRootID = domain.ModeLLM, &root
	for _, n := range nodes[4:] {
		in.SourceIDs = append(in.SourceIDs, n.ID)
	}
	f.heads[1].FrontmatterValid = false
	f.heads[6].Extractor = 1
	f.parents = []uuid.UUID{lintID(30), lintID(32), lintID(33), lintID(34)}
	f.excluded = []uuid.UUID{lintID(30), lintID(31), lintID(32), lintID(33), lintID(34)}
	f.sqlIDs[domain.Orphans] = []uuid.UUID{lintID(20), lintID(21)}
	f.sqlIDs[domain.InvalidFrontmatter] = []uuid.UUID{lintID(11)}
	f.sqlIDs[domain.MissingType] = []uuid.UUID{lintID(20), lintID(21)}
	f.sqlIDs[domain.MissingSummary] = []uuid.UUID{lintID(20), lintID(21)}
	f.candidates[domain.Orphans] = []uuid.UUID{lintID(20), lintID(21)}
	f.candidates[domain.InvalidFrontmatter] = []uuid.UUID{lintID(10), lintID(11), lintID(20), lintID(21)}
	f.candidates[domain.MissingType] = []uuid.UUID{lintID(20), lintID(21)}
	f.candidates[domain.MissingSummary] = []uuid.UUID{lintID(20), lintID(21)}
	f.properties = []app.SourceProperty{
		{SourcePropertyPos: app.SourcePropertyPos{PageID: root}, Key: "sources", JSON: []byte(`"not a link"`)},
		{SourcePropertyPos: app.SourcePropertyPos{PageID: lintID(20)}, Key: "Sources", JSON: []byte(`["[[source-a]]",0,"[[0]]","[[outside]]"]`)},
	}
	f.slots = []app.SourceSlotEdge{
		{EdgePos: app.EdgePos{SourceID: lintID(20), Start: 0}, PropertyKey: lintPtr("Sources.0"), Kind: "wikilink", Target: "source-a", ResolvedID: lintPtr(lintID(30))},
		{EdgePos: app.EdgePos{SourceID: lintID(20), Start: 2}, PropertyKey: lintPtr("Sources.2"), Kind: "wikilink", Target: "0"},
		{EdgePos: app.EdgePos{SourceID: lintID(20), Start: 3}, PropertyKey: lintPtr("Sources.3"), Kind: "wikilink", Target: "outside", ResolvedID: lintPtr(lintID(20))},
	}
	f.edges = []app.LintEdge{lintEdge(10, 5, "root-need", "wikilink"), lintEdge(11, 0, "agent-need", "wikilink"),
		lintEdge(20, 2, "0", "wikilink"),
		{EdgePos: app.EdgePos{SourceID: lintID(20), Start: 9}, Kind: "wikilink", Target: "amb", ResolvedID: lintPtr(lintID(21)), Ambiguous: true},
		lintEdge(30, 20, "source-need", "wikilink"), lintEdge(32, 20, "stale-need", "wikilink")}
	f.bodies = []app.BodyReference{
		{EdgePos: app.EdgePos{SourceID: lintID(11)}, ResolvedID: lintID(33)}, // AGENTS cannot digest B
		{EdgePos: app.EdgePos{SourceID: lintID(20)}, ResolvedID: lintID(43), ResolvedAsset: true},
		{EdgePos: app.EdgePos{SourceID: lintID(21)}, ResolvedID: lintID(34)}, // empty emitter still qualifies
		{EdgePos: app.EdgePos{SourceID: lintID(30)}, ResolvedID: lintID(40), ResolvedAsset: true},
		{EdgePos: app.EdgePos{SourceID: lintID(31)}, ResolvedID: lintID(42), ResolvedAsset: true},
		{EdgePos: app.EdgePos{SourceID: lintID(32)}, ResolvedID: lintID(42), ResolvedAsset: true},
		{EdgePos: app.EdgePos{SourceID: lintID(33)}, ResolvedID: lintID(41), ResolvedAsset: true},
	}
	f.assetBytes[lintID(42)] = 420
	return f, in
}

func TestLintPrepareSources(t *testing.T) {
	t.Run("shared-results-and-attachment-follows", func(t *testing.T) {
		f, in := lintSourceFixture()
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range []struct {
			r domain.Rule
			n int64
		}{{domain.Wanted, 3}, {domain.Orphans, 2}, {domain.Ambiguous, 1}, {domain.InvalidFrontmatter, 1},
			{domain.MissingType, 2}, {domain.MissingSummary, 2}, {domain.UndigestedSources, 2}, {domain.InvalidSources, 4}} {
			items, more := lintAnswer(t, p, row.r, row.n, false)
			if int64(len(items)) != row.n || more {
				t.Fatalf("%s preview = %+v more=%t", row.r, items, more)
			}
		}
		pending, _ := lintAnswer(t, p, domain.UndigestedSources, 2, false)
		if *pending[0].NodeID != lintID(33) || pending[0].Kind != domain.NodePage || pending[0].Bytes != 8 ||
			*pending[1].NodeID != lintID(42) || pending[1].Kind != domain.NodeAsset || pending[1].Bytes != 420 ||
			!pending[1].CreatedAt.Equal(in.Nodes[11].CreatedAt) {
			t.Fatalf("pending = %+v, want only source B and independently pending c.pdf with original metadata", pending)
		}
		bad, _ := lintAnswer(t, p, domain.InvalidSources, 4, false)
		var zero []domain.Item
		for _, item := range bad {
			if item.Target == "0" {
				zero = append(zero, item)
			}
		}
		if len(zero) != 2 || zero[0].NotLink == zero[1].NotLink || zero[0].ItemKey == zero[1].ItemKey {
			t.Fatalf("same-text L/J identities were merged: %+v", bad)
		}
		if p.StalePages() != 1 || !reflect.DeepEqual(f.assetCalls, [][]uuid.UUID{{lintID(42)}}) {
			t.Fatalf("stale=%d asset calls=%v, want one stale page and final asset 42 once", p.StalePages(), f.assetCalls)
		}
		before := len(f.calls)
		lintAnswer(t, p, domain.InvalidSources, 4, false)
		lintAnswer(t, p, domain.UndigestedSources, 2, false)
		if len(f.calls) != before {
			t.Fatal("projection re-read shared sources")
		}
	})
	t.Run("slot-cardinality-crosses-every-cut", func(t *testing.T) {
		f, in := lintSourceFixture()
		f.properties = []app.SourceProperty{{SourcePropertyPos: app.SourcePropertyPos{PageID: lintID(20)}, Key: "sources", JSON: []byte(`"[[source-a]]"`)}}
		f.slots = nil
		target := strings.Repeat("a", 32*1024) + "/x"
		for i := range 600 {
			f.slots = append(f.slots, app.SourceSlotEdge{EdgePos: app.EdgePos{SourceID: lintID(20), Start: int32(i)},
				PropertyKey: lintPtr("sources"), Kind: "wikilink", Target: target, ResolvedID: lintPtr(lintID(30))})
		}
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.InvalidSources)})
		if err != nil {
			t.Fatal(err)
		}
		items, _ := lintAnswer(t, p, domain.InvalidSources, 1, false)
		if len(items) != 1 || !items[0].NotLink || items[0].Target != `"[[source-a]]"` {
			t.Fatalf("colliding slot was accepted after a batch cut: %+v", items)
		}
	})
	t.Run("full-tree-presence-includes-source-assets", func(t *testing.T) {
		f, in := lintSourceFixture()
		f.edges = []app.LintEdge{lintEdge(20, 0, "c.pdf", "wikilink"), lintEdge(20, 1, "needed", "wikilink")}
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted)})
		if err != nil {
			t.Fatal(err)
		}
		items, _ := lintAnswer(t, p, domain.Wanted, 1, false)
		if len(items) != 1 || items[0].Target != "needed" {
			t.Fatalf("presence lost the source attachment: %+v", items)
		}
	})
}

func TestLintPreparePreservesItsBoundary(t *testing.T) {
	t.Run("input-and-output-are-owned-values", func(t *testing.T) {
		f, in := lintSourceFixture()
		rule := domain.UndigestedSources
		f.onCall = func(name string) {
			if name == "heads" {
				in.Nodes[7].PageBytes = 999
				in.SourceIDs[0] = lintID(999)
				*in.AreaRootID = lintID(998)
				rule = domain.Wanted
			}
		}
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: &rule})
		if err != nil {
			t.Fatal(err)
		}
		items, _ := lintAnswer(t, p, domain.UndigestedSources, 2, false)
		if items[0].Bytes != 8 {
			t.Fatalf("caller mutation changed preparation: %+v", items)
		}
		*items[0].NodeID = lintID(999)
		items[0].Bytes, items[0].ItemKey = 777, "changed"
		again, _ := lintAnswer(t, p, domain.UndigestedSources, 2, false)
		if *again[0].NodeID != lintID(33) || again[0].Bytes != 8 || again[0].ItemKey == "changed" {
			t.Fatalf("caller mutation changed cached projection: %+v", again)
		}
		if _, err := p.RuleCount(t.Context(), domain.Wanted); err == nil {
			t.Fatal("unprepared rule unexpectedly existed")
		}
	})
	for _, failed := range []string{"heads", "edges", "page-orphans", "source-values", "slot-values", "bodies", "pending-assets", "asset-bytes"} {
		t.Run("no-partial-result/"+failed, func(t *testing.T) {
			f, in := lintSourceFixture()
			f.fail = failed
			p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{})
			if p != nil || !errors.Is(err, f.boom) {
				t.Fatalf("failed stage %s returned preparation=%v error=%v", failed, p, err)
			}
		})
	}
	t.Run("cancellation-after-a-nonempty-read", func(t *testing.T) {
		f, in := lintSourceFixture()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		f.onCall = func(name string) {
			if name == "edges" {
				cancel()
			}
		}
		p, err := f.rules().Prepare(ctx, in, app.RuleQuery{Rule: lintPtr(domain.Wanted)})
		if p != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled read returned preparation=%v error=%v", p, err)
		}
	})
	for _, negative := range []bool{false, true} {
		t.Run(fmt.Sprintf("asset-missing-or-negative/%t", negative), func(t *testing.T) {
			f, in := lintSourceFixture()
			delete(f.assetBytes, lintID(42))
			if negative {
				f.assetBytes[lintID(42)] = -1
			}
			p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.UndigestedSources)})
			if p != nil || err == nil {
				t.Fatalf("invalid asset metadata returned preparation=%v error=%v", p, err)
			}
		})
	}
}
