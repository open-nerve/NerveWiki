package obsidian

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Keep the actual tree/destination map for repeated extraction, outside the
// allocation measurement. No product hook or alternate extractor is used.
func extractCapacityTree(t *testing.T, content string) (markdown.Tree, parser.Parser, Extracted) {
	t.Helper()
	values := harden.NewParser(dialect()...)
	var tree markdown.Tree
	ext := Extension(Options{})
	ext.Extract = func(tr markdown.Tree) any {
		tree = tr
		return extract(tr, values)
	}
	md, err := markdown.New([]markdown.Extension{ext})
	if err != nil {
		t.Fatal(err)
	}
	got := md.Parse([]byte(content)).Facts().Extracted(Name).(Extracted)
	return tree, values, got
}

func equalCapacityFacts(t *testing.T, got, want Extracted) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	if len(got.Links) == len(want.Links) {
		for i := range got.Links {
			if got.Links[i] != want.Links[i] {
				t.Fatalf("complete fact %d = %+v, want %+v", i, got.Links[i], want.Links[i])
			}
		}
	}
	t.Fatalf("complete facts differ: links %d/%d nil %t/%t; tags %+v/%+v", len(got.Links), len(want.Links), got.Links == nil, want.Links == nil, got.Tags, want.Tags)
}

func TestExtractCapacitySemantics(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		want          Extracted
	}{
		{"empty", "", Extracted{}},
		{"external_anchor_empty", "[](https://x) [](#h) []()", Extracted{}},
		{"decode_and_literal_colon", "[](1x:y) [](a%20b#h%20i) [](bad%zz)", Extracted{Links: []Link{
			{Kind: KindLink, Target: "1x:y", Range: markdown.Span{Start: 3, Stop: 7}},
			{Kind: KindLink, Target: "a b", Anchor: "h i", Range: markdown.Span{Start: 12, Stop: 17}},
			{Kind: KindLink, Target: "bad%zz", Range: markdown.Span{Start: 28, Stop: 34}},
		}}},
		{"hidden_fact", "%%[[secret]]%%\n", Extracted{Links: []Link{
			{Kind: KindWikilink, Target: "secret", Range: markdown.Span{Start: 4, Stop: 10}},
		}}},
		{"image_in_link", "[![pic](image.md)](page.md)", Extracted{Links: []Link{
			{Kind: KindImage, Target: "image.md", Range: markdown.Span{Start: 8, Stop: 16}},
			{Kind: KindLink, Target: "page.md", Range: markdown.Span{Start: 19, Stop: 26}},
		}}},
		{"skip_image_caption", "![ [[secret]] ](image.md)", Extracted{Links: []Link{
			{Kind: KindImage, Target: "image.md", Range: markdown.Span{Start: 16, Stop: 24}},
		}}},
		{"property_and_body", "---\np: '[[prop]]'\n---\n[](body)", Extracted{Links: []Link{
			{Kind: KindWikilink, Target: "prop", Key: "p", Range: markdown.Span{Start: 10, Stop: 14}},
			{Kind: KindLink, Target: "body", Range: markdown.Span{Start: 25, Stop: 29}},
		}}},
		{"reference_first_range", "[a][r] ![b][r]\n\n[r]: dest\n", Extracted{Links: []Link{
			{Kind: KindLink, Target: "dest", Range: markdown.Span{Start: 21, Stop: 25}},
		}}},
		{"tags_unchanged", "[[first]] #tag", Extracted{Links: []Link{
			{Kind: KindWikilink, Target: "first", Range: markdown.Span{Start: 2, Stop: 7}},
		}, Tags: []Tag{{Name: "tag", Range: markdown.Span{Start: 10, Stop: 14}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree, values, got := extractCapacityTree(t, tc.content)
			equalCapacityFacts(t, got, tc.want)
			equalCapacityFacts(t, extract(tree, values).(Extracted), tc.want)
			if string(tree.Content) != tc.content {
				t.Fatal("extraction changed the source bytes")
			}
		})
	}
	for _, n := range []int{1, 255, 256, 257, 10000} {
		t.Run(fmt.Sprintf("dense_%d", n), func(t *testing.T) {
			tree, values, got := extractCapacityTree(t, strings.Repeat("[](aa.a)", n))
			want := capacityDenseFacts(n, 0)
			equalCapacityFacts(t, got, want)
			equalCapacityFacts(t, extract(tree, values).(Extracted), want)
			if cap(got.Links) != len(got.Links) {
				t.Fatalf("kept dense facts retain capacity %d for %d facts", cap(got.Links), len(got.Links))
			}
		})
	}
	t.Run("custom_hidden_root_and_duplicates", func(t *testing.T) {
		parent := ast.NewDocument()
		root := &capacityWrapper{}
		parent.AppendChild(parent, root)
		outside := &wikilink{parts: parts{target: "outside", at: markdown.Span{Start: 30, Stop: 37}}}
		parent.AppendChild(parent, outside)
		first := &wikilink{parts: parts{target: "first", at: markdown.Span{Start: 10, Stop: 15}}}
		root.AppendChild(root, first)
		h := &hidden{}
		root.AppendChild(root, h)
		h.AppendChild(h, &wikilink{embed: true, parts: parts{target: "hidden", at: markdown.Span{Start: -1, Stop: 5}}})
		image := ast.NewImage(ast.NewLink())
		image.AppendChild(image, &wikilink{parts: parts{target: "caption-leak", at: markdown.Span{Start: 0, Stop: 1}}})
		root.AppendChild(root, image)
		root.AppendChild(root, &wikilink{parts: parts{target: "wrong-first", at: first.at}})
		root.AppendChild(root, &wikilink{parts: parts{target: "same-start", at: markdown.Span{Start: 10, Stop: 16}}})
		got := extract(markdown.Tree{Root: root}, nil).(Extracted)
		equalCapacityFacts(t, got, Extracted{Links: []Link{
			{Kind: KindEmbed, Target: "hidden", Range: markdown.Span{Start: -1, Stop: 5}},
			{Kind: KindWikilink, Target: "first", Range: markdown.Span{Start: 10, Stop: 15}},
			{Kind: KindWikilink, Target: "same-start", Range: markdown.Span{Start: 10, Stop: 16}},
		}})
	})
}

// A distinct extension node type: traversal may not infer absence from its kind.
type capacityWrapper struct{ ast.BaseBlock }

func (*capacityWrapper) Kind() ast.NodeKind { return ast.KindBlockquote }
func (n *capacityWrapper) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Each literal [] (without that space) plus (aa.a) is exactly eight bytes.
// This independent arithmetic does not call the parser or the range allocator.
func capacityDenseFacts(n, offset int) Extracted {
	out := Extracted{Links: make([]Link, n)}
	for i := range n {
		out.Links[i] = Link{Kind: KindLink, Target: "aa.a", Range: markdown.Span{Start: offset + 8*i + 3, Stop: offset + 8*i + 7}}
	}
	return out
}
