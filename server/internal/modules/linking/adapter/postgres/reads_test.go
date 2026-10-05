package postgresadapter_test

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

var _ app.Reads = (*postgresadapter.Store)(nil)

// resolve has the links of source at starts resolve to target.
func (f fixture) resolve(t *testing.T, source uuid.UUID, target domain.Resolution, starts ...int) {
	t.Helper()
	links := make([]app.Link, len(starts))
	for i, s := range starts {
		links[i] = app.Link{SourceID: source, Start: s, Resolution: target}
	}
	if err := f.tx.WithinTx(context.Background(), func(ctx context.Context) error { return f.s.SetResolutions(ctx, links) }); err != nil {
		t.Fatal(err)
	}
}

// wikilinks is a page's facts with a wikilink at each of starts, its
// target a byte long.
func wikilinks(starts ...int) domain.Facts {
	f := domain.Facts{FrontmatterValid: true}
	for _, s := range starts {
		f.Links = append(f.Links, domain.Link{Kind: "wikilink", Target: "x", Start: s, End: s + 1})
	}
	return f
}

// ids are n new page ids, in order.
func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.NewV7()
	}
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// A page's backlinks are the pages with a link that resolves to it, an
// ambiguous one too, but for itself, by id, at most a page's size of them
// after the id given; each with its revision and extractor, none for a
// page without rows, how many of its links lead there, up to a count, and
// the ranges of the first of them, by start (M6/P5 design 3, review r1-1,
// r2-M1, r3).
func TestThePagesThatLinkToAPage(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := ids(6)
	many, ambiguous, target, one, other, unindexed := p[0], p[1], p[2], p[3], p[4], p[5]
	starts := []int{110, 0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	f.replace(t, app.Page{ID: many, NotebookID: f.eng, Revision: 4}, wikilinks(append(starts, 200)...))
	f.resolve(t, many, domain.Resolution{ID: target}, starts...)
	f.resolve(t, many, domain.Resolution{ID: other}, 200)
	f.replace(t, app.Page{ID: ambiguous, NotebookID: f.eng, Revision: 1}, wikilinks(5))
	f.resolve(t, ambiguous, domain.Resolution{ID: target, Ambiguous: true}, 5)
	f.replace(t, app.Page{ID: one, NotebookID: f.eng, Revision: 2}, wikilinks(7, 9))
	f.resolve(t, one, domain.Resolution{ID: target}, 9)
	f.replace(t, app.Page{ID: target, NotebookID: f.eng, Revision: 1}, wikilinks(3, 4))
	f.resolve(t, target, domain.Resolution{ID: target}, 3, 4)
	f.replace(t, app.Page{ID: other, NotebookID: f.eng, Revision: 1}, wikilinks(3))
	// A link whose page has no row of its own: the tables have no foreign key.
	if _, err := f.pool.Exec(ctx, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, resolved_id,
		ambiguous) VALUES ($1, 8, 9, $2, 'wikilink', 'x', $3, false)`, unindexed, f.eng, target); err != nil {
		t.Fatal(err)
	}

	var first []domain.Range
	for s := 0; s < 100; s += 10 {
		first = append(first, domain.Range{Start: s, End: s + 1})
	}
	e := domain.Extractor
	got, err := f.s.Backlinks(ctx, target, uuid.UUID{}, 2, 100, 10)
	want := []app.Backlink{
		{SourceID: many, Revision: 4, Extractor: e, Links: 12, Ranges: first},
		{SourceID: ambiguous, Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 5, End: 6}}},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("the first two: %+v, %v\nwant %+v", got, err, want)
	}
	got, err = f.s.Backlinks(ctx, target, ambiguous, 2, 100, 10)
	want = []app.Backlink{
		{SourceID: one, Revision: 2, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 9, End: 10}}},
		{SourceID: unindexed, Links: 1, Ranges: []domain.Range{{Start: 8, End: 9}}},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("after the first two, past the page itself: %+v, %v\nwant %+v", got, err, want)
	}
	// A step past the page itself, which links to itself, between the others.
	got, err = f.s.Backlinks(ctx, target, uuid.UUID{}, 3, 100, 1)
	if ids := sourcesOf(got); err != nil || !slices.Equal(ids, []uuid.UUID{many, ambiguous, one}) {
		t.Errorf("three from the first: %v, %v", ids, err)
	}
	if got, err := f.s.Backlinks(ctx, target, unindexed, 2, 100, 10); err != nil || len(got) != 0 {
		t.Errorf("after the last: %+v, %v", got, err)
	}
	got, err = f.s.Backlinks(ctx, target, uuid.UUID{}, 1, 5, 3)
	if want := []app.Backlink{{SourceID: many, Revision: 4, Extractor: e, Links: 5, Ranges: first[:3]}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("counted up to 5: %+v, %v\nwant %+v", got, err, want)
	}
	got, err = f.s.Backlinks(ctx, other, uuid.UUID{}, 2, 100, 3)
	if want := []app.Backlink{{SourceID: many, Revision: 4, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 200, End: 201}}}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("of the other page: %+v, %v\nwant %+v", got, err, want)
	}
}

// sourcesOf is the pages of backlinks.
func sourcesOf(backlinks []app.Backlink) []uuid.UUID {
	var out []uuid.UUID
	for _, b := range backlinks {
		out = append(out, b.SourceID)
	}
	return out
}

// A page's properties are its properties' keys and values in the order
// written, and its property links' paths, each with where it resolves, the
// zero id for none, by start; whether its frontmatter is valid. A page the
// index does not have has none (M6/P5 design 4).
func TestAPagesProperties(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, invalid, x := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	facts := facts()
	facts.Links = append(facts.Links, domain.Link{Kind: "wikilink", Property: "up", Target: "Up", Start: 4, End: 6})
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 2}, facts)
	f.resolve(t, p, domain.Resolution{ID: x}, 12)
	f.replace(t, app.Page{ID: invalid, NotebookID: f.eng, Revision: 1}, domain.Facts{})

	got, ok, err := f.s.Properties(ctx, p)
	want := app.Properties{
		Valid:      true,
		Properties: []app.Property{{Key: "sources", Value: json.RawMessage(`["[[Other]]"]`)}, {Key: "n", Value: json.RawMessage(`{"a": 1}`)}},
		Links:      []app.PropertyLink{{Key: "up"}, {Key: "sources.0", NodeID: x}},
	}
	if err != nil || !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("Properties = %+v, %v, %v\nwant %+v", got, ok, err, want)
	}
	if got, ok, err := f.s.Properties(ctx, invalid); err != nil || !ok || !reflect.DeepEqual(got, app.Properties{}) {
		t.Errorf("an invalid frontmatter's: %+v, %v, %v", got, ok, err)
	}
	if got, ok, err := f.s.Properties(ctx, uuid.NewV7()); err != nil || ok {
		t.Errorf("not indexed: %+v, %v, %v", got, ok, err)
	}
}

// tagged is a page's facts with the tags of names, by their keys as
// lower-case ASCII.
func tagged(names ...string) domain.Facts {
	f := domain.Facts{FrontmatterValid: true}
	for _, n := range names {
		f.Tags = append(f.Tags, domain.Tag{Key: strings.ToLower(n), Name: n, Count: 1})
	}
	return f
}

// A notebook's tags are its tags' keys, by key: each as most of its pages
// write it, the first by bytes of those as many; and how many pages have
// it. Another notebook's are its own (M6/P5 design 5).
func TestTheTagsOfANotebook(t *testing.T) {
	f := newFixture(t)
	for _, names := range [][]string{{"Project", "a/b", "X"}, {"project"}, {"PROJECT", "x"}, {"Project"}} {
		f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}, tagged(names...))
	}
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, tagged("project", "project2"))
	got, err := f.s.Tags(context.Background(), f.eng)
	want := []app.Tag{{Tag: "a/b", Pages: 1}, {Tag: "Project", Pages: 4}, {Tag: "X", Pages: 2}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Tags = %+v, %v\nwant %+v", got, err, want)
	}
}

// The pages of a tag are those with it or a tag under it, key/…, each
// once, by id: not those whose key only starts with it, nor those of
// another notebook; '_' is no wildcard (M6/P5 design 5).
func TestThePagesOfATag(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := ids(8)
	for i, names := range [][]string{{"a"}, {"a/b"}, {"a/b/c", "a"}, {"ab"}, {"a0"}, {"a.b"}, {"a_b"}, {"axb"}} {
		f.replace(t, app.Page{ID: p[i], NotebookID: f.eng, Revision: 1}, tagged(names...))
	}
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, tagged("a"))
	for _, tt := range []struct {
		key  string
		want []uuid.UUID
	}{
		{"a", p[:3]},
		{"a/b", p[1:3]},
		{"a/b/c", p[2:3]},
		{"a_b", p[6:7]},
		{"b", nil},
	} {
		if got, err := f.s.TagPages(ctx, f.eng, tt.key); err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("TagPages(%q) = %v, %v; want %v", tt.key, got, err, tt.want)
		}
	}
}

// A notebook's aliases are its pages', by page, each page's by key.
func TestTheAliasesOfANotebook(t *testing.T) {
	f := newFixture(t)
	p, q := uuid.NewV7(), uuid.NewV7()
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts())
	// By key, alpha before zed, not by name, "Zed" before "alpha" (review r3-6).
	f.replace(t, app.Page{ID: q, NotebookID: f.eng, Revision: 1}, domain.Facts{Aliases: []domain.Alias{{Key: "zed", Name: "Zed"}, {Key: "alpha", Name: "alpha"}}})
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, facts())
	got, err := f.s.NotebookAliases(context.Background(), f.eng)
	if want := map[uuid.UUID][]string{p: {"Al", "Straße"}, q: {"alpha", "Zed"}}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("NotebookAliases = %v, %v\nwant %v", got, err, want)
	}
}

// The backlinks are read from the index of the links to a page by the page
// they are written in, and so are those a page's move or deletion
// reaches; a page's property links from their own (M6/P5 design 8). Scans
// of the whole table are off, as they would be past a few rows. The
// statements here are the queries' parts, their predicates and orders as
// written there: each index serves them, which the plans of the queries
// themselves, measured on data of the size they are for, confirmed (the
// design's 13).
func TestTheReadsUseTheirIndexes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, tt := range []struct {
		query string
		index string
	}{
		// Backlinks' step to the next page, its count and its contexts.
		{`SELECT source_id FROM page_links WHERE resolved_id = $1 AND source_id > $1 AND source_id <> $1 ORDER BY source_id LIMIT 1`, "page_links_resolved_id_source_id_idx"},
		{`SELECT 1 FROM page_links WHERE resolved_id = $1 AND source_id = $1 LIMIT 1000`, "page_links_resolved_id_source_id_idx"},
		{`SELECT range_start, range_end FROM page_links WHERE resolved_id = $1 AND source_id = $1 ORDER BY range_start LIMIT 10`, "page_links_resolved_id_source_id_idx"},
		// LinksReached's part by the pages its links resolve to.
		{`SELECT source_id FROM page_links WHERE resolved_id = ANY(ARRAY[$1::uuid])`, "page_links_resolved_id_source_id_idx"},
		{`SELECT property_key FROM page_links WHERE source_id = $1 AND property_key IS NOT NULL ORDER BY range_start`, "page_links_source_id_range_start_idx"},
	} {
		var plan []string
		err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `EXPLAIN `+tt.query, uuid.NewV7())
			if err != nil {
				return err
			}
			plan, err = pgx.CollectRows(rows, pgx.RowTo[string])
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(plan, "\n"), tt.index) {
			t.Errorf("%s\nis planned without %s:\n%s", tt.query, tt.index, strings.Join(plan, "\n"))
		}
	}
}
