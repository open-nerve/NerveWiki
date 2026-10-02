package bootstrap

import (
	"context"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// appMarkdown is the Markdown serve builds, with the extensions it
// registers.
func appMarkdown(t *testing.T) (*markdown.Markdown, []markdown.Extension) {
	t.Helper()
	exts := markdownExtensions()
	md, err := markdown.New(exts)
	if err != nil {
		t.Fatal(err)
	}
	return md, exts
}

// The application's Markdown renders every fixture and every generated
// input to HTML that passes the check with its extensions' markup (M4/P3
// design 3.10).
func TestTheAppsMarkdownRendersCheckedHTML(t *testing.T) {
	md, exts := appMarkdown(t)
	inputs := map[string]string{"ordinary": markdowntest.Normal(64 << 10)}
	for _, f := range markdowntest.Fixtures(t) {
		inputs[f.Name] = string(f.Content)
	}
	for _, in := range markdowntest.Pathological() {
		inputs[in.Name] = in.Make(16 << 10)
	}
	page := markdown.Page{NotebookID: uuid.NewV7(), PageID: uuid.NewV7()}
	for name, content := range inputs {
		html, err := md.Render(context.Background(), md.Parse([]byte(content)), page)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := markdowntest.CheckHTML(html, exts...); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The application's Markdown costs about the size of what it parses, with
// its extensions' parsers: each must be linear too (M4/P3 design 3.4).
func TestTheAppsMarkdownCostsAboutItsSize(t *testing.T) {
	md, _ := appMarkdown(t)
	markdowntest.CheckCosts(t, md)
}
