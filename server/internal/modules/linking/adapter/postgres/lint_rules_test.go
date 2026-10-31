package postgresadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type lintPageCalls struct {
	name  string
	count func(context.Context, app.PageRuleInput) (domain.Count, error)
	items func(context.Context, app.PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
}

func lintPageRules(s *postgresadapter.Store) []lintPageCalls {
	return []lintPageCalls{
		{"orphans", s.LintOrphanCount, s.LintOrphanItems},
		{"invalid_frontmatter", s.LintFrontmatterCount, s.LintFrontmatterItems},
		{"missing_type", s.LintMissingTypeCount, s.LintMissingTypeItems},
		{"missing_summary", s.LintMissingSummaryCount, s.LintMissingSummaryItems},
	}
}

func lintAssertPages(t *testing.T, calls lintPageCalls, in app.PageRuleInput, want []uuid.UUID) {
	t.Helper()
	n, err := calls.count(t.Context(), in)
	if err != nil || n != (domain.Count{Value: int64(len(want))}) {
		t.Fatalf("%s count: %+v, %v; want %d", calls.name, n, err, len(want))
	}
	preview, err := calls.items(t.Context(), in, nil, 6)
	if err != nil || !slices.Equal(preview, want[:min(6, len(want))]) {
		t.Fatalf("%s preview: %v, %v; want %v", calls.name, preview, err, want)
	}
	var got []uuid.UUID
	var after *uuid.UUID
	for {
		ids, err := calls.items(t.Context(), in, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(ids) == 0 {
			break
		}
		got = append(got, ids...)
		last := ids[len(ids)-1]
		after = &last
		if len(got) > len(want)+2 {
			t.Fatalf("%s item cursor made no progress: %v", calls.name, got)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s pages: %v; want %v", calls.name, got, want)
	}
}

func TestLintRules(t *testing.T) {
	t.Run("root-agents-scopes", testLintRootAgentsScopes)
	t.Run("empty-page-versus-folder", testLintEmptyPageVersusFolder)
	t.Run("eight-rules-real-consumer", testLintEightRules)
	t.Run("page-qualification", func(t *testing.T) {
		f := newFixture(t)
		var candidates []uuid.UUID
		for n := uint64(0); n < 10; n++ {
			id := lintID(n)
			candidates = append(candidates, id)
			notebook := f.eng
			if n == 9 {
				notebook = f.ops
			}
			f.replace(t, app.Page{ID: id, NotebookID: notebook, Revision: 1}, domain.Facts{FrontmatterValid: n != 7})
		}
		// Literal v1 target, foreign target and a missing head are not current candidates.
		lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, lintID(8))
		candidates = append(candidates, lintID(10))
		emitters := []struct {
			n      uint64
			book   uuid.UUID
			old    bool
			target uint64
			asset  bool
		}{
			{100, f.eng, false, 6, false}, // ordinary current inbound excludes 6
			{101, f.eng, true, 1, false},  // stale inbound cannot rescue 1
			{102, f.ops, false, 2, false}, // foreign inbound cannot rescue 2
			{103, f.eng, false, 3, false}, // complete source exclusion cannot rescue 3
			{104, f.eng, false, 4, true},  // asset resolution cannot rescue a page
		}
		for _, e := range emitters {
			id := lintID(e.n)
			f.replace(t, app.Page{ID: id, NotebookID: e.book, Revision: 1}, domain.Facts{FrontmatterValid: true, Links: []domain.Link{{Kind: "link", Property: "named", Target: "t", Start: 1, End: 2}}})
			f.resolve(t, id, domain.Resolution{ID: lintID(e.target), Asset: e.asset}, 1)
			if e.old {
				lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, id)
			}
		}
		lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,resolved_id,ambiguous) VALUES($1,0,1,$2,'wikilink','self','self',$1,false)`, lintID(5), f.eng)
		in := app.PageRuleInput{NotebookID: f.eng, CandidateIDs: candidates, ExcludedEmitters: []uuid.UUID{lintID(103)}}
		calls := lintPageRules(f.s)
		lintAssertPages(t, calls[0], in, []uuid.UUID{lintID(0), lintID(1), lintID(2), lintID(3), lintID(4), lintID(5), lintID(7)})
		lintAssertPages(t, calls[1], in, []uuid.UUID{lintID(7)})
		for _, c := range calls[2:] {
			lintAssertPages(t, c, in, candidates[:7])
		}
		// A zero UUID is an actual after position, not an alias for First.
		for _, c := range calls[2:] {
			zero := uuid.UUID{}
			ids, err := c.items(t.Context(), in, &zero, 1)
			if err != nil || !slices.Equal(ids, []uuid.UUID{lintID(1)}) {
				t.Fatalf("%s zero after: %v, %v", c.name, ids, err)
			}
		}
	})
	t.Run("pending-effective-parents", func(t *testing.T) {
		f := newFixture(t)
		var parents, assets []uuid.UUID
		for n := uint64(0); n < 8; n++ {
			p, a := lintID(n), lintID(100+n)
			parents = append(parents, p)
			assets = append(assets, a)
			link := domain.Link{Kind: []string{"wikilink", "link", "embed", "image"}[n%4], Target: "asset.pdf", Start: 0, End: 10}
			if n == 4 {
				link.Property = ""
				link.Quote = '\''
			} // real empty property origin
			if n == 5 {
				link.Property = "named"
			}
			book := f.eng
			if n == 7 {
				book = f.ops
			}
			f.replace(t, app.Page{ID: p, NotebookID: book, Revision: 1}, domain.Facts{FrontmatterValid: true, Links: []domain.Link{link}})
			f.resolve(t, p, domain.Resolution{ID: a, Asset: true}, 0)
			if n == 6 {
				lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id=$1`, p)
			}
		}
		got, err := f.s.LintPendingAssets(t.Context(), app.PendingAssetInput{NotebookID: f.eng, ParentCandidates: parents}, assets)
		if err != nil || !slices.Equal(got, assets[4:]) {
			t.Fatalf("real F body/current/notebook eligibility: %v, %v; want %v", got, err, assets[4:])
		}
		got, err = f.s.LintPendingAssets(t.Context(), app.PendingAssetInput{NotebookID: f.eng}, assets)
		if err != nil || !slices.Equal(got, assets) {
			t.Fatalf("empty P: %v, %v", got, err)
		}
	})
	t.Run("planned-and-input-boundaries", func(t *testing.T) {
		f := newFixture(t)
		tr := &lintTrace{}
		pool := lintPool(t, f, tr, 1)
		s := postgresadapter.New(pool)
		id := uuid.UUID{}
		f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: true})
		in := app.PageRuleInput{NotebookID: f.eng, CandidateIDs: []uuid.UUID{id}}
		for _, c := range lintPageRules(s) {
			_, err := c.count(t.Context(), in)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.items(t.Context(), in, nil, 6)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.items(t.Context(), in, &id, 101)
			if err != nil {
				t.Fatal(err)
			}
			for _, take := range []int{0, 102} {
				if _, err := c.items(t.Context(), in, nil, take); err == nil {
					t.Fatalf("%s accepted take %d", c.name, take)
				}
			}
			bad := in
			bad.CandidateIDs = []uuid.UUID{id, id}
			if _, err := c.count(t.Context(), bad); err == nil {
				t.Fatalf("%s accepted duplicate relation", c.name)
			}
		}
		_, err := s.LintPendingAssets(t.Context(), app.PendingAssetInput{NotebookID: f.eng, ParentCandidates: []uuid.UUID{id}}, []uuid.UUID{lintID(1)})
		if err != nil {
			t.Fatal(err)
		}
		if got := tr.names(); len(got) != 13 {
			t.Fatalf("static count/first/after/F statement count: %v", got)
		}
		tr.mu.Lock()
		defer tr.mu.Unlock()
		if len(tr.modes) != 13 {
			t.Fatalf("missing Planned modes: %v", tr.modes)
		}
		for _, mode := range tr.modes {
			if mode != pgx.QueryExecModeDescribeExec {
				t.Fatalf("unexpected Planned mode: %v", mode)
			}
		}
	})
}

func TestLintMissingProperties(t *testing.T) {
	// Handwritten code points: Go unicode.IsSpace and PostgreSQL btrim defaults
	// would both misclassify at least one of FEFF and 0085.
	trim := []rune{'\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff}
	for _, key := range []string{"type", "summary"} {
		t.Run(key, func(t *testing.T) {
			f := newFixture(t)
			var ids, want []uuid.UUID
			add := func(props []domain.Property, valid, missing bool) {
				id := lintID(uint64(len(ids)))
				ids = append(ids, id)
				f.replace(t, app.Page{ID: id, NotebookID: f.eng, Revision: 1}, domain.Facts{FrontmatterValid: valid, Properties: props})
				if missing {
					want = append(want, id)
				}
			}
			val := func(s string) []byte {
				b, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				return b
			}
			add(nil, true, true)
			for _, raw := range []string{`null`, `1`, `false`, `[]`, `{}`} {
				add([]domain.Property{{Key: key, Value: []byte(raw)}, {Key: strings.ToUpper(key), Value: val("later")}}, true, true)
			}
			add([]domain.Property{{Key: key, Value: val("")}}, true, true)
			for _, r := range trim {
				add([]domain.Property{{Key: strings.ToUpper(key), Value: val(string(r))}}, true, true)
			}
			for _, r := range []rune{0x0085, 0x200b, 0x180e, 'x'} {
				add([]domain.Property{{Key: key, Value: val(string(r))}}, true, false)
			}
			add([]domain.Property{{Key: key, Value: val(strings.Repeat(" ", 257))}}, true, true)
			add([]domain.Property{{Key: key, Value: val(strings.Repeat(" ", 258))}}, true, true)
			add([]domain.Property{{Key: key, Value: val(strings.Repeat(" ", 257) + "x")}}, true, false)
			add([]domain.Property{{Key: key, Value: val("x" + strings.Repeat(" ", 4096))}}, true, false)
			add([]domain.Property{{Key: key, Value: val(" ")}}, false, false)
			// Only ASCII folding. A dotted key and Unicode lookalike precede the real key.
			add([]domain.Property{{Key: key + ".child", Value: val("x")}, {Key: "ſummary", Value: val("x")}, {Key: strings.ToUpper(key), Value: val(" ")}, {Key: key, Value: val("later")}}, true, true)
			c := lintPageRules(f.s)[2]
			if key == "summary" {
				c = lintPageRules(f.s)[3]
			}
			lintAssertPages(t, c, app.PageRuleInput{NotebookID: f.eng, CandidateIDs: ids}, want)
		})
	}
}

func TestLintCountsAndPages(t *testing.T) {
	t.Run("eligibility-before-cap", testLintEligibilityBeforeCap)
	t.Run("long-prefix-distinct-groups", testLintLongIdentities)
	t.Run("consumer-items-after-cap", testLintConsumerCountBeyondCap)
	for rule := 0; rule < 4; rule++ {
		t.Run(fmt.Sprint(rule), func(t *testing.T) {
			f := newFixture(t)
			var ids []uuid.UUID
			for n := uint64(0); n < 10007; n++ {
				ids = append(ids, lintID(n))
			}
			lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) SELECT unnest($1::uuid[]),$2,1,2,$3`, ids, f.eng, rule != 1)
			c := lintPageRules(f.s)[rule]
			in := app.PageRuleInput{NotebookID: f.eng, CandidateIDs: ids}
			for _, n := range []int{9999, 10000, 10007} {
				in.CandidateIDs = ids[:n]
				got, err := c.count(t.Context(), in)
				if err != nil || got != (domain.Count{Value: int64(min(n, 10000)), Capped: n >= 10000}) {
					t.Fatalf("%s cap n=%d: %+v, %v", c.name, n, got, err)
				}
			}
			after := ids[9998]
			got, err := c.items(t.Context(), in, &after, 101)
			if err != nil || !slices.Equal(got, ids[9999:]) {
				t.Fatalf("%s items must continue after count cap: %v, %v", c.name, got, err)
			}
			// Ten thousand rejected low IDs precede valid IDs. Qualification must run
			// before the count cap or this would silently count an empty prefix.
			lintExec(t, f, `UPDATE indexed_pages SET extractor=1 WHERE node_id<$1`, ids[10000])
			lintAssertPages(t, c, in, ids[10000:])
		})
	}
}
