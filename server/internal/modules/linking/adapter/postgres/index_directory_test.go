package postgresadapter_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"uuid"
)

// These literal qualifications catch treating every stored string as a type,
// or selecting a later string after the first ASCII type key was not a string.
func TestDirectoryIndex(t *testing.T) {
	t.Run("typed_first", func(t *testing.T) {
		f := newFixture(t)
		ctx := context.Background()
		cases := []struct {
			name       string
			properties []property
			valid      bool
			typed      bool
		}{
			{"A", []property{{"type", jsonText("A")}}, true, true},
			{"topic", []property{{"TYPE", jsonText("topic")}}, true, true},
			{"late nonblank", []property{{"Type", jsonText(strings.Repeat(" ", 257) + "x")}}, true, true},
			{"empty", []property{{"type", jsonText("")}}, true, false},
			{"JS whitespace", []property{{"type", jsonText("\ufeff \t")}}, true, false},
			{"first is array", []property{{"TYPE", `["x"]`}, {"type", jsonText("later")}}, true, false},
			{"first is number", []property{{"type", `5`}}, true, false},
			{"invalid frontmatter", []property{{"type", jsonText("invalid")}}, false, false},
		}
		want := make(map[uuid.UUID]bool, len(cases))
		names := make(map[uuid.UUID]string, len(cases))
		ids := make([]uuid.UUID, 0, len(cases))
		for _, c := range cases {
			id := f.indexPage(t, f.eng, c.properties)
			if _, err := f.pool.Exec(ctx, `UPDATE indexed_pages SET extractor = 2, frontmatter_valid = $2 WHERE node_id = $1`, id, c.valid); err != nil {
				t.Fatal(err)
			}
			want[id], names[id] = c.typed, c.name
			ids = append(ids, id)
		}
		f.indexPage(t, f.ops, []property{{"type", jsonText("topic")}})
		pages, err := f.s.IndexPages(ctx, f.eng, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 8 {
			t.Fatalf("index has %d pages, want eight from this notebook", len(pages))
		}
		typed := 0
		seen := make(map[uuid.UUID]bool, len(pages))
		for _, p := range pages {
			expected, ok := want[p.ID]
			if !ok || seen[p.ID] {
				t.Fatalf("unexpected or duplicate index page %s", p.ID)
			}
			seen[p.ID] = true
			if p.Typed != expected {
				t.Errorf("%s typed=%v, want %v", names[p.ID], p.Typed, expected)
			}
			if p.Typed {
				typed++
			}
		}
		if typed != 3 {
			t.Errorf("typed pages = %d, want the three qualified pages", typed)
		}
		items, err := f.s.IndexItems(ctx, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 8 {
			t.Fatalf("index items = %d, want eight", len(items))
		}
		for _, id := range ids {
			if (items[id].Type != nil) != want[id] {
				t.Errorf("%s item type presence=%v, want %v", names[id], items[id].Type != nil, want[id])
			}
		}
	})
	t.Run("first_key", func(t *testing.T) {
		f := newFixture(t)
		ctx := t.Context()
		first := f.indexPage(t, f.eng, []property{{"TYPE", `["not text"]`}, {"type", jsonText("later")}})
		real := f.indexPage(t, f.eng, []property{{"Type", jsonText("Z")}, {"type", jsonText("later")}})
		pages, err := f.s.IndexPages(ctx, f.eng, nil)
		if err != nil {
			t.Fatal(err)
		}
		typed := map[uuid.UUID]bool{}
		for _, p := range pages {
			typed[p.ID] = p.Typed
		}
		if len(typed) != 2 || typed[first] || !typed[real] {
			t.Fatalf("first-key qualification=%v", typed)
		}
		later := "later"
		filtered, err := f.s.IndexPages(ctx, f.eng, &later)
		if err != nil || len(filtered) != 0 {
			t.Fatalf("later key matched: %v %v", filtered, err)
		}
		ids := []uuid.UUID{first, real}
		slices.SortFunc(ids, uuid.UUID.Compare)
		rows, err := f.s.CatalogueHeads(ctx, f.eng, ids)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == first && (r.Typed || r.FilterType != nil) || r.ID == real && (!r.Typed || r.FilterType == nil || *r.FilterType != "Z") {
				t.Fatalf("first-key head=%+v", r)
			}
		}
	})
	t.Run("catalogue_heads", func(t *testing.T) {
		f := newFixture(t)
		at := strings.Repeat("界", 341) + "a"
		past := at + "b"
		slow := strings.Repeat(" ", 600) + "x"
		values := []string{at, past, slow, "\ufeff ", "\u0085"}
		ids := []uuid.UUID{}
		want := map[uuid.UUID]*string{}
		typed := map[uuid.UUID]bool{}
		for i, v := range values {
			id := f.indexPage(t, f.eng, []property{{"type", jsonText(v)}})
			ids = append(ids, id)
			typed[id] = i != 3
			if i != 1 && i != 3 {
				value := v
				want[id] = &value
			}
		}
		other := f.indexPage(t, f.ops, []property{{"type", jsonText("foreign")}})
		ids = append(ids, other, uuid.NewV7())
		slices.SortFunc(ids, uuid.UUID.Compare)
		for _, bad := range [][]uuid.UUID{{ids[1], ids[0]}, {ids[0], ids[0]}, make([]uuid.UUID, 513)} {
			if _, err := f.s.CatalogueHeads(t.Context(), f.eng, bad); err == nil {
				t.Fatal("invalid catalogue batch accepted")
			}
		}
		rows, err := f.s.CatalogueHeads(t.Context(), f.eng, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(ids) {
			t.Fatalf("heads=%d want=%d", len(rows), len(ids))
		}
		for i, r := range rows {
			v := want[r.ID]
			if r.ID != ids[i] || r.Typed != typed[r.ID] || (v == nil) != (r.FilterType == nil) || v != nil && *v != *r.FilterType {
				t.Fatalf("head %d=%+v expected typed=%v value=%v", i, r, typed[r.ID], v)
			}
		}
	})

}
