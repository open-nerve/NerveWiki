package markdowntest

import (
	"bytes"
	"errors"
	"fmt"
	"math"
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
// (M6 design 4.7; M6/P2 review M2), as FactsError tells.
func CheckFacts(t *testing.T, exts []markdown.Extension, contents ...string) {
	t.Helper()
	if err := FactsError(exts, contents...); err != nil {
		t.Error(err)
	}
}

// FactsError tells whether the Facts of a parse with exts outlive its
// tree: a write keeps them through its unit, and the tree, some 300 times
// the content at worst, must be garbage once the parse is done. A probe
// extension notes the tree weakly, and the Markdown, which an extension's
// state would keep the tree in, is kept alive. The contents are an
// ordinary document under a frontmatter, and contents, which must write
// the syntax of each extension the ordinary document does not: one that
// takes from no content more than from an empty one is an error too, the
// check seeing nothing of it (M6/P2 fix check L-3). A tree that an
// extension parses apart, as obsidian parses a property's value, is not
// seen.
func FactsError(exts []markdown.Extension, contents ...string) error {
	var root weak.Pointer[ast.Document]
	probe := markdown.Extension{Name: "markdowntest.facts", Extract: func(tr markdown.Tree) any {
		if d, ok := tr.Root.(*ast.Document); ok {
			root = weak.Make(d)
		}
		return nil
	}}
	m, err := markdown.New(append(slices.Clone(exts), probe))
	if err != nil {
		return err
	}
	var errs []error
	empty := m.Parse(nil).Facts()
	took := map[string]bool{}
	for i, content := range append([]string{factsDocument()}, contents...) {
		root = weak.Pointer[ast.Document]{}
		facts := m.Parse([]byte(content)).Facts()
		if root == (weak.Pointer[ast.Document]{}) {
			return errors.New("the probe saw no tree")
		}
		runtime.GC()
		if root.Value() != nil {
			errs = append(errs, fmt.Errorf("content %d: the tree outlives its parse, which its facts or the Markdown hold", i))
		}
		for _, e := range exts {
			if e.Extract != nil && !reflect.DeepEqual(facts.Extracted(e.Name), empty.Extracted(e.Name)) {
				took[e.Name] = true
			}
		}
		runtime.KeepAlive(facts)
	}
	runtime.KeepAlive(m)
	for _, e := range exts {
		if e.Extract != nil && !took[e.Name] {
			errs = append(errs, fmt.Errorf("%s took nothing from the contents: the check would see nothing of it", e.Name))
		}
	}
	return errors.Join(errs...)
}

// kept is the heap the Facts of content keep once its parse is done, the
// bytes the parse reads included, the least of three runs: the heap's
// accounting only adds noise; and what the facts may keep, their Limit.
// Each reading follows two collections: the first leaves what the parse
// pooled (sync.Pool) to the second (M6/P2 fix check 2 L3).
func kept(m *markdown.Markdown, content []byte) (uint64, int) {
	least, limit := uint64(math.MaxUint64), 0
	for range 3 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&before)
		facts := m.Parse(bytes.Clone(content)).Facts()
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(facts)
		runtime.KeepAlive(content)
		least, limit = min(least, after.HeapAlloc-min(before.HeapAlloc, after.HeapAlloc)), facts.Limit(len(content))
	}
	return least, limit
}
