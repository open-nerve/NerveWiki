package domain_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func testVirtualIngestLog(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		for _, tt := range []struct {
			count int
			text  string
		}{{0, "0"}, {25, "25"}, {9999, "9999"}, {10000, "10000+"}} {
			t.Run(tt.text, func(t *testing.T) {
				body, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{IngestCount: tt.count}}, Total: 1}, 40<<10)
				want := "# Log\n\n- 0001-01-01T00:00:00Z 00000000-0000-0000-0000-000000000000 — ; client: ; by: ; nodes: 0; contents: 0; ingests: " + tt.text +
					"\n\nShowing 1 of the most recent 1 changesets. Use log to filter and page through more.\n"
				if err != nil || string(body) != want {
					t.Fatalf("count display: %q %v, want %q", body, err, want)
				}
			})
		}
	})
	t.Run("whole_entry", func(t *testing.T) {
		item := domain.RenderLogChangeset{Kind: "edit", Client: "web", ByName: "Reader", NodeCount: 1, ContentCount: 1, IngestCount: 25,
			Changes:       []domain.RenderLogChange{{Kind: "content", Path: filePath("Page.md")}},
			IngestSources: []domain.FilePath{{Label: "Missing source"}, filePath("raw/Z.md"), {Label: "Missing source"}, filePath("raw/A.png"), filePath("raw/M.md")}}
		// Independent literal bytes include all five slots, the ordinary change,
		// both mandatory sections, and the footer's actual written count.
		header := "# Log\n\n"
		entry := "- 0001-01-01T00:00:00Z 00000000-0000-0000-0000-000000000000 — edit; client: web; by: Reader; nodes: 1; contents: 1; ingests: 25\n" +
			"  - content page: [Page.md](<Page.md>)\n" +
			"  - source: Missing source\n  - source: [raw/Z.md](<raw/Z.md>)\n  - source: Missing source\n" +
			"  - source: [raw/A.png](<raw/A.png>)\n  - source: [raw/M.md](<raw/M.md>)\n"
		footer := "\nShowing 1 of the most recent 2 changesets. Use log to filter and page through more.\n"
		in := domain.RenderLogInput{Items: []domain.RenderLogChangeset{item, item}, Total: 2}
		body, err := domain.RenderLog(in, len(header)+len(entry)+len(footer))
		if err != nil || string(body) != header+entry+footer {
			t.Fatalf("exact complete entry: %q, %v", body, err)
		}
		body, err = domain.RenderLog(in, len(header)+len(entry)+len(footer)-1)
		zero := "# Log\n\n\nShowing 0 of the most recent 2 changesets. Use log to filter and page through more.\n"
		if err != nil || string(body) != zero {
			t.Fatalf("one byte short retained a partial entry: %q, %v", body, err)
		}
	})
	t.Run("encoded_budget", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			items, budget int
		}{{"read", 50, 40 << 10}, {"export", 1000, 1 << 20}} {
			t.Run(tt.name, func(t *testing.T) {
				items := make([]domain.RenderLogChangeset, tt.items)
				var sources []domain.FilePath
				var destinations []string
				for i := range 5 {
					path := strings.Repeat(strings.Repeat("%", 200)+"/", 6) + fmt.Sprintf("S%d.md", i)
					sources = append(sources, filePath(path))
					destinations = append(destinations, strings.Repeat(strings.Repeat("%25", 200)+"/", 6)+fmt.Sprintf("S%d.md", i))
				}
				for i := range items {
					items[i] = domain.RenderLogChangeset{Kind: "edit", IngestCount: 25, IngestSources: sources,
						Changes: []domain.RenderLogChange{{Kind: "content", Path: filePath("Page.md")}}}
				}
				body, err := domain.RenderLog(domain.RenderLogInput{Items: items, Total: tt.items}, tt.budget)
				if err != nil || len(body) > tt.budget || !utf8.Valid(body) {
					t.Fatalf("encoded source budget: bytes=%d, %v", len(body), err)
				}
				got := string(body)
				n := strings.Count(got, "\n- ")
				if n < 1 || n >= tt.items || strings.Count(got, "  - source: ") != 5*n || strings.Count(got, "  - content page: ") != n ||
					!strings.HasSuffix(got, fmt.Sprintf("\nShowing %d of the most recent %d changesets. Use log to filter and page through more.\n", n, tt.items)) {
					t.Fatalf("partial sources/change or dishonest footer: entries=%d", n)
				}
				for _, destination := range destinations {
					if strings.Count(got, "](<"+destination+">)") != n {
						t.Fatal("source destination was truncated, repaired, or incompletely written")
					}
				}
			})
		}
	})
	t.Run("invalid_source", func(t *testing.T) {
		for _, path := range []string{"secret\x00name", "secret\nname", "secret\xffname"} {
			body, err := domain.RenderLog(domain.RenderLogInput{Items: []domain.RenderLogChangeset{{IngestCount: 1,
				IngestSources: []domain.FilePath{filePath(path)}}}, Total: 1}, 40<<10)
			if body != nil || err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("invalid source identity was repaired, hidden, or exposed: %q %v", body, err)
			}
		}
	})
}
