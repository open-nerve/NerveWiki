// Package markdowntest holds what tests of Markdown use: ordinary and
// pathological documents of a given size (M4/P3 design 3.10) and the
// fixture set. Only test code may import it (enforced by internal/archtest).
package markdowntest

import (
	"fmt"
	"strings"
)

// ordinary is a few kilobytes of what pages hold: headings, paragraphs with
// emphasis, links and code, lists, tasks, a table, a quote, fenced code, a
// footnote and a little inline HTML, in English and Chinese.
const ordinary = `## Section heading

Some *emphasis*, **strong** and ~~struck~~ words, ` + "`inline code`" + `, a [link](other-page.md "title"),
an autolink <https://example.com/path> and www.example.org, and a footnote[^n].
中文的段落也有**加粗**、*斜体*与[链接](../上一页.md)，还有 <kbd>Ctrl</kbd> 这样的标签。

- an item with [a reference][ref]
- [ ] a task
- [x] a done task
  1. nested ordered
  2. second

> A quote with *emphasis*
> over two lines.

| Column | Other |
|:-------|------:|
| cell   | 1.5   |
| ` + "`code`" + ` | **b** |

` + "```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```" + `

[ref]: https://example.com/ref
[^n]: The footnote's text.

`

// Normal is an ordinary document of about size bytes.
func Normal(size int) string {
	var b strings.Builder
	for b.Len() < size {
		b.WriteString(ordinary)
	}
	return b.String()
}

// Input makes a document of about size bytes of one kind.
type Input struct {
	Name string
	Make func(size int) string
}

func repeat(unit string) func(int) string {
	return func(size int) string { return strings.Repeat(unit, max(1, size/len(unit))) }
}

// Pathological are the inputs whose parse goldmark makes quadratic, or that
// stress a limit: each must cost about what an ordinary document of its size
// costs.
func Pathological() []Input {
	return []Input{
		{"mismatched emphasis a*_", repeat("a*_")},
		{"mismatched emphasis *a_", repeat("*a_ ")},
		{"closers in multiples of three", func(n int) string { return "a**b" + repeat("c* ")(n) }},
		{"runs of three a***", repeat("a***")},
		{"strikethrough openers ~~a", repeat("~~a")},
		{"strikethrough closers a~", repeat("a~")},
		{"mismatched strikethrough ~a~~", repeat("~a~~")},
		{"emphasis in links", repeat("[*a*](b) ")},
		{"code spans `a", repeat("`a")},
		{"code span openers with no closer ``a`", repeat("``a`")},
		{"backtick strings of every length", func(n int) string {
			var b strings.Builder
			for i := 1; b.Len() < n; i++ {
				b.WriteString("e")
				b.WriteString(strings.Repeat("`", i))
			}
			return b.String()
		}},
		{"e-mail runs _www", repeat("_www")},
		{"link destinations [a](", repeat("[a](")},
		{"image destinations ![a](", repeat("![a](")},
		{"bare destinations [a](b", repeat("[a](b")},
		{"angle destinations [a](<b", repeat("[a](<b")},
		{"destinations then a long run", func(n int) string { return strings.Repeat("[a](", 1000) + strings.Repeat("b", n) }},
		{"brackets [ (](", repeat("[ (](")},
		{"images ![[]()", repeat("![[]()")},
		{"nested brackets", func(n int) string { return strings.Repeat("[", n/2) + "a" + strings.Repeat("]", n/2) }},
		{"link openers [a", repeat("[a")},
		{"unclosed comments <!--", func(n int) string { return "</" + repeat("<!--")(n) }},
		{"unclosed instructions <?", func(n int) string { return "a" + repeat("<?")(n) }},
		{"unclosed declarations <!A", func(n int) string { return "a" + repeat("<!A")(n) }},
		{"unclosed CDATA", func(n int) string { return "a" + repeat("<![CDATA[")(n) }},
		{"nested block quotes", func(n int) string { return repeat("> ")(n) + "a\n" }},
		{"nested list markers", func(n int) string { return repeat("- ")(n) + "a\n" }},
		{"nested quotes and lists", func(n int) string { return repeat("> - ")(n) + "a\n" }},
		{"nested lists by indentation", func(n int) string {
			var b strings.Builder
			for i := 0; b.Len() < n; i++ {
				b.WriteString(strings.Repeat(" ", 2*(i%200)))
				b.WriteString("* a\n")
			}
			return b.String()
		}},
		{"shortcut labels on every line", repeat("[a]\n")},
		{"reference definitions and uses", func(n int) string {
			var defs, uses strings.Builder
			for i := 0; defs.Len()+uses.Len() < n; i++ {
				fmt.Fprintf(&defs, "[%d]: u\n", i)
				fmt.Fprintf(&uses, "[%d] ", i)
			}
			return defs.String() + "\n" + uses.String()
		}},
		{"footnote references and definitions", func(n int) string {
			var refs, defs strings.Builder
			for i := 0; refs.Len()+defs.Len() < n; i++ {
				fmt.Fprintf(&refs, "x[^%d] ", i)
				fmt.Fprintf(&defs, "[^%d]: d\n", i)
			}
			return refs.String() + "\n\n" + defs.String()
		}},
		{"link titles on every line", repeat("[a](b \"c\")\n")},
		{"one long line", repeat("ab ")},
	}
}
