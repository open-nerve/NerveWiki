package app_test

import (
	"maps"
	"slices"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// lintByPageFixture is an explicit LLM contract's notebook: the area raw
// (10) holds a and b (100, 101) and c (102), whose index is stale; outside
// it p, q, r and s (20–23). p links Alpha twice and Beta; q links Alpha,
// Gamma and Delta; r links nothing; s links only an image read as an
// attachment and the virtual index; a links Alpha and Epsilon, b Zeta.
func lintByPageFixture() (*lintData, app.ReadInput) {
	root := lintID(10)
	f, in := lintFixture(lintNode(10, nil, "raw", domain.NodePage, 0), lintNode(11, nil, "agents", domain.NodePage, 1),
		lintNode(20, nil, "p", domain.NodePage, 1), lintNode(21, nil, "q", domain.NodePage, 1),
		lintNode(22, nil, "r", domain.NodePage, 1), lintNode(23, nil, "s", domain.NodePage, 1),
		lintNode(100, &root, "a", domain.NodePage, 1), lintNode(101, &root, "b", domain.NodePage, 1),
		lintNode(102, &root, "c", domain.NodePage, 1))
	for i := range f.heads {
		if f.heads[i].ID == lintID(102) {
			f.heads[i].Extractor = 1
		}
	}
	resolved := lintEdge(20, 3, "Here", "wikilink")
	resolved.ResolvedID = lintPtr(lintID(21))
	f.edges = []app.LintEdge{
		lintEdge(20, 0, "Alpha", "wikilink"), lintEdge(20, 1, "alpha", "link"), lintEdge(20, 2, "Beta", "wikilink"), resolved,
		lintEdge(21, 0, "Alpha", "wikilink"), lintEdge(21, 1, "Gamma", "wikilink"), lintEdge(21, 2, "Delta", "link"),
		lintEdge(23, 0, "chart.png", "image"), lintEdge(23, 1, "index", "wikilink"),
		lintEdge(100, 0, "Alpha", "wikilink"), lintEdge(100, 1, "Epsilon", "wikilink"), lintEdge(101, 0, "Zeta", "wikilink"),
		lintEdge(102, 0, "Eta", "wikilink"),
	}
	in.EffectiveMode, in.AreaRootID, in.Explicit = domain.ModeLLM, &root, true
	in.SourceIDs = []uuid.UUID{lintID(100), lintID(101), lintID(102)}
	return f, in
}

// An upgrade's start records each page's count of distinct wanted targets
// (M11/P1 design 3.13): a target twice counts once, a target read as an
// attachment or a virtual file not at all, a page without one is absent;
// the area's pages are counted in a second pass only, never in the rule's
// count, which a preparation that does not ask for the counts never reads.
func TestTheWantedCountsByPage(t *testing.T) {
	t.Run("an-llm-contract", func(t *testing.T) {
		f, in := lintByPageFixture()
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted), Limit: 1, WantedByPage: true})
		if err != nil {
			t.Fatal(err)
		}
		want := map[uuid.UUID]int{lintID(20): 2, lintID(21): 3, lintID(100): 2, lintID(101): 1}
		if got := p.WantedByPage(); !maps.Equal(got, want) {
			t.Fatalf("wanted by page = %v, want %v", got, want)
		}
		items, more := lintAnswer(t, p, domain.Wanted, 4, false)
		if len(items) != 1 || !more || items[0].Target != "Alpha" || items[0].Pages != 2 {
			t.Fatalf("wanted rule = %+v more=%t: the area's pages count in it", items, more)
		}
		if len(f.areaCalls) != 1 || !slices.Equal(f.areaCalls[0].pages, []uuid.UUID{lintID(100), lintID(101)}) {
			t.Fatalf("the second pass read %+v, want the area's current pages", f.areaCalls)
		}
		got := p.WantedByPage()
		got[lintID(20)] = 99
		if p.WantedByPage()[lintID(20)] != 2 {
			t.Fatal("the preparation shares its counts")
		}
	})
	t.Run("not-asked-for", func(t *testing.T) {
		f, in := lintByPageFixture()
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted), Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if got := p.WantedByPage(); got != nil {
			t.Fatalf("counts nobody asked for: %v", got)
		}
		if slices.Contains(f.calls, "area-heads") {
			t.Fatalf("the second pass ran unasked: %v", f.calls)
		}
		lintAnswer(t, p, domain.Wanted, 4, false)
	})
	t.Run("a-standard-notebook", func(t *testing.T) {
		f, in := lintByPageFixture()
		in.EffectiveMode, in.AreaRootID, in.Explicit, in.SourceIDs = domain.ModeStandard, nil, false, nil
		p, err := f.rules().Prepare(t.Context(), in, app.RuleQuery{Rule: lintPtr(domain.Wanted), Limit: 1, WantedByPage: true})
		if err != nil {
			t.Fatal(err)
		}
		// No area: one pass counts every current page, the virtual index
		// a wanted page of a standard notebook.
		want := map[uuid.UUID]int{lintID(20): 2, lintID(21): 3, lintID(23): 1, lintID(100): 2, lintID(101): 1}
		if got := p.WantedByPage(); !maps.Equal(got, want) {
			t.Fatalf("wanted by page = %v, want %v", got, want)
		}
		if slices.Contains(f.calls, "area-heads") {
			t.Fatalf("a standard notebook read a second pass: %v", f.calls)
		}
	})
	t.Run("only-with-the-wanted-rule", func(t *testing.T) {
		f, in := lintByPageFixture()
		for _, q := range []app.RuleQuery{{WantedByPage: true}, {Rule: lintPtr(domain.Orphans), WantedByPage: true}} {
			if _, err := f.rules().Prepare(t.Context(), in, q); err == nil {
				t.Errorf("%+v was prepared", q)
			}
		}
	})
}

// The second pass reads its pages by chunks of 512, each in batches whose
// cursor advances and which pass only the chunk's pages from the cursor's
// page on (M11/P1 design 3.13; decision 235); unsorted pages are an error.
func TestTheAreaPassReadsByChunk(t *testing.T) {
	var nodes []app.CompactNode
	var pages []uuid.UUID
	for i := range 1500 {
		n := uint64(1000 + i)
		nodes = append(nodes, lintNode(n, nil, "page", domain.NodePage, 1))
		pages = append(pages, lintID(n))
	}
	f, _ := lintFixture(nodes...)
	for i := range pages {
		// Two links a page, a third on every hundredth.
		f.edges = append(f.edges, lintEdge(uint64(1000+i), 0, "x", "wikilink"), lintEdge(uint64(1000+i), 1, "y", "wikilink"))
		if i%100 == 0 {
			f.edges = append(f.edges, lintEdge(uint64(1000+i), 2, "z", "wikilink"))
		}
	}
	stream := app.NewLintReads(f, 5<<20).WantedOf(f.nb, pages)
	var got []app.EdgePos
	for {
		rows, err := stream.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			got = append(got, row.EdgePos)
		}
	}
	var want []app.EdgePos
	for _, e := range f.edges {
		want = append(want, e.EdgePos)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("read %d edges, want all %d in order", len(got), len(want))
	}
	chunks := [][]uuid.UUID{pages[:512], pages[512:1024], pages[1024:]}
	chunk := -1
	var cursor app.EdgePos
	for i, call := range f.areaCalls {
		if call.after == (app.EdgePos{Start: -1}) {
			chunk++
			if chunk >= len(chunks) || !slices.Equal(call.pages, chunks[chunk]) {
				t.Fatalf("call %d starts a chunk with %d pages, not chunk %d", i, len(call.pages), chunk)
			}
		} else {
			if chunk < 0 || call.after.Compare(cursor) <= 0 || call.pages[0] != call.after.SourceID ||
				!slices.Equal(call.pages, chunks[chunk][len(chunks[chunk])-len(call.pages):]) {
				t.Fatalf("call %d: cursor %+v after %+v, pages from %v", i, call.after, cursor, call.pages[0])
			}
		}
		cursor = call.after
	}
	if chunk != 2 {
		t.Fatalf("read %d chunks, want 3", chunk+1)
	}
	unsorted := slices.Clone(pages[:3])
	unsorted[0], unsorted[1] = unsorted[1], unsorted[0]
	before := len(f.areaCalls)
	if _, err := app.NewLintReads(f, 5<<20).WantedOf(f.nb, unsorted).Next(t.Context()); err == nil {
		t.Fatal("unsorted pages were read")
	}
	if _, err := app.NewLintReads(f, 5<<20).WantedOf(f.nb, []uuid.UUID{pages[0], pages[0]}).Next(t.Context()); err == nil {
		t.Fatal("a page twice was read")
	}
	if len(f.areaCalls) != before {
		t.Fatal("unsorted pages reached the store")
	}
}
