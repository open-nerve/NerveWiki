package markdown_test

import (
	"reflect"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
)

// A document's facts are its frontmatter and what each extension took, as
// the document has them.
func TestADocumentsFactsAreWhatItFound(t *testing.T) {
	m, err := markdown.New([]markdown.Extension{markdown.Words()})
	if err != nil {
		t.Fatal(err)
	}
	d := m.Parse([]byte("---\ntitle: T\n---\n@@one@@ and @@two@@\n"))
	f := d.Facts()
	if !reflect.DeepEqual(f.Frontmatter(), d.Frontmatter()) || !f.Frontmatter().Valid ||
		!reflect.DeepEqual(f.Extracted("words"), d.Extracted("words")) || f.Extracted("words") == nil ||
		f.Extracted("none") != nil {
		t.Errorf("facts %+v %v, the document's %+v %v", f.Frontmatter(), f.Extracted("words"), d.Frontmatter(), d.Extracted("words"))
	}
}

// What a frontmatter's facts may keep counts each of its values: each
// property, each item of a list and each nested property; beside it,
// FactsRatio times the content.
func TestTheFactsLimitCountsTheFrontmattersValues(t *testing.T) {
	m, err := markdown.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		content string
		values  int
	}{
		{"body\n", 0},
		{"---\na: 1\n---\n", 1},
		{"---\na: [x, [y, z]]\nb: {c: 1, d: [e]}\n---\n", 9},
		{"---\n- not a mapping\n---\n", 0},
	} {
		f := m.Parse([]byte(tt.content)).Facts()
		if got, want := f.Limit(100), markdown.FactsRatio*100+400*tt.values; got != want {
			t.Errorf("%q: Limit(100) = %d, want %d: %d values", tt.content, got, want, tt.values)
		}
	}
}

// The facts of a parse keep none of its tree, with an extension or none.
func TestAParsesFactsOutliveItsTree(t *testing.T) {
	markdowntest.CheckFacts(t, nil)
	markdowntest.CheckFacts(t, []markdown.Extension{markdown.Words()}, "say @@hello@@\n")
}
