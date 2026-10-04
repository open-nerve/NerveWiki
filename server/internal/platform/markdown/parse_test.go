package markdown

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func newMarkdown(t *testing.T, exts ...Extension) *Markdown {
	t.Helper()
	m, err := New(exts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// kinds is the tree's node kinds in order, its frontmatter's absence shown.
func kinds(d *Document) string {
	var out []string
	_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() != ast.KindDocument {
			out = append(out, n.Kind().String())
		}
		return ast.WalkContinue, nil
	})
	return strings.Join(out, " ")
}

// The frontmatter never reaches the Markdown: the body parses as it parses
// alone, at the content's offsets.
func TestTheFrontmatterNeverTouchesTheBody(t *testing.T) {
	m := newMarkdown(t)
	tests := []struct{ name, src, body string }{
		{"a fence in the YAML", "---\na: |\n  ```\n---\nbody\n", "body\n"},
		{"a heading in the YAML", "---\n# a\n---\n## b\n", "## b\n"},
		{"a setext underline right after", "---\na: 1\n---\n===\n", "===\n"},
		{"a byte order mark first", "\xef\xbb\xbf---\na: 1\n---\n# t\n", "# t\n"},
		{"a byte order mark without a frontmatter", "\xef\xbb\xbf# t\n", "# t\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, want := m.Parse([]byte(tt.src)), m.Parse([]byte(tt.body))
			if kinds(got) != kinds(want) {
				t.Errorf("tree %q, the body alone %q", kinds(got), kinds(want))
			}
			offset := len(tt.src) - len(tt.body)
			first := func(d *Document) *ast.Text {
				var found *ast.Text
				_ = ast.Walk(d.root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
					if v, ok := n.(*ast.Text); ok && entering && found == nil {
						found = v
					}
					return ast.WalkContinue, nil
				})
				return found
			}
			if g, w := first(got), first(want); g == nil || g.Segment.Start != w.Segment.Start+offset ||
				!bytes.Equal(g.Segment.Value([]byte(tt.src)), w.Segment.Value([]byte(tt.body))) {
				t.Errorf("the first text is not the body's at the content's offset")
			}
		})
	}
}

func TestParseReadsTheFrontmatter(t *testing.T) {
	m := newMarkdown(t)
	tests := []struct {
		name, src      string
		present, valid bool
		props          int
	}{
		{"none", "body", false, false, 0},
		{"valid", "---\na: 1\nb: 2\n---\nbody", true, true, 2},
		{"empty", "---\n---\nbody", true, true, 0},
		{"not a mapping", "---\n- a\n---\nbody", true, false, 0},
		{"unclosed", "---\na: 1\nbody", false, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := m.Parse([]byte(tt.src)).Frontmatter()
			if fm.Present != tt.present || fm.Valid != tt.valid || len(fm.Properties) != tt.props {
				t.Errorf("frontmatter = %+v", fm)
			}
		})
	}
}

func TestHeadingsGetPrefixedUniqueIDs(t *testing.T) {
	m := newMarkdown(t)
	src := "# Hello *World*\n# hello world\n# Hello World-1\n# 中文 标题！\n# ***\n# \n" +
		"# a_b--c  d\n# [link](http://x.y) `code`\n# " + strings.Repeat("abcdefghij ", 10) + "\n" +
		"Setext\n===\n> # Quoted\n- # Listed\n\nx[^1]\n\n[^1]: # Noted\n"
	var ids []string
	_ = ast.Walk(m.Parse([]byte(src)).root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			v, _ := h.AttributeString("id")
			ids = append(ids, string(v.([]byte)))
		}
		return ast.WalkContinue, nil
	})
	want := []string{
		"nw-hello-world", "nw-hello-world-1", "nw-hello-world-1-1", "nw-中文-标题", "nw-section", "nw-section-1",
		"nw-a-b-c-d", "nw-link-code", "nw-abcdefghij-abcdefghij-abcdefghij-abcdefghij-abcdefghij-abcdefghi",
		"nw-setext", "nw-quoted", "nw-listed", "nw-noted",
	}
	if strings.Join(ids, " ") != strings.Join(want, " ") {
		t.Errorf("ids\n%q\nwant\n%q", ids, want)
	}
}

// word is a test double of an extension: "@@word@@" is a node whose word
// it extracts.
type wordNode struct {
	ast.BaseInline
	word []byte
}

var kindWord = ast.NewNodeKind("TestWord") //nolint:gochecknoglobals // a kind is made once

func (n *wordNode) Kind() ast.NodeKind            { return kindWord }
func (n *wordNode) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type wordParser struct{}

func (wordParser) Trigger() []byte { return []byte{'@'} }

func (wordParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("@@")) {
		return nil
	}
	end := bytes.Index(line[2:], []byte("@@"))
	if end < 0 {
		return nil
	}
	n := &wordNode{word: bytes.Clone(line[2 : 2+end])}
	block.Advance(end + 4)
	return n
}

func words() Extension {
	return Extension{
		Name:   "words",
		Parser: []parser.Option{parser.WithInlineParsers(util.Prioritized(wordParser{}, 50))},
		Extract: func(t Tree) any {
			var out []string
			_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if w, ok := n.(*wordNode); ok && entering {
					out = append(out, string(w.word))
				}
				return ast.WalkContinue, nil
			})
			return out
		},
	}
}

func TestAnExtensionParsesAndExtracts(t *testing.T) {
	m := newMarkdown(t, words())
	d := m.Parse([]byte("---\na: \"@@no@@\"\n---\nsay @@hello@@ and *@@world@@*\n"))
	if got, _ := d.Extracted("words").([]string); strings.Join(got, ",") != "hello,world" {
		t.Errorf("extracted %q", got)
	}
	if got := newMarkdown(t).Parse([]byte("say @@hello@@")).Extracted("words"); got != nil {
		t.Errorf("without the extension, extracted %v", got)
	}
}

// Extract gets the content as it was written, the frontmatter with it, and
// the tree's offsets are the content's.
func TestAnExtensionExtractsFromTheContent(t *testing.T) {
	ext := words()
	ext.Extract = func(t Tree) any {
		var at []string
		_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if txt, ok := n.(*ast.Text); ok && entering {
				at = append(at, string(t.Content[txt.Segment.Start:txt.Segment.Stop]))
			}
			return ast.WalkContinue, nil
		})
		return string(t.Content[:3]) + strings.Join(at, "|")
	}
	m := newMarkdown(t, ext)
	content := "---\na: 1\n---\nsay *hi*\n"
	if got := m.Parse([]byte(content)).Extracted("words"); got != "---say |hi" {
		t.Errorf("extracted %q", got)
	}
}

func TestNewTurnsDownNamesMissingOrTwice(t *testing.T) {
	if _, err := New([]Extension{{}}); err == nil {
		t.Error("an extension without a name")
	}
	if _, err := New([]Extension{words(), words()}); err == nil {
		t.Error("two extensions named alike")
	}
}
