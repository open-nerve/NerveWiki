package markdown

import (
	"reflect"
	"testing"

	"github.com/yuin/goldmark/ast"
)

// treeOf is the Tree an extension's Extract gets for content.
func treeOf(t *testing.T, content string) Tree {
	t.Helper()
	var got Tree
	m, err := New([]Extension{{Name: "tree", Extract: func(tr Tree) any { got = tr; return nil }}})
	if err != nil {
		t.Fatal(err)
	}
	m.Parse([]byte(content))
	return got
}

// written is the bytes each link's and image's destination is written in,
// in the tree's order.
func written(t *testing.T, content string) []string {
	t.Helper()
	tr := treeOf(t, content)
	var out []string
	_ = ast.Walk(tr.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n.(type) {
		case *ast.Link, *ast.Image:
			if entering {
				s, ok := tr.Destination(n)
				if !ok {
					out = append(out, "-")
				} else {
					out = append(out, string(tr.Content[s.Start:s.Stop]))
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return out
}

// A link's or an image's destination is where it is written: its own,
// inside angle brackets, or its definition's (M6/P1 design 3.2).
func TestADestinationIsWhereItIsWritten(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		want          []string
	}{
		{"inline", "[a](b.md \"t\") ![i](c%20d.png)\n", []string{"b.md", "c%20d.png"}},
		{"angle brackets", "[a](<b c.md>)\n", []string{"b c.md"}},
		{"empty", "[a]() [b](<>)\n", []string{"-", "-"}},
		{"after a frontmatter and multibyte text", "---\nk: v\n---\n中文 [a](目标.md#h)\n", []string{"目标.md#h"}},
		{"full, collapsed and shortcut references", "[a][r] [r][] [r]\n\n[r]: <r.md> 'title'\n", []string{"r.md", "r.md", "r.md"}},
		{"a reference image", "![i][p]\n\n[p]: p.png\n", []string{"p.png"}},
		{"the first definition of a label", "[a][R]\n\n[r]: first.md\n[R]: second.md\n", []string{"first.md"}},
		{"a definition's destination on the next line", "[a]\n\n[a]:\n  next.md\n", []string{"next.md"}},
		{"a definition in a list", "- [a]\n\n  [a]: in.md\n", []string{"in.md"}},
		{"CRLF", "[a](b.md)\r\n[c][d]\r\n\r\n[d]: e.md\r\n", []string{"b.md", "e.md"}},
		{"BOM", "\ufeff[a](b.md)\n", []string{"b.md"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := written(t, tt.content); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("written %q, want %q", got, tt.want)
			}
		})
	}
}

// noted is a Scalar as the tests write it: its bytes as the content
// writes them, and each byte's offset.
type noted struct {
	Path, Value, Written string
	Quote                byte
	Offsets              []int
}

func scalarsOf(t *testing.T, content string) []noted {
	t.Helper()
	fm := treeOf(t, content).Frontmatter
	out := make([]noted, 0, len(fm.Scalars))
	for _, s := range fm.Scalars {
		offsets := make([]int, len(s.Value)+1)
		for i := range offsets {
			offsets[i] = s.Offset(i)
		}
		out = append(out, noted{
			Path: s.Path, Value: s.Value, Written: content[s.Offset(0):s.Offset(len(s.Value))],
			Quote: s.Quote, Offsets: offsets,
		})
	}
	return out
}

// A frontmatter's strings written on one line are where they are written,
// each byte's offset the content's (M6/P1 design 3.2).
func TestAFrontmattersStringsAreWhereTheyAreWritten(t *testing.T) {
	for _, tt := range []struct {
		name, content string
		want          []noted
	}{
		{"plain", "---\na: x y\n---\n", []noted{{"a", "x y", "x y", 0, []int{7, 8, 9, 10}}}},
		{"single quotes", "---\na: 'Bob''s'\n---\n", []noted{{"a", "Bob's", "Bob''s", '\'', []int{8, 9, 10, 11, 13, 14}}}},
		{"double quotes and escapes", "---\na: \"\\x41\\\"b\"\n---\n", []noted{{"a", "A\"b", "\\x41\\\"b", '"', []int{8, 12, 14, 15}}}},
		{"a multibyte escape", "---\na: \"\\u4e2d\"\n---\n", []noted{{"a", "中", "\\u4e2d", '"', []int{8, 8, 8, 14}}}},
		{"after multibyte text", "---\n中文: '[[x]]'\n---\n", []noted{{"中文", "[[x]]", "[[x]]", '\'', []int{13, 14, 15, 16, 17, 18}}}},
		{"nested paths", "---\nl:\n  - m: v\n  - [p, \"q\"]\n---\n", []noted{
			{"l.0.m", "v", "v", 0, []int{14, 15}},
			{"l.1.0", "p", "p", 0, []int{21, 22}},
			{"l.1.1", "q", "q", '"', []int{25, 26}},
		}},
		{"CRLF", "---\r\na: x\r\nb: 'y'\r\n---\r\n", []noted{{"a", "x", "x", 0, []int{8, 9}}, {"b", "y", "y", '\'', []int{15, 16}}}},
		{"BOM", "\ufeff---\na: x\n---\n", []noted{{"a", "x", "x", 0, []int{10, 11}}}},
		{"not strings", "---\na: 1\nb: true\nc: null\nd: 2024-01-02\n---\n", []noted{{"d", "2024-01-02", "2024-01-02", 0, []int{28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38}}}},
		{"an alias repeats a string where it is written once", "---\na: &x v\nb: *x\nc: !!str w\n---\n", []noted{{"a", "v", "v", 0, []int{10, 11}}, {"c", "w", "w", 0, []int{27, 28}}}},
		{"its anchor and tag on a line of their own", "---\na: &x\n  '[[p]]'\nb: !!str\n  q\n---\n", []noted{
			{"a", "[[p]]", "[[p]]", '\'', []int{13, 14, 15, 16, 17, 18}}, {"b", "q", "q", 0, []int{31, 32}},
		}},
		{"many on a line", "---\na: [x, 'y', \"z\"]\n---\n", []noted{
			{"a.0", "x", "x", 0, []int{8, 9}}, {"a.1", "y", "y", '\'', []int{12, 13}}, {"a.2", "z", "z", '"', []int{17, 18}},
		}},
		{"after the YAML library's other line breaks", "---\na: \"p\u0085q\"\rb: x\u2028c: y\u2029d: z\n---\n", []noted{
			{"b", "x", "x", 0, []int{17, 18}}, {"c", "y", "y", 0, []int{24, 25}}, {"d", "z", "z", 0, []int{31, 32}},
		}},
		{"over lines", "---\na: \"x\n  y\"\nb: |\n  z\nc: >-\n  w\nd: plain\n  more\n---\n", nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarsOf(t, tt.content)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("scalars\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// A frontmatter that is not valid has no scalars.
func TestAnInvalidFrontmatterHasNoScalars(t *testing.T) {
	if got := treeOf(t, "---\na: x\na: y\n---\n").Frontmatter; got.Valid || len(got.Scalars) != 0 {
		t.Errorf("frontmatter %+v", got)
	}
}
