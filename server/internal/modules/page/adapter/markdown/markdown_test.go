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

func TestTheAdapterRendersAContent(t *testing.T) {
	m := newAdapter(t)
	got, err := m.Render(context.Background(), "# Hello *world*", app.PageRef{})
	if want := "<h1 id=\"nw-hello-world\">Hello <em>world</em></h1>\n"; err != nil || got != want {
		t.Errorf("Render = %q, %v; want %q", got, err, want)
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
