package markdowntest

import (
	"runtime"
	"slices"
	"testing"
	"weak"

	"github.com/yuin/goldmark/ast"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// CheckFacts checks that the Facts of a parse with exts outlive its tree
// (M6 design 4.7): a write keeps them through its unit, and the tree, some
// 300 times the content at worst, must be garbage once the parse is done.
// A probe extension notes the tree weakly; the content is an ordinary
// document, which writes each extension's syntax.
func CheckFacts(t *testing.T, exts ...markdown.Extension) {
	t.Helper()
	var root weak.Pointer[ast.Document]
	probe := markdown.Extension{Name: "markdowntest.facts", Extract: func(tr markdown.Tree) any {
		if d, ok := tr.Root.(*ast.Document); ok {
			root = weak.Make(d)
		}
		return nil
	}}
	m, err := markdown.New(append(slices.Clone(exts), probe))
	if err != nil {
		t.Fatal(err)
	}
	facts := m.Parse([]byte(Normal(64 << 10))).Facts()
	if root == (weak.Pointer[ast.Document]{}) {
		t.Fatal("the probe saw no tree")
	}
	runtime.GC()
	if root.Value() != nil {
		t.Error("the tree outlives its parse: its facts hold it")
	}
	runtime.KeepAlive(facts)
}
