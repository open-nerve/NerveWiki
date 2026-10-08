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
	checkRendersWith(t, newMarkdown(t), tests)
}

// checkRendersWith checks each case's HTML as m renders it, and that it
// passes the check of a reading view's HTML with the extensions' markup.
func checkRendersWith(t *testing.T, m *markdown.Markdown, tests []renderCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, err := m.Render(context.Background(), m.Parse([]byte(tt.src)), markdown.Page{})
			if err != nil {
				t.Fatal(err)
			}
			got := view.HTML
			if got != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.src, got, tt.want)
			}
			if err := markdowntest.CheckHTML(got, tasks.Extension(), obsidian.Extension(obsidian.Options{})); err != nil {
				t.Error(err)
			}
		})
	}
}

// Wikilinks and embeds are links, here to no page, and tags are links to
// their pages; what they show is escaped, as their attributes are (M6/P1
// design 3.10, M6/P3 design 6.2, M6/P6 design 3).
func TestWikilinksAndTagsAreLinks(t *testing.T) {
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
			`<p><a class="nw-tag" data-nw-tag="tag">#tag</a> and <em><a class="nw-tag" data-nw-tag="t1">#t1</a></em> x#no</p>` + "\n",
		},
		{
			"a heading's id has what its wikilinks and tags show", "# Head [[x|y]] #t\n",
			`<h1 id="nw-head-y-t">Head <a class="nw-wikilink nw-unresolved" data-nw-target="x">y</a> ` +
				`<a class="nw-tag" data-nw-tag="t">#t</a></h1>` + "\n",
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
			"<div class=\"nw-scroll\"><table>\n<thead>\n<tr>\n<th>a</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n" +
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
			`<p><em><a class="nw-tag" data-nw-tag="t5">#t5</a></em> <strong><a class="nw-tag" data-nw-tag="t6">#t6</a></strong></p>` + "\n",
		},
		{
			"none", "#t5_ #t7__ #___\n",
			`<p><a class="nw-tag" data-nw-tag="t5_">#t5_</a> <a class="nw-tag" data-nw-tag="t7__">#t7__</a> ` +
				`<a class="nw-tag" data-nw-tag="___">#___</a></p>` + "\n",
		},
		{"a run left as text, a bracket", "a*#t [#u\n", "<p>a*#t [#u</p>\n"},
		{
			"an emphasis its underscores open", "#a/___.b_ c\n",
			`<p><a class="nw-tag" data-nw-tag="a/__">#a/__</a><em>.b</em> c</p>` + "\n",
		},
		{
			"a strong emphasis its underscores open", "#a-___(b__ c\n",
			`<p><a class="nw-tag" data-nw-tag="a-_">#a-_</a><strong>(b</strong> c</p>` + "\n",
		},
		{"a number and an underscore", "#123_ #123\n", `<p><a class="nw-tag" data-nw-tag="123_">#123_</a> #123</p>` + "\n"},
		{
			"at the start of a quote's line", "> a\n>#b\n",
			"<blockquote>\n<p>a<br>\n" + `<a class="nw-tag" data-nw-tag="b">#b</a></p>` + "\n</blockquote>\n",
		},
	})
}

// A tag Obsidian's tag pane counts is a link to the pages with it, by the
// name the pane counts, as written, but for its last '/'; one the pane does
// not count is a span, as is one in a Markdown link's text; a user's link
// around one goes, as around a wikilink (M6/P6 design 3).
func TestATagIsALinkToItsPages(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"as written, nested", "#Tag #a/b/c #a/\n",
			`<p><a class="nw-tag" data-nw-tag="Tag">#Tag</a> <a class="nw-tag" data-nw-tag="a/b/c">#a/b/c</a> ` +
				`<a class="nw-tag" data-nw-tag="a">#a/</a></p>` + "\n",
		},
		{
			// U+2E2F is a letter in the Supplemental Punctuation block.
			"not counted, or no address's", "#1/ #/ #// #a\u2e2fb\n",
			`<p><span class="nw-tag" data-nw-tag="1/">#1/</span> <span class="nw-tag" data-nw-tag="/">#/</span> ` +
				`<span class="nw-tag" data-nw-tag="//">#//</span> ` +
				`<span class="nw-tag" data-nw-tag="a` + "\u2e2f" + `b">#a` + "\u2e2f" + `b</span></p>` + "\n",
		},
		{
			"in a link's text, a span", "[see #t and [[P]]](https://x.example)\n",
			`<p><a href="https://x.example">see <span class="nw-tag" data-nw-tag="t">#t</span> and <span class="nw-wikilink">P</span></a></p>` + "\n",
		},
		{
			"in brackets that are no link, a link", "[see #t] [x]\n",
			`<p>[see <a class="nw-tag" data-nw-tag="t">#t</a>] [x]</p>` + "\n",
		},
		{
			"in a user's link, which goes", `<a href="https://x.example">see *#t*</a>` + "\n",
			`<p>see <em><a class="nw-tag" data-nw-tag="t">#t</a></em></p>` + "\n",
		},
		{
			// Neither renders a link: the user's stays (M6/P6 review).
			"not counted in a user's link, which stays", `<a href="/x">#1/ [[#^b]]</a>` + "\n",
			`<p><a href="/x"><span class="nw-tag" data-nw-tag="1/">#1/</span> <span class="nw-wikilink">^b</span></a></p>` + "\n",
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
		{"a block", "$$\nx = <1>\n$$\n\nafter\n", "<div class=\"nw-scroll\"><div class=\"nw-math nw-math-block\">x = &lt;1&gt;\n</div></div>\n<p>after</p>\n"},
		{"a block's first and last lines", "$$ x\ny $$\n", "<div class=\"nw-scroll\"><div class=\"nw-math nw-math-block\"> x\ny </div></div>\n"},
		{"a block in a quote", "> $$\n> x\n> $$\n", "<blockquote>\n<div class=\"nw-scroll\"><div class=\"nw-math nw-math-block\">x\n</div></div>\n</blockquote>\n"},
		{"a block to the end", "a\n$$\nb\n\nc\n", "<p>a</p>\n<div class=\"nw-scroll\"><div class=\"nw-math nw-math-block\">b\n\nc\n</div></div>\n"},
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
		{"part of the blocks at its ends: no line of its own shown", "a\n%%\nb\n\nc %%\nd\n", "<p>a<br>\n</p>\n<p>d</p>\n"},
		{"in a paragraph, its lines not shown, nor its last's hard line break", "a\n%%\nb\n%%  \nc\n", "<p>a<br>\nc</p>\n"},
		{"nor its last's line break after blanks", "a\n%%\nb\n%%    \nc\n\n%%\nd\n%% \t\ne\n", "<p>a<br>\n c</p>\n<p> e</p>\n"},
		{"a text after its last marker keeps its line break", "%%\nb\n%%\rc\nd\n", "<p>\rc<br>\nd</p>\n"},
		{"a table it spans", "%%\n\n| a |\n| - |\n| b |\n\n%%\n\nshown\n", "<p>shown</p>\n"},
		{"in code, not a marker", "`%%` a `%%`\n", "<p><code>%%</code> a <code>%%</code></p>\n"},
		{"in a heading or a cell, alone, text", "# %%\n\n| %% |\n| - |\n| a |\n\nb\n",
			"<h1 id=\"nw-section\">%%</h1>\n<div class=\"nw-scroll\"><table>\n<thead>\n<tr>\n<th>%%</th>\n</tr>\n</thead>\n" +
				"<tbody>\n<tr>\n<td>a</td>\n</tr>\n</tbody>\n</table>\n</div>\n<p>b</p>\n"},
		{
			"ending in a cell's line: the table's rows and cells stay", "%%\n\n| a | b |\n| - | - |\n| c | d %%\n\nf\n",
			"<div class=\"nw-scroll\"><table>\n<thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n</thead>\n" +
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
				`<p>body <a class="nw-tag" data-nw-tag="t">#t</a></p>` + "\n</div>\n",
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
		{"not one", "> [!a b]\n> \\[!c]\n", "<blockquote>\n<p>[!a b]<br>\n[!c]</p>\n</blockquote>\n"},
		{
			"its title's line ends with no line break, its body's lines are shown apart", "> [!note] T\n> a\n> b\n",
			`<div class="nw-callout" data-callout="note">` + "\n" + `<div class="nw-callout-title">T</div>` + "\n<p>a<br>\nb</p>\n</div>\n",
		},
		{
			"its title's hard line break is not shown either", "> [!note] T  \n> a\n",
			`<div class="nw-callout" data-callout="note">` + "\n" + `<div class="nw-callout-title">T</div>` + "\n<p>a</p>\n</div>\n",
		},
		{
			"an emphasis over its title's line takes the next line in, shown as one (a known limit)", "> [!note] **a\n> b**\n",
			`<div class="nw-callout" data-callout="note">` + "\n" + `<div class="nw-callout-title"><strong>a<br>` + "\n" +
				`b</strong></div>` + "\n</div>\n",
		},
	})
}

// A line break in a block is shown as one, after any node that ends its
// line too, as Obsidian's reading view shows it; a heading's id reads it as
// a space, as before (M6/P8 design 3).
func TestALineBreakIsShownAsOne(t *testing.T) {
	checkRenders(t, []renderCase{
		{
			"after an embed, a formula, a highlight", "![[x]]\n$y$\n==h==\nb\n",
			`<p><a class="nw-wikilink nw-embed nw-unresolved" data-nw-target="x">x</a><br>` + "\n" +
				`<span class="nw-math">y</span><br>` + "\n<mark>h</mark><br>\nb</p>\n",
		},
		{
			"after a tag, a wikilink, a comment", "#t\n[[x]]\na %%c%%\nb\n",
			`<p><a class="nw-tag" data-nw-tag="t">#t</a><br>` + "\n" +
				`<a class="nw-wikilink nw-unresolved" data-nw-target="x">x</a><br>` + "\na <br>\nb</p>\n",
		},
		{
			"after an address, a strikethrough, an image, HTML", "www.a.com\n~~s~~\n![i](u)\n<b>x</b>\nb\n",
			`<p><a href="http://www.a.com">www.a.com</a><br>` + "\n<del>s</del><br>\n" +
				`<span class="nw-image">i <a class="nw-unresolved" data-nw-target="u">u</a></span><br>` + "\n<b>x</b><br>\nb</p>\n",
		},
		{
			"after a footnote's reference, and in its definition", "a [^1]\nb\n\n[^1]: c\nd\n",
			`<p>a <sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup><br>` + "\nb</p>\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n<p>c<br>\nd&#160;" +
				`<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` + "\n</li>\n</ol>\n</div>\n",
		},
		{
			"in a task item", "- [ ] a\n  b\n",
			`<ul>` + "\n" + `<li><input disabled="" type="checkbox" data-task="3"> a<br>` + "\nb</li>\n</ul>\n",
		},
		{"a hard line break is one", "a  \nb\\\nc\n", "<p>a<br>\nb<br>\nc</p>\n"},
		{
			"after a task item's checkbox alone", "- [ ]\n  b\n- [x]  \n  c\n",
			`<ul>` + "\n" + `<li><input disabled="" type="checkbox" data-task="3"> <br>` + "\nb</li>\n" +
				`<li><input checked="" disabled="" type="checkbox" data-task="13"> <br>` + "\nc</li>\n</ul>\n",
		},
		{
			"none after a $$ formula ending a paragraph's line, shown as a block", "a\n$$x$$\nb $$y$$  \nc\n",
			`<p>a<br>` + "\n" + `<span class="nw-math nw-math-block">x</span>b <span class="nw-math nw-math-block">y</span>c</p>` + "\n",
		},
		{
			"none after a $$ formula ending a list item's line", "- a\n  $$x$$\n  b\n",
			"<ul>\n<li>a<br>\n" + `<span class="nw-math nw-math-block">x</span>b</li>` + "\n</ul>\n",
		},
		{
			"after a $$ formula with text after it, or in a heading", "$$x$$ a\nb\n\n$$y$$\nc\n===\n",
			`<p><span class="nw-math nw-math-block">x</span> a<br>` + "\nb</p>\n" +
				`<h1 id="nw-y-c"><span class="nw-math nw-math-block">y</span><br>` + "\nc</h1>\n",
		},
		{
			"after a $$ formula and a backslash, a tag, or in an emphasis, as Obsidian's", "$$x$$\\\na\n$$y$$ #t\nb\n*$$z$$*\nc\n",
			`<p><span class="nw-math nw-math-block">x</span><br>` + "\na<br>\n" +
				`<span class="nw-math nw-math-block">y</span> <a class="nw-tag" data-nw-tag="t">#t</a><br>` + "\nb<br>\n" +
				`<em><span class="nw-math nw-math-block">z</span></em><br>` + "\nc</p>\n",
		},
		{
			"after a $$ formula and blanks and a backslash, or a backslash and a CR LF", "$$x$$ \\\na\n$$y$$\\\r\nb\n",
			`<p><span class="nw-math nw-math-block">x</span> <br>` + "\na<br>\n" +
				`<span class="nw-math nw-math-block">y</span><br>` + "\nb</p>\n",
		},
		{"after a CR LF", "a\r\nb\r\n", "<p>a<br>\nb</p>\n"},
		{"a heading of two lines: its id as before", "a\nb\n===\n", `<h1 id="nw-a-b">a<br>` + "\nb</h1>\n"},
	})
}
