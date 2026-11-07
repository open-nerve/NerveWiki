package postgresadapter

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
)

// The same small relation has old first/middle/last gaps and incoming new
// first/middle/last ranges. Expectations are literal, independent of Keys,
// linksParams, the matcher, and the production equality function.
func TestPreparedSourceAllocationGapsAndDense(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	for _, tc := range []struct {
		name   string
		order  []int
		dense  bool
		params gen.InsertLinksParams
	}{
		{"sorted_sparse", []int{0, 1, 2, 3, 4, 5}, false, gen.InsertLinksParams{
			RangeStarts: []int32{0, 20, 40, 70, 100}, RangeEnds: []int32{9, 26, 44, 78, 104},
			Kinds: []string{"image", "link", "embed", "link", "wikilink"}, PropertyKeys: []string{"", "", "", "", ""},
			InFrontmatter: []bool{false, false, false, false, false}, Aliases: []bool{false, false, false, true, false},
			Targets: []string{"First.png", "Middle", "N.md", "Changed", "Last"}, Anchors: []string{"", "", "here", "", ""},
			Displays: []string{"front", "", "", "", ""}, TargetKeys: []string{"first.png", "middle", "n", "changed", "last"},
			TargetAltKeys: []string{"", "", "n.md", "", ""},
		}},
		{"unordered_sparse", []int{5, 2, 0, 4, 3, 1}, false, gen.InsertLinksParams{
			RangeStarts: []int32{100, 0, 70, 40, 20}, RangeEnds: []int32{104, 9, 78, 44, 26},
			Kinds: []string{"wikilink", "image", "link", "embed", "link"}, PropertyKeys: []string{"", "", "", "", ""},
			InFrontmatter: []bool{false, false, false, false, false}, Aliases: []bool{false, false, true, false, false},
			Targets: []string{"Last", "First.png", "Changed", "N.md", "Middle"}, Anchors: []string{"", "", "", "here", ""},
			Displays: []string{"", "front", "", "", ""}, TargetKeys: []string{"last", "first.png", "changed", "n", "middle"},
			TargetAltKeys: []string{"", "", "", "n.md", ""},
		}},
		{"dense", []int{0, 1, 2, 3, 4, 5}, true, gen.InsertLinksParams{
			RangeStarts: []int32{0, 20, 30, 40, 70, 100}, RangeEnds: []int32{9, 26, 35, 44, 78, 104},
			Kinds: []string{"image", "link", "link", "embed", "link", "wikilink"}, PropertyKeys: []string{"", "", "", "", "", ""},
			InFrontmatter: []bool{false, false, true, false, false, false}, Aliases: []bool{false, false, false, false, true, false},
			Targets: []string{"First.png", "Middle", "Note", "N.md", "Changed", "Last"}, Anchors: []string{"", "", "", "here", "", ""},
			Displays: []string{"front", "", "d", "", "", ""}, TargetKeys: []string{"first.png", "middle", "note", "n", "changed", "last"},
			TargetAltKeys: []string{"", "", "", "n.md", "", ""},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(t.Context(), "TRUNCATE page_links,indexed_pages"); err != nil {
				t.Fatal(err)
			}
			s := New(pool)
			p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
			a := uuid.UUID{15: 11}
			old := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
				{Kind: "link", Target: "Note", Start: 10, End: 15},
				{Kind: "link", Target: "Note", Start: 30, End: 35, Quote: '"'},
				{Kind: "link", Target: "Note", Start: 50, End: 55},
				{Kind: "link", Target: "Note", Start: 70, End: 75},
				{Kind: "link", Target: "Note", Start: 90, End: 95},
			}}
			if err := s.AddPage(t.Context(), p, old); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$2,ambiguous=true,resolved_asset=true WHERE source_id=$1", p.ID, a); err != nil {
				t.Fatal(err)
			}
			links := []domain.Link{
				{Kind: "image", Target: "First.png", Display: "front", Start: 0, End: 9},
				{Kind: "link", Target: "Middle", Start: 20, End: 26},
				{Kind: "link", Target: "Note", Start: 30, End: 35, Quote: '"'},
				{Kind: "embed", Target: "N.md", Anchor: "here", Start: 40, End: 44},
				{Kind: "link", Target: "Changed", Aliases: true, Start: 70, End: 78},
				{Kind: "wikilink", Target: "Last", Start: 100, End: 104},
			}
			if tc.dense {
				links[2].Display = "d"
			}
			facts := domain.Facts{FrontmatterValid: true}
			for _, i := range tc.order {
				facts.Links = append(facts.Links, links[i])
			}
			original := slices.Clone(facts.Links)
			want := []app.Link{
				{SourceID: p.ID, Start: 0, Target: "First.png"},
				{SourceID: p.ID, Start: 20, Target: "Middle"},
				{SourceID: p.ID, Start: 30, Target: "Note", Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
				{SourceID: p.ID, Start: 40, Target: "N.md"},
				{SourceID: p.ID, Start: 70, Target: "Changed", Aliases: true},
				{SourceID: p.ID, Start: 100, Target: "Last"},
			}
			if tc.dense {
				want[2].Resolution = domain.Resolution{}
			}
			wantParams := tc.params
			wantParams.SourceID, wantParams.NotebookID = p.ID, p.NotebookID
			before := retentionCollectorImage(t, pool)
			rollback := errors.New("allocation literal rollback")
			err := postgres.NewTxManager(pool, time.Second).WithinTx(t.Context(), func(ctx context.Context) error {
				if err := s.Lock(ctx, p.NotebookID); err != nil {
					return err
				}
				tx, _ := postgres.TxFrom(ctx)
				w := &preparedWatch{rows: &linksCollectorRows{stable: true, columns: 15}, pid: tx.Conn().PgConn().PID()}
				watched := preparedContext{Context: ctx, watch: w}
				unit := &app.SourcePreparationUnit{NotebookID: p.NotebookID}
				p.Revision = 2
				plan := s.NewSource(watched, unit, p, facts)
				if plan == nil {
					t.Fatal("eligible literal source did not prepare")
				}
				assets, err := plan.ReadBefore(watched, unit)
				if err != nil {
					return err
				}
				read := plan.(*preparedSource).read
				if !read.canRetain || !slices.Equal(assets, []uuid.UUID{a, a, a, a}) ||
					!reflect.DeepEqual(read.links, wantParams) || !slices.Equal(read.missing, []int32{10, 50, 90}) || !slices.Equal(read.inputs, want) {
					t.Fatalf("PREPARED_GAPS: eligible=%t F=%v params=%+v missing=%v inputs=%+v", read.canRetain, assets, read.links, read.missing, read.inputs)
				}
				values := reflect.ValueOf(read.links)
				for i := 0; i < values.NumField(); i++ {
					v := values.Field(i)
					if v.Kind() == reflect.Slice && (v.IsNil() || v.Cap() != len(wantParams.RangeStarts)) {
						t.Fatalf("PREPARED_EXACT_CAP: %s cap=%d nil=%t", values.Type().Field(i).Name, v.Cap(), v.IsNil())
					}
				}
				w.rows.check(t, 5, 1, 5, 1)
				dropped, supplied, err := plan.Apply(watched, unit)
				if err != nil {
					return err
				}
				if supplied == nil || supplied.SourceID != p.ID || !slices.Equal(supplied.Links, want) || !slices.Equal(dropped.Targets, []uuid.UUID{a}) {
					t.Fatal("PREPARED_GAPS_APPLY: full fresh inputs or old D changed")
				}
				got, err := s.Links(ctx, p.NotebookID, domain.Reach{Sources: []uuid.UUID{p.ID}})
				if err != nil || !slices.Equal(got, want) {
					t.Fatalf("PREPARED_GAPS_SQL: persisted input mismatch: %v", err)
				}
				stages := retentionCollectorStages()
				stages[0] = "InspectPageLinksForPreparation"
				if w.early || !slices.Equal(w.calls, stages) {
					t.Fatalf("prepared stages/EOF/PID: %v early=%t", w.calls, w.early)
				}
				return rollback
			})
			if !errors.Is(err, rollback) || !slices.Equal(retentionCollectorImage(t, pool), before) || !slices.Equal(facts.Links, original) {
				t.Fatalf("prepared rollback or caller ownership changed: %v", err)
			}
		})
	}
}

// This is the private collector's compatibility boundary, not a claim that
// the production ORDER BY can emit descending rows. PostgreSQL produces all
// fifteen real values in a deliberate order; Next/Scan are only forwarded.
func TestPreparedSourceCollectorBackwardsRows(t *testing.T) {
	pool, _ := newLinksCollectorPool(t)
	p := app.Page{ID: uuid.UUID{15: 50}, NotebookID: uuid.UUID{15: 1}, Revision: 1}
	a := uuid.UUID{15: 11}
	facts := domain.Facts{FrontmatterValid: true, Links: []domain.Link{
		{Kind: "link", Property: "p", Target: "Note", Start: 0, End: 5},
		{Kind: "link", Property: "p", Target: "Note", Start: 10, End: 15},
		{Kind: "link", Property: "p", Target: "Note", Start: 20, End: 25},
	}}
	if err := New(pool).AddPage(t.Context(), p, facts); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), "UPDATE page_links SET resolved_id=$2,ambiguous=true,resolved_asset=true WHERE source_id=$1", p.ID, a); err != nil {
		t.Fatal(err)
	}
	for _, body := range []bool{false, true} {
		name := "frontmatter"
		if body {
			name = "body_error"
		}
		t.Run(name, func(t *testing.T) {
			if body {
				if _, err := pool.Exec(t.Context(), "UPDATE page_links SET property_key=NULL WHERE source_id=$1", p.ID); err != nil {
					t.Fatal(err)
				}
				for i := range facts.Links {
					facts.Links[i].Property = ""
				}
			}
			before := retentionCollectorImage(t, pool)
			native, err := pool.Query(t.Context(), `SELECT true, range_start, range_end, notebook_id, kind,
    property_key, target, anchor, display, target_key, target_alt_key,
    aliases, resolved_id, ambiguous, resolved_asset
FROM page_links WHERE source_id=$1
ORDER BY CASE range_start WHEN 20 THEN 0 WHEN 0 THEN 1 ELSE 2 END`, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			rows := &linksCollectorRows{Rows: native, stable: true, columns: 15}
			incoming := newPreparedLinkFacts(p, facts.Links)
			read, err := collectPreparedSource(rows, &incoming)
			rows.check(t, 3, 1, 3, 1)
			if body {
				if err == nil || err.Error() != "lint reads: body references do not advance" || !reflect.DeepEqual(read, preparedLinkRead{}) {
					t.Fatalf("PREPARED_BACKWARDS_BODY: result=%+v cause=%v", read, err)
				}
			} else {
				want := []app.Link{
					{SourceID: p.ID, Start: 0, Target: "Note", Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
					{SourceID: p.ID, Start: 10, Target: "Note", Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
					{SourceID: p.ID, Start: 20, Target: "Note", Resolution: domain.Resolution{ID: a, Ambiguous: true, Asset: true}},
				}
				if err != nil || !read.canRetain || read.assets == nil || len(read.assets) != 0 || len(read.missing) != 0 ||
					!slices.Equal(read.targets, []uuid.UUID{a}) || !slices.Equal(read.inputs, want) || len(read.links.RangeStarts) != 0 {
					t.Fatalf("PREPARED_BACKWARDS_VALUES: read=%+v cause=%v", read, err)
				}
				values := reflect.ValueOf(read.links)
				for i := 0; i < values.NumField(); i++ {
					v := values.Field(i)
					if v.Kind() == reflect.Slice && (v.IsNil() || v.Len() != 0 || v.Cap() != 0) {
						t.Fatalf("backwards retained array %s is not nonnil exact empty", values.Type().Field(i).Name)
					}
				}
			}
			if !slices.Equal(retentionCollectorImage(t, pool), before) {
				t.Fatal("collector changed persisted image")
			}
		})
	}
}
