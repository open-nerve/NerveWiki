package markdown

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"uuid"
)

// pathLinks is an extension's test double that knows every link and image:
// each carries, in place of its address, the start of its destination and
// what Fetch got, the page rendered.
func pathLinks(name string) Extension {
	return Extension{
		Name:  name,
		Fetch: func(_ context.Context, page Page, _ any) (any, error) { return page.PageID.String(), nil },
		Links: func(data any) func(int) ([]Attr, bool) {
			return func(start int) ([]Attr, bool) {
				return []Attr{{Name: "data-" + name, Value: strconv.Itoa(start) + " " + data.(string)}}, true
			}
		},
	}
}

// renderWith renders src with exts for page.
func renderWith(t *testing.T, page Page, src string, exts ...Extension) string {
	t.Helper()
	m := newMarkdown(t, exts...)
	out, err := m.Render(context.Background(), m.Parse([]byte(src)), page)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// An extension's Links has the links and images it knows carry its
// attributes in place of their addresses, by where their destinations
// start (M6/P3 design 6.3).
func TestTheLinksAnExtensionKnowsCarryItsAttributes(t *testing.T) {
	page := Page{PageID: uuid.New()}
	at := func(start int) string { return `data-p="` + strconv.Itoa(start) + " " + page.PageID.String() + `"` }
	tests := []renderCase{
		{"a link with a title", `[a](x.md "t")`, `<p><a ` + at(4) + ` title="t">a</a></p>` + "\n"},
		{
			"a reference link's uses, by the definition", "[a][r] [b][r]\n\n[r]: x.md\n",
			`<p><a ` + at(20) + `>a</a> <a ` + at(20) + `>b</a></p>` + "\n",
		},
		{"an image", "![i](x.md)", `<p><span class="nw-image">i <a ` + at(5) + `>x.md</a></span></p>` + "\n"},
		{"an image without text", "![](x.md)", `<p><span class="nw-image"><a ` + at(4) + `>x.md</a></span></p>` + "\n"},
		{"an image's address escaped", "![](<x y.md>)", `<p><span class="nw-image"><a ` + at(5) + `>x%20y.md</a></span></p>` + "\n"},
		{
			"an image in a link", "[![i](x.md)](y.md)",
			`<p><a ` + at(13) + `><span class="nw-image">i</span></a></p>` + "\n",
		},
		{"an address SafeURL turns down", "[a](<//x.md>)", `<p><a ` + at(5) + `>a</a></p>` + "\n"},
		{"an empty destination", "[a]()", "<p><a href=\"\">a</a></p>\n"},
		{"an autolink", "<https://x.example/a.md>", "<p><a href=\"https://x.example/a.md\">https://x.example/a.md</a></p>\n"},
	}
	m := newMarkdown(t, pathLinks("p"))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := m.Render(context.Background(), m.Parse([]byte(tt.src)), page)
			if err != nil {
				t.Fatal(err)
			}
			if out != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.src, out, tt.want)
			}
		})
	}
}

// The renderer escapes an attribute's value.
func TestALinksAttributesAreEscaped(t *testing.T) {
	ext := Extension{Name: "q", Links: func(any) func(int) ([]Attr, bool) {
		return func(int) ([]Attr, bool) {
			return []Attr{{Name: "data-a", Value: `"><b x='y'>&`}, {Name: "class", Value: "c d"}}, true
		}
	}}
	want := `<p><a data-a="&quot;&gt;&lt;b x='y'&gt;&amp;" class="c d">a</a></p>` + "\n"
	if got := renderWith(t, Page{}, "[a](x)", ext); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// An attribute an extension gives is written only when its name is lower
// case letters, digits and '-', and an address only through SafeURL (P3B
// review L2).
func TestALinksAttributesAreCheckedAsWritten(t *testing.T) {
	ext := Extension{Name: "q", Links: func(any) func(int) ([]Attr, bool) {
		return func(start int) ([]Attr, bool) {
			if start == 4 {
				return []Attr{{Name: "href", Value: "/ok?a=1&b"}, {Name: "src", Value: "//evil.example"}}, true
			}
			return []Attr{
				{Name: "href", Value: "javascript:alert(1)"}, {Name: `onclick="x" data-a`, Value: "v"},
				{Name: "Data-A", Value: "v"}, {Name: "data-b2", Value: "v"},
			}, true
		}
	}}
	want := `<p><a href="/ok?a=1&amp;b">a</a> <span class="nw-image">i <a data-b2="v">y</a></span></p>` + "\n"
	if got := renderWith(t, Page{}, "[a](x) ![i](y)", ext); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// Of the extensions that know a link, the first registered gives its
// attributes; one that answers false leaves the address.
func TestTheFirstExtensionThatKnowsALinkIsTaken(t *testing.T) {
	md := pathLinks("md")
	all := md.Links
	md.Links = func(data any) func(int) ([]Attr, bool) {
		known := all(data)
		return func(start int) ([]Attr, bool) {
			if start != 4 {
				return nil, false
			}
			return known(start)
		}
	}
	src := "[a](x.md) [b](y)"
	page := Page{PageID: uuid.New()}
	id := page.PageID.String()
	got := renderWith(t, page, src, md, pathLinks("any"))
	want := `<p><a data-md="4 ` + id + `">a</a> <a data-any="14 ` + id + `">b</a></p>` + "\n"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if got, want := renderWith(t, page, src, md), `<p><a data-md="4 `+id+`">a</a> <a href="y">b</a></p>`+"\n"; got != want {
		t.Errorf("one extension: got  %q\nwant %q", got, want)
	}
	if got, want := renderWith(t, page, src), "<p><a href=\"x.md\">a</a> <a href=\"y\">b</a></p>\n"; got != want {
		t.Errorf("none: got  %q\nwant %q", got, want)
	}
}

// HeadingID is the id of the first heading of a text.
func TestHeadingIDIsAHeadingsID(t *testing.T) {
	texts := []string{
		"Hello World", "a_b--c  d", "中文 标题", "Ünïcode ŝtring", "!!!", strings.Repeat("abcdefghij ", 10), "İstanbul",
	}
	m := newMarkdown(t)
	for _, text := range texts {
		got := renderString(t, m, "# "+text+"\n")
		if want := `<h1 id="` + HeadingID(text) + `">`; !strings.HasPrefix(got, want) {
			t.Errorf("# %s: %q, want it to start %q", text, got, want)
		}
	}
	if got := HeadingID("A b"); got != "nw-a-b" {
		t.Errorf("HeadingID(%q) = %q", "A b", got)
	}
}
