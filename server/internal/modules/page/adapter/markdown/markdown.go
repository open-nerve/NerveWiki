// Package markdownadapter connects platform/markdown to the page module's
// app.Markdown (M4/P3 design 3.9) and app.ParseBudget: app.Facts are the
// platform's Facts, which only Tasks looks into (M6 design 4.7).
package markdownadapter

import (
	"context"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// Markdown implements app.Markdown.
type Markdown struct {
	md *markdown.Markdown
}

// New returns the adapter of md, the composition root's one instance.
func New(md *markdown.Markdown) *Markdown {
	return &Markdown{md: md}
}

// Facts implements app.Markdown: the parse's tree is garbage once they are
// taken.
func (m *Markdown) Facts(content string) app.Facts {
	return m.md.Parse([]byte(content)).Facts()
}

// Render implements app.Markdown.
func (m *Markdown) Render(ctx context.Context, content string, page app.PageRef) (app.Rendered, error) {
	v, err := m.md.Render(ctx, m.md.Parse([]byte(content)),
		markdown.Page{NotebookID: page.NotebookID, PageID: page.PageID, Revision: page.Revision})
	return app.Rendered{HTML: v.HTML, Expires: v.Expires}, err
}

// Links implements app.Markdown: the links the obsidian extension took,
// the content's; those of its frontmatter's properties, which the YAML's
// limits bound, are not counted. A Markdown without the extension, or
// facts that Facts did not return, have none.
func (m *Markdown) Links(facts app.Facts) int {
	f, ok := facts.(markdown.Facts)
	if !ok {
		return 0
	}
	x, _ := f.Extracted(obsidian.Name).(obsidian.Extracted)
	return len(x.Links)
}

// Tasks implements app.Markdown: what the tasks extension took. A Markdown
// without the extension, or facts that Facts did not return, have none.
func (m *Markdown) Tasks(facts app.Facts) []app.Task {
	f, ok := facts.(markdown.Facts)
	if !ok {
		return nil
	}
	found, _ := f.Extracted(tasks.Name).([]tasks.Task)
	out := make([]app.Task, len(found))
	for i, t := range found {
		out[i] = app.Task{Offset: t.Offset, Checked: t.Checked}
	}
	return out
}
