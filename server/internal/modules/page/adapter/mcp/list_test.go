package mcpadapter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// listOf is the tools of a list that answers l.
func listOf(l app.Listing) tools {
	return tools{uc: UseCases{List: listFunc(func(uuid.UUID, string, *int, *string) (app.Listing, error) { return l, nil })}}
}

// An item is a page's path, kind, version, size and last change in UTC,
// or an attachment's path, kind "attachment", type and size; next_cursor
// is the listing's, null without one; hint only when set (M9/P1 design
// 3.10.2).
func TestTheListAnswersItsItems(t *testing.T) {
	page, png, pdf := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	at := time.Date(2026, 10, 17, 10, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	l := app.Listing{Items: []app.Listed{
		{Path: "Guide.md", Node: domain.Node{ID: page}, HasChildren: true, Page: &app.PageState{Revision: 3, Bytes: 1204, UpdatedAt: at}, Cursor: "c1"},
		{Path: "Guide/diagram.png", Node: domain.Node{ID: png}, Attachment: &app.AttachmentInfo{MIME: "image/png", Bytes: 34567}, Cursor: "c2"},
		{Path: "Guide/spec.pdf", Node: domain.Node{ID: pdf}, Attachment: &app.AttachmentInfo{MIME: "application/pdf", Bytes: 9}, Cursor: "c3"},
	}, Next: "next", Hint: "a hint"}
	res, err := listOf(l).list(context.Background(), mcpserver.Call{}, listIn{})
	if err != nil {
		t.Fatal(err)
	}
	raw := rawOf(t, res)
	want := `{"items":[{"path":"Guide.md","kind":"page","has_children":true,"version":"` + page.String() +
		`:3","byte_size":1204,"updated_at":"2026-10-17T08:30:00Z"},{"path":"Guide/diagram.png","kind":"attachment",` +
		`"has_children":false,"mime":"image/png","byte_size":34567},{"path":"Guide/spec.pdf","kind":"attachment",` +
		`"has_children":false,"mime":"application/pdf","byte_size":9}],"next_cursor":"next","hint":"a hint"}`
	if raw != want {
		t.Errorf("the answer is\n%s\nwant\n%s", raw, want)
	}
	res, err = listOf(app.Listing{}).list(context.Background(), mcpserver.Call{}, listIn{})
	if raw := rawOf(t, res); err != nil || raw != `{"items":[],"next_cursor":null}` {
		t.Errorf("an empty list = %s, %v", raw, err)
	}
	// The glob's work ran out before any item: no items, a cursor.
	res, err = listOf(app.Listing{Next: "on"}).list(context.Background(), mcpserver.Call{}, listIn{})
	if raw := rawOf(t, res); err != nil || raw != `{"items":[],"next_cursor":"on"}` {
		t.Errorf("a list whose work ran out = %s, %v", raw, err)
	}
}

// An answer holds as many items as MaxAnswerBytes does, as the SDK writes
// them: 1,000 items of the longest paths, all &, which a text block
// writes in six bytes each, answer some, within the bound, next_cursor
// the cursor after the last of them; the whole list fits with the
// listing's next_cursor (M9/P1 design 3.10.5).
func TestTheListFitsItsAnswer(t *testing.T) {
	segment := strings.Repeat("&", 255)
	path := strings.Repeat(segment+"/", 10) + segment + ".md"
	items := make([]app.Listed, 1000)
	for i := range items {
		items[i] = app.Listed{Path: path, Node: domain.Node{ID: uuid.NewV7()}, Page: &app.PageState{Revision: 1}, Cursor: strings.Repeat("c", i+1)}
	}
	res, err := listOf(app.Listing{Items: items, Next: "next"}).list(context.Background(), mcpserver.Call{}, listIn{})
	if err != nil {
		t.Fatal(err)
	}
	a := answerOf(t, res)
	k := len(a["items"].([]any))
	if n := mcpserver.TextBytes(rawOf(t, res)); n > mcpserver.MaxAnswerBytes || k == 0 || k >= 10 {
		t.Errorf("%d items in %d bytes, want a few within %d", k, n, mcpserver.MaxAnswerBytes)
	}
	if a["next_cursor"] != items[k-1].Cursor {
		t.Errorf("next_cursor %v after %d items, want the %dth item's", a["next_cursor"], k, k)
	}
	res, err = listOf(app.Listing{Items: items[:2], Next: "next"}).list(context.Background(), mcpserver.Call{}, listIn{})
	if a := answerOf(t, res); err != nil || len(a["items"].([]any)) != 2 || a["next_cursor"] != "next" {
		t.Errorf("two items = %v, %v; want both and the listing's cursor", a, err)
	}
	res, err = listOf(app.Listing{Items: items[:2]}).list(context.Background(), mcpserver.Call{}, listIn{})
	if a := answerOf(t, res); err != nil || len(a["items"].([]any)) != 2 || a["next_cursor"] != nil {
		t.Errorf("two items without more = %v, %v; want both and a null cursor", a, err)
	}
	// No path's bound lets an item not fit; one that does not is the
	// tool's fault, not an empty page.
	over := []app.Listed{{Path: "a.md", Page: &app.PageState{}, Cursor: strings.Repeat("c", mcpserver.MaxAnswerBytes)}, {Path: "b.md", Page: &app.PageState{}}}
	if _, err := listOf(app.Listing{Items: over}).list(context.Background(), mcpserver.Call{}, listIn{}); !errors.Is(err, errNoItemFits) {
		t.Errorf("an item that does not fit = %v, want errNoItemFits", err)
	}
}
