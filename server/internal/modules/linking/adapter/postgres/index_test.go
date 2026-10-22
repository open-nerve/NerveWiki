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
// whole value; another notebook's pages are not the index's. A type is read
// by its value_start, its first IndexStartChars characters (S2 review
// B-I1): a type of 3 MiB has its start; a filter shorter than that is
// decided by it, a longer one by its length, value_chars, and then by the
// whole value where the starts and the lengths are equal (fix check
// F-m1): a type of the filter and one more character, the 3 MiB type,
// which starts as the filter does, and a type as long as the filter that
// differs from it at its end, are not of it.
func indexPagesTypes(t *testing.T, f fixture) {
	ctx := context.Background()
	start, whole := map[uuid.UUID]*string{}, map[uuid.UUID]string{}
	page := func(typ *string, props ...property) {
		id := f.indexPage(t, f.eng, props)
		start[id] = nil
		if typ != nil {
			whole[id] = *typ
			s := *typ
			if r := []rune(s); len(r) > domain.IndexTypeChars {
				s = string(r[:domain.IndexTypeChars])
			}
			start[id] = &s
		}
	}
	s := func(v string) *string { return &v }
	long := strings.Repeat("𝐱", 200)
	wide := strings.Repeat("项", 300)
	wideEnd := strings.Repeat("项", 299) + "y"
	huge := strings.Repeat("项", 1<<20)
	at := strings.Repeat("项", domain.IndexStartChars)
	short := strings.Repeat("项", domain.IndexStartChars-1)
	page(s("a"), property{"Type", jsonText("a")})
	page(s("b"), property{"TYPE", jsonText("b")})
	page(s("c"), property{"tYpE", jsonText("c")})
	page(nil, property{"types", jsonText("d")})
	page(nil, property{"ｔｙｐｅ", jsonText("e")})
	page(nil, property{"Type", `["x"]`}, property{"type", jsonText("y")})
	page(nil, property{"type", `5`})
	page(s("f"), property{"title", jsonText("t")}, property{"summary", jsonText("s")}, property{"type", jsonText("f")})
	page(s("g"), property{"type", jsonText("g")}, property{"Type", jsonText("h")})
	page(&long, property{"type", jsonText(long)})
	page(&wide, property{"type", jsonText(wide)})
	page(s(wide+"x"), property{"type", jsonText(wide + "x")})
	page(&wideEnd, property{"type", jsonText(wideEnd)})
	page(&huge, property{"type", jsonText(huge)}, property{"summary", jsonText(huge)})
	page(&at, property{"type", jsonText(at)})
	page(&short, property{"type", jsonText(short)})
	page(s(""), property{"type", jsonText("")})
	page(nil)
	f.indexPage(t, f.ops, []property{{"type", jsonText("a")}})

	got, err := f.s.IndexPages(ctx, f.eng, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uuid.UUID]bool{}
	for _, p := range got {
		w, ok := start[p.ID]
		switch {
		case !ok || seen[p.ID]:
			t.Errorf("the page %v is another notebook's, or twice", p.ID)
		case (w != nil) != p.Typed || w != nil && *w != p.TypeStart:
			t.Errorf("the page %v's type %.20q, %v; want %.20q", p.ID, p.TypeStart, p.Typed, deref(w))
		}
		seen[p.ID] = true
	}
	if len(seen) != len(start) {
		t.Errorf("%d pages, want %d", len(seen), len(start))
	}
	for _, typ := range []string{"b", "", "y", "h", long, string([]rune(long)[:domain.IndexTypeChars]), wide, wideEnd, at,
		short, string([]rune(wide)[:domain.IndexStartChars+10])} {
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
			t.Errorf("the pages of type %.10q (%d characters): %v, %v; want %v", typ, len([]rune(typ)), ids, err, wants)
		}
	}
}

// A page's item is its type and its summary, each the first of its key
// but for ASCII case when its value is a string, their first
// IndexStartChars characters, of 3 MiB too; its first 21 tags by key, not
// by text, each's first 512 characters. A page the index does not hold
// has none.
func indexItems(t *testing.T, f fixture) {
	ctx := context.Background()
	long := strings.Repeat("項", 600)
	// Tags of both cases, more than 21: their first 21 by key are not
	// their first 21 by text (S2 review C-m1).
	var tags []string
	for i := range 13 {
		tags = append(tags, fmt.Sprintf("Z%02d", i), fmt.Sprintf("a%02d", i))
	}
	tags = append(tags, "A"+long)
	byKey := slices.SortedFunc(slices.Values(tags), func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if slices.Equal(byKey[:21], slices.Sorted(slices.Values(tags))[:21]) {
		t.Fatal("the tags' first 21 by key are their first 21 by text: the row tells nothing")
	}
	full := f.indexPage(t, f.ops, []property{{"SUMMARY", jsonText(long)}, {"Type", jsonText("Project")}, {"summary", jsonText("later")}},
		tags...)
	bare := f.indexPage(t, f.ops, []property{{"summary", `{"a": 1}`}, {"type", `null`}})
	huge := strings.Repeat("项", 1<<20)
	big := f.indexPage(t, f.ops, []property{{"type", jsonText(huge)}, {"summary", jsonText("x" + huge)}})
	edge := f.indexPage(t, f.ops, []property{{"type", jsonText(strings.Repeat("a", domain.MaxIndexString))},
		{"summary", jsonText(strings.Repeat("a", domain.MaxIndexString+1))}})
	got, err := f.s.IndexItems(ctx, []uuid.UUID{full, bare, big, edge, uuid.NewV7()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("items of %d pages, want 4: %v", len(got), slices.Collect(maps.Keys(got)))
	}
	it := got[full]
	var wantTags []string
	for _, tag := range byKey[:21] {
		if r := []rune(tag); len(r) > 512 {
			tag = string(r[:512])
		}
		wantTags = append(wantTags, tag)
	}
	if it.Type == nil || *it.Type != "Project" || it.Summary == nil || *it.Summary != string([]rune(long)[:domain.IndexStartChars]) ||
		!slices.Equal(it.Tags, wantTags) {
		t.Errorf("the item: type %v, summary %.20q, tags %.40q; want Project, %d characters, %.40q", it.Type, deref(it.Summary), it.Tags,
			domain.IndexStartChars, wantTags)
	}
	if it := got[bare]; it.Type != nil || it.Summary != nil || len(it.Tags) != 0 {
		t.Errorf("a page of values not strings: %+v", it)
	}
	// A type and a summary of 3 MiB: their starts, which an answer cuts to
	// 256 bytes and marks; one of 256 bytes is whole, one of 257 cut.
	for _, c := range []struct {
		name  string
		s     *string
		whole string
		cut   bool
	}{
		{"the type of 3 MiB", got[big].Type, huge, true},
		{"the summary of 3 MiB", got[big].Summary, "x" + huge, true},
		{"a type of 256 bytes", got[edge].Type, strings.Repeat("a", domain.MaxIndexString), false},
		{"a summary of 257 bytes", got[edge].Summary, strings.Repeat("a", domain.MaxIndexString+1), true},
	} {
		want, _ := domain.CutIndexString(c.whole)
		if c.s == nil {
			t.Errorf("%s: none", c.name)
			continue
		}
		if v, cut := domain.CutIndexString(*c.s); v != want || cut != c.cut || len([]rune(*c.s)) > domain.IndexStartChars {
			t.Errorf("%s: read %d characters, cut to %d bytes, %v; want %d bytes, %v", c.name, len([]rune(*c.s)), len(v), cut, len(want), c.cut)
		}
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
