package harden

import (
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

// A destination opens at most MaxDestinationParens parentheses; one more
// and the brackets are no link, where goldmark makes one.
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
