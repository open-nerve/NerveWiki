package postgresadapter_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres/pgtest"
)

var _ app.Store = (*postgresadapter.Store)(nil)

type fixture struct {
	s    *postgresadapter.Store
	pool *pgxpool.Pool
	tx   *postgres.TxManager
	// Two notebooks; the index has no foreign key, so no rows of theirs.
	eng, ops uuid.UUID
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool, err := postgres.NewPool(context.Background(), config.DatabaseConfig{URL: pgtest.NewDatabase(t), MaxConns: 6})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return fixture{
		s: postgresadapter.New(pool), pool: pool, tx: postgres.NewTxManager(pool, time.Second),
		eng: uuid.NewV7(), ops: uuid.NewV7(),
	}
}

// replace has p's rows hold f.
func (f fixture) replace(t *testing.T, p app.Page, facts domain.Facts) app.Dropped {
	t.Helper()
	dropped, err := f.s.ReplacePage(context.Background(), p, facts)
	if err != nil {
		t.Fatal(err)
	}
	return dropped
}

// rows is what a query answers, a row a slice of its columns, each cast to
// text in the query.
func (f fixture) rows(t *testing.T, sql string, args ...any) [][]string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			t.Fatal(err)
		}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = text(v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func text(v any) string {
	if v == nil {
		return "NULL"
	}
	return fmt.Sprint(v)
}

// facts is a page's facts with a link of each kind, one in a property, one
// written with ".md", one whose target resolves to nothing; two tags, two
// properties and two aliases.
func facts() domain.Facts {
	return domain.Facts{
		FrontmatterValid: true,
		Links: []domain.Link{
			{Kind: "wikilink", Target: "A/Note", Anchor: "Part", Display: "the note", Start: 40, End: 46},
			{Kind: "embed", Target: "Straße.md", Start: 60, End: 69},
			{Kind: "link", Property: "sources.0", Target: "Other", Start: 12, End: 17},
			{Kind: "image", Target: "A//B", Start: 80, End: 84},
		},
		Tags:       []domain.Tag{{Key: "todo", Name: "ToDo", Count: 2}, {Key: "x", Name: "x", Count: 1}},
		Properties: []domain.Property{{Key: "sources", Value: []byte(`["[[Other]]"]`)}, {Key: "n", Value: []byte(`{"a": 1}`)}},
		Aliases:    []domain.Alias{{Key: "al", Name: "Al"}, {Key: "strasse", Name: "Straße"}},
	}
}

// A page's rows hold its facts: its links resolved to none, each with the
// keys of its target's last segment, without ".md" and with it, none for a
// target that resolves to nothing, an empty field none; its tags, its
// properties in order, its aliases, and the revision and extraction they
// are of. Another page's rows are its own.
func TestAPagesRowsHoldItsFacts(t *testing.T) {
	f := newFixture(t)
	p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 3}
	other := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
	if got := f.replace(t, p, facts()); !reflect.DeepEqual(got, app.Dropped{}) {
		t.Errorf("a new page dropped %+v", got)
	}
	f.replace(t, other, domain.Facts{Links: []domain.Link{{Kind: "wikilink", Target: "Note", Start: 2, End: 6}}})

	id, nb := p.ID.String(), f.eng.String()
	if got, want := f.rows(t, `SELECT node_id::text, notebook_id::text, revision::text, extractor::text, frontmatter_valid::text
		FROM indexed_pages ORDER BY node_id`), [][]string{{id, nb, "3", "1", "true"}, {other.ID.String(), nb, "1", "1", "false"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("indexed pages = %v\nwant %v", got, want)
	}
	if got, want := f.rows(t, `SELECT range_start::text, range_end::text, notebook_id::text, kind, property_key, target, anchor,
			display, target_key, target_alt_key, resolved_id::text, ambiguous::text
		FROM page_links WHERE source_id = $1 ORDER BY range_start`, p.ID), [][]string{
		{"12", "17", nb, "link", "sources.0", "Other", "NULL", "NULL", "other", "NULL", "NULL", "false"},
		{"40", "46", nb, "wikilink", "NULL", "A/Note", "Part", "the note", "note", "NULL", "NULL", "false"},
		{"60", "69", nb, "embed", "NULL", "Straße.md", "NULL", "NULL", "strasse", "strasse.md", "NULL", "false"},
		{"80", "84", nb, "image", "NULL", "A//B", "NULL", "NULL", "NULL", "NULL", "NULL", "false"},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("links = %v\nwant %v", got, want)
	}
	if got, want := f.rows(t, `SELECT tag_key, notebook_id::text, tag, count::text FROM page_tags WHERE source_id = $1 ORDER BY tag_key`, p.ID),
		[][]string{{"todo", nb, "ToDo", "2"}, {"x", nb, "x", "1"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("tags = %v\nwant %v", got, want)
	}
	if got, want := f.rows(t, `SELECT position::text, notebook_id::text, key, value::text FROM page_properties WHERE source_id = $1 ORDER BY position`, p.ID),
		[][]string{{"0", nb, "sources", `["[[Other]]"]`}, {"1", nb, "n", `{"a": 1}`}}; !reflect.DeepEqual(got, want) {
		t.Errorf("properties = %v\nwant %v", got, want)
	}
	if got, want := f.rows(t, `SELECT alias_key, notebook_id::text, alias FROM page_aliases WHERE source_id = $1 ORDER BY alias_key`, p.ID),
		[][]string{{"al", nb, "Al"}, {"strasse", nb, "Straße"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("aliases = %v\nwant %v", got, want)
	}
	if got := f.rows(t, `SELECT range_start::text FROM page_links WHERE source_id = $1`, other.ID); !reflect.DeepEqual(got, [][]string{{"2"}}) {
		t.Errorf("the other page's links = %v", got)
	}
}

// Replacing a page's facts drops all its rows for the new ones, and tells
// what they held: the pages its links resolved to and its aliases' keys,
// each once. Deleting pages drops theirs alike; another page's stay.
func TestReplacingOrDeletingAPageTellsWhatItsRowsHeld(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
	q := app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}
	f.replace(t, p, facts())
	f.replace(t, q, facts())
	x, y := uuid.NewV7(), uuid.NewV7()
	err := f.s.SetResolutions(ctx, []app.Link{
		{SourceID: p.ID, Start: 12, Resolution: domain.Resolution{ID: y}},
		{SourceID: p.ID, Start: 40, Resolution: domain.Resolution{ID: x, Ambiguous: true}},
		{SourceID: p.ID, Start: 60, Resolution: domain.Resolution{ID: x}},
		{SourceID: q.ID, Start: 12, Resolution: domain.Resolution{ID: x}},
	})
	if err != nil {
		t.Fatal(err)
	}

	p.Revision = 2
	dropped := f.replace(t, p, domain.Facts{
		FrontmatterValid: true,
		Links:            []domain.Link{{Kind: "wikilink", Target: "New", Start: 5, End: 8}},
		Aliases:          []domain.Alias{{Key: "al", Name: "AL"}},
	})
	targets := []uuid.UUID{x, y}
	slices.SortFunc(targets, uuid.UUID.Compare)
	want := app.Dropped{Targets: targets, AliasKeys: []string{"al", "strasse"}}
	if !reflect.DeepEqual(dropped, want) {
		t.Errorf("replacing dropped %+v, want %+v", dropped, want)
	}
	for table, n := range map[string]int{"page_links": 1, "page_tags": 0, "page_properties": 0, "page_aliases": 1} {
		if got := f.rows(t, "SELECT count(*)::text FROM "+table+" WHERE source_id = $1", p.ID); got[0][0] != strconv.Itoa(n) {
			t.Errorf("%s holds %s rows of the page replaced, want %d", table, got[0][0], n)
		}
	}
	if got := f.rows(t, `SELECT revision::text FROM indexed_pages WHERE node_id = $1`, p.ID); !reflect.DeepEqual(got, [][]string{{"2"}}) {
		t.Errorf("the page replaced is indexed at %v", got)
	}

	dropped, err = f.s.DeletePages(ctx, []uuid.UUID{p.ID, uuid.NewV7()})
	if err != nil {
		t.Fatal(err)
	}
	if want := (app.Dropped{AliasKeys: []string{"al"}}); !reflect.DeepEqual(dropped, want) {
		t.Errorf("deleting dropped %+v, want %+v", dropped, want)
	}
	for _, table := range []string{"page_links", "page_tags", "page_properties", "page_aliases"} {
		if got := f.rows(t, "SELECT DISTINCT source_id::text FROM "+table); !reflect.DeepEqual(got, [][]string{{q.ID.String()}}) {
			t.Errorf("%s holds the rows of %v, want only the page kept", table, got)
		}
	}
	if got := f.rows(t, "SELECT node_id::text FROM indexed_pages"); !reflect.DeepEqual(got, [][]string{{q.ID.String()}}) {
		t.Errorf("indexed pages = %v, want only the page kept", got)
	}
}

// Deleting notebooks drops every row of theirs, and only theirs.
func TestDeletingNotebooksDropsTheirRows(t *testing.T) {
	f := newFixture(t)
	third := uuid.NewV7()
	keep := app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.eng, Revision: 1}, facts())
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: third, Revision: 1}, facts())
	f.replace(t, keep, facts())
	if err := f.s.DeleteNotebooks(context.Background(), []uuid.UUID{f.eng, third}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"indexed_pages", "page_links", "page_tags", "page_properties", "page_aliases"} {
		if got := f.rows(t, "SELECT DISTINCT notebook_id::text FROM "+table); !reflect.DeepEqual(got, [][]string{{f.ops.String()}}) {
			t.Errorf("%s holds the rows of %v, want only the notebook kept", table, got)
		}
	}
}

// The links a change reaches are its notebook's whose target's key, with
// ".md" or without, is among the keys; those resolved to one of the
// targets; and those written in one of the sources: with where each
// resolves.
func TestTheLinksAChangeReaches(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, q := uuid.NewV7(), uuid.NewV7()
	if p.Compare(q) > 0 {
		p, q = q, p // the links come in the order of their pages' ids
	}
	x := uuid.NewV7()
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts())
	f.replace(t, app.Page{ID: q, NotebookID: f.eng, Revision: 1}, domain.Facts{Links: []domain.Link{
		{Kind: "wikilink", Target: "B/Note.md", Start: 0, End: 9},
		{Kind: "wikilink", Target: "Else", Start: 20, End: 24},
	}})
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, facts())
	if err := f.s.SetResolutions(ctx, []app.Link{{SourceID: q, Start: 20, Resolution: domain.Resolution{ID: x, Ambiguous: true}}}); err != nil {
		t.Fatal(err)
	}
	link := func(source uuid.UUID, start int, target string) app.Link {
		return app.Link{SourceID: source, Start: start, Target: target}
	}
	pOther, pNote, pStrasse := link(p, 12, "Other"), link(p, 40, "A/Note"), link(p, 60, "Straße.md")
	pNone := link(p, 80, "A//B")
	qNote, qElse := link(q, 0, "B/Note.md"), link(q, 20, "Else")
	qElse.Resolution = domain.Resolution{ID: x, Ambiguous: true}
	tests := []struct {
		name  string
		reach domain.Reach
		want  []app.Link
	}{
		{"nothing", domain.Reach{}, nil},
		{"by key", domain.Reach{Keys: []string{"note", "other"}}, []app.Link{pOther, pNote, qNote}},
		{"by key with .md", domain.Reach{Keys: []string{"note.md", "strasse.md"}}, []app.Link{pStrasse, qNote}},
		{"by target", domain.Reach{Targets: []uuid.UUID{x}}, []app.Link{qElse}},
		{"by source", domain.Reach{Sources: []uuid.UUID{p}}, []app.Link{pOther, pNote, pStrasse, pNone}},
		{"each once", domain.Reach{Keys: []string{"else"}, Targets: []uuid.UUID{x}, Sources: []uuid.UUID{q}}, []app.Link{qNote, qElse}},
	}
	for _, tt := range tests {
		got, err := f.s.Links(ctx, f.eng, tt.reach)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %+v\nwant %+v", tt.name, got, tt.want)
		}
	}
}

// The pages with an alias are found by its key, in their notebook only.
func TestThePagesWithAnAlias(t *testing.T) {
	f := newFixture(t)
	p, q := uuid.NewV7(), uuid.NewV7()
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts())
	f.replace(t, app.Page{ID: q, NotebookID: f.eng, Revision: 1}, domain.Facts{Aliases: []domain.Alias{{Key: "al", Name: "al"}}})
	f.replace(t, app.Page{ID: uuid.NewV7(), NotebookID: f.ops, Revision: 1}, facts())
	got, err := f.s.Aliases(context.Background(), f.eng, []string{"al", "strasse", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []app.Alias{{PageID: p, Key: "al"}, {PageID: p, Key: "strasse"}, {PageID: q, Key: "al"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("aliases = %+v\nwant %+v", got, want)
	}
}

// A resolution to none clears the page and the tie; setting a link that is
// not there is a defect, and sets nothing.
func TestSettingResolutions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p := uuid.NewV7()
	f.replace(t, app.Page{ID: p, NotebookID: f.eng, Revision: 1}, facts())
	x := uuid.NewV7()
	set := func(links ...app.Link) error {
		return f.tx.WithinTx(ctx, func(ctx context.Context) error { return f.s.SetResolutions(ctx, links) })
	}
	if err := set(app.Link{SourceID: p, Start: 40, Resolution: domain.Resolution{ID: x, Ambiguous: true}}); err != nil {
		t.Fatal(err)
	}
	if err := set(app.Link{SourceID: p, Start: 40}); err != nil {
		t.Fatal(err)
	}
	if got := f.rows(t, `SELECT resolved_id::text, ambiguous::text FROM page_links WHERE range_start = 40`); !reflect.DeepEqual(got, [][]string{{"NULL", "false"}}) {
		t.Errorf("a link resolved to none is %v", got)
	}
	err := set(app.Link{SourceID: p, Start: 12, Resolution: domain.Resolution{ID: x}}, app.Link{SourceID: p, Start: 13, Resolution: domain.Resolution{ID: x}})
	if err == nil {
		t.Error("setting a link that is not there succeeded")
	}
	if got := f.rows(t, `SELECT count(*)::text FROM page_links WHERE resolved_id IS NOT NULL`); got[0][0] != "0" {
		t.Errorf("a refused setting set %s links", got[0][0])
	}
	if err := f.s.SetResolutions(ctx, nil); err != nil {
		t.Errorf("setting no links: %v", err)
	}
}

// The index's lock is a notebook's, held until the transaction ends: a
// second taker of the notebook waits for it, a taker of another notebook
// does not. Outside a transaction it is refused.
func TestTheIndexsLockIsANotebooksUntilTheTransactionEnds(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.s.Lock(ctx, f.eng); err == nil {
		t.Error("the lock was taken outside a transaction")
	}
	held, release := make(chan struct{}), make(chan struct{})
	let := sync.OnceFunc(func() { close(release) })
	defer let()
	first := make(chan error, 1)
	go func() {
		first <- f.tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := f.s.Lock(ctx, f.eng); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	// The takers that must not wait for good give up, rather than hang.
	soon, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := f.tx.WithinTx(soon, func(ctx context.Context) error { return f.s.Lock(ctx, f.ops) }); err != nil {
		t.Fatalf("another notebook's lock: %v", err)
	}
	second := make(chan error, 1)
	go func() {
		second <- f.tx.WithinTx(soon, func(ctx context.Context) error { return f.s.Lock(ctx, f.eng) })
	}()
	pgtest.WaitForLockWaits(t, f.pool, 1, 10*time.Second)
	select {
	case err := <-second:
		t.Fatalf("the second taker did not wait: %v", err)
	default:
	}
	let()
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
}
