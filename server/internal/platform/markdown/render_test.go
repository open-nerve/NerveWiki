package markdown

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"uuid"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

func renderString(t *testing.T, m *Markdown, src string) string {
	t.Helper()
	out, err := m.Render(context.Background(), m.Parse([]byte(src)), Page{})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type renderCase struct{ name, src, want string }

func checkRenders(t *testing.T, tests []renderCase) {
	t.Helper()
	m := newMarkdown(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renderString(t, m, tt.src); got != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.src, got, tt.want)
			}
		})
	}
}

// The renderer's marks (M4/P3 design 3.6).
func TestTheMarksAreTheReadingViews(t *testing.T) {
	checkRenders(t, []renderCase{
		{"a heading's id", "# Hello *World*\n", "<h1 id=\"nw-hello-world\">Hello <em>World</em></h1>\n"},
		{
			"footnotes, one referred to twice", "a[^x] b[^x] c[^y]\n\n[^x]: one\n[^y]: two\n",
			`<p>a<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup>` +
				` b<sup id="nw-fnref1:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup>` +
				` c<sup id="nw-fnref:2"><a href="#nw-fn:2" class="footnote-ref" role="doc-noteref">2</a></sup></p>` + "\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n" +
				`<p>one&#160;<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a>` +
				`&#160;<a href="#nw-fnref1:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` +
				"\n</li>\n" + `<li id="nw-fn:2">` + "\n" +
				`<p>two&#160;<a href="#nw-fnref:2" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` +
				"\n</li>\n</ol>\n</div>\n",
		},
		// Task items are the tasks extension's (its own tests render them).
		{"task items without their extension", "- [ ] a\n- [x] b\n", "<ul>\n<li>[ ] a</li>\n<li>[x] b</li>\n</ul>\n"},
		{
			"a table's alignment", "| a | b | c | d |\n|:--|:-:|--:|---|\n| 1 | 2 | 3 | 4 |\n",
			"<table>\n<thead>\n<tr>\n<th align=\"left\">a</th>\n<th align=\"center\">b</th>\n" +
				"<th align=\"right\">c</th>\n<th>d</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td align=\"left\">1</td>\n" +
				"<td align=\"center\">2</td>\n<td align=\"right\">3</td>\n<td>4</td>\n</tr>\n</tbody>\n</table>\n",
		},
		{"code in go", "```go\na < b\n```\n", "<pre><code class=\"language-go\">a &lt; b\n</code></pre>\n"},
		{"code in C++", "```C++\nx\n```\n", "<pre><code class=\"language-c++\">x\n</code></pre>\n"},
		{"code with more info", "```  Go  extra\nx\n```\n", "<pre><code class=\"language-go\">x\n</code></pre>\n"},
		{"code with a tag", "```<script>\nx\n```\n", "<pre><code>x\n</code></pre>\n"},
		{"code with a quote", "```a\"b\nx\n```\n", "<pre><code>x\n</code></pre>\n"},
		{"code with a long name", "```" + strings.Repeat("a", 33) + "\nx\n```\n", "<pre><code>x\n</code></pre>\n"},
		{"code without info", "```\nx\n```\n", "<pre><code>x\n</code></pre>\n"},
		{"indented code", "    x\n", "<pre><code>x\n</code></pre>\n"},
		{"indented code after a byte order mark", "\ufeff    x\n", "<pre><code>x\n</code></pre>\n"},
	})
}

func TestLinksAndImagesGoThroughSafeURL(t *testing.T) {
	checkRenders(t, []renderCase{
		{"a link with a title", `[a](/p "t")`, "<p><a href=\"/p\" title=\"t\">a</a></p>\n"},
		{"a link's query", "[a](https://x.example/?a=1&b=2)", "<p><a href=\"https://x.example/?a=1&amp;b=2\">a</a></p>\n"},
		{"a link to another page", "[a](../b/c.md#d)", "<p><a href=\"../b/c.md#d\">a</a></p>\n"},
		{"a script link", "[a *b*](javascript:alert(1))", "<p>a <em>b</em></p>\n"},
		{"a script link in entities", "[a](&#106;avascript:alert(1))", "<p>a</p>\n"},
		{"a script link in escapes", `[a](java\script:alert(1))`, "<p>a</p>\n"},
		{"a link to another host", "[a](<//evil.example>)", "<p>a</p>\n"},
		{"a backslash escaped to a path", `[a](/\evil.example)`, "<p><a href=\"/%5Cevil.example\">a</a></p>\n"},
		{"a data link", "[a](data:text/html,x)", "<p>a</p>\n"},
		{"an autolink", "<https://x.example/a>", "<p><a href=\"https://x.example/a\">https://x.example/a</a></p>\n"},
		{"a script autolink", "<javascript:alert(1)>", "<p>javascript:alert(1)</p>\n"},
		{"an e-mail autolink", "<a@b.example>", "<p><a href=\"mailto:a@b.example\">a@b.example</a></p>\n"},
		{"a bare e-mail", "a@b.example", "<p><a href=\"mailto:a@b.example\">a@b.example</a></p>\n"},
		{"a www link", "www.x.example", "<p><a href=\"http://www.x.example\">www.x.example</a></p>\n"},
		{
			"an image elsewhere", "![a *b*](https://x.example/i.png)",
			"<p><span class=\"nw-image\">a b <a href=\"https://x.example/i.png\">https://x.example/i.png</a></span></p>\n",
		},
		{"an image here", "![a](i.png)", "<p><span class=\"nw-image\">a <a href=\"i.png\">i.png</a></span></p>\n"},
		{"an image without text", "![](i.png)", "<p><span class=\"nw-image\"><a href=\"i.png\">i.png</a></span></p>\n"},
		{"a data image", "![a](data:image/png;base64,AA)", "<p><span class=\"nw-image\">a</span></p>\n"},
		{"an image in a link", "[![a](i.png)](/p)", "<p><a href=\"/p\"><span class=\"nw-image\">a</span></a></p>\n"},
		{
			"an image after a link", "[x](/p) ![a](i.png)",
			"<p><a href=\"/p\">x</a> <span class=\"nw-image\">a <a href=\"i.png\">i.png</a></span></p>\n",
		},
		{
			"a link's title with quotes", `[a](/p "x\" onclick=\"y" ) [b](/q 'a < b & c')`,
			"<p><a href=\"/p\" title=\"x&quot; onclick=&quot;y\">a</a> <a href=\"/q\" title=\"a &lt; b &amp; c\">b</a></p>\n",
		},
		{
			"an image's text escaped", `![a < b & "c" <b>](i.png)`,
			"<p><span class=\"nw-image\">a &lt; b &amp; &quot;c&quot;  <a href=\"i.png\">i.png</a></span></p>\n",
		},
	})
}

func TestThePropertiesComeFirstAsATable(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"each kind of value",
			"---\ns: a<b\nn: 010\nf: 1.5\nbig: 1e21\nb: true\nz: ~\nl: [a, 1]\nm: {k: v}\nq: \"x\"\n---\nbody\n",
			`<table class="nw-props"><tr><th>s</th><td>a&lt;b</td></tr><tr><th>n</th><td>10</td></tr>` +
				`<tr><th>f</th><td>1.5</td></tr><tr><th>big</th><td>1e+21</td></tr><tr><th>b</th><td>true</td></tr>` +
				`<tr><th>z</th><td></td></tr><tr><th>l</th><td><ul><li>a</li><li>1</li></ul></td></tr>` +
				`<tr><th>m</th><td><table class="nw-props"><tr><th>k</th><td>v</td></tr></table></td></tr>` +
				`<tr><th>q</th><td>x</td></tr></table>` + "\n<p>body</p>\n",
		},
		{"a key escaped", "---\n\"<k>\": 1\n---\n", "<table class=\"nw-props\"><tr><th>&lt;k&gt;</th><td>1</td></tr></table>\n"},
		{"a large number in decimal", "---\nn: 1e20\n---\n", "<table class=\"nw-props\"><tr><th>n</th><td>100000000000000000000</td></tr></table>\n"},
		{"zero, negative or not", "---\nn: 0.0\nm: -0.0\n---\n", "<table class=\"nw-props\"><tr><th>n</th><td>0</td></tr><tr><th>m</th><td>0</td></tr></table>\n"},
		{"a small number with an exponent", "---\nn: 1.5e-7\n---\n", "<table class=\"nw-props\"><tr><th>n</th><td>1.5e-7</td></tr></table>\n"},
		{"an empty frontmatter", "---\n---\nbody\n", "<p>body</p>\n"},
		{"a frontmatter not valid", "---\n- a\n---\nbody\n", "<p>body</p>\n"},
		{"no frontmatter", "body\n", "<p>body</p>\n"},
	})
}

// wordsRendered is words() rendering what fetch got: each word in a mark
// that carries it.
func wordsRendered(fetch func(ctx context.Context, page Page, extracted any) (any, error)) Extension {
	e := words()
	e.Fetch = fetch
	e.Renderer = func(data any) []util.PrioritizedValue {
		return []util.PrioritizedValue{util.Prioritized(wordRenderer{fmt.Sprint(data)}, 100)}
	}
	e.Markup = Markup{Elements: map[string][]string{"mark": {"class", "data-fetched"}}, Classes: []string{"nw-word"}}
	return e
}

type wordRenderer struct{ fetched string }

func (r wordRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWord, func(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = fmt.Fprintf(w, `<mark class="nw-word" data-fetched="%s">%s</mark>`,
				html.EscapeString(r.fetched), html.EscapeString(string(n.(*wordNode).word)))
		}
		return ast.WalkContinue, nil
	})
}

// fetchFor fetches the page's id and the words taken from it.
func fetchFor(_ context.Context, page Page, extracted any) (any, error) {
	return page.PageID.String() + ":" + strings.Join(extracted.([]string), ","), nil
}

// An extension's Fetch gets the page and what it took from the page; its
// renderer gets what Fetch got.
func TestAnExtensionRendersWhatItFetchedForThePage(t *testing.T) {
	m := newMarkdown(t, wordsRendered(fetchFor))
	page := Page{NotebookID: uuid.New(), PageID: uuid.New()}
	got, err := m.Render(context.Background(), m.Parse([]byte("say @@hello@@ and @@world@@\n")), page)
	if err != nil {
		t.Fatal(err)
	}
	fetched := page.PageID.String() + ":hello,world"
	want := `<p>say <mark class="nw-word" data-fetched="` + fetched + `">hello</mark> and ` +
		`<mark class="nw-word" data-fetched="` + fetched + `">world</mark></p>` + "\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestRenderReturnsFetchsError(t *testing.T) {
	down := errors.New("down")
	m := newMarkdown(t, wordsRendered(func(context.Context, Page, any) (any, error) { return nil, down }))
	if _, err := m.Render(context.Background(), m.Parse([]byte("@@a@@")), Page{}); !errors.Is(err, down) {
		t.Errorf("Render: %v, want %v", err, down)
	}
}
