package harden

import (
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// marking renders a highlight as <mark>, for the tests.
type marking struct{}

func (marking) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindHighlight, func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString("<mark>")
		} else {
			_, _ = w.WriteString("</mark>")
		}
		return ast.WalkContinue, nil
	})
}

func highlighting() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParser(NewParser(parser.WithInlineParsers(HighlightRuns()))),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
				util.Prioritized(marking{}, 500),
			),
		),
	)
}

// A parser with highlights pairs runs of two '=' as it pairs '~~', and an
// autolink does not start right after one (M6/P1 design 3.3).
func TestTwoEqualsSignsHighlight(t *testing.T) {
	h := highlighting()
	for _, tt := range []struct{ name, in, want string }{
		{"a highlight", "==a==", "<p><mark>a</mark></p>\n"},
		{"in a word", "a==b==c", "<p>a<mark>b</mark>c</p>\n"},
		{"around emphasis", "==**b**==", "<p><mark><strong>b</strong></mark></p>\n"},
		{"in a strikethrough", "~~a ==b== c~~", "<p><del>a <mark>b</mark> c</del></p>\n"},
		{"crossing emphasis, the first closed wins", "**a ==b** c==", "<p><strong>a ==b</strong> c==</p>\n"},
		{"three are text", "===a===", "<p>===a===</p>\n"},
		{"one is text", "=a=", "<p>=a=</p>\n"},
		{"a space inside does not close", "==a ==", "<p>==a ==</p>\n"},
		{"over a line", "==a\nb==", "<p><mark>a\nb</mark></p>\n"},
		{"in code, text", "`==a==`", "<p><code>==a==</code></p>\n"},
		{"no autolink right after one", "a==www.example.com==", "<p>a<mark>www.example.com</mark></p>\n"},
		{"nor after one left alone", "==www.example.com", "<p>==www.example.com</p>\n"},
		{"after a space, an autolink", "== www.example.com", `<p>== <a href="http://www.example.com">www.example.com</a></p>` + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(t, h, tt.in); got != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

// Without HighlightRuns, '=' is text and autolinks are goldmark's: the
// random inputs, '=' among their pieces, check it; this checks the guard's
// one place by name.
func TestWithoutHighlightsAnAutolinkAfterEqualsIsGoldmarks(t *testing.T) {
	h, o := hardened(), original()
	for _, in := range []string{"a==www.example.com", "= www.example.com", "x==http://example.com y", "==www.a.b=="} {
		if got, want := render(t, h, in), render(t, o, in); got != want {
			t.Errorf("input %q\nhardened %q\noriginal %q", in, got, want)
		}
	}
}
