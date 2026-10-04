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

// The facts of a parse keep none of its tree, with an extension or none.
func TestAParsesFactsOutliveItsTree(t *testing.T) {
	markdowntest.CheckFacts(t)
	markdowntest.CheckFacts(t, markdown.Words())
}
