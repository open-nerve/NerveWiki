// Package markdownadapter connects platform/markdown to the page module's
// app.Markdown (M4/P3 design 3.9): a Parsed is the platform's Document,
// which only Render looks into.
package markdownadapter

import (
	"context"
	"fmt"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
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
