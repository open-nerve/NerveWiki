package harden

import (
	"fmt"
	"strings"
	"testing"

	"github.com/yuin/goldmark/util"
)

// Block quotes and list items nest MaxNesting deep as goldmark nests them;
// a line that would open one deeper is the deepest one's text.
func TestContainersNestAtMostMaxNestingDeep(t *testing.T) {
	h, o := hardened(), original()
	tests := []struct {
		name, marker, open string
	}{
		{"block quotes", "> ", "<blockquote>"},
		{"list items", "- ", "<li>"},
		{"block quotes and list items", "> - ", "<blockquote>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			per := strings.Count(tt.marker, " ")
			atLimit := strings.Repeat(tt.marker, MaxNesting/per) + "a\n"
			if got, want := render(t, h, atLimit), render(t, o, atLimit); got != want {
				t.Fatalf("at the limit:\nhardened %q\noriginal %q", got, want)
			}
			over := strings.Repeat(tt.marker, MaxNesting/per+1) + "a\n"
			got := render(t, h, over)
			if n := strings.Count(got, tt.open); n != MaxNesting/per {
				t.Errorf("%d %s past the limit, want %d: %q", n, tt.open, MaxNesting/per, got)
			}
			if !strings.Contains(got, string(util.EscapeHTML([]byte(tt.marker)))+"a") {
				t.Errorf("the line past the limit is not text: %q", got)
			}
		})
	}
}

// An inline link's destination opens at most MaxDestinationParens
// parentheses; one more and the brackets are no link, where goldmark makes
// one. A definition's has no limit (parts_test.go).
func TestADestinationOpensAtMostMaxDestinationParensParentheses(t *testing.T) {
	dest := func(n int) string {
		return "[a](" + strings.Repeat("(", n) + "b" + strings.Repeat(")", n) + ")"
	}
	h, o := hardened(), original()
	if got, want := render(t, h, dest(MaxDestinationParens)), render(t, o, dest(MaxDestinationParens)); got != want ||
		!strings.Contains(got, "<a href=") {
		t.Errorf("at the limit:\nhardened %q\noriginal %q", got, want)
	}
	if got := render(t, h, dest(MaxDestinationParens+1)); strings.Contains(got, "<a href=") {
		t.Errorf("past the limit, a link: %q", got)
	}
	if got := render(t, o, dest(MaxDestinationParens+1)); !strings.Contains(got, "<a href=") {
		t.Errorf("goldmark makes no link past the limit either, the test proves nothing: %q", got)
	}
}

// Footnote definitions nest MaxNesting deep, block quotes and list items
// counted, as goldmark nests them; one deeper is the deepest one's text.
func TestFootnoteDefinitionsNestAtMostMaxNestingDeep(t *testing.T) {
	nest := func(n int) string {
		var defs, refs strings.Builder
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&defs, "[^%d]: ", i)
			fmt.Fprintf(&refs, "[^%d]", i)
		}
		return defs.String() + "x\n\n" + refs.String() + "\n"
	}
	h, o := hardened(), original()
	if got, want := render(t, h, nest(MaxNesting)), render(t, o, nest(MaxNesting)); got != want {
		t.Fatalf("at the limit:\nhardened %q\noriginal %q", got, want)
	}
	// Past the limit "[^33]: x" is the deepest footnote's text, which makes
	// it a link reference definition.
	got := render(t, h, nest(MaxNesting+1))
	if n := strings.Count(got, `<li id="fn:`); n != MaxNesting || strings.Contains(got, fmt.Sprintf(`"fn:%d"`, MaxNesting+1)) {
		t.Errorf("%d footnotes past the limit, want %d: %q", n, MaxNesting, got)
	}
	if got := render(t, h, "> "+nest(MaxNesting)); strings.Count(got, `<li id="fn:`) != MaxNesting-1 {
		t.Errorf("in a block quote, not %d footnotes: %q", MaxNesting-1, got)
	}
}

// A table holds as many cells as it has bytes, as goldmark makes it; a row
// more, filled to the header's width, and it is a paragraph. The delimiter
// row is read as goldmark reads it, in each of its forms.
func TestATableHoldsAtMostAsManyCellsAsItHasBytes(t *testing.T) {
	const cols = 4
	h, o := hardened(), original()
	for _, cell := range []string{"-", ":-", "-:", ":-:", " - ", "---", "\t:--\t"} {
		t.Run(cell, func(t *testing.T) {
			table := func(rows int) string {
				return strings.Repeat("|a", cols) + "\n" + strings.Repeat("|"+cell, cols) + "\n" + strings.Repeat("|\n", rows)
			}
			// (rows+1)·cols cells, 2·cols+1 + (len(cell)+1)·cols+1 + 2·rows bytes.
			rows := 2*len(cell) + 5
			if (rows+1)*cols != len(table(rows)) {
				t.Fatalf("%d rows: %d cells for %d bytes, the test wants them equal", rows, (rows+1)*cols, len(table(rows)))
			}
			if got, want := render(t, h, table(rows)), render(t, o, table(rows)); got != want || !strings.Contains(got, "<table>") {
				t.Errorf("at the limit:\nhardened %q\noriginal %q", got, want)
			}
			if got := render(t, h, table(rows+1)); strings.Contains(got, "<table>") || !strings.HasPrefix(got, "<p>|a|a") {
				t.Errorf("past the limit, not a paragraph: %q", got)
			}
			if got := render(t, o, table(rows+1)); !strings.Contains(got, "<table>") {
				t.Errorf("goldmark makes no table past the limit either, the test proves nothing: %q", got)
			}
		})
	}
}

// Reference links repeat at most the larger of the source's size and
// MinExpansion bytes of destinations and titles, in each form; past that a
// reference is text, where goldmark makes a link.
func TestReferenceLinksRepeatAtMostTheirBudget(t *testing.T) {
	long := "[x]: /" + strings.Repeat("d", 99) + " \"" + strings.Repeat("t", 900) + "\"\n\n"
	tests := []struct {
		name, head, use string
		each            int // the bytes a use repeats
	}{
		{"shortcut", long, "[x] ", 1000},
		{"collapsed", long, "[x][] ", 1000},
		{"full", long, "[x][x] ", 1000},
		{"image", long, "![x] ", 1000},
		{"in a source past MinExpansion", "[x]: /" + strings.Repeat("d", 99) + "\n\n" + strings.Repeat("a", 2*MinExpansion) + "\n", "[x] ", 100},
	}
	h, o := hardened(), original()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uses := 0
			for (uses+1)*tt.each <= max(len(tt.head)+(uses+1)*len(tt.use), MinExpansion) {
				uses++
			}
			at, over := tt.head+strings.Repeat(tt.use, uses), tt.head+strings.Repeat(tt.use, uses+1)
			if got, want := render(t, h, at), render(t, o, at); got != want {
				t.Errorf("at the limit, %d uses, not goldmark's", uses)
			}
			links := func(s string) int { return strings.Count(s, "<a href=") + strings.Count(s, "<img src=") }
			got, want := render(t, h, over), render(t, o, over)
			if links(got) != uses || !strings.HasSuffix(got, strings.TrimSpace(tt.use)+"</p>\n") {
				t.Errorf("past the limit, %d links of %d uses, want %d and the last one text", links(got), uses+1, uses)
			}
			if links(want) != uses+1 {
				t.Errorf("goldmark makes %d links of %d uses, the test proves nothing", links(want), uses+1)
			}
		})
	}
}
