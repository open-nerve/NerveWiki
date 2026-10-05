package obsidian_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
)

// known are the pages the tests' Resolve knows, by the target of a link to
// each.
//
//nolint:gochecknoglobals // read only
var known = map[string]uuid.UUID{
	"Page":        uuid.MustParse("00000000-0000-4000-8000-000000000001"),
	"dir/Page.md": uuid.MustParse("00000000-0000-4000-8000-000000000002"),
}

// resolveKnown resolves the links to the pages known has.
func resolveKnown(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]uuid.UUID, error) {
	to := map[int]uuid.UUID{}
	for _, l := range links {
		if id, ok := known[l.Target]; ok {
			to[l.Range.Start] = id
		}
	}
	return to, nil
}

// resolveAll resolves every link, to the same page.
func resolveAll(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]uuid.UUID, error) {
	to := map[int]uuid.UUID{}
	for _, l := range links {
		to[l.Range.Start] = uuid.Max()
	}
	return to, nil
}

// A wikilink, an embed, a Markdown link and an image's link to a page lead
// to the page it resolves to, with the heading its anchor leads to; one
// that resolves to none carries its target (M6/P3 design 6.2).
func TestLinksLeadWhereTheyResolve(t *testing.T) {
	page := `data-nw-node="` + known["Page"].String() + `"`
	inDir := `data-nw-node="` + known["dir/Page.md"].String() + `"`
	checkRenders(t, []renderCase{
		{
			"a wikilink, its anchor", "[[Page]] [[Page#Part Two|see]]\n",
			`<p><a class="nw-wikilink" ` + page + `>Page</a> ` +
				`<a class="nw-wikilink" ` + page + ` data-nw-anchor="nw-part-two">see</a></p>` + "\n",
		},
		{"an embed", "![[Page]]\n", `<p><a class="nw-wikilink nw-embed" ` + page + `>Page</a></p>` + "\n"},
		{
			"an anchor's last part; a block's, an empty one, none", "[[Page#A#B]] [[Page#^b]] [[Page# ]] [[Page#A# ^c]]\n",
			`<p><a class="nw-wikilink" ` + page + ` data-nw-anchor="nw-b">Page &gt; A#B</a> ` +
				`<a class="nw-wikilink" ` + page + `>Page &gt; ^b</a> <a class="nw-wikilink" ` + page + `>Page</a> ` +
				`<a class="nw-wikilink" ` + page + `>Page &gt; A# ^c</a></p>` + "\n",
		},
		{
			"an anchor alone; a block's alone, a span", "[[#Part Two]] [[#^b]] ![[#c]]\n",
			`<p><a class="nw-wikilink" href="#nw-part-two">Part Two</a> <span class="nw-wikilink">^b</span> ` +
				`<a class="nw-wikilink nw-embed" href="#nw-c">c</a></p>` + "\n",
		},
		{
			"Markdown links and an image", `[t](dir/Page.md#Part%20Two "T") ![i](dir/Page.md) [u](x.md#h)` + "\n",
			`<p><a ` + inDir + ` data-nw-anchor="nw-part-two" title="T">t</a> ` +
				`<span class="nw-image">i <a ` + inDir + `>dir/Page.md</a></span> ` +
				`<a class="nw-unresolved" data-nw-target="x.md">u</a></p>` + "\n",
		},
		{
			"a reference link's uses", "[a][r] [b][r]\n\n[r]: Page\n",
			`<p><a ` + page + `>a</a> <a ` + page + `>b</a></p>` + "\n",
		},
		{
			"addresses elsewhere", "[e](https://x.example/Page) <https://x.example/a> [f](#Page)\n",
			`<p><a href="https://x.example/Page">e</a> <a href="https://x.example/a">https://x.example/a</a> ` +
				`<a href="#nw-page">f</a></p>` + "\n",
		},
		{
			"in a link's text, a span", "[see [[Page]] *and ![[x]]*](Page)\n",
			`<p><a ` + page + `>see <span class="nw-wikilink">Page</span> <em>and <span class="nw-wikilink nw-embed">x</span></em></a></p>` + "\n",
		},
		{
			"in brackets that are no link, a link", "[see [[Page]]] [x]\n",
			`<p>[see <a class="nw-wikilink" ` + page + `>Page</a>] [x]</p>` + "\n",
		},
		{
			"in a user's link, which goes", "<a href=\"https://x.example\">see *[[Page]]*</a> <a href=\"/x\">![[Page]]</a>\n",
			`<p>see <em><a class="nw-wikilink" ` + page + `>Page</a></em> <a class="nw-wikilink nw-embed" ` + page + `>Page</a></p>` + "\n",
		},
		{
			"after a link, a link", "[a](https://x.example) [[Page]]\n",
			`<p><a href="https://x.example">a</a> <a class="nw-wikilink" ` + page + `>Page</a></p>` + "\n",
		},
		{
			"unresolved, escaped", "[[a\"<b>|x]] ![[c&d]]\n",
			`<p><a class="nw-wikilink nw-unresolved" data-nw-target="a&quot;&lt;b&gt;">x</a> ` +
				`<a class="nw-wikilink nw-embed nw-unresolved" data-nw-target="c&amp;d">c&amp;d</a></p>` + "\n",
		},
	})
}

// The property table writes a property link as the body writes its kind,
// leading where it resolves or carrying its target, and showing what the
// link shows: a wikilink's display text or target, a Markdown link's
// text. It goes by the value, not its path: the key "a.b" and the key b
// under a are each its own. A value an alias repeats, one over lines, an
// embed and a value with more than its link are text (M6/P6 design 4).
func TestAPropertyLinkIsALinkInTheTable(t *testing.T) {
	page := `data-nw-node="` + known["Page"].String() + `"`
	row := func(key, value string) string { return "<tr><th>" + key + "</th><td>" + value + "</td></tr>" }
	checkRenders(t, []renderCase{{
		"each kind",
		"---\nup: \"[[Page]]\"\nsee: \"[[Page#Part Two|the <part>]]\"\nmd: \"[t *x*](Page#h)\"\nnone: '[[Missing]]'\n" +
			"\"a.b\": \"[[Page]]\"\na: {b: \"[[Missing]]\"}\nl: ['[a](b.md)', x]\n" +
			"r: &r \"[[Page]]\"\nagain: *r\nm: |\n  [[Page]]\nmore: \"[[Page]] and\"\nemb: \"![[Page]]\"\n---\n",
		`<div class="nw-scroll"><table class="nw-props">` +
			row("up", `<a class="nw-wikilink" `+page+`>Page</a>`) +
			row("see", `<a class="nw-wikilink" `+page+` data-nw-anchor="nw-part-two">the &lt;part&gt;</a>`) +
			row("md", `<a `+page+` data-nw-anchor="nw-h">t x</a>`) +
			row("none", `<a class="nw-wikilink nw-unresolved" data-nw-target="Missing">Missing</a>`) +
			row("a.b", `<a class="nw-wikilink" `+page+`>Page</a>`) +
			row("a", `<table class="nw-props">`+row("b", `<a class="nw-wikilink nw-unresolved" data-nw-target="Missing">Missing</a>`)+`</table>`) +
			row("l", `<ul><li><a class="nw-unresolved" data-nw-target="b.md">a</a></li><li>x</li></ul>`) +
			row("r", `<a class="nw-wikilink" `+page+`>Page</a>`) + row("again", "[[Page]]") + row("m", "[[Page]]\n") +
			row("more", "[[Page]] and") + row("emb", "![[Page]]") + "</table></div>\n",
	}})
}

// Fetch asks Resolve where the links the page's content has lead, for the
// page rendered, and asks nothing of a page without links; Resolve's error
// is Render's; a page id that is no one's resolves to none.
func TestFetchAsksResolveOfThePagesLinks(t *testing.T) {
	var asked []markdown.Page
	var got []obsidian.Link
	down := errors.New("down")
	fail := false
	m := newMarkdownWith(t, obsidian.Options{Resolve: func(_ context.Context, p markdown.Page, links []obsidian.Link) (map[int]uuid.UUID, error) {
		asked, got = append(asked, p), links
		if fail {
			return nil, down
		}
		return map[int]uuid.UUID{2: known["Page"], 11: uuid.Nil()}, nil
	}})
	page := markdown.Page{NotebookID: uuid.New(), PageID: uuid.New(), Revision: 3}
	content := "[[Page]] [[x]]\n"
	out, err := m.Render(context.Background(), m.Parse([]byte(content)), page)
	if err != nil {
		t.Fatal(err)
	}
	want := `<p><a class="nw-wikilink" data-nw-node="` + known["Page"].String() + `">Page</a> ` +
		`<a class="nw-wikilink nw-unresolved" data-nw-target="x">x</a></p>` + "\n"
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
	if !reflect.DeepEqual(asked, []markdown.Page{page}) || !reflect.DeepEqual(got, extracted(m, content).Links) {
		t.Errorf("asked of %v for %v", asked, got)
	}
	asked = nil
	if _, err := m.Render(context.Background(), m.Parse([]byte("# no links, #tag\n")), page); err != nil || asked != nil {
		t.Errorf("a page without links: %v, asked of %v", err, asked)
	}
	fail = true
	if _, err := m.Render(context.Background(), m.Parse([]byte(content)), page); !errors.Is(err, down) {
		t.Errorf("Render: %v, want %v", err, down)
	}
}

// Without Resolve, no link leads anywhere.
func TestWithoutResolveNoLinkLeads(t *testing.T) {
	m := newMarkdownWith(t, obsidian.Options{})
	out, err := m.Render(context.Background(), m.Parse([]byte("[[Page]] [p](Page)\n")), markdown.Page{})
	if err != nil {
		t.Fatal(err)
	}
	want := `<p><a class="nw-wikilink nw-unresolved" data-nw-target="Page">Page</a> ` +
		`<a class="nw-unresolved" data-nw-target="Page">p</a></p>` + "\n"
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}
