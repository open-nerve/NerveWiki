package obsidian_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// The fuzz tests with the dialect (M6/P1 design 5): a plain go test runs
// their seeds; the mutations run by hand:
//
//	cd server && go test -run '^$' -fuzz '^FuzzRender$' -fuzztime 10m ./internal/platform/markdown/obsidian

func seeds(f *testing.F) {
	for _, fx := range markdowntest.Fixtures(f) {
		f.Add(fx.Content)
	}
	for _, in := range append(markdowntest.Pathological(), markdowntest.Amplifying()...) {
		f.Add([]byte(in.Make(1 << 10)))
	}
	f.Add([]byte(markdowntest.Normal(4 << 10)))
	// Tags as links (M6/P6 design 3): in a link's text, in a user's link.
	f.Add([]byte("[see #t and [[P]]](https://x.example) <a href=\"/x\">#u *#v*</a> #a/ #1/ #/\n"))
}

// Any bytes parse, and each link's and tag's range is in the content: a
// wikilink's writes its target, a body's Markdown link's writes what
// decodes to its target.
func FuzzParse(f *testing.F) {
	seeds(f)
	m := newMarkdown(f)
	f.Fuzz(func(t *testing.T, content []byte) {
		got := extracted(m, string(content))
		for _, l := range got.Links {
			r := l.Range
			if r.Start < 0 || r.Stop > len(content) || r.Start >= r.Stop {
				t.Fatalf("%q: link %+v out of the content", content, l)
			}
			written := string(content[r.Start:r.Stop])
			switch {
			case l.Key != "":
			case (l.Kind == obsidian.KindWikilink || l.Kind == obsidian.KindEmbed) && written != l.Target:
				t.Errorf("%q: link %+v writes %q", content, l, written)
			case (l.Kind == obsidian.KindLink || l.Kind == obsidian.KindImage) && markdown.DecodeURI(written) != l.Target:
				t.Errorf("%q: link %+v writes %q", content, l, written)
			}
		}
		for _, tg := range got.Tags {
			if r := tg.Range; r.Start < 0 || r.Stop > len(content) || string(content[r.Start:r.Stop]) != "#"+tg.Name {
				t.Errorf("%q: tag %+v", content, tg)
			}
		}
	})
}

// Any bytes render to HTML that passes the checks with the extensions'
// markup, every link resolved or none.
func FuzzRender(f *testing.F) {
	seeds(f)
	all, none := newMarkdownWith(f, obsidian.Options{Resolve: resolveAll}), newMarkdownWith(f, obsidian.Options{})
	f.Fuzz(func(t *testing.T, content []byte) {
		for _, m := range []*markdown.Markdown{all, none} {
			out, err := m.Render(context.Background(), m.Parse(content), markdown.Page{})
			if err != nil {
				t.Fatal(err)
			}
			if err := markdowntest.CheckHTML(out, tasks.Extension(), obsidian.Extension(obsidian.Options{})); err != nil {
				t.Errorf("%q\nrenders to\n%q:\n%v", content, out, err)
			}
			if err := markdowntest.CheckSize(content, out); err != nil {
				t.Errorf("%q: %v", content, err)
			}
		}
	})
}
