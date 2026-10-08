package markdown

import (
	"context"
	"html"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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
	view, err := m.Render(context.Background(), m.Parse([]byte(src)), page)
	if err != nil {
		t.Fatal(err)
	}
	return view.HTML
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
			view, err := m.Render(context.Background(), m.Parse([]byte(tt.src)), page)
			if err != nil {
				t.Fatal(err)
			}
			out := view.HTML
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
// case letters, digits and '-', an address only through SafeURL (P3B
// review L2), and a src only when it is a path of this site from its root
// (M7/P3 review B2).
func TestALinksAttributesAreCheckedAsWritten(t *testing.T) {
	ext := Extension{Name: "q", Links: func(any) func(int) ([]Attr, bool) {
		return func(start int) ([]Attr, bool) {
			if start == 4 {
				return []Attr{
					{Name: "href", Value: "/ok?a=1&b"}, {Name: "src", Value: "//evil.example"}, {Name: "src", Value: "https://x.example/i.png"},
					{Name: "src", Value: "i.png"}, {Name: "src", Value: "?i"}, {Name: "src", Value: "/i.png?a&b"},
				}, true
			}
			return []Attr{
				{Name: "href", Value: "javascript:alert(1)"}, {Name: `onclick="x" data-a`, Value: "v"},
				{Name: "Data-A", Value: "v"}, {Name: "data-b2", Value: "v"},
			}, true
		}
	}}
	want := `<p><a href="/ok?a=1&amp;b" src="/i.png?a&amp;b">a</a> <span class="nw-image">i <a data-b2="v">y</a></span></p>` + "\n"
	if got := renderWith(t, Page{}, "[a](x) ![i](y)", ext); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// What an extension's Links gives a link is asked once, as it opens, an
// image's in a link too: the extension may count what it writes (M7/P3
// review B1).
func TestALinksAttributesAreAskedOnce(t *testing.T) {
	asked := map[int]int{}
	ext := Extension{Name: "q", Links: func(any) func(int) ([]Attr, bool) {
		return func(start int) ([]Attr, bool) {
			asked[start]++
			return []Attr{{Name: "href", Value: "/" + strconv.Itoa(start)}}, start != 24
		}
	}}
	got := renderWith(t, Page{}, "[a](x) [![i](y)](z) [b](w)", ext)
	want := `<p><a href="/4">a</a> <a href="/17"><span class="nw-image">i</span></a> <a href="w">b</a></p>` + "\n"
	if got != want || len(asked) != 3 || asked[4] != 1 || asked[17] != 1 || asked[24] != 1 {
		t.Errorf("got  %q\nwant %q, asked %v", got, want, asked)
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

// images is an extension's test double whose Images writes the images whose
// destinations start at the starts it is given: a <b> with the text the
// image shows, whether it is in a link, and what Fetch got; and whose
// Expires is expires.
func images(name string, expires time.Time, starts ...int) Extension {
	return Extension{
		Name:  name,
		Fetch: func(context.Context, Page, any) (any, error) { return name, nil },
		Images: func(data any) func(int) (Image, bool) {
			return func(start int) (Image, bool) {
				if !slices.Contains(starts, start) {
					return nil, false
				}
				return func(w Writer, shown string, inLink bool) {
					_, _ = w.WriteString("<b")
					WriteAttrs(w, []Attr{{Name: "data-by", Value: data.(string)}, {Name: "data-in-link", Value: strconv.FormatBool(inLink)}})
					_, _ = w.WriteString(">" + html.EscapeString(shown) + "</b>")
				}, true
			}
		},
		Expires: func(any) time.Time { return expires },
	}
}

// An extension's Images writes the Markdown images it knows, by where
// their destinations start, the first registered that knows one; the
// others are the core's (M7/P3 design 5.2).
func TestTheImagesAnExtensionKnowsAreWrittenByIt(t *testing.T) {
	tests := []struct {
		name, src string
		a, b      string // the destinations each knows
		want      string
	}{
		{"an image", "![i <&>](x.png)", "x.png", "", `<p><b data-by="a" data-in-link="false">i &lt;&amp;&gt;</b></p>` + "\n"},
		{"an image of its caption's emphasis", "![*i*](x.png)", "x.png", "", `<p><b data-by="a" data-in-link="false">i</b></p>` + "\n"},
		{"an image in a link", "[![i](x.png)](y)", "x.png", "", `<p><a href="y"><b data-by="a" data-in-link="true">i</b></a></p>` + "\n"},
		{
			"a reference image's uses, by the definition", "![a][r] ![b][r]\n\n[r]: x.png\n", "x.png", "",
			`<p><b data-by="a" data-in-link="false">a</b> <b data-by="a" data-in-link="false">b</b></p>` + "\n",
		},
		{"an image both know", "![i](x.png)", "x.png", "x.png", `<p><b data-by="a" data-in-link="false">i</b></p>` + "\n"},
		{"an image the second knows", "![i](x.png)", "", "x.png", `<p><b data-by="b" data-in-link="false">i</b></p>` + "\n"},
		{"an image none knows", "![i](x.png)", "", "", `<p><span class="nw-image">i <a href="x.png">x.png</a></span></p>` + "\n"},
		{"a link", "[i](x.png)", "x.png", "", `<p><a href="x.png">i</a></p>` + "\n"},
	}
	starts := func(src, dest string) []int {
		var out []int
		for at := 0; dest != "" && strings.Contains(src[at:], dest); at += strings.Index(src[at:], dest) + 1 {
			out = append(out, at+strings.Index(src[at:], dest))
		}
		return out
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := images("a", time.Time{}, starts(tt.src, tt.a)...), images("b", time.Time{}, starts(tt.src, tt.b)...)
			if got := renderWith(t, Page{}, tt.src, a, b); got != tt.want {
				t.Errorf("render %q\n got %q\nwant %q", tt.src, got, tt.want)
			}
		})
	}
}

// A view expires at the earliest of its extensions' Expires, none of them
// zero: never (M7/P3 design 5.2). Each is asked once the view is written,
// so that it may tell of what it wrote alone (M7/P3 review A1).
func TestAViewExpiresAtItsExtensionsEarliest(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		exts []Extension
		want time.Time
	}{
		{"none", nil, time.Time{}},
		{"never", []Extension{images("a", time.Time{})}, time.Time{}},
		{"one", []Extension{images("a", at)}, at},
		{"the earliest", []Extension{images("a", at.Add(time.Hour)), images("b", at), images("c", at.Add(2*time.Hour))}, at},
		{"never and one", []Extension{images("a", time.Time{}), images("b", at), images("c", time.Time{})}, at},
		{"of what it wrote", []Extension{writtenExpires("a", at, 7), writtenExpires("b", at.Add(-time.Hour), 99)}, at},
		{"nothing written", []Extension{writtenExpires("a", at, 99)}, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMarkdown(t, tt.exts...)
			view, err := m.Render(context.Background(), m.Parse([]byte("a ![i](x.png)")), Page{})
			if err != nil {
				t.Fatal(err)
			}
			if !view.Expires.Equal(tt.want) {
				t.Errorf("Expires = %v, want %v", view.Expires, tt.want)
			}
		})
	}
}

// writtenExpires is an extension's test double whose Images writes the
// image whose destination starts at start, and whose Expires is expires
// once it has written it, else never.
func writtenExpires(name string, expires time.Time, start int) Extension {
	ext := images(name, time.Time{}, start)
	ext.Fetch = func(context.Context, Page, any) (any, error) { return new(bool), nil }
	written := ext.Images
	ext.Images = func(data any) func(int) (Image, bool) {
		known := written(name)
		return func(at int) (Image, bool) {
			img, ok := known(at)
			if !ok {
				return nil, false
			}
			return func(w Writer, shown string, inLink bool) {
				*data.(*bool) = true
				img(w, shown, inLink)
			}, true
		}
	}
	ext.Expires = func(data any) time.Time {
		if *data.(*bool) {
			return expires
		}
		return time.Time{}
	}
	return ext
}
