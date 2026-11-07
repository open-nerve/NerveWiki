package obsidian

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// The real parser and extraction body run here. Only the private range state
// is supplied by the test, so it can observe work that output equality cannot.
func observedRangeExtraction(t *testing.T, content string) (Extracted, *linkRanges) {
	t.Helper()
	ranges := &linkRanges{}
	values := harden.NewParser(dialect()...)
	ext := Extension(Options{})
	ext.Extract = func(tree markdown.Tree) any {
		return extractWithRanges(tree, values, ranges)
	}
	md, err := markdown.New([]markdown.Extension{tasks.Extension(), ext})
	if err != nil {
		t.Fatal(err)
	}
	return md.Parse([]byte(content)).Facts().Extracted(Name).(Extracted), ranges
}

// A supplied tree permits literal negative and zero offsets, which remain
// valid keys of the extractor even though a normal source has positive spans.
func literalRangeExtraction(links []Link) (Extracted, *linkRanges) {
	root := ast.NewDocument()
	body := ast.NewParagraph()
	root.AppendChild(root, body)
	for _, l := range links {
		n := &wikilink{
			embed: l.Kind == KindEmbed, inTable: l.InTable,
			parts: parts{target: l.Target, anchor: l.Anchor, display: l.Display, at: l.Range},
		}
		body.AppendChild(body, n)
	}
	ranges := &linkRanges{}
	return extractWithRanges(markdown.Tree{Root: root}, nil, ranges), ranges
}

func TestExtractRangesGrowStrictlyWithoutSeen(t *testing.T) {
	const n = 1025
	var source strings.Builder
	want := Extracted{Links: make([]Link, 0, n)}
	for i := range n {
		target := fmt.Sprintf("v%04d", i)
		// Each literal is ten bytes: [] followed by (v0000). The independent
		// range arithmetic does not ask a parser, extractor or sorter.
		source.WriteString("[](" + target + ")\n")
		want.Links = append(want.Links, Link{Kind: KindLink, Target: target, Range: markdown.Span{Start: 10*i + 3, Stop: 10*i + 8}})
	}
	got, ranges := observedRangeExtraction(t, source.String())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("complete increasing extraction = %+v, want %+v", got, want)
	}
	if ranges.seen != nil {
		t.Fatalf("strictly increasing extraction built seen for %d ranges", len(ranges.seen))
	}
	if !ranges.started || ranges.last != 10*(n-1)+3 {
		t.Fatalf("range fast path did not observe the final input: %+v", ranges)
	}
}

func TestExtractRangesPreserveLiteralOrderAndFirstSpan(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    []Link
		want     []Link
		fallback bool
	}{
		{
			name: "negative_zero_increasing",
			input: []Link{
				{Kind: KindWikilink, Target: "negative", Range: markdown.Span{Start: -5, Stop: -2}},
				{Kind: KindEmbed, Target: "zero", Range: markdown.Span{Start: 0, Stop: 0}},
				{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 3, Stop: 2}},
			},
			want: []Link{
				{Kind: KindWikilink, Target: "negative", Range: markdown.Span{Start: -5, Stop: -2}},
				{Kind: KindEmbed, Target: "zero", Range: markdown.Span{Start: 0, Stop: 0}},
				{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 3, Stop: 2}},
			},
		},
		{
			name: "same_start_different_end",
			input: []Link{
				{Kind: KindWikilink, Target: "first", Anchor: "h", Display: "shown", Range: markdown.Span{Start: 0, Stop: 1}},
				{Kind: KindEmbed, Target: "other-end", InTable: true, Range: markdown.Span{Start: 0, Stop: 2}},
				{Kind: KindEmbed, Target: "wrong-first", Range: markdown.Span{Start: 0, Stop: 1}},
				{Kind: KindWikilink, Target: "earlier", Range: markdown.Span{Start: -2, Stop: 0}},
				{Kind: KindWikilink, Target: "wrong-other", Range: markdown.Span{Start: 0, Stop: 2}},
				{Kind: KindWikilink, Target: "tail", Range: markdown.Span{Start: 5, Stop: 9}},
			},
			want: []Link{
				{Kind: KindWikilink, Target: "earlier", Range: markdown.Span{Start: -2, Stop: 0}},
				{Kind: KindWikilink, Target: "first", Anchor: "h", Display: "shown", Range: markdown.Span{Start: 0, Stop: 1}},
				{Kind: KindEmbed, Target: "other-end", InTable: true, Range: markdown.Span{Start: 0, Stop: 2}},
				{Kind: KindWikilink, Target: "tail", Range: markdown.Span{Start: 5, Stop: 9}},
			},
			fallback: true,
		},
		{
			name: "zero_first_then_negative",
			input: []Link{
				{Kind: KindEmbed, Target: "zero-first", Range: markdown.Span{Start: 0, Stop: 0}},
				{Kind: KindWikilink, Target: "negative", Range: markdown.Span{Start: -2, Stop: -1}},
				{Kind: KindWikilink, Target: "wrong-zero", Range: markdown.Span{Start: 0, Stop: 0}},
				{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 2, Stop: 3}},
			},
			want: []Link{
				{Kind: KindWikilink, Target: "negative", Range: markdown.Span{Start: -2, Stop: -1}},
				{Kind: KindEmbed, Target: "zero-first", Range: markdown.Span{Start: 0, Stop: 0}},
				{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 2, Stop: 3}},
			},
			fallback: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ranges := literalRangeExtraction(tc.input)
			if want := (Extracted{Links: tc.want}); !reflect.DeepEqual(got, want) {
				t.Fatalf("literal extraction = %+v, want %+v", got, want)
			}
			if tc.fallback != (ranges.seen != nil) {
				t.Fatalf("fallback map present=%t, want %t", ranges.seen != nil, tc.fallback)
			}
			if tc.fallback {
				if len(ranges.seen) != len(tc.want) {
					t.Fatalf("seen has %d complete ranges, want %d", len(ranges.seen), len(tc.want))
				}
				for _, l := range tc.want {
					if !ranges.seen[l.Range] {
						t.Fatalf("seen omitted literal range %+v", l.Range)
					}
				}
			}
		})
	}
}

func TestExtractRangesBackfillEveryStagingBlock(t *testing.T) {
	const n = 1025
	input := make([]Link, 0, n+15)
	for i := range n {
		input = append(input, Link{Kind: KindWikilink, Target: fmt.Sprintf("keep-%04d", i), Range: markdown.Span{Start: 2 * i, Stop: 2*i + 1}})
	}
	// Completed small/full blocks and both entries of the current tail.
	// Every duplicate changes kind and target, so retaining it is observable.
	for _, i := range []int{0, 254, 255, 510, 511, 766, 767, 1022, 1023, 1024} {
		input = append(input, Link{Kind: KindEmbed, Target: "wrong-duplicate", Range: markdown.Span{Start: 2 * i, Stop: 2*i + 1}})
	}
	input = append(input,
		Link{Kind: KindEmbed, Target: "other-end", Range: markdown.Span{Start: 1000, Stop: 1002}},
		Link{Kind: KindWikilink, Target: "earlier", Range: markdown.Span{Start: -1, Stop: 0}},
		Link{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 2050, Stop: 2052}},
		Link{Kind: KindWikilink, Target: "wrong-other-end", Range: markdown.Span{Start: 1000, Stop: 1002}},
	)
	want := Extracted{Links: []Link{{Kind: KindWikilink, Target: "earlier", Range: markdown.Span{Start: -1, Stop: 0}}}}
	for i := range n {
		want.Links = append(want.Links, Link{Kind: KindWikilink, Target: fmt.Sprintf("keep-%04d", i), Range: markdown.Span{Start: 2 * i, Stop: 2*i + 1}})
		if i == 500 {
			want.Links = append(want.Links, Link{Kind: KindEmbed, Target: "other-end", Range: markdown.Span{Start: 1000, Stop: 1002}})
		}
	}
	want.Links = append(want.Links, Link{Kind: KindWikilink, Target: "later", Range: markdown.Span{Start: 2050, Stop: 2052}})
	got, ranges := literalRangeExtraction(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-block extraction = %+v, want %+v", got, want)
	}
	if len(ranges.seen) != n+3 {
		t.Fatalf("backfilled %d ranges, want all %d", len(ranges.seen), n+3)
	}
	for _, l := range want.Links {
		if !ranges.seen[l.Range] {
			t.Fatalf("backfill omitted complete range %+v", l.Range)
		}
	}
}

func TestExtractRangesKeepBodyFrontmatterAndReferenceGolden(t *testing.T) {
	const content = "---\nsource: '[[front#top|Front]]'\n---\n[first][shared]\n\n[](body#h) ![again][shared]\n\n[shared]: <dest#anchor>\n"
	want := Extracted{Links: []Link{
		{Kind: KindWikilink, Target: "front", Anchor: "top", Display: "Front", Key: "source", Range: markdown.Span{Start: 15, Stop: 20}},
		{Kind: KindLink, Target: "body", Anchor: "h", Range: markdown.Span{Start: 58, Stop: 62}},
		{Kind: KindLink, Target: "dest", Anchor: "anchor", Range: markdown.Span{Start: 95, Stop: 99}},
	}}
	got, ranges := observedRangeExtraction(t, content)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body/frontmatter/reference extraction = %+v, want literal %+v", got, want)
	}
	if len(ranges.seen) != 3 {
		t.Fatalf("reference/body/frontmatter fallback kept %d ranges, want 3", len(ranges.seen))
	}
}

func TestExtractRangesKeepFactsIndependentAcrossParses(t *testing.T) {
	md, err := markdown.New([]markdown.Extension{tasks.Extension(), Extension(Options{})})
	if err != nil {
		t.Fatal(err)
	}
	first := md.Parse([]byte("[](one) ![a](two)")).Facts().Extracted(Name).(Extracted)
	wantFirst := Extracted{Links: []Link{
		{Kind: KindLink, Target: "one", Range: markdown.Span{Start: 3, Stop: 6}},
		{Kind: KindImage, Target: "two", Range: markdown.Span{Start: 13, Stop: 16}},
	}}
	second := md.Parse([]byte("[[third]]")).Facts().Extracted(Name).(Extracted)
	wantSecond := Extracted{Links: []Link{{Kind: KindWikilink, Target: "third", Range: markdown.Span{Start: 2, Stop: 7}}}}
	if !reflect.DeepEqual(first, wantFirst) || !reflect.DeepEqual(second, wantSecond) {
		t.Fatalf("successive facts changed: first=%+v second=%+v", first, second)
	}
}
