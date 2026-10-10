package markdownadapter_test

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

func newAdapter(t *testing.T) *markdownadapter.Markdown {
	t.Helper()
	md, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return markdownadapter.New(md)
}

// The adapter renders a content for the page, and its view expires when
// the Markdown's does (M7/P3 design 5.7).
func TestTheAdapterRendersAContent(t *testing.T) {
	m := newAdapter(t)
	got, err := m.Render(context.Background(), "# Hello *world*", app.PageRef{})
	if want := (app.Rendered{HTML: "<h1 id=\"nw-hello-world\">Hello <em>world</em></h1>\n"}); err != nil || got != want {
		t.Errorf("Render = %+v, %v; want %+v", got, err, want)
	}
	at := time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)
	page := app.PageRef{NotebookID: uuid.NewV7(), PageID: uuid.NewV7(), Revision: 3}
	var fetched markdown.Page
	md, err := markdown.New([]markdown.Extension{{
		Name: "expiring",
		Fetch: func(_ context.Context, p markdown.Page, _ any) (any, error) {
			fetched = p
			return nil, nil
		},
		Expires: func(any) time.Time { return at },
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err = markdownadapter.New(md).Render(context.Background(), "a", page)
	want := markdown.Page{NotebookID: page.NotebookID, PageID: page.PageID, Revision: 3}
	if err != nil || got.HTML != "<p>a</p>\n" || !got.Expires.Equal(at) || fetched != want {
		t.Errorf("Render = %+v, %v, for %+v", got, err, fetched)
	}
}

// Tasks are what the tasks extension took, in order; a Markdown without the
// extension, or facts of elsewhere, have none.
func TestTheAdapterGivesTheTasksOfTheFacts(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{tasks.Extension()})
	if err != nil {
		t.Fatal(err)
	}
	m := markdownadapter.New(md)
	want := []app.Task{{Offset: 3, Checked: false}, {Offset: 12, Checked: true}}
	if got := m.Tasks(m.Facts("- [ ] a\r\n- [X] b\n")); !slices.Equal(got, want) {
		t.Errorf("Tasks = %v, want %v", got, want)
	}
	plain := newAdapter(t)
	for name, got := range map[string][]app.Task{
		"without the extension": plain.Tasks(plain.Facts("- [ ] a\n")),
		"facts of elsewhere":    m.Tasks("- [ ] a\n"),
		"no facts":              m.Tasks(markdown.Facts{}),
		"nothing":               m.Tasks(nil),
	} {
		if len(got) != 0 {
			t.Errorf("%s: Tasks = %v, want none", name, got)
		}
	}
}

// Links counts the links the obsidian extension took of the content; a
// Markdown without the extension, or facts of elsewhere, have none.
func TestTheAdapterCountsTheLinksOfTheFacts(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{obsidian.Extension(obsidian.Options{})})
	if err != nil {
		t.Fatal(err)
	}
	m := markdownadapter.New(md)
	if got := m.Links(m.Facts("[[a]] and ![[b.png]], [c](c.md) and ![d](d.png); `[[not]]`\n")); got != 4 {
		t.Errorf("Links = %d, want 4", got)
	}
	plain := newAdapter(t)
	for name, got := range map[string]int{
		"without the extension": plain.Links(plain.Facts("[[a]]\n")),
		"facts of elsewhere":    m.Links("[[a]]\n"),
		"no facts":              m.Links(markdown.Facts{}),
		"nothing":               m.Links(nil),
	} {
		if got != 0 {
			t.Errorf("%s: Links = %d, want 0", name, got)
		}
	}
}
