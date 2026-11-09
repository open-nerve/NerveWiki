package postgresadapter_test

import (
	"fmt"
	"slices"
	"testing"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

func lintAreaID(prefix string, n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("%s-0000-0000-0000-%012x", prefix, n))
}

// lintAreaLinks writes links links of page from position from on, one in
// every three resolved when resolved, and the notebook's indexed page.
func lintAreaLinks(t *testing.T, f fixture, notebook, page uuid.UUID, from, links int, resolved bool) {
	t.Helper()
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid)
 VALUES ($1,$2,1,2,true) ON CONFLICT DO NOTHING`, page, notebook)
	lintExec(t, f, `INSERT INTO page_links(source_id,range_start,range_end,notebook_id,kind,target,target_key,ambiguous,resolved_id)
 SELECT $1::uuid,k,k+1,$2::uuid,'wikilink','t'||k,'t'||k,false,CASE WHEN $5::boolean AND k%3=0 THEN $1::uuid END
 FROM generate_series($3::integer,$3::integer+$4::integer-1) k`, page, notebook, from, links, resolved)
}

// An upgrade's start's second pass reads an area's wanted heads by chunks of
// 512 pages, each chunk in batches past a (page, position) cursor (M11/P1
// design 3.13): a chunk's first batch starts before every position, one page
// crosses two batches, the stream crosses chunks and passes an empty one.
// Resolved edges and pages not asked for are not read.
func TestTheAreaWantedHeadsGoByBatch(t *testing.T) {
	f := newFixture(t)
	reads := postgresadapter.NewLintReads(f.pool, 5*1024*1024)
	big, small, none := lintAreaID("30000000", 1), lintAreaID("30000000", 2), lintAreaID("30000000", 3)
	lintAreaLinks(t, f, f.eng, big, 0, 900, true)
	lintAreaLinks(t, f, f.eng, small, 5, 3, false)
	lintExec(t, f, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) VALUES ($1,$2,1,2,true)`, none, f.eng)
	lintAreaLinks(t, f, f.eng, lintAreaID("30000000", 4), 0, 50, false) // in the notebook, not asked for
	start := app.EdgePos{Start: -1}
	t.Run("first", func(t *testing.T) {
		got, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{big, small, none}, start, 512)
		if err != nil || len(got) != 512 {
			t.Fatalf("first batch = %d heads, %v; want 512", len(got), err)
		}
		for i, h := range got {
			// Every third of big's links is resolved: the i-th wanted one is
			// at 3(i/2)+1+i%2.
			if h.SourceID != big || h.Start != int32(3*(i/2)+1+i%2) || h.TargetBytes != int64(len(fmt.Sprint("t", h.Start))) {
				t.Fatalf("head %d = %+v", i, h)
			}
		}
	})
	t.Run("one-page-across-two-batches", func(t *testing.T) {
		after := app.EdgePos{SourceID: big, Start: 767}
		got, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{big, small, none}, after, 512)
		if err != nil {
			t.Fatal(err)
		}
		// big's wanted links past 767: 88 of them; then small's three.
		if len(got) != 91 || got[0].EdgePos != (app.EdgePos{SourceID: big, Start: 769}) || got[87].SourceID != big ||
			got[88].EdgePos != (app.EdgePos{SourceID: small, Start: 5}) || got[90].EdgePos != (app.EdgePos{SourceID: small, Start: 7}) {
			t.Fatalf("second batch = %d heads: %+v ... %+v", len(got), got[0], got[len(got)-1])
		}
	})
	t.Run("empty", func(t *testing.T) {
		got, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{none}, start, 512)
		if err != nil || len(got) != 0 {
			t.Fatalf("a page of no links = %+v, %v", got, err)
		}
		got, err = reads.AreaWantedEdgeHeads(t.Context(), f.eng, nil, start, 512)
		if err != nil || len(got) != 0 {
			t.Fatalf("no pages = %+v, %v", got, err)
		}
	})
	t.Run("refused", func(t *testing.T) {
		many := make([]uuid.UUID, 513)
		for i := range many {
			many[i] = lintAreaID("50000000", i+1)
		}
		for name, call := range map[string]func() error{
			"unsorted": func() error {
				_, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{small, big}, start, 512)
				return err
			},
			"duplicate": func() error {
				_, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{big, big}, start, 512)
				return err
			},
			"too-many": func() error { _, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, many, start, 512); return err },
			"take": func() error {
				_, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{big}, start, 513)
				return err
			},
			"cursor": func() error {
				_, err := reads.AreaWantedEdgeHeads(t.Context(), f.eng, []uuid.UUID{big}, app.EdgePos{Start: -2}, 512)
				return err
			},
		} {
			if call() == nil {
				t.Errorf("%s was read", name)
			}
		}
	})
	t.Run("across-chunks", func(t *testing.T) {
		// 1,100 pages: the first chunk's pages have one wanted link each,
		// the second chunk's none, the third's two each.
		g := newFixture(t)
		var pages []uuid.UUID
		for i := range 1100 {
			page := lintAreaID("60000000", i+1)
			pages = append(pages, page)
			switch {
			case i < 512:
				lintAreaLinks(t, g, g.eng, page, 0, 1, false)
			case i < 1024:
				lintExec(t, g, `INSERT INTO indexed_pages(node_id,notebook_id,revision,extractor,frontmatter_valid) VALUES ($1,$2,1,2,true)`, page, g.eng)
			default:
				lintAreaLinks(t, g, g.eng, page, 0, 2, false)
			}
		}
		stream := app.NewLintReads(postgresadapter.NewLintReads(g.pool, 5*1024*1024), 5*1024*1024).WantedOf(g.eng, pages)
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
				if row.ResolvedID != nil || row.TargetKey == nil || row.Target != fmt.Sprint("t", row.Start) {
					t.Fatalf("not a wanted edge: %+v", row)
				}
				got = append(got, row.EdgePos)
			}
		}
		var want []app.EdgePos
		for i, page := range pages {
			switch {
			case i < 512:
				want = append(want, app.EdgePos{SourceID: page})
			case i >= 1024:
				want = append(want, app.EdgePos{SourceID: page}, app.EdgePos{SourceID: page, Start: 1})
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("the stream read %d edges, want %d in order", len(got), len(want))
		}
	})
}
