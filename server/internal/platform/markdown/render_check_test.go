package markdown_test

import (
	"context"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

func render(t *testing.T, m *markdown.Markdown, src []byte) string {
	t.Helper()
	out, err := m.Render(context.Background(), m.Parse(src), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Every fixture renders to HTML that passes the check, and a frontmatter
// leaves the body's HTML as the body alone renders it.
func TestTheFixturesRenderToCheckedHTML(t *testing.T) {
	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range markdowntest.Fixtures(t) {
		t.Run(f.Name, func(t *testing.T) {
			got := render(t, m, f.Content)
			if err := markdowntest.CheckHTML(got); err != nil {
				t.Error(err)
			}
			body, ok := afterFrontmatter(f.Content)
			if !ok {
				return
			}
			alone := render(t, m, body)
			props, found := strings.CutSuffix(got, alone)
			if !found || props != "" && !strings.HasPrefix(props, `<table class="nw-props">`) {
				t.Errorf("with its frontmatter\n%q\nthe body alone\n%q", got, alone)
			}
		})
	}
}

// afterFrontmatter is what follows a frontmatter's closing line, found
// apart from the package's reading of it: after a byte order mark maybe,
// a first line "---", then the next line "---".
func afterFrontmatter(src []byte) ([]byte, bool) {
	s := strings.TrimPrefix(string(src), "\ufeff")
	lines := strings.SplitAfter(s, "\n")
	if lines[0] != "---\n" && lines[0] != "---\r\n" {
		return nil, false
	}
	n := len(lines[0])
	for _, l := range lines[1:] {
		n += len(l)
		if strings.TrimSuffix(strings.TrimSuffix(l, "\n"), "\r") == "---" {
			return []byte(s[n:]), true
		}
	}
	return nil, false
}

// An extension's markup passes the check given its Markup, and only so.
func TestAnExtensionsMarkupPassesTheCheckWithItsMarkup(t *testing.T) {
	words := markdown.Words()
	m, err := markdown.New([]markdown.Extension{words})
	if err != nil {
		t.Fatal(err)
	}
	got := render(t, m, []byte("say @@hello@@\n"))
	if !strings.Contains(got, `<mark class="nw-word"`) {
		t.Fatalf("no word rendered: %q", got)
	}
	if err := markdowntest.CheckHTML(got, words); err != nil {
		t.Errorf("with its Markup: %v", err)
	}
	if err := markdowntest.CheckHTML(got); err == nil {
		t.Errorf("without its Markup, %q passed", got)
	}
}
