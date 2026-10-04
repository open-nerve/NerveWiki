// Package markdowntest holds what tests of Markdown use: ordinary and
// pathological documents of a given size, the check of a reading view's
// HTML (M4/P3 design 3.10) and the fixture set. Only test code may import it
// (enforced by internal/archtest).
package markdowntest

import (
	"fmt"
	"strings"
)

// ordinary is a few kilobytes of what pages hold: headings, paragraphs with
// emphasis, links and code, lists, tasks, a table, a quote, fenced code, a
// footnote, a little inline HTML and Obsidian's dialect, in English and
// Chinese.
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

Obsidian's dialect: [[Another page]], [[页面#小节|别名]], ![[image.png|100]], #tag and #标签/子标签,
==highlighted==, $e^{i\pi} + 1 = 0$ and a %%hidden%% comment.

> [!note]- A callout
> with [[a link]].

$$
\int_0^1 x\,dx
$$

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

// Pathological are the inputs whose parse goldmark makes quadratic, that
// stress a limit, or that load the frontmatter's YAML, the sanitizer or the
// addresses: each must cost about what an ordinary document of its size
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
		{"\"www.\" that is no link _www.1", repeat("_www.1")},
		{"\"www.\" after strikethrough ~www.1", repeat("~www.1")},
		{"an e-mail run to an '@' with no domain a*", func(n int) string { return repeat("a*")(n) + "@" }},
		{"an e-mail run of code spans to an '@' a`", func(n int) string { return repeat("a`")(n) + "@" }},
		{"an e-mail run to a domain with no '.' a*", func(n int) string { return repeat("a*")(n) + "@b" }},
		{"an e-mail run to a domain then '-' a_", func(n int) string { return repeat("a_")(n) + "@a.b-" }},
		{"an e-mail run to a domain then '_' a*", func(n int) string { return repeat("a*")(n) + "@a.b_" }},
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
		{"nested footnote definitions", func(n int) string { return repeat("[^a]: ")(n) + "x\n" }},
		{"task items - [ ] a", repeat("- [ ] a\n")},
		{"task items in quotes > - [x]", repeat("> - [x] \t \n")},
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
		{"reference definitions with a title on the next line, then text", func(n int) string {
			var b strings.Builder
			b.WriteString("[a0]: /u\n\"t\n")
			for i := 1; b.Len() < n; i++ {
				fmt.Fprintf(&b, "t\" [a%d]: /u\n\"t\n", i)
			}
			return b.String()
		}},
		{"a long title, then reference definitions in its text", func(n int) string {
			var b strings.Builder
			b.WriteString("[a]: /u\n\"" + strings.Repeat("t\n", n/4))
			for i := 0; b.Len() < n; i++ {
				fmt.Fprintf(&b, "t\" [b%d]: /u\n\"t\n", i)
			}
			return b.String()
		}},
		{"table cells of escaped pipes in code", func(n int) string { return "| a |\n|---|\n" + repeat("|`\\|`|\n")(n) }},
		{"footnote references and definitions", func(n int) string {
			var refs, defs strings.Builder
			for i := 0; refs.Len()+defs.Len() < n; i++ {
				fmt.Fprintf(&refs, "x[^%d] ", i)
				fmt.Fprintf(&defs, "[^%d]: d\n", i)
			}
			return refs.String() + "\n\n" + defs.String()
		}},
		{"link titles on every line", repeat("[a](b \"c\")\n")},
		{"headings alike", repeat("# a\n")},
		{"headings alike and their ids", repeat("# a\n# a-1\n# a 2\n")},
		{"one long line", repeat("ab ")},

		{"a frontmatter of aliases", func(n int) string {
			var b strings.Builder
			b.WriteString("---\na0: &a0 [x, x]\n")
			for i := 1; b.Len() < n; i++ {
				fmt.Fprintf(&b, "a%d: &a%d [*a%d, *a%d]\n", i, i, i-1, i-1)
			}
			return b.String() + "---\nbody\n"
		}},
		{"a frontmatter nested deep", func(n int) string {
			return "---\na: " + strings.Repeat("[", n/2) + strings.Repeat("]", n/2) + "\n---\nbody\n"
		}},
		{"a frontmatter of many keys", func(n int) string {
			var b strings.Builder
			b.WriteString("---\n")
			for i := 0; b.Len() < n; i++ {
				fmt.Fprintf(&b, "k%d: [v, %d, {a: \"b\"}]\n", i, i)
			}
			return b.String() + "---\nbody\n"
		}},
		{"a frontmatter's strings on one line", func(n int) string {
			return "---\na: [" + repeat("xxxx, 'y', ")(n) + "]\n---\nbody\n"
		}},
		{"a frontmatter of one long value", func(n int) string { return "---\na: " + strings.Repeat("x", n) + "\n---\nbody\n" }},
		{"a frontmatter of one long number", func(n int) string { return "---\na: " + strings.Repeat("9", n) + "\n---\nbody\n" }},
		{"inline tags <b>", repeat("<b>")},
		{"end tags after start tags", func(n int) string { return repeat("<b>")(n/2) + repeat("</i>")(n/2) }},
		{"inline tags with attributes", repeat(`<a href="/p" title="t" x=y>`)},
		{"a tag of many attributes", func(n int) string { return "<b" + repeat(" a=1")(n) + ">" }},
		{"tags nested in emphasis", func(n int) string { return repeat("*<a>")(n/2) + "x" + repeat("*")(n/2) }},
		{"HTML blocks", repeat("<div>\n\n")},
		{"an HTML block nested deep", func(n int) string { return "<div>\n" + repeat("<div>")(n) + "\n" }},
		{"an HTML block of script", func(n int) string { return "<script>\n" + repeat("a<b>")(n) + "\n</script>\n" }},
		{"links with long addresses", repeat("[a](/" + strings.Repeat("p", 200) + ") ")},
		{"autolinks", repeat("<https://example.com/a> www.example.com a@b.co ")},
		{"images in links", repeat("[![a](i.png)](/p) ")},
		{"addresses in tags", repeat(`<a href="http://a/\b?c#d">x</a>`)},

		// Obsidian's dialect (M6/P1 design 5).
		{"wikilink openers [[a", repeat("[[a")},
		{"embed openers ![[a", repeat("![[a ")},
		{"a wikilink open to the line's end", func(n int) string { return "[[" + repeat("a")(n) }},
		{"wikilinks and their display texts", repeat("[[a#b|c]] ")},
		{"a '$' a line", repeat("$a\n")},
		{"formula openers $a", repeat("$a ")},
		{"a block formula left open", func(n int) string { return "$$\n" + repeat("a [[b]] #c\n")(n) }},
		{"inline display formulas $$a", repeat("$$a ")},
		{"comment markers %%", repeat("%% ")},
		{"a block comment left open", func(n int) string { return "%%\n" + repeat("a [[b]] #c\n\n")(n) }},
		{"block comments", repeat("%%\na\n%%\n\n")},
		{"comments across emphasis", repeat("*a %%b* c%% ")},
		{"a comment deep in emphasis", func(n int) string {
			return repeat("*a ")(n/4) + repeat("%%b%% ")(n/4) + repeat(" a*")(n/4)
		}},
		{"comments climbing emphasis", func(n int) string { return repeat("%%*a ")(n/2) + repeat("a*%% ")(n/2) }},
		{"callouts", repeat("> [!note] a\n\n")},
		{"callouts nested deep", func(n int) string { return repeat("> ")(n) + "[!note] a\n" }},
		{"tags", repeat("#tag ")},
		{"tags after runs left as text", repeat("a*#t ")},
		{"a tag's underscores", func(n int) string { return "#a" + repeat("_")(n) }},
		{"highlight runs ==a", repeat("==a")},
		{"highlights", repeat("==a== ")},
	}
}

// Amplifying are the inputs whose HTML goldmark or YAML's aliases make
// grow faster than the input: a table's short rows filled to the header's
// width, a reference link repeating a long destination or title, an alias
// repeating a long value. They are checked at AmplifyingSize, which a
// regression cannot make exhaust the machine's memory: their HTML must
// pass CheckSize.
func Amplifying() []Input {
	return []Input{
		{"a wide header over short rows", func(n int) string {
			return strings.Repeat("|a", n/8) + "\n" + strings.Repeat("|-", n/8) + "\n" + strings.Repeat("a\n", n/4)
		}},
		{"a long destination referred to often", func(n int) string {
			return "[x]: /" + strings.Repeat("a", n/2) + "\n\n" + strings.Repeat("[x]", n/6)
		}},
		{"a long title referred to often", func(n int) string {
			return "[x]: / \"" + strings.Repeat("t", n/2) + "\"\n\n" + strings.Repeat("[x]", n/6)
		}},
		{"a long value aliased often", func(n int) string {
			return "---\na: &a " + strings.Repeat("x", n/2) + "\nb: [" + strings.Repeat("*a, ", n/8) + "]\n---\nbody\n"
		}},
		// v0.1 shows an embed as a link (M6 design 4.1): it repeats nothing.
		{"a long page embedded often", func(n int) string {
			return "[[" + strings.Repeat("a", n/2) + "]]\n\n" + strings.Repeat("![[p]]", n/12)
		}},
	}
}

// AmplifyingSize is the size the Amplifying inputs are checked at.
const AmplifyingSize = 16 << 10
