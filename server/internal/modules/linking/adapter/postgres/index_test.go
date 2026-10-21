package postgresadapter_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"uuid"

	postgresadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

var _ app.IndexReads = (*postgresadapter.Store)(nil)

// property is a page's property as the index keeps it: its key and its
// value's JSON.
type property struct{ key, value string }

// indexPage writes the index's rows of a page of notebook: its
// properties in order and its tags, each its own key.
func (f fixture) indexPage(t *testing.T, notebook uuid.UUID, props []property, tags ...string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.NewV7()
	if _, err := f.pool.Exec(ctx, `INSERT INTO indexed_pages (node_id, notebook_id, revision, extractor, frontmatter_valid)
		VALUES ($1, $2, 1, 1, true)`, id, notebook); err != nil {
		t.Fatal(err)
	}
	for i, p := range props {
		if _, err := f.pool.Exec(ctx, `INSERT INTO page_properties (source_id, position, notebook_id, key, value)
			VALUES ($1, $2, $3, $4, $5::jsonb)`, id, i, notebook, p.key, p.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, tag := range tags {
		if _, err := f.pool.Exec(ctx, `INSERT INTO page_tags (source_id, tag_key, notebook_id, tag, count)
			VALUES ($1, lower($2), $3, $2, 1)`, id, tag, notebook); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// link writes n links of source that resolve to target.
func (f fixture) link(t *testing.T, notebook, source, target uuid.UUID, n int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind,
			target, resolved_id, ambiguous)
		SELECT $1, k * 10, k * 10 + 1, $2, 'wikilink', 'x', $3, false FROM generate_series(0, $4::integer - 1) k`,
		source, notebook, target, n); err != nil {
		t.Fatal(err)
	}
}

// jsonText is s as a JSON string.
func jsonText(s string) string { return fmt.Sprintf("%q", s) }

// The index's reads (M9/P4 design 3.9.3, 6.1): the pages' types, the
// items' facts and the inbound links.
func TestIndexReads(t *testing.T) {
	f := newFixture(t)
	t.Run("types", func(t *testing.T) { indexPagesTypes(t, f) })
	t.Run("items", func(t *testing.T) { indexItems(t, f) })
	t.Run("inbound", func(t *testing.T) { inboundCounts(t, f) })
}

// A page's type is its first property whose key is "type" but for ASCII
// case, when that one's value is a string, never a later one; its start
// is 128 characters (translate's six rows, and the first a list); with a
// type to keep, the statement keeps only the pages of exactly it, by the
// whole value; another notebook's pages are not the index's.
func indexPagesTypes(t *testing.T, f fixture) {
	ctx := context.Background()
	long := strings.Repeat("𝐱", 200)
	want, whole := map[uuid.UUID]*string{}, map[uuid.UUID]string{}
	page := func(typ *string, props ...property) {
		id := f.indexPage(t, f.eng, props)
		want[id] = typ
		if typ != nil {
			whole[id] = *typ
		}
	}
	s := func(v string) *string { return &v }
	page(s("a"), property{"Type", jsonText("a")})
	page(s("b"), property{"TYPE", jsonText("b")})
	page(s("c"), property{"tYpE", jsonText("c")})
	page(nil, property{"types", jsonText("d")})
	page(nil, property{"ｔｙｐｅ", jsonText("e")})
	page(nil, property{"Type", `["x"]`}, property{"type", jsonText("y")})
	page(nil, property{"type", `5`})
	page(s("f"), property{"title", jsonText("t")}, property{"summary", jsonText("s")}, property{"type", jsonText("f")})
	page(s("g"), property{"type", jsonText("g")}, property{"Type", jsonText("h")})
	page(s(string([]rune(long)[:domain.IndexTypeChars])), property{"type", jsonText(long)})
	for id, w := range want {
		if w != nil && len(*w) > 100 {
			whole[id] = long
		}
	}
	page(s(""), property{"type", jsonText("")})
	page(nil)
	f.indexPage(t, f.ops, []property{{"type", jsonText("a")}})

	got, err := f.s.IndexPages(ctx, f.eng, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uuid.UUID]bool{}
	for _, p := range got {
		w, ok := want[p.ID]
		switch {
		case !ok || seen[p.ID]:
			t.Errorf("the page %v is another notebook's, or twice", p.ID)
		case (w != nil) != p.Typed || w != nil && *w != p.TypeStart:
			t.Errorf("the page %v's type %q, %v; want %v", p.ID, p.TypeStart, p.Typed, w)
		}
		seen[p.ID] = true
	}
	if len(seen) != len(want) {
		t.Errorf("%d pages, want %d", len(seen), len(want))
	}
	for _, typ := range []string{"b", "", "y", "h", long, string([]rune(long)[:domain.IndexTypeChars])} {
		got, err := f.s.IndexPages(ctx, f.eng, &typ)
		var ids, wants []uuid.UUID
		for _, p := range got {
			ids = append(ids, p.ID)
		}
		for id, w := range whole {
			if w == typ {
				wants = append(wants, id)
			}
		}
		slices.SortFunc(ids, uuid.UUID.Compare)
		slices.SortFunc(wants, uuid.UUID.Compare)
		if err != nil || !slices.Equal(ids, wants) {
			t.Errorf("the pages of type %.10q: %v, %v; want %v", typ, ids, err, wants)
		}
	}
}

// A page's item is its type and its summary, each the first of its key
// but for ASCII case when its value is a string, their first 512
// characters; its first 21 tags by key, each's first 512 characters. A
// page the index does not hold has none.
func indexItems(t *testing.T, f fixture) {
	ctx := context.Background()
	long := strings.Repeat("項", 600)
	var tags []string
	for i := range 25 {
		tags = append(tags, fmt.Sprintf("Tag%02d", 24-i))
	}
	tags = append(tags, "A"+long)
	full := f.indexPage(t, f.ops, []property{{"SUMMARY", jsonText(long)}, {"Type", jsonText("Project")}, {"summary", jsonText("later")}},
		tags...)
	bare := f.indexPage(t, f.ops, []property{{"summary", `{"a": 1}`}, {"type", `null`}})
	got, err := f.s.IndexItems(ctx, []uuid.UUID{full, bare, uuid.NewV7()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("items of %d pages, want 2: %v", len(got), slices.Collect(maps.Keys(got)))
	}
	it := got[full]
	wantTags := []string{string([]rune("A" + long)[:512])}
	for i := range 20 {
		wantTags = append(wantTags, fmt.Sprintf("Tag%02d", i))
	}
	if it.Type == nil || *it.Type != "Project" || it.Summary == nil || *it.Summary != string([]rune(long)[:512]) ||
		!slices.Equal(it.Tags, wantTags) {
		t.Errorf("the item: type %v, summary %.20q, tags %.40q; want Project, 512 characters, %.40q", it.Type, deref(it.Summary), it.Tags, wantTags)
	}
	if it := got[bare]; it.Type != nil || it.Summary != nil || len(it.Tags) != 0 {
		t.Errorf("a page of values not strings: %+v", it)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// A page's inbound links are the pages that link to it, not their links,
// never itself, counted to the most asked; a page none links to has none,
// or 0 when it links to itself alone.
func inboundCounts(t *testing.T, f fixture) {
	ctx := context.Background()
	target, many, self, none := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	f.link(t, f.eng, uuid.NewV7(), target, 3)
	f.link(t, f.eng, uuid.NewV7(), target, 1)
	f.link(t, f.eng, target, target, 2)
	f.link(t, f.eng, self, self, 1)
	if _, err := f.pool.Exec(ctx, `INSERT INTO page_links (source_id, range_start, range_end, notebook_id, kind, target, resolved_id, ambiguous)
		SELECT uuidv7(), 0, 1, $1, 'wikilink', 'x', $2, false FROM generate_series(1, 1001)`, f.eng, many); err != nil {
		t.Fatal(err)
	}
	got, err := f.s.InboundCounts(ctx, []uuid.UUID{target, many, self, none}, domain.MaxInbound)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[uuid.UUID]int{target: 2, many: domain.MaxInbound, self: 0}; !maps.Equal(got, want) {
		t.Errorf("inbound = %v, want %v", got, want)
	}
	if got, err := f.s.InboundCounts(ctx, []uuid.UUID{target}, 1); err != nil || got[target] != 1 {
		t.Errorf("inbound to 1 = %v, %v", got, err)
	}
}
