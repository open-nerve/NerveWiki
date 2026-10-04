package markdownadapter_test

import (
	"context"
	"slices"
	"testing"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
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

func TestTheAdapterRendersWhatItParsed(t *testing.T) {
	m := newAdapter(t)
	got, err := m.Render(context.Background(), m.Parse("# Hello *world*"), app.PageRef{})
	if want := "<h1 id=\"nw-hello-world\">Hello <em>world</em></h1>\n"; err != nil || got != want {
		t.Errorf("Render = %q, %v; want %q", got, err, want)
	}
}

func TestTheAdapterRefusesAParseOfElsewhere(t *testing.T) {
	m := newAdapter(t)
	for _, parsed := range []app.Parsed{"# Hello", nil, (*markdown.Document)(nil)} {
		if _, err := m.Render(context.Background(), parsed, app.PageRef{}); err == nil {
			t.Errorf("Render(%#v) rendered", parsed)
		}
	}
}

// Tasks are what the tasks extension took, in order; a Markdown without the
// extension, or a parse of elsewhere, has none.
func TestTheAdapterGivesTheTasksItParsed(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{tasks.Extension()})
	if err != nil {
		t.Fatal(err)
	}
	m := markdownadapter.New(md)
	want := []app.Task{{Offset: 3, Checked: false}, {Offset: 12, Checked: true}}
	if got := m.Tasks(m.Parse("- [ ] a\r\n- [X] b\n")); !slices.Equal(got, want) {
		t.Errorf("Tasks = %v, want %v", got, want)
	}
	plain := newAdapter(t)
	for name, got := range map[string][]app.Task{
		"without the extension": plain.Tasks(plain.Parse("- [ ] a\n")),
		"a parse of elsewhere":  m.Tasks("- [ ] a\n"),
		"no parse":              m.Tasks((*markdown.Document)(nil)),
	} {
		if len(got) != 0 {
			t.Errorf("%s: Tasks = %v, want none", name, got)
		}
	}
}
