package harden

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func TestLinkCounterDoesNotAllocatePerSuccessfulLink(t *testing.T) {
	const count = 10_000
	const maxMallocs = 5_000
	source := []byte(strings.Repeat("[]()", count))
	wantHTML := "<p>" + strings.Repeat(`<a href=""></a>`, count) + "</p>\n"
	m := hardened()
	p := m.Parser()
	// Initialize the registered parser before measuring a fresh context's
	// complete parse, including its nodes and all AST transformers.
	p.Parse(text.NewReader(nil))
	pc := parser.NewContext()
	reader := text.NewReader(source)
	withContext := parser.WithContext(pc)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	root := p.Parse(reader, withContext)
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(root)
	mallocs := after.Mallocs - before.Mallocs

	// These checks are outside the measured interval and use literal
	// expectations, so omitting real links cannot satisfy the cost guard.
	lookup := Destinations(pc)
	links := 0
	if err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		if got, want := link.Pos(), 4*links; got != want {
			t.Fatalf("link %d position=%d want=%d", links, got, want)
		}
		if len(link.Destination) != 0 {
			t.Fatalf("link %d destination=%q want empty", links, link.Destination)
		}
		if seg, ok := lookup(link); ok {
			t.Fatalf("link %d has a destination record %+v for an empty destination", links, seg)
		}
		links++
		return ast.WalkContinue, nil
	}); err != nil {
		t.Fatal(err)
	}
	if links != count {
		t.Fatalf("parsed %d actual Link nodes, want %d", links, count)
	}
	var out bytes.Buffer
	if err := m.Renderer().Render(&out, source, root); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != wantHTML {
		t.Fatalf("HTML differs from the literal %d-link paragraph: got %d bytes, want %d", count, len(got), len(wantHTML))
	}
	// This broad half-per-link ceiling is fixed before measurement. In
	// particular, boxing a new count after every successful link exceeds it.
	t.Logf("LINK_COUNTER_ALLOC links=%d mallocs=%d ceiling=%d", links, mallocs, maxMallocs)
	if mallocs >= maxMallocs {
		t.Fatalf("LINK_COUNTER_ALLOC: complete parse allocated %d times for %d links, want < %d", mallocs, count, maxMallocs)
	}
}

func TestLinkCounterPreservesLiteralSemanticsAtPrefixes(t *testing.T) {
	for _, prefix := range []struct {
		name  string
		count int
	}{{"0", 0}, {"256", 256}, {"10001", 10_001}} {
		t.Run(prefix.name, func(t *testing.T) {
			sourcePrefix := strings.Repeat("[]()", prefix.count)
			htmlPrefix := "<p>" + strings.Repeat(`<a href=""></a>`, prefix.count)
			for _, tc := range []struct {
				name, source, html string
				made               int
			}{
				{
					name:   "nested_link",
					source: "[outer [inner](one) tail](two) [ok](three)",
					html:   `[outer <a href="one">inner</a> tail](two) <a href="three">ok</a>`,
					made:   2,
				},
				{
					name:   "image_inside_link",
					source: "[outer ![inner](one) tail](two) [ok](three)",
					html:   `<a href="two">outer <img src="one" alt="inner"> tail</a> <a href="three">ok</a>`,
					made:   2,
				},
				{
					name:   "failed_label_before_success",
					source: "[outer [missing] tail](two) [ok](three)",
					html:   `<a href="two">outer [missing] tail</a> <a href="three">ok</a>`,
					made:   2,
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					m := hardened()
					pc := parser.NewContext()
					_, got := labelSlotParse(t, m, pc, sourcePrefix+tc.source)
					if want := htmlPrefix + tc.html + "</p>\n"; got != want {
						t.Fatalf("LINK_COUNTER_LITERAL: suffix=%q want=%q after %d links", strings.TrimPrefix(got, htmlPrefix), tc.html+"</p>\n", prefix.count)
					}
					if got, want := linksMade(pc), prefix.count+tc.made; got != want {
						t.Fatalf("successful link count=%d want=%d", got, want)
					}
				})
			}
		})
	}
}

func TestLinkCounterStartsLazilyAndPersistsPerContext(t *testing.T) {
	m := hardened()
	pc := parser.NewContext()
	if got := linksMade(pc); got != 0 || pc.Get(linksKey) != nil {
		t.Fatalf("fresh context has count=%d or allocated counter", got)
	}
	checkParse := func(context parser.Context, source, wantHTML string, wantCount int) {
		t.Helper()
		_, got := labelSlotParse(t, m, context, source)
		if got != wantHTML {
			t.Fatalf("context parse differs from literal HTML: got %q want %q", got, wantHTML)
		}
		if got := linksMade(context); got != wantCount {
			t.Fatalf("context count=%d want=%d", got, wantCount)
		}
	}
	for _, tc := range []struct{ source, html string }{
		{"plain text", "<p>plain text</p>\n"},
		{"[missing]", "<p>[missing]</p>\n"},
		{"![image](one)", "<p><img src=\"one\" alt=\"image\"></p>\n"},
	} {
		checkParse(pc, tc.source, tc.html, 0)
		if pc.Get(linksKey) != nil {
			t.Fatalf("%q allocated a counter without a successful Link", tc.source)
		}
	}
	checkParse(pc, strings.Repeat("[]()", 256), "<p>"+strings.Repeat(`<a href=""></a>`, 256)+"</p>\n", 256)
	checkParse(pc, "[next](two) [missing] ![image](three)", "<p><a href=\"two\">next</a> [missing] <img src=\"three\" alt=\"image\"></p>\n", 257)

	other := parser.NewContext()
	if got := linksMade(other); got != 0 || other.Get(linksKey) != nil {
		t.Fatalf("independent context has count=%d or allocated counter", got)
	}
	checkParse(other, "[outer [inner](other)](ignored)", "<p>[outer <a href=\"other\">inner</a>](ignored)</p>\n", 1)
	if got := linksMade(pc); got != 257 {
		t.Fatalf("independent parse changed retained context count to %d, want 257", got)
	}
	checkParse(pc, "[last](four)", "<p><a href=\"four\">last</a></p>\n", 258)
	if got := linksMade(other); got != 1 {
		t.Fatalf("reused context changed independent count to %d, want 1", got)
	}
}
