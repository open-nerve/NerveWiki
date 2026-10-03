package markdowntest_test

import (
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

func TestCheckHTMLPassesWhatTheRenderersWrite(t *testing.T) {
	for _, s := range []string{
		`<h2 id="nw-a">a</h2><p><a href="/p" title="t">x</a> <a href="https://x.example">y</a> <a href="mailto:a@b">z</a></p>`,
		`<p><a href="../a.md#b">a</a> <a href="?a">b</a> <a href="#nw-fn:1">c</a></p>`,
		`<pre><code class="language-c++">x</code></pre>`,
		`<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup>`,
		`<table class="nw-props"><tr><th>a</th><td><ul><li>1</li></ul></td></tr></table>`,
		`<table><thead><tr><th align="left">a</th></tr></thead><tbody><tr><td align="right">1</td></tr></tbody></table>`,
		`<p><span class="nw-image">a <a href="i.png">i.png</a></span></p>`,
		`<details open><summary>s</summary><ol start="3" reversed><li>a</li></ol></details>`,
		strings.Repeat("<b>", 600) + "deep" + strings.Repeat("</b>", 600),
		`<p>a<br>b<br/>c<wbr>d</p><hr><ul><li>e</li></ul>`,
	} {
		if err := markdowntest.CheckHTML(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
}

func TestCheckHTMLReportsWhatNoRendererWrites(t *testing.T) {
	for name, s := range map[string]string{
		"an event attribute":           `<p onclick="x()">a</p>`,
		"a style":                      `<span style="color: red">a</span>`,
		"another site":                 `<a href="//evil.example">a</a>`,
		"another site, backslashed":    `<a href="/\evil.example">a</a>`,
		"another site, three slashes":  `<a href="///evil.example">a</a>`,
		"a script address":             `<a href="javascript:alert(1)">a</a>`,
		"a script address with a tab":  `<a href="java&#9;script:alert(1)">a</a>`,
		"a data address":               `<a href="data:text/html,x">a</a>`,
		"https without a host":         `<a href="https:evil.example">a</a>`,
		"an id without the prefix":     `<h2 id="a">a</h2>`,
		"an id where none goes":        `<p id="nw-a">a</p>`,
		"a class of no renderer":       `<span class="x">a</span>`,
		"a language off code":          `<span class="language-go">a</span>`,
		"an image":                     `<img src="i.png">`,
		"a script":                     `<script>x</script>`,
		"a comment":                    `<!-- x -->`,
		"a form":                       `<form><input type="text"></form>`,
		"a checkbox, the tasks' alone": `<ul><li><input disabled="" type="checkbox"> a</li></ul>`,
		"an end tag of no renderer":    `a</script>`,
		"an end tag closing another":   `<p><b>a</p></b>`,
		"an element left open":         `<p><b>a</b>`,
		"an end tag with none open":    `<p>a</p></b>`,
		"a void element's end tag":     `<p>a<br></br></p>`,
	} {
		if err := markdowntest.CheckHTML(s); err == nil {
			t.Errorf("%s: %s passed", name, s)
		}
	}
}

func TestCheckHTMLTakesAnExtensionsMarkup(t *testing.T) {
	ext := markdown.Extension{Name: "w", Markup: markdown.Markup{
		Elements: map[string][]string{"mark": {"class", "data-to"}},
		URLs:     []string{"data-to"},
		Classes:  []string{"nw-word"},
	}}
	ok := `<mark class="nw-word" data-to="/p">a</mark>`
	if err := markdowntest.CheckHTML(ok, ext); err != nil {
		t.Errorf("with its Markup: %v", err)
	}
	if err := markdowntest.CheckHTML(ok); err == nil {
		t.Errorf("without its Markup, %s passed", ok)
	}
	img := markdown.Extension{Name: "i", Markup: markdown.Markup{Elements: map[string][]string{"img": {"src"}}, URLs: []string{"src"}}}
	if err := markdowntest.CheckHTML(`<p>a <img src="/i.png"> b</p>`, img); err != nil {
		t.Errorf("an extension's void element: %v", err)
	}
	for _, s := range []string{
		`<mark class="nw-word" data-to="//evil.example">a</mark>`,
		`<mark class="nw-word" data-to="/p" data-x="1">a</mark>`,
		`<mark class="nw-other">a</mark>`,
	} {
		if err := markdowntest.CheckHTML(s, ext); err == nil {
			t.Errorf("%s passed", s)
		}
	}
}

func TestCheckSizeTakesAmplificationTimesTheContentPlusHeadroom(t *testing.T) {
	content := []byte("abc")
	limit := markdowntest.Amplification*len(content) + markdowntest.Headroom
	if err := markdowntest.CheckSize(content, strings.Repeat("x", limit)); err != nil {
		t.Errorf("at the limit: %v", err)
	}
	if err := markdowntest.CheckSize(content, strings.Repeat("x", limit+1)); err == nil {
		t.Error("a byte past the limit passed")
	}
}
