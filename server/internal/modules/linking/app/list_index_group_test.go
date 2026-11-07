package app_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
)

// Override only the later facts. The actual use case still sorts its first pass,
// filters, encodes each cursor, and resolves paths through the existing fixture.
type changedIndexFacts struct {
	*indexReads
	later map[uuid.UUID]*string
}

func (r changedIndexFacts) IndexItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]app.IndexFacts, error) {
	out, err := r.indexReads.IndexItems(ctx, ids)
	for id, typ := range r.later {
		if f, ok := out[id]; ok {
			f.Type = typ
			out[id] = f
		}
	}
	return out, err
}

func TestListIndexGroupKeyUsesSortingPass(t *testing.T) {
	l := newIndexLib()
	text := func(value string) *string { return &value }
	ids := []uuid.UUID{
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000701"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000702"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000703"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000704"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000705"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000706"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000707"),
		uuid.MustParse("0199a2b4-0000-7000-8000-000000000708"),
	}
	l.reads.held = ids
	values := []*string{text("A"), text("a"), text("Straße"), text("STRASSE"), text("e\u0301"), text("é"), nil, text(strings.Repeat("A", 80))}
	paths := []string{"Alpha.md", "Bravo.md", "Charlie.md", "Delta.md", "Echo.md", "Foxtrot.md", "Golf.md", "Hotel.md"}
	for i, id := range ids {
		l.paths.add(id, paths[i])
		l.reads.facts[id] = app.IndexFacts{Type: values[i]}
	}
	read := l.listIndex()
	// Non-RR changes: typed -> another display type; typed -> nil; untyped -> typed.
	read.Reads = changedIndexFacts{indexReads: l.reads, later: map[uuid.UUID]*string{ids[0]: text("Zulu"), ids[1]: nil, ids[6]: text("Now typed")}}
	page, err := read.Execute(reader(), l.nb, app.IndexQuery{})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []uuid.UUID{ids[0], ids[1], ids[7], ids[2], ids[3], ids[4], ids[5], ids[6]}
	if !reflect.DeepEqual(idsOf(page), wantOrder) {
		t.Fatalf("ordered ids=%v want %v", idsOf(page), wantOrder)
	}
	wantKeys := []*string{text("a"), text("a"), text(strings.Repeat("a", 64)), text("strasse"), text("strasse"), text("é"), text("é"), nil}
	for i, item := range page.Items {
		if !reflect.DeepEqual(item.TypeKey, wantKeys[i]) {
			t.Fatalf("item %d type key=%v want %v", i, item.TypeKey, wantKeys[i])
		}
	}
	if page.Items[0].Type == nil || *page.Items[0].Type != "Zulu" || page.Items[1].Type != nil || page.Items[7].Type == nil || *page.Items[7].Type != "Now typed" {
		t.Fatalf("later display facts not retained: %+v", page.Items)
	}
	for i, exact := range []string{"A", "a"} {
		filtered, err := read.Execute(reader(), l.nb, app.IndexQuery{Type: &exact})
		if err != nil || !reflect.DeepEqual(idsOf(filtered), []uuid.UUID{ids[i]}) || filtered.Items[0].TypeKey == nil || *filtered.Items[0].TypeKey != "a" {
			t.Fatalf("exact filter %q: page=%+v err=%v", exact, filtered, err)
		}
	}
}
