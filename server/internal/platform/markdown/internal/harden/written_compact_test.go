package harden

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func writtenCompactWant(t *testing.T, lookup func(ast.Node) (text.Segment, bool), n ast.Node, want text.Segment, found bool) {
	t.Helper()
	got, ok := lookup(n)
	if got != want || ok != found {
		t.Fatalf("destination of %T = %+v, %v; want %+v, %v", n, got, ok, want, found)
	}
}

func TestWrittenCompactKeepsNilKindsDistinct(t *testing.T) {
	pc := parser.NewContext()
	lookup := Destinations(pc)
	w := writtenOf(pc)
	var link *ast.Link
	var image *ast.Image
	w.inline(link, text.Segment{Start: 1, Stop: 4})
	w.inline(image, text.Segment{Start: 7, Stop: 11})
	w.inline(nil, text.Segment{Start: 20, Stop: 24})
	writtenCompactWant(t, lookup, nil, text.Segment{Start: 20, Stop: 24}, true)
	writtenCompactWant(t, lookup, image, text.Segment{Start: 7, Stop: 11}, true)
	writtenCompactWant(t, lookup, link, text.Segment{Start: 1, Stop: 4}, true)
	writtenCompactWant(t, lookup, ast.NewLink(), text.Segment{}, false)
	writtenCompactWant(t, lookup, ast.NewImage(ast.NewLink()), text.Segment{}, false)
}

func TestWrittenCompactUpdatesAndEmptyNoop(t *testing.T) {
	for _, tc := range []struct {
		name string
		node ast.Node
	}{
		{"link", ast.NewLink()},
		{"image", ast.NewImage(ast.NewLink())},
		{"other", ast.NewParagraph()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pc := parser.NewContext()
			lookup := Destinations(pc) // captured before the first record
			w := writtenOf(pc)
			w.inline(tc.node, text.Segment{Start: 3, Stop: 3})
			writtenCompactWant(t, lookup, tc.node, text.Segment{}, false)
			w.inline(tc.node, text.Segment{Start: 2, Stop: 9})
			writtenCompactWant(t, lookup, tc.node, text.Segment{Start: 2, Stop: 9}, true)
			full := text.Segment{Start: -3, Stop: 4, Padding: 2, ForceNewline: true}
			w.inline(tc.node, full)
			writtenCompactWant(t, lookup, tc.node, full, true)
			switch n := tc.node.(type) {
			case *ast.Link:
				if _, ok := w.links[n]; ok {
					t.Fatal("compact link entry retained after full write")
				}
			case *ast.Image:
				if _, ok := w.images[n]; ok {
					t.Fatal("compact image entry retained after full write")
				}
			}
			w.inline(tc.node, text.Segment{Start: 11, Stop: 19})
			writtenCompactWant(t, lookup, tc.node, text.Segment{Start: 11, Stop: 19}, true)
			if _, stale := w.at[tc.node]; stale && tc.name != "other" {
				t.Fatal("full entry retained after compact write")
			}
			w.inline(tc.node, text.Segment{Start: 30, Stop: 30})
			w.inline(tc.node, text.Segment{Start: 30, Stop: 29})
			tc.node.SetPos(-99)
			writtenCompactWant(t, lookup, tc.node, text.Segment{Start: 11, Stop: 19}, true)
			writtenCompactWant(t, Destinations(pc), tc.node, text.Segment{Start: 11, Stop: 19}, true)
		})
	}
}

func TestWrittenCompactPreservesFullSegments(t *testing.T) {
	cases := []text.Segment{
		{Start: 0, Stop: 1},
		{Start: -4, Stop: 3},
		{Start: -9, Stop: -2},
		{Start: 5, Stop: 5, Padding: 2},
		{Start: 3, Stop: 9, Padding: 2},
		{Start: 3, Stop: 9, Padding: -2},
		{Start: 3, Stop: 9, ForceNewline: true},
		{Start: 0, Stop: 1, Padding: 4, ForceNewline: true},
	}
	if strconv.IntSize == 64 {
		wide := uint64(1) << 32
		cases = append(cases,
			text.Segment{Start: int(wide) - 2, Stop: int(wide) - 1},
			text.Segment{Start: int(wide), Stop: int(wide) + 3},
			text.Segment{Start: 1, Stop: int(wide)},
		)
	}
	for i, want := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			pc := parser.NewContext()
			for _, n := range []ast.Node{ast.NewLink(), ast.NewImage(ast.NewLink()), ast.NewParagraph()} {
				writtenOf(pc).inline(n, want)
				writtenCompactWant(t, Destinations(pc), n, want, true)
			}
		})
	}
}

func TestWrittenCompactKeepsReferencesAndInlinePriority(t *testing.T) {
	pc := parser.NewContext()
	lookup := Destinations(pc)
	w := writtenOf(pc)
	n := ast.NewLink()
	w.reference(n, []byte("MiXeD"))
	writtenCompactWant(t, lookup, n, text.Segment{}, false)
	w.define([]byte("mixed"), text.Segment{Start: 40, Stop: 48, Padding: 2, ForceNewline: true})
	w.define([]byte("MIXED"), text.Segment{Start: 90, Stop: 99})
	writtenCompactWant(t, lookup, n, text.Segment{Start: 40, Stop: 48, Padding: 2, ForceNewline: true}, true)
	w.inline(n, text.Segment{Start: 3, Stop: 7})
	w.reference(n, []byte("missing"))
	writtenCompactWant(t, lookup, n, text.Segment{Start: 3, Stop: 7}, true)
	empty := ast.NewImage(ast.NewLink())
	w.reference(empty, []byte("empty"))
	w.define([]byte("EMPTY"), text.Segment{Start: 8, Stop: 8})
	w.define([]byte("empty"), text.Segment{Start: 10, Stop: 20})
	writtenCompactWant(t, lookup, empty, text.Segment{Start: 8, Stop: 8}, false)
}

func writtenCompactNodes(root ast.Node) []ast.Node {
	var out []ast.Node
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.(type) {
			case *ast.Link, *ast.Image:
				out = append(out, n)
			}
		}
		return ast.WalkContinue, nil
	})
	return out
}

func TestWrittenCompactParserGoldenAndContextReuse(t *testing.T) {
	const source = "[α](one) ![b](<two>) [c][Ref] ![d][ref] [empty]()\n\n[REF]: <first>\n[ref]: second\n"
	md := goldmark.New(goldmark.WithParser(NewParser()))
	pc := parser.NewContext()
	lookup := Destinations(pc)
	root := md.Parser().Parse(text.NewReader([]byte(source)), parser.WithContext(pc))
	nodes := writtenCompactNodes(root)
	want := []struct {
		kind    ast.NodeKind
		pos     int
		segment text.Segment
		found   bool
	}{
		{ast.KindLink, 0, text.Segment{Start: 5, Stop: 8}, true},
		{ast.KindImage, 10, text.Segment{Start: 16, Stop: 19}, true},
		{ast.KindLink, 22, text.Segment{Start: 60, Stop: 65}, true},
		{ast.KindImage, 31, text.Segment{Start: 60, Stop: 65}, true},
		{ast.KindLink, 41, text.Segment{}, false},
	}
	if len(nodes) != len(want) {
		t.Fatalf("nodes=%d want=%d", len(nodes), len(want))
	}
	for i, n := range nodes {
		if n.Kind() != want[i].kind || n.Pos() != want[i].pos || n.Attributes() != nil {
			t.Fatalf("node %d: kind=%v pos=%d attrs=%v", i, n.Kind(), n.Pos(), n.Attributes())
		}
		writtenCompactWant(t, lookup, n, want[i].segment, want[i].found)
	}
	var html bytes.Buffer
	if err := md.Renderer().Render(&html, []byte(source), root); err != nil {
		t.Fatal(err)
	}
	const rendered = "<p><a href=\"one\">α</a> <img src=\"two\" alt=\"b\"> <a href=\"first\">c</a> <img src=\"first\" alt=\"d\"> <a href=\"\">empty</a></p>\n"
	if html.String() != rendered {
		t.Fatalf("rendered=%q want=%q", html.String(), rendered)
	}
	nodes[0].SetAttributeString("data-user", "kept")
	for _, i := range []int{3, 0, 4, 2, 1, 0, 3} {
		nodes[i].SetPos(-100 - i)
		writtenCompactWant(t, lookup, nodes[i], want[i].segment, want[i].found)
	}
	copy := *nodes[0].(*ast.Link)
	writtenCompactWant(t, lookup, &copy, text.Segment{}, false)
	second := md.Parser().Parse(text.NewReader([]byte("[x](later)")), parser.WithContext(pc))
	writtenCompactWant(t, lookup, writtenCompactNodes(second)[0], text.Segment{Start: 4, Stop: 9}, true)
	other := md.Parser().Parse(text.NewReader([]byte("[x](later)")), parser.WithContext(parser.NewContext()))
	writtenCompactWant(t, lookup, writtenCompactNodes(other)[0], text.Segment{}, false)
	for i, n := range nodes {
		writtenCompactWant(t, lookup, n, want[i].segment, want[i].found)
	}
	if v, ok := nodes[0].AttributeString("data-user"); !ok || v != "kept" {
		t.Fatalf("user attribute changed: %v, %v", v, ok)
	}
}

func TestWrittenCompactUsesTypedMapsForParsedInlineRanges(t *testing.T) {
	const count = 1025
	pc := parser.NewContext()
	root := NewParser().Parse(text.NewReader([]byte(strings.Repeat("[](a) ![](b)\n", count))), parser.WithContext(pc))
	nodes := writtenCompactNodes(root)
	if len(nodes) != 2*count {
		t.Fatalf("parsed nodes=%d want=%d", len(nodes), 2*count)
	}
	lookup := Destinations(pc)
	for i := range count {
		writtenCompactWant(t, lookup, nodes[2*i], text.Segment{Start: 13*i + 3, Stop: 13*i + 4}, true)
		writtenCompactWant(t, lookup, nodes[2*i+1], text.Segment{Start: 13*i + 10, Stop: 13*i + 11}, true)
	}
	w := writtenOf(pc)
	if w.at != nil || len(w.links) != count || len(w.images) != count {
		t.Fatalf("parsed inline destinations used full map: full=%d links=%d images=%d", len(w.at), len(w.links), len(w.images))
	}
}
