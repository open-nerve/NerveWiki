package markdown

import "testing"

// A user's HTML as the allowlist keeps it (M4/P3 design 3.7).
func TestAUsersHTMLKeepsTheTypographicAllowlist(t *testing.T) {
	checkRenders(t, []renderCase{
		{"an event attribute", "a <b onclick=\"x()\">b</b>", "<p>a <b>b</b></p>\n"},
		{"a style", "a <span style=\"color: red\">b</span>", "<p>a <span>b</span></p>\n"},
		{"a script address", "<a href=\"javascript:alert(1)\">a</a>", "<p><a>a</a></p>\n"},
		{"another host", "<a href=\"//evil.example\">a</a>", "<p><a>a</a></p>\n"},
		{"another host, backslashed", "<a href=\"/\\evil.example\">a</a>", "<p><a>a</a></p>\n"},
		{"a script address in entities", "<a href=\"&#106;ava&#x09;script&colon;x\">a</a>", "<p><a>a</a></p>\n"},
		{"an address kept", "<a href=\"/p?a=1&amp;b=2\" title=\"t\">a</a>", "<p><a href=\"/p?a=1&amp;b=2\" title=\"t\">a</a></p>\n"},
		{"a repeated attribute", "<a href=\"/a\" href=\"javascript:x\">a</a>", "<p><a href=\"/a\">a</a></p>\n"},
		{"a repeated attribute, the first not allowed", "<a href=\"javascript:x\" HREF=\"/a\">a</a>", "<p><a>a</a></p>\n"},
		{
			"a repeated attribute in a block", "<div><a href=\"javascript:x\" href=\"/a\">a</a></div>\n",
			"<div><a>a</a></div>\n",
		},
		{"attributes unquoted and in capitals", "<A HREF=/p/q TITLE='t \"u\"'>a</A>", "<p><a href=\"/p/q\" title=\"t &#34;u&#34;\">a</a></p>\n"},
		{"an attribute over lines", "a <abbr\ntitle=\"t\">b</abbr>", "<p>a <abbr title=\"t\">b</abbr></p>\n"},
		{"a raw image", "<img src=\"x\" onerror=\"alert(1)\">\n", "\n"},
		{"a raw image inline", "a <img src=\"x\" onerror=\"alert(1)\"> b", "<p>a  b</p>\n"},
		{"a script and what it holds", "<script>alert(1)</script>\n", "\n"},
		{"a script inline and what it holds", "a <script>b *c*</script> d", "<p>a  d</p>\n"},
		{"a script in an svg", "a <svg><script>alert(1)</script></svg> b", "<p>a  b</p>\n"},
		{"a style and what it holds", "<style>p { color: red }</style>\n", "\n"},
		{"a comment", "<!-- a -->\n", "\n"},
		{"a comment inline", "a <!-- b --> c", "<p>a  c</p>\n"},
		{"a block element inline", "a <p>b</p> c", "<p>a b c</p>\n"},
		{"split in one paragraph", "a <b>b *c*</b> d", "<p>a <b>b <em>c</em></b> d</p>\n"},
		{"unclosed in a paragraph", "a <b>b\n\nc\n", "<p>a <b>b</b></p>\n<p>c</p>\n"},
		{"unclosed, the last opened closed first", "a <b><i>c", "<p>a <b><i>c</i></b></p>\n"},
		{"void elements, never closed", "a <br> b <wbr>c", "<p>a <br> b <wbr>c</p>\n"},
		{"a list's start not a number", "<ol start=\"x\" reversed><li>a</li></ol>\n", "<ol reversed><li>a</li></ol>\n"},
		{"an end tag out of its scope", "*<b>*</b> c", "<p><em><b></b></em> c</p>\n"},
		{"an end tag closing what follows it", "<b><i>a</b> b</i>", "<p><b><i>a</i></b> b</p>\n"},
		{"an end tag never opened", "a </div></b> b", "<p>a  b</p>\n"},
		{"a renderer's class and id", "<span class=\"footnote-ref\" id=\"nw-fn:1\">a</span>", "<p><span>a</span></p>\n"},
		{"an a in a link", "[a <a href=\"/b\">b</a> c](/d)", "<p><a href=\"/d\">a b c</a></p>\n"},
		{"an a after a link", "[a](/d)\n\n<a href=\"/b\">b</a>", "<p><a href=\"/d\">a</a></p>\n<p><a href=\"/b\">b</a></p>\n"},
		{"an a in emphasis after a link", "[a](/d) *<a href=\"/b\">b</a>*", "<p><a href=\"/d\">a</a> <em><a href=\"/b\">b</a></em></p>\n"},
		{"an a after an a", "<a href=\"/d\">a</a> <a href=\"/b\">b</a>", "<p><a href=\"/d\">a</a> <a href=\"/b\">b</a></p>\n"},
		// A link holds no link (M6/P3B review L1).
		{"an a around a link", `<a href="/x">see [l](/y)</a>`, "<p>see <a href=\"/y\">l</a></p>\n"},
		{"an a around a link in emphasis", `<a href="/x">*[l](/y)*</a>`, "<p><em><a href=\"/y\">l</a></em></p>\n"},
		{"an a around an autolink", `<a href="/x"><https://y.example></a>`, "<p><a href=\"https://y.example\">https://y.example</a></p>\n"},
		{"an a around an image", `<a href="/x">![i](p.png)</a>`, "<p><span class=\"nw-image\">i <a href=\"p.png\">p.png</a></span></p>\n"},
		{
			"an a around a footnote's reference", "<a href=\"/x\">a[^1]</a>\n\n[^1]: n\n",
			`<p>a<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup></p>` + "\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n" +
				`<p>n&#160;<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` + "\n</li>\n</ol>\n</div>\n",
		},
		{"an a closed before a link", `<a href="/x">t</a> [l](/y)`, "<p><a href=\"/x\">t</a> <a href=\"/y\">l</a></p>\n"},
		{"an a another end tag closes before a link, to its scope's end", `<b><a href="/x">t</b> [l](/y)`, "<p><b>t</b> <a href=\"/y\">l</a></p>\n"},
		{"an a in an a", `<a href="/x">a <a href="/y">b</a></a>`, "<p><a href=\"/x\">a b</a></p>\n"},
		{"an a in emphasis in an a", `<a href="/x">*<a href="/y">b</a>*</a>`, "<p><a href=\"/x\"><em>b</em></a></p>\n"},
		{"an autolink in a link", `[see <https://y.example>](/x)`, "<p><a href=\"/x\">see https://y.example</a></p>\n"},
		{
			"a footnote's reference in a link", "[a[^1]](/x) b[^1]\n\n[^1]: n\n",
			`<p><a href="/x">a<sup id="nw-fnref:1">1</sup></a> b<sup id="nw-fnref1:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup></p>` + "\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n" +
				`<p>n&#160;<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a>` +
				`&#160;<a href="#nw-fnref1:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` + "\n</li>\n</ol>\n</div>\n",
		},
		{
			"an a unclosed in a footnote, before its back link", "x[^1]\n\n[^1]: <a href=\"/x\">note\n",
			`<p>x<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup></p>` + "\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n" +
				`<p>note&#160;<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` + "\n</li>\n</ol>\n</div>\n",
		},
		// An </a> that a dropped element holds ends nothing (P3B fix check).
		{"an a whose end a script holds, around a link", `x <a href="/1"><script></a></script>[y](/u)</a>`, "<p>x <a href=\"/u\">y</a></p>\n"},
		{"an a whose end a textarea holds, around a link", `x <a href="/1"><textarea></a></textarea>[y](/u)</a> z`, "<p>x <a href=\"/u\">y</a> z</p>\n"},
		{"an a closed after a script, before a link", `<a href="/x"><script>s</script>t</a> [l](/y)`, "<p><a href=\"/x\">t</a> <a href=\"/y\">l</a></p>\n"},
		{"an a closed in an element, before a link", `<i><a href="/x">t</a></i> [l](/y)`, "<p><i><a href=\"/x\">t</a></i> <a href=\"/y\">l</a></p>\n"},
		{
			"a footnote's reference in a link, referred to before", "b[^1] [a[^1]](/x)\n\n[^1]: n\n",
			`<p>b<sup id="nw-fnref:1"><a href="#nw-fn:1" class="footnote-ref" role="doc-noteref">1</a></sup> <a href="/x">a<sup id="nw-fnref1:1">1</sup></a></p>` + "\n" +
				`<div class="footnotes" role="doc-endnotes">` + "\n<hr>\n<ol>\n" + `<li id="nw-fn:1">` + "\n" +
				`<p>n&#160;<a href="#nw-fnref:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a>` +
				`&#160;<a href="#nw-fnref1:1" class="footnote-backref" role="doc-backlink">&#x21a9;&#xfe0e;</a></p>` + "\n</li>\n</ol>\n</div>\n",
		},
		{"an autolink in a link whose address is refused, its label", `[a <https://y.example> b](javascript:x)`, "<p>a https://y.example b</p>\n"},
		{
			"an address's parameters like references", "<a href=\"/s?q=x&section=n&copy=2&amp;t=1&sect\" title=\"&amp=\">a</a>",
			"<p><a href=\"/s?q=x&amp;section=n&amp;copy=2&amp;t=1§\" title=\"&amp;amp=\">a</a></p>\n",
		},
		{
			"an address's parameters like references in a block", "<div><a href=\"/s?q=x&section=n&copy=2&amp;t=1&sect\" title=\"&amp=\">a</a></div>\n",
			"<div><a href=\"/s?q=x&amp;section=n&amp;copy=2&amp;t=1§\" title=\"&amp;amp=\">a</a></div>\n",
		},
		{"plaintext", "a <plaintext>b", "<p>a </p>\n"},
		{"text re-escaped", "<div>&lt;script&gt; &amp; \"</div>\n", "<div>&lt;script&gt; &amp; &#34;</div>\n"},
		{
			"block elements in a block", "<details open><summary>s</summary>\n<ol start=\"3\" reversed><li>a</li></ol>\n</details>\n",
			"<details open><summary>s</summary>\n<ol start=\"3\" reversed><li>a</li></ol>\n</details>\n",
		},
		{
			"cells and their spans", "<table><tr><td colspan=\"2\" rowspan=\"x\" align=\"left\">a</td></tr></table>\n",
			"<table><tr><td colspan=\"2\">a</td></tr></table>\n",
		},
		{"a direction", "<bdo dir=\"rtl\">a</bdo> <bdo dir=\"up\">b</bdo>", "<p><bdo dir=\"rtl\">a</bdo> <bdo>b</bdo></p>\n"},
		{"an unclosed block", "<div>\na\n\nb\n", "<div>\na\n</div><p>b</p>\n"},
	})
}
