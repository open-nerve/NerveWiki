package postgresadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
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
// r2-M1, r3). So under each plan: the rows are written in an order of
// their own, not the ids' (review c4).
func TestThePagesThatLinkToAPage(t *testing.T) {
	f := newFixture(t)
	// No statistics: with them, planning reads an index's ends, which the
	// check that a plan is heeded would count (review c8).
	f.still(t)
	ctx := context.Background()
	p := ids(6)
	many, ambiguous, target, one, other, unindexed := p[0], p[1], p[2], p[3], p[4], p[5]
	starts := []int{110, 0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	f.replace(t, app.Page{ID: one, NotebookID: f.eng, Revision: 2}, wikilinks(7, 9))
	f.resolve(t, one, domain.Resolution{ID: target}, 9)
	f.replace(t, app.Page{ID: many, NotebookID: f.eng, Revision: 4}, wikilinks(append(starts, 200)...))
	f.resolve(t, many, domain.Resolution{ID: target}, starts...)
	f.resolve(t, many, domain.Resolution{ID: other}, 200)
	f.replace(t, app.Page{ID: ambiguous, NotebookID: f.eng, Revision: 1}, wikilinks(5))
	f.resolve(t, ambiguous, domain.Resolution{ID: target, Ambiguous: true}, 5)
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
	for _, plan := range plans() {
		t.Run(plan.name, func(t *testing.T) {
			backlinks := func(target, after uuid.UUID, size, count, contexts int) (got []app.Backlink, err error) {
				ctx, cancel := context.WithTimeout(ctx, readTime)
				defer cancel()
				err = f.tx.WithinTx(ctx, func(ctx context.Context) error {
					if err := plan.set(ctx, postgres.DB(ctx, f.pool)); err != nil {
						return err
					}
					table0, index0, err := f.counts(ctx)
					if err != nil {
						return err
					}
					if got, err = f.s.Backlinks(ctx, target, after, size, count, contexts); err != nil {
						return err
					}
					table, index, err := f.counts(ctx)
					if index > index0 && !plan.indexes || table > table0 && !plan.tables {
						t.Errorf("the plan is not heeded: %d rows of the table and %d entries of its indexes read", table-table0, index-index0)
					}
					return err
				})
				return got, err
			}
			got, err := backlinks(target, uuid.UUID{}, 2, 100, 10)
			want := []app.Backlink{
				{SourceID: many, Revision: 4, Extractor: e, Links: 12, Ranges: first},
				{SourceID: ambiguous, Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 5, End: 6}}},
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("the first two: %+v, %v\nwant %+v", got, err, want)
			}
			got, err = backlinks(target, ambiguous, 2, 100, 10)
			want = []app.Backlink{
				{SourceID: one, Revision: 2, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 9, End: 10}}},
				{SourceID: unindexed, Links: 1, Ranges: []domain.Range{{Start: 8, End: 9}}},
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("after the first two, past the page itself: %+v, %v\nwant %+v", got, err, want)
			}
			got, err = backlinks(target, uuid.UUID{}, 10, 100, 10)
			want = []app.Backlink{
				{SourceID: many, Revision: 4, Extractor: e, Links: 12, Ranges: first},
				{SourceID: ambiguous, Revision: 1, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 5, End: 6}}},
				{SourceID: one, Revision: 2, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 9, End: 10}}},
				{SourceID: unindexed, Links: 1, Ranges: []domain.Range{{Start: 8, End: 9}}},
			}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("all: %+v, %v\nwant %+v", got, err, want)
			}
			// A step past the page itself, which links to itself, between the others.
			got, err = backlinks(target, uuid.UUID{}, 3, 100, 1)
			if ids := sourcesOf(got); err != nil || !slices.Equal(ids, []uuid.UUID{many, ambiguous, one}) {
				t.Errorf("three from the first: %v, %v", ids, err)
			}
			if got, err := backlinks(target, unindexed, 2, 100, 10); err != nil || len(got) != 0 {
				t.Errorf("after the last: %+v, %v", got, err)
			}
			got, err = backlinks(target, uuid.UUID{}, 1, 5, 3)
			if want := []app.Backlink{{SourceID: many, Revision: 4, Extractor: e, Links: 5, Ranges: first[:3]}}; err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("counted up to 5: %+v, %v\nwant %+v", got, err, want)
			}
			got, err = backlinks(other, uuid.UUID{}, 2, 100, 3)
			if want := []app.Backlink{{SourceID: many, Revision: 4, Extractor: e, Links: 1, Ranges: []domain.Range{{Start: 200, End: 201}}}}; err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("of the other page: %+v, %v\nwant %+v", got, err, want)
			}
		})
	}
}

// The backlinks read a few of the links to a page, not each: a page of
// them among two thousand, from the first or after a thousand (review c8);
// of a page that links to itself 25,000 times, from before it, past it by
// the walk's first read or by a step, with no statistics or with them
// (review c1, c4, c6); of a page one page writes
// most of the links to, the others' links too, though each writes more
// links to other pages, before the table is vacuumed, with no scan of the
// whole table (review c1, c3, c4), and on a table with no statistics yet
// (review c5). The table that is analyzed is under the 30,000 rows ANALYZE
// samples, so it reads them all, and the plans do not vary by its sample;
// no table is vacuumed or analyzed but by the test (review c7).
func TestTheBacklinksReadAFewLinks(t *testing.T) {
	t.Run("a page of them among many", func(t *testing.T) {
		f := newFixture(t)
		f.still(t)
		p := ids(2001)
		target, sources := p[0], p[1:]
		if _, err := f.pool.Exec(context.Background(), `INSERT INTO page_links (source_id, range_start, range_end, notebook_id,
			kind, target, resolved_id, ambiguous) SELECT s, 0, 1, $2, 'wikilink', 'x', $3, false FROM unnest($1::uuid[]) s`,
			sources, f.eng, target); err != nil {
			t.Fatal(err)
		}
		for _, from := range []int{0, 1000} {
			after := uuid.UUID{}
			if from > 0 {
				after = sources[from-1]
			}
			// Each of the page's sources: a step, a count and a context.
			got, table, index := f.readBacklinks(t, target, after)
			if !slices.Equal(sourcesOf(got), sources[from:from+51]) || table > 0 || index > 3*51+20 {
				t.Errorf("from %d: %d pages, %d rows of the table and %d entries of its indexes read", from, len(got), table, index)
			}
		}
	})
	t.Run("of a page that links to itself", func(t *testing.T) {
		f := newFixture(t)
		f.still(t)
		p := ids(3)
		before, target, after := p[0], p[1], p[2]
		f.links(t, target, target, 0, 25000)
		f.links(t, before, target, 0, 1)
		f.links(t, after, target, 0, 1)
		for _, analyzed := range []bool{false, true} {
			if analyzed {
				f.analyze(t)
			}
			for _, tt := range []struct {
				after uuid.UUID
				want  []uuid.UUID
			}{{uuid.UUID{}, []uuid.UUID{before, after}}, {before, []uuid.UUID{after}}} {
				got, table, index := f.readBacklinks(t, target, tt.after)
				if !slices.Equal(sourcesOf(got), tt.want) || table > 0 || index > 100 {
					t.Errorf("analyzed %t, after %v: %v, %d rows of the table and %d entries of its indexes read", analyzed, tt.after,
						sourcesOf(got), table, index)
				}
			}
		}
	})
	for _, tt := range []struct {
		name            string
		many, elsewhere int  // the links of the page that writes most, of each other's to other pages
		others          int  // the other pages, each with 3 links to the page after its links elsewhere
		analyzed        bool // the table
		slack           int  // the index entries a read may take past the count of the page that writes most
	}{
		{"of a page one page writes most links to", 25000, 200, 21, true, 200},
		{"of a page one page writes most links to, with no statistics", 250000, 1000, 50, false, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.still(t)
			p := ids(tt.others + 2)
			many, target, others := p[1], p[2], append([]uuid.UUID{p[0]}, p[3:]...)
			f.links(t, many, target, 0, tt.many)
			for _, o := range others {
				f.links(t, o, uuid.NewV7(), 0, tt.elsewhere)
				f.links(t, o, target, tt.elsewhere, 3)
			}
			if tt.analyzed {
				f.analyze(t)
			}
			got, table, index := f.readBacklinks(t, target, uuid.UUID{})
			if want := slices.Insert(slices.Clone(others), 1, many); !slices.Equal(sourcesOf(got), want) || got[1].Links != domain.MaxCount ||
				table > 0 || index > int64(domain.MaxCount+tt.slack) {
				t.Errorf("%d pages, %d rows of the table and %d entries of its indexes read", len(got), table, index)
			}
		})
	}
}

// links writes n links of source that resolve to target, from start on.
func (f fixture) links(t *testing.T, source, target uuid.UUID, start, n int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO page_links (source_id, range_start, range_end, notebook_id,
		kind, target, resolved_id, ambiguous) SELECT $1, g, g + 1, $2, 'wikilink', 'x', $3, false FROM generate_series($4::integer, $4::integer + $5::integer - 1) g`,
		source, f.eng, target, start, n); err != nil {
		t.Fatal(err)
	}
}

// readTime is how long a test's read of the backlinks may take: a walk
// that never ends fails it, not the run's limit (review c6).
const readTime = 10 * time.Second

// readBacklinks is target's backlinks after the page after, a page of 50,
// up to domain.MaxCount links counted, and how many rows of page_links the
// read took by scans of the whole table, and how many entries of its
// indexes (counts).
func (f fixture) readBacklinks(t *testing.T, target, after uuid.UUID) (got []app.Backlink, table, index int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), readTime)
	defer cancel()
	err := f.tx.WithinTx(ctx, func(ctx context.Context) error {
		table0, index0, err := f.counts(ctx)
		if err != nil {
			return err
		}
		if got, err = f.s.Backlinks(ctx, target, after, 51, domain.MaxCount, domain.MaxContexts); err != nil {
			return err
		}
		table, index, err = f.counts(ctx)
		table, index = table-table0, index-index0
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, table, index
}

// counts is how many rows of page_links the connection of ctx's
// transaction has read by scans of the whole table, and how many entries
// of its indexes, since it last reported them: the difference of two in a
// transaction is its statements', as a connection reports them, about each
// second, between transactions (review c5, c6).
func (f fixture) counts(ctx context.Context) (table, index int64, err error) {
	err = postgres.DB(ctx, f.pool).QueryRow(ctx, `SELECT pg_stat_get_xact_tuples_returned('page_links'::regclass),
		(SELECT sum(pg_stat_get_xact_tuples_returned(indexrelid))::bigint FROM pg_index WHERE indrelid = 'page_links'::regclass)`,
	).Scan(&table, &index)
	return table, index, err
}

// still has page_links's statistics change only by analyze: autovacuum,
// on in the tests' cluster, could analyze or vacuum it in a test.
func (f fixture) still(t *testing.T) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `ALTER TABLE page_links SET (autovacuum_enabled = off)`); err != nil {
		t.Fatal(err)
	}
}

// analyze has the planner's statistics of page_links read.
func (f fixture) analyze(t *testing.T) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `ANALYZE page_links`); err != nil {
		t.Fatal(err)
	}
}

// plan is a way the planner may read a query, which its answer must not
// tell: the planner's settings it turns off, and whether it reads indexes
// and whole tables.
type plan struct {
	name            string
	off             []string
	indexes, tables bool
}

// plans are the ways a query is read in a test: as planned, and with each
// kind of scan or join the plans choose off. Nested loops are not: the
// lateral joins have no other (review c6).
func plans() []plan {
	return []plan{
		{"as planned", nil, true, true},
		{"without index scans", []string{"enable_indexscan", "enable_indexonlyscan", "enable_bitmapscan"}, false, true},
		{"without scans of whole tables", []string{"enable_seqscan"}, true, false},
		{"without hash and merge joins", []string{"enable_hashjoin", "enable_mergejoin"}, true, true},
	}
}

// set turns p's settings off for the transaction db is, each statement
// planned anew: a plan cached before, once a statement has run a few
// times, would not heed them.
func (p plan) set(ctx context.Context, db postgres.Querier) error {
	for _, setting := range p.off {
		if _, err := db.Exec(ctx, "SET LOCAL "+setting+" = off"); err != nil {
			return err
		}
	}
	if len(p.off) == 0 {
		return nil
	}
	_, err := db.Exec(ctx, "SET LOCAL plan_cache_mode = force_custom_plan")
	return err
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
// reaches; a page's property links from their own (M6/P5 design 8). The
// plans are the statements' own, their arguments bound, with scans of whole
// tables off, as they would be past a few rows (review c4).
func TestTheReadsUseTheirIndexes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Their columns: a plan may scan an index by a column not its first.
	for name, want := range map[string]string{
		"page_links_resolved_id_source_id_idx": "(resolved_id, source_id, range_start) INCLUDE (range_end) WHERE (resolved_id IS NOT NULL)",
		"page_links_source_id_range_start_idx": "(source_id, range_start) WHERE (property_key IS NOT NULL)",
	} {
		if got := f.rows(t, `SELECT pg_get_indexdef($1::regclass)`, name); !strings.HasSuffix(got[0][0], " USING btree "+want) {
			t.Errorf("%s: %s", name, got[0][0])
		}
	}
	id := uuid.NewV7()
	for _, tt := range []struct {
		name  string
		s     statement
		scans int    // of page_links
		scan  string // each of them
		cond  string // how each's index condition starts
		also  string // a line the plan has too
	}{
		{"Backlinks: the walk, the count and the contexts", statementOf(t, func(q *gen.Queries) error {
			_, err := q.Backlinks(ctx, gen.BacklinksParams{Target: id, After: id, Size: 51, MaxCount: 1000, Contexts: 10})
			return err
		}), 4, "Index Only Scan using page_links_resolved_id_source_id_idx", "((resolved_id = (InitPlan ",
			"Index Scan using indexed_pages_pkey on indexed_pages ip"},
		{"PageProperties: the property links", statementOf(t, func(q *gen.Queries) error {
			_, err := q.PageProperties(ctx, id)
			return err
		}), 2, "Index Scan using page_links_source_id_range_start_idx", "(source_id = ", ""},
		// LinksReached's part by the pages its links resolve to: the whole
		// statement, of four parts, is planned on another index while the
		// tables are empty.
		{"LinksReached: by the pages resolved to", statement{
			sql: `SELECT source_id FROM page_links WHERE resolved_id = ANY($1::uuid[])`, args: []any{[]uuid.UUID{id}},
		}, 1, "Index Only Scan using page_links_resolved_id_source_id_idx", "(resolved_id = ANY ", ""},
	} {
		plan := f.planOf(t, tt.s)
		scans := scansOf(plan)
		if len(scans) != tt.scans || slices.ContainsFunc(scans, func(s [2]string) bool {
			return s[0] != tt.scan || !strings.HasPrefix(s[1], tt.cond)
		}) || !slices.ContainsFunc(plan, func(line string) bool { return strings.Contains(line, tt.also) }) {
			t.Errorf("%s: %d scans of page_links, want %d, each a %s on %s…, and %q:\n%s", tt.name, len(scans), tt.scans, tt.scan,
				tt.cond, tt.also, strings.Join(plan, "\n"))
		}
	}
}

// scansOf is each scan of page_links in plan: its node and the index
// condition under it, if any.
func scansOf(plan []string) [][2]string {
	var out [][2]string
	for i, line := range plan {
		node := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "->"))
		before, after, ok := strings.Cut(node, " on page_links")
		if !ok || after != "" && after[0] != ' ' {
			continue
		}
		var cond string
		for _, next := range plan[i+1:] {
			next = strings.TrimSpace(next)
			if strings.HasPrefix(next, "->") {
				break
			}
			if c, ok := strings.CutPrefix(next, "Index Cond: "); ok {
				cond = c
				break
			}
		}
		out = append(out, [2]string{before, cond})
	}
	return out
}

// statement is the SQL and the arguments a query's call sends, caught
// before they reach a database.
type statement struct {
	sql  string
	args []any
}

// errCaught ends a call whose statement is caught.
var errCaught = errors.New("caught")

func (s *statement) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	s.sql, s.args = sql, args
	return pgconn.CommandTag{}, errCaught
}

func (s *statement) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.sql, s.args = sql, args
	return nil, errCaught
}

func (s *statement) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	s.sql, s.args = sql, args
	return caughtRow{}
}

// caughtRow is the row of a caught statement.
type caughtRow struct{}

func (caughtRow) Scan(...any) error { return errCaught }

// statementOf is the statement call sends.
func statementOf(t *testing.T, call func(q *gen.Queries) error) statement {
	t.Helper()
	var s statement
	if err := call(gen.New(&s)); !errors.Is(err, errCaught) {
		t.Fatalf("the call: %v", err)
	}
	return s
}

// planOf is the plan of s, scans of whole tables off.
func (f fixture) planOf(t *testing.T, s statement) []string {
	t.Helper()
	ctx := context.Background()
	var plan []string
	err := pgx.BeginFunc(ctx, f.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL enable_seqscan = off`); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `EXPLAIN (COSTS OFF) `+s.sql, s.args...)
		if err != nil {
			return err
		}
		plan, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
