package markdown_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// The fuzz tests (M4/P3 design 3.10): a plain go test runs their seeds, the
// fixtures and the generators' inputs; the mutations run by hand:
//
//	cd server && go test -run '^$' -fuzz '^FuzzRender$' -fuzztime 10m ./internal/platform/markdown

func seeds(f *testing.F) {
	for _, fx := range markdowntest.Fixtures(f) {
		f.Add(fx.Content)
	}
	for _, in := range markdowntest.Pathological() {
		f.Add([]byte(in.Make(1 << 10)))
	}
	f.Add([]byte(markdowntest.Normal(4 << 10)))
}

// Any bytes parse: the parser's text is as long as the content, and the
// frontmatter is where rule 1 puts it.
func FuzzParse(f *testing.F) {
	seeds(f)
	m := newMarkdown(f)
	f.Fuzz(func(t *testing.T, content []byte) {
		d := m.Parse(content)
		if got := len(markdown.SourceOf(d)); got != len(content) {
			t.Errorf("the parser's text is %d bytes, the content %d", got, len(content))
		}
		if _, ok := afterFrontmatter(content); d.Frontmatter().Present != ok {
			t.Errorf("a frontmatter %v, rule 1 says %v", d.Frontmatter().Present, ok)
		}
	})
}

// Any bytes render to HTML that passes the check.
func FuzzRender(f *testing.F) {
	seeds(f)
	m := newMarkdown(f)
	f.Fuzz(func(t *testing.T, content []byte) {
		out, err := m.Render(context.Background(), m.Parse(content), markdown.Page{})
		if err != nil {
			t.Fatal(err)
		}
		if err := markdowntest.CheckHTML(out); err != nil {
			t.Errorf("%q\nrenders to\n%q:\n%v", content, out, err)
		}
	})
}
