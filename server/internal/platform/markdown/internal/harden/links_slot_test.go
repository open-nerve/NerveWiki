package harden

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type labelSlotOpening struct {
	node  *label
	start int
	image bool
}

// labelSlotTrace replaces the registered links parser with a forwarding
// observer. Retaining every returned label prevents allocator address reuse
// from looking like reuse by the parser. CloseBlock and Trigger are promoted
// from the real parser, and the observer does not mutate a node or context.
type labelSlotTrace struct {
	links
	t         *testing.T
	installed int
	opened    []labelSlotOpening
}

func (r *labelSlotTrace) SetParserOption(c *parser.Config) {
	for i := range c.InlineParsers {
		if _, ok := c.InlineParsers[i].Value.(links); ok {
			c.InlineParsers[i].Value = r
			r.installed++
		}
	}
}

func (r *labelSlotTrace) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	n := r.links.Parse(parent, block, pc)
	if l, ok := n.(*label); ok {
		// Goldmark assigns Pos and attaches the opening node after Parse
		// returns. A borrowed node must look newly constructed here.
		if l.Pos() != -1 || l.Parent() != nil || l.PreviousSibling() != nil || l.NextSibling() != nil || l.HasChildren() {
			r.t.Fatalf("opening label retained AST state: pos=%d parent=%v", l.Pos(), l.Parent())
		}
		r.opened = append(r.opened, labelSlotOpening{l, l.seg.Start, l.isImage})
	}
	return n
}

func labelSlotMarkdown(t *testing.T) (goldmark.Markdown, *labelSlotTrace) {
	t.Helper()
	m := hardened()
	r := &labelSlotTrace{t: t}
	m.Parser().AddOptions(r)
	if r.installed != 1 {
		t.Fatalf("forwarding observer replaced %d links parsers, want 1", r.installed)
	}
	return m, r
}

func labelSlotRender(t *testing.T, m goldmark.Markdown, source []byte, root ast.Node) string {
	t.Helper()
	var out bytes.Buffer
	if err := m.Renderer().Render(&out, source, root); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func labelSlotParse(t *testing.T, m goldmark.Markdown, pc parser.Context, source string) (ast.Node, string) {
	t.Helper()
	src := []byte(source)
	root := m.Parser().Parse(text.NewReader(src), parser.WithContext(pc))
	return root, labelSlotRender(t, m, src, root)
}

func labelSlotIDs(opened []labelSlotOpening) []int {
	ids := map[*label]int{}
	var out []int
	for _, o := range opened {
		id, ok := ids[o.node]
		if !ok {
			id = len(ids)
			ids[o.node] = id
		}
		out = append(out, id)
	}
	return out
}

type labelSlotDestination struct {
	kind        ast.NodeKind
	pos         int
	target      string
	start, stop int
}

func labelSlotDestinations(t *testing.T, root ast.Node, pc parser.Context) []labelSlotDestination {
	t.Helper()
	destination := Destinations(pc)
	var out []labelSlotDestination
	if err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var target string
		switch n := n.(type) {
		case *ast.Link:
			target = string(n.Destination)
		case *ast.Image:
			target = string(n.Destination)
		default:
			return ast.WalkContinue, nil
		}
		seg, ok := destination(n)
		if !ok {
			t.Fatalf("link at %d has no written destination", n.Pos())
		}
		out = append(out, labelSlotDestination{n.Kind(), n.Pos(), target, seg.Start, seg.Stop})
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLabelSlotAllocatesOneTemporaryLabelForSuccessiveLinks(t *testing.T) {
	m, trace := labelSlotMarkdown(t)
	src := strings.Repeat("[x](one)![y](two)", 64)
	_, got := labelSlotParse(t, m, parser.NewContext(), src)
	if want := render(t, original(), src); got != want {
		t.Fatalf("rendered=%q want=%q", got, want)
	}
	if len(trace.opened) != 128 {
		t.Fatalf("opened %d labels, want 128", len(trace.opened))
	}
	for i, id := range labelSlotIDs(trace.opened) {
		if id != 0 {
			t.Fatalf("opening %d used temporary label object %d, want the first object", i, id)
		}
	}
}

func TestLabelSlotPreservesRenderingAndDestinationPositions(t *testing.T) {
	for _, tc := range []struct {
		name, src, html string
		ids             []int
		positions       []int
		images          []bool
		destinations    []labelSlotDestination
	}{
		{
			name: "consecutive_links_images_and_references",
			src:  "[a](one) ![b](two) [c][r] ![d][r]\n\n[r]: dest\n",
			html: "<p><a href=\"one\">a</a> <img src=\"two\" alt=\"b\"> <a href=\"dest\">c</a> <img src=\"dest\" alt=\"d\"></p>\n",
			ids:  []int{0, 0, 0, 0}, positions: []int{0, 9, 19, 26}, images: []bool{false, true, false, true},
			destinations: []labelSlotDestination{
				{ast.KindLink, 0, "one", 4, 7}, {ast.KindImage, 9, "two", 14, 17},
				{ast.KindLink, 19, "dest", 40, 44}, {ast.KindImage, 26, "dest", 40, 44},
			},
		},
		{
			name: "nested_image_closes_then_slot_is_reused",
			src:  "[seed](s) ![outer [inner](one)](two) [ok](three)\n",
			html: "<p><a href=\"s\">seed</a> <img src=\"two\" alt=\"outer inner\"> <a href=\"three\">ok</a></p>\n",
			ids:  []int{0, 0, 1, 0}, positions: []int{0, 10, 18, 37}, images: []bool{false, true, false, false},
			destinations: []labelSlotDestination{
				{ast.KindLink, 0, "s", 7, 8}, {ast.KindImage, 10, "two", 32, 35},
				{ast.KindLink, 18, "one", 26, 29}, {ast.KindLink, 37, "three", 42, 47},
			},
		},
		{
			name: "failed_outer_link_is_text_before_later_success",
			src:  "[seed](s) [outer [inner](one) tail](two) [ok](three)\n",
			html: "<p><a href=\"s\">seed</a> [outer <a href=\"one\">inner</a> tail](two) <a href=\"three\">ok</a></p>\n",
			ids:  []int{0, 0, 1, 2}, positions: []int{0, 10, 17, 41}, images: []bool{false, false, false, false},
			destinations: []labelSlotDestination{
				{ast.KindLink, 0, "s", 7, 8}, {ast.KindLink, 17, "one", 25, 28},
				{ast.KindLink, 41, "three", 46, 51},
			},
		},
		{
			name: "two_unclosed_labels_survive_inner_successes_until_close_block",
			src:  "[seed](s) [outer [middle ![inner](one) [ok](two)\n\n[last](three)\n",
			html: "<p><a href=\"s\">seed</a> [outer [middle <img src=\"one\" alt=\"inner\"> <a href=\"two\">ok</a></p>\n<p><a href=\"three\">last</a></p>\n",
			ids:  []int{0, 0, 1, 2, 3, 4}, positions: []int{0, 10, 17, 25, 39, 50}, images: []bool{false, false, false, true, false, false},
			destinations: []labelSlotDestination{
				{ast.KindLink, 0, "s", 7, 8}, {ast.KindImage, 25, "one", 34, 37},
				{ast.KindLink, 39, "two", 44, 47}, {ast.KindLink, 50, "three", 57, 62},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, trace := labelSlotMarkdown(t)
			pc := parser.NewContext()
			root, got := labelSlotParse(t, m, pc, tc.src)
			if got != tc.html {
				t.Fatalf("rendered=%q want=%q", got, tc.html)
			}
			if want := render(t, original(), tc.src); got != want {
				t.Fatalf("rendered=%q original Goldmark=%q", got, want)
			}
			if got := labelSlotIDs(trace.opened); !slices.Equal(got, tc.ids) {
				t.Fatalf("temporary label identities=%v want=%v", got, tc.ids)
			}
			var positions []int
			var images []bool
			for _, o := range trace.opened {
				positions = append(positions, o.start)
				images = append(images, o.image)
			}
			if !slices.Equal(positions, tc.positions) || !slices.Equal(images, tc.images) {
				t.Fatalf("opening positions=%v images=%v want=%v %v", positions, images, tc.positions, tc.images)
			}
			if got := labelSlotDestinations(t, root, pc); !slices.Equal(got, tc.destinations) {
				t.Fatalf("destinations=%+v want=%+v", got, tc.destinations)
			}
		})
	}
}

func TestLabelSlotIsScopedToParseContext(t *testing.T) {
	m, trace := labelSlotMarkdown(t)
	pc := parser.NewContext()
	first, html := labelSlotParse(t, m, pc, "[a](one)")
	const firstHTML = "<p><a href=\"one\">a</a></p>\n"
	if html != firstHTML {
		t.Fatalf("first render=%q want=%q", html, firstHTML)
	}
	second, html := labelSlotParse(t, m, pc, "![b](two) [c](three)")
	if want := "<p><img src=\"two\" alt=\"b\"> <a href=\"three\">c</a></p>\n"; html != want {
		t.Fatalf("second render=%q want=%q", html, want)
	}
	if got, want := labelSlotDestinations(t, second, pc), []labelSlotDestination{
		{ast.KindImage, 0, "two", 5, 8}, {ast.KindLink, 10, "three", 14, 19},
	}; !slices.Equal(got, want) {
		t.Fatalf("second destinations=%+v want=%+v", got, want)
	}
	other := parser.NewContext()
	_, html = labelSlotParse(t, m, other, "[d](four)")
	if want := "<p><a href=\"four\">d</a></p>\n"; html != want {
		t.Fatalf("other-context render=%q want=%q", html, want)
	}
	_, html = labelSlotParse(t, m, pc, "[e](five)")
	if want := "<p><a href=\"five\">e</a></p>\n"; html != want {
		t.Fatalf("returned-context render=%q want=%q", html, want)
	}
	if got, want := labelSlotIDs(trace.opened), []int{0, 0, 0, 1, 0}; !slices.Equal(got, want) {
		t.Fatalf("label identities across reused/distinct contexts=%v want=%v", got, want)
	}
	if got := labelSlotRender(t, m, []byte("[a](one)"), first); got != firstHTML {
		t.Fatalf("old tree changed after context reuse: %q", got)
	}
	if got, want := labelSlotDestinations(t, first, pc), []labelSlotDestination{{ast.KindLink, 0, "one", 4, 7}}; !slices.Equal(got, want) {
		t.Fatalf("old destinations changed after context reuse: %+v want=%+v", got, want)
	}
}
