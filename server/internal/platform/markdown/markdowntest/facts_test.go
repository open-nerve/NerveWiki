package markdowntest_test

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// breaks is an extension's test double: it takes the thematic breaks, or,
// keeping the tree, their nodes, in a slice that is empty but not nil when
// there are none, as an extension's result may be.
func breaks(keepNodes bool) markdown.Extension {
	return markdown.Extension{Name: "breaks", Extract: func(t markdown.Tree) any {
		nodes, offsets := make([]ast.Node, 0), make([]int, 0)
		_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering && n.Kind() == ast.KindThematicBreak {
				nodes, offsets = append(nodes, n), append(offsets, n.Lines().Len())
			}
			return ast.WalkContinue, nil
		})
		if keepNodes {
			return nodes
		}
		return offsets
	}}
}

// remembering is an extension's test double that keeps the last tree it
// read in its own state, which the Markdown holds, and takes how many
// blocks it has.
func remembering() markdown.Extension {
	var last ast.Node
	return markdown.Extension{Name: "remembering", Extract: func(t markdown.Tree) any {
		last = t.Root
		return last.ChildCount()
	}}
}

// The check fails an extension that keeps the tree in its result or in its
// state, and one
// that took from the contents nothing it does not take from an empty
// document, which an empty slice that is not nil is (M6/P2 fix check L-3);
// it passes one that keeps nothing of the tree (M6/P2 review M2).
func TestFactsErrorSeesTheTreeKept(t *testing.T) {
	for _, tt := range []struct {
		name     string
		ext      markdown.Extension
		contents []string
		want     string
	}{
		{"a result that keeps the tree", breaks(true), []string{"a\n\n***\n"}, "the tree outlives its parse"},
		{"a result that seems to have taken something", breaks(true), nil, "took nothing"},
		{"a state that keeps the tree", remembering(), nil, "the tree outlives its parse"},
		{"a result that keeps nothing of the tree", breaks(false), []string{"a\n\n***\n"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := markdowntest.FactsError([]markdown.Extension{tt.ext}, tt.contents...)
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("FactsError = %v, want none", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("FactsError = %v, want %q", err, tt.want)
			}
		})
	}
}
