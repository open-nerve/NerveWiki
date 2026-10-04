// Package markdownadapter connects platform/markdown to the page module's
// app.Markdown (M4/P3 design 3.9): a Parsed is the platform's Document,
// which only Render and Tasks look into.
package markdownadapter

import (
	"context"
	"fmt"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
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

// Parse implements app.Markdown.
func (m *Markdown) Parse(content string) app.Parsed {
	return m.md.Parse([]byte(content))
}

// Render implements app.Markdown. A Parsed that Parse did not return is an
// error.
func (m *Markdown) Render(ctx context.Context, parsed app.Parsed, page app.PageRef) (string, error) {
	d, ok := parsed.(*markdown.Document)
	if !ok || d == nil {
		return "", fmt.Errorf("render: %T is not a parse of the adapter", parsed)
	}
	return m.md.Render(ctx, d, markdown.Page{NotebookID: page.NotebookID, PageID: page.PageID})
}

// Tasks implements app.Markdown: what the tasks extension took from the
// Document. A Markdown without the extension, or a Parsed that Parse did
// not return, has none.
func (m *Markdown) Tasks(parsed app.Parsed) []app.Task {
	d, ok := parsed.(*markdown.Document)
	if !ok || d == nil {
		return nil
	}
	found, _ := d.Extracted(tasks.Name).([]tasks.Task)
	out := make([]app.Task, len(found))
	for i, t := range found {
		out[i] = app.Task{Offset: t.Offset, Checked: t.Checked}
	}
	return out
}
