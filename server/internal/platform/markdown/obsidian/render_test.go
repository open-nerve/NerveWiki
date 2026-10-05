package obsidian_test

import (
	"context"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

type renderCase struct{ name, src, want string }

// checkRenders checks each case's HTML, and that it passes the check of a
// reading view's HTML with the extensions' markup.
func checkRenders(t *testing.T, tests []renderCase) {
	t.Helper()
	m := newMarkdown(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Render(context.Background(), m.Parse([]byte(tt.src)), markdown.Page{})
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.src, got, tt.want)
			}
			if err := markdowntest.CheckHTML(got, tasks.Extension(), obsidian.Extension(obsidian.Options{})); err != nil {
				t.Error(err)
			}
		})
	}
}

// Wikilinks and embeds are links, here to no page; a tag is a span; what
// they show is escaped, as their attributes are (M6/P1 design 3.10, M6/P3
// design 6.2).
func TestWikilinksAreLinksAndTagsSpans(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"a wikilink, its display text, its anchor, an anchor alone", "[[a]] [[b|显示]] [[c#h]] [[#h]]\n",
			`<p><a class="nw-wikilink nw-unresolved" data-nw-target="a">a</a> ` +
				`<a class="nw-wikilink nw-unresolved" data-nw-target="b">显示</a> ` +
				`<a class="nw-wikilink nw-unresolved" data-nw-target="c">c &gt; h</a> <a class="nw-wikilink" href="#nw-h">h</a></p>` + "\n",
		},
		{
			"an embed shows its target, not its size", "![[d.png|100]]\n",
			`<p><a class="nw-wikilink nw-embed nw-unresolved" data-nw-target="d.png">d.png</a></p>` + "\n",
		},
		{
			"escaped", "[[a <b>&\"x]] [[y|<i>\\_]]\n",
			`<p><a class="nw-wikilink nw-unresolved" data-nw-target="a &lt;b&gt;&amp;&quot;x">a &lt;b&gt;&amp;&quot;x</a> ` +
				`<a class="nw-wikilink nw-unresolved" data-nw-target="y">&lt;i&gt;\_</a></p>` + "\n",
		},
		{"no target and no anchor is text", "[[]] [[ | x]] ![[ ]]\n", "<p>[[]] [[ | x]] ![[ ]]</p>\n"},
		{
			"a tag", "#tag and *#t1* x#no\n",
			`<p><span class="nw-tag" data-nw-tag="tag">#tag</span> and <em><span class="nw-tag" data-nw-tag="t1">#t1</span></em> x#no</p>` + "\n",
		},
		{
			"a heading's id has what its wikilinks and tags show", "# Head [[x|y]] #t\n",
			`<h1 id="nw-head-y-t">Head <a class="nw-wikilink nw-unresolved" data-nw-target="x">y</a> ` +
				`<span class="nw-tag" data-nw-tag="t">#t</span></h1>` + "\n",
		},
		{
			"an image's text has what they show", "![alt [[w]] #t](p.png)\n",
			`<p><span class="nw-image">alt w #t <a class="nw-unresolved" data-nw-target="p.png">p.png</a></span></p>` + "\n",
		},
		{
			"in a link's text", "[see [[x]]](z.md)\n",
			`<p><a class="nw-unresolved" data-nw-target="z.md">see <span class="nw-wikilink">x</span></a></p>` + "\n",
		},
		{
			"in a table, the escaped pipe", "| a |\n| - |\n| [[x\\|y]] |\n",
			"<div class=\"nw-scroll\" tabindex=\"0\"><table>\n<thead>\n<tr>\n<th>a</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n" +
				`<td><a class="nw-wikilink nw-unresolved" data-nw-target="x">y</a></td>` + "\n</tr>\n</tbody>\n</table>\n</div>\n",
		},
	})
}

// A tag's '_' at its end closes an emphasis around it, and is its name's
// when it closes none; a tag after the text a run left is text (rule 9).
func TestATagsUnderscoresAndRuns(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"an emphasis around it", "_#t5_ __#t6__\n",
			`<p><em><span class="nw-tag" data-nw-tag="t5">#t5</span></em> <strong><span class="nw-tag" data-nw-tag="t6">#t6</span></strong></p>` + "\n",
		},
		{
			"none", "#t5_ #t7__ #___\n",
			`<p><span class="nw-tag" data-nw-tag="t5_">#t5_</span> <span class="nw-tag" data-nw-tag="t7__">#t7__</span> ` +
				`<span class="nw-tag" data-nw-tag="___">#___</span></p>` + "\n",
		},
		{"a run left as text, a bracket", "a*#t [#u\n", "<p>a*#t [#u</p>\n"},
		{
			"an emphasis its underscores open", "#a/___.b_ c\n",
			`<p><span class="nw-tag" data-nw-tag="a/__">#a/__</span><em>.b</em> c</p>` + "\n",
		},
		{
			"a strong emphasis its underscores open", "#a-___(b__ c\n",
			`<p><span class="nw-tag" data-nw-tag="a-_">#a-_</span><strong>(b</strong> c</p>` + "\n",
		},
		{"a number and an underscore", "#123_ #123\n", `<p><span class="nw-tag" data-nw-tag="123_">#123_</span> #123</p>` + "\n"},
		{
			"at the start of a quote's line", "> a\n>#b\n",
			"<blockquote>\n<p>a\n" + `<span class="nw-tag" data-nw-tag="b">#b</span></p>` + "\n</blockquote>\n",
		},
	})
}

// Highlights are marks; formulas are their TeX, escaped, for the front end
// to typeset (rule 5).
func TestHighlightsAndFormulas(t *testing.T) {
	checkRenders(t, []renderCase{
		{"highlights", "==hi== ==a *b* c==\n", "<p><mark>hi</mark> <mark>a <em>b</em> c</mark></p>\n"},
		{
			"inline formulas", "$x^2<y$ and $$ z $$\n",
			`<p><span class="nw-math">x^2&lt;y</span> and <span class="nw-math nw-math-block"> z </span></p>` + "\n",
		},
		{"over lines of a quote", "> $a\n> b$ c\n", "<blockquote>\n<p><span class=\"nw-math\">a\nb</span> c</p>\n</blockquote>\n"},
		{"a block", "$$\nx = <1>\n$$\n\nafter\n", "<div class=\"nw-math nw-math-block\">x = &lt;1&gt;\n</div>\n<p>after</p>\n"},
		{"a block's first and last lines", "$$ x\ny $$\n", "<div class=\"nw-math nw-math-block\"> x\ny </div>\n"},
		{"a block in a quote", "> $$\n> x\n> $$\n", "<blockquote>\n<div class=\"nw-math nw-math-block\">x\n</div>\n</blockquote>\n"},
		{"a block to the end", "a\n$$\nb\n\nc\n", "<p>a</p>\n<div class=\"nw-math nw-math-block\">b\n\nc\n</div>\n"},
		{"not formulas", "$ a $ and $5 or 6$7 and \\$b\\$\n", "<p>$ a $ and $5 or 6$7 and $b$</p>\n"},
		{"an escaped '$' does not close one", "$a\\$b$ c\n", "<p><span class=\"nw-math\">a\\$b</span> c</p>\n"},
		{"a $$ that nothing closes", "a $$b $c\n", "<p>a $$b $c</p>\n"},
	})
}

// A comment hides what it spans, a block or the part of one it covers; a
// marker left alone shows (rule 6).
func TestCommentsHide(t *testing.T) {
	checkRenders(t, []renderCase{
		{"inline, in pairs", "a %%b%% c %%d%% e %% f\n", "<p>a  c  e %% f</p>\n"},
		{"across an emphasis", "*a %%b* c%% d\n", "<p><em>a </em> d</p>\n"},
		{"across a link", "[x %%y](z) w%%\n", `<p><a class="nw-unresolved" data-nw-target="z">x </a></p>` + "\n"},
		{"a whole paragraph", "%%a%%\n\nb\n", "<p>b</p>\n"},
		{"a block comment", "a\n\n%%\nb\n\nc\n%%\n\nd\n", "<p>a</p>\n<p>d</p>\n"},
		{"one left open hides the rest", "a\n\n%%\nb\n\nc\n", "<p>a</p>\n"},
		{"a marker not at its line's end does not end it", "%%\na %% b\nc %%\n\nd\n", "<p>d</p>\n"},
		{"part of the blocks at its ends", "a\n%%\nb\n\nc %%\nd\n", "<p>a\n</p>\n<p>\nd</p>\n"},
		{"a table it spans", "%%\n\n| a |\n| - |\n| b |\n\n%%\n\nshown\n", "<p>shown</p>\n"},
		{"in code, not a marker", "`%%` a `%%`\n", "<p><code>%%</code> a <code>%%</code></p>\n"},
		{"in a heading or a cell, alone, text", "# %%\n\n| %% |\n| - |\n| a |\n\nb\n",
			"<h1 id=\"nw-section\">%%</h1>\n<div class=\"nw-scroll\" tabindex=\"0\"><table>\n<thead>\n<tr>\n<th>%%</th>\n</tr>\n</thead>\n" +
				"<tbody>\n<tr>\n<td>a</td>\n</tr>\n</tbody>\n</table>\n</div>\n<p>b</p>\n"},
		{
			"ending in a cell's line: the table's rows and cells stay", "%%\n\n| a | b |\n| - | - |\n| c | d %%\n\nf\n",
			"<div class=\"nw-scroll\" tabindex=\"0\"><table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n" +
				"<tbody>\n<tr>\n<td>c</td>\n<td></td>\n</tr>\n</tbody>\n</table>\n</div>\n<p>f</p>\n",
		},
		{"a marker before a heading's closing '#' does not end it", "%%\n\n# b %% #\n\nc %%\n\nd\n", "<p>d</p>\n"},
		{"a marker before a cell's end does not end it", "%%\n\n| a |\n| - |\n| b %% |\n\nc %%\n\nd\n", "<p>d</p>\n"},
		{"an address ends before a marker", "a %%see https://example.com/x%% b\n", "<p>a  b</p>\n"},
		{"an address at a block comment's end", "%%\nsecret www.example.com/%%\n\nshown\n", "<p>shown</p>\n"},
		{
			"in an image's text, text", "![a %% b](i.png) %%\n",
			`<p><span class="nw-image">a %% b <a class="nw-unresolved" data-nw-target="i.png">i.png</a></span> %%</p>` + "\n",
		},
		{"a heading it hides takes no id", "%%\n# A\n%%\n\n# A\n", "<h1 id=\"nw-a\">A</h1>\n"},
		{"what it hides is no heading's id", "# a %%b%%\n", "<h1 id=\"nw-a\">a </h1>\n"},
		{
			"a list it starts in keeps its container", "- %%\n  a\n- b\n\nc %%\n\nd\n",
			"<ul>\n<li>\n</li>\n</ul>\n<p>d</p>\n",
		},
	})
}

// A block quote that starts with [!type] is a callout: folded with '-',
// open with '+'; its title is its type's when its line has none (M6/P1
// design 3.9).
func TestCallouts(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"a title, links and tags", "> [!note] Title [[c1]]\n> body #t\n",
			`<div class="nw-callout" data-callout="note">` + "\n" +
				`<div class="nw-callout-title">Title <a class="nw-wikilink nw-unresolved" data-nw-target="c1">c1</a></div>` + "\n" +
				`<p>body <span class="nw-tag" data-nw-tag="t">#t</span></p>` + "\n</div>\n",
		},
		{
			"folded, no title", "> [!TIP]-\n> folded\n",
			`<details class="nw-callout" data-callout="tip">` + "\n<summary>Tip</summary>\n<p>folded</p>\n</details>\n",
		},
		{
			"open", "> [!info]+ Open\n> body\n",
			`<details class="nw-callout" data-callout="info" open="">` + "\n<summary>Open</summary>\n<p>body</p>\n</details>\n",
		},
		{
			"no body", "> [!warning]\n",
			`<div class="nw-callout" data-callout="warning">` + "\n" + `<div class="nw-callout-title">Warning</div>` + "\n</div>\n",
		},
		{
			"nested", "> [!a] A\n> > [!b] B\n> > inner\n",
			`<div class="nw-callout" data-callout="a">` + "\n" + `<div class="nw-callout-title">A</div>` + "\n" +
				`<div class="nw-callout" data-callout="b">` + "\n" + `<div class="nw-callout-title">B</div>` + "\n" +
				"<p>inner</p>\n</div>\n</div>\n",
		},
		{
			"a title it hides is the type's", "> [!note] %%x%%\n> body\n",
			`<div class="nw-callout" data-callout="note">` + "\n" + `<div class="nw-callout-title">Note</div>` + "\n<p>body</p>\n</div>\n",
		},
		{
			"a title of comments and spaces is the type's", "> [!note] %%a%% %%b%%\n> body\n",
			`<div class="nw-callout" data-callout="note">` + "\n" + `<div class="nw-callout-title">Note </div>` + "\n<p>body</p>\n</div>\n",
		},
		{"not one", "> [!a b]\n> \\[!c]\n", "<blockquote>\n<p>[!a b]\n[!c]</p>\n</blockquote>\n"},
	})
}
