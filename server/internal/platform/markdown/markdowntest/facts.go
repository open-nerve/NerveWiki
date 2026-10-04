package markdowntest

import (
	"reflect"
	"runtime"
	"slices"
	"testing"
	"weak"

	"github.com/yuin/goldmark/ast"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// factsDocument is an ordinary document under a frontmatter with a property
// link, so that the frontmatter's scalars are in the facts too.
func factsDocument() string {
	return "---\nsources: ['[[A page]]', \"[t](b.md)\"]\n---\n" + Normal(64<<10)
}

// CheckFacts checks that the Facts of a parse with exts outlive its tree
// (M6 design 4.7; M6/P2 review M2): a write keeps them through its unit,
// and the tree, some 300 times the content at worst, must be garbage once
// the parse is done. A probe extension notes the tree weakly, and the
// Markdown, which an extension's state would keep the tree in, is kept
// alive. The contents are an ordinary document under a frontmatter, and
// contents, which must write the syntax of each extension the ordinary
// document does not: one that takes nothing from any content fails the
// check, which would see nothing of it. A tree that an extension parses
// apart, as obsidian parses a property's value, is not seen.
func CheckFacts(t *testing.T, exts []markdown.Extension, contents ...string) {
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
	took := map[string]bool{}
	for i, content := range append([]string{factsDocument()}, contents...) {
		root = weak.Pointer[ast.Document]{}
		facts := m.Parse([]byte(content)).Facts()
		if root == (weak.Pointer[ast.Document]{}) {
			t.Fatal("the probe saw no tree")
		}
		runtime.GC()
		if root.Value() != nil {
			t.Errorf("content %d: the tree outlives its parse, which its facts or the Markdown hold", i)
		}
		for _, e := range exts {
			if v := reflect.ValueOf(facts.Extracted(e.Name)); e.Extract != nil && v.IsValid() && !v.IsZero() {
				took[e.Name] = true
			}
		}
		runtime.KeepAlive(facts)
	}
	runtime.KeepAlive(m)
	for _, e := range exts {
		if e.Extract != nil && !took[e.Name] {
			t.Errorf("%s took nothing from the contents: the check would see nothing of it", e.Name)
		}
	}
}

// kept is the heap the Facts of content keep once its parse is done, the
// bytes of the content the parse reads included.
func kept(m *markdown.Markdown, content string) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	facts := m.Parse([]byte(content)).Facts()
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(facts)
	if after.HeapAlloc < before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}
