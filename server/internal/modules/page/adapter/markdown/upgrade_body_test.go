package markdownadapter_test

import (
	"bytes"
	"testing"

	markdownadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// bom is the byte order mark a content may start with, written as its
// bytes.
const bom = "\xef\xbb\xbf"

// The body an upgrade's guardrails compare, before and after a write (M11
// design 4.6, decision 189; M11/P2 design 3.3), cut by the parse's own
// FrontmatterEnd, each shape with its counterexample (05-lessons 1): a
// frontmatter added, changed, or written with other line ends keeps the
// body; one byte order mark past the frontmatter, or at the start of a
// page without one, is not the body's, a second is; an invalid block with
// its closing line is a frontmatter, which may be fixed in place but not
// pushed below another; a lone opening line is the body's, which a block
// may go above but not close; an empty body stays empty. The adapter's
// Frontmatter tells the same end as the parse.
func TestTheUpgradeBodyShapes(t *testing.T) {
	for _, tt := range []struct {
		name, before, after string
		same                bool
	}{
		{"no frontmatter, a valid one added", "Text\n", "---\ntype: note\n---\nText\n", true},
		{"a leading byte order mark, no frontmatter, one added", bom + "Text", "---\ntype: note\n---\nText", true},
		{"a byte order mark past the frontmatter", "---\na: 1\n---\nText", "---\na: 2\n---\n" + bom + "Text", true},
		{"two byte order marks, one taken", "Text", "---\na: 1\n---\n" + bom + bom + "Text", false},
		{"lines ending in CRLF", "---\r\na: 1\r\n---\r\nText", "---\r\na: 2\r\n---\r\nText", true},
		{"a CRLF frontmatter written with LF", "---\r\na: 1\r\n---\r\nText", "---\na: 1\n---\nText", true},
		{"a valid one, the body changed by a letter", "---\na: 1\n---\nText", "---\na: 1\n---\nTexT", false},
		{"an invalid one with its closing line, fixed in place", "---\na: [\n---\nText", "---\nraw: \"a: [\"\n---\nText", true},
		{"an invalid one with its closing line, another added above", "---\na: [\n---\nText", "---\ntype: x\n---\n---\na: [\n---\nText",
			false},
		{"a lone opening line, a block added above", "---\nText", "---\ntype: x\n---\n---\nText", true},
		{"a lone opening line, closed as a frontmatter", "---\nText", "---\nText\n---\n", false},
		{"an empty body", "---\na: 1\n---\n", "---\na: 2\n---\n", true},
		{"an empty body, a letter written", "---\na: 1\n---\n", "---\na: 1\n---\nx", false},
	} {
		before := domain.UpgradeBodyHash(tt.before, markdownadapter.FrontmatterEnd(tt.before))
		after := domain.UpgradeBodyHash(tt.after, markdownadapter.FrontmatterEnd(tt.after))
		if got := bytes.Equal(before, after); got != tt.same {
			t.Errorf("%s: the same body %v, want %v (bodies %q and %q)", tt.name, got, tt.same,
				domain.UpgradeBody(tt.before, markdownadapter.FrontmatterEnd(tt.before)),
				domain.UpgradeBody(tt.after, markdownadapter.FrontmatterEnd(tt.after)))
		}
		for _, content := range []string{tt.before, tt.after} {
			if got, want := (markdownadapter.Frontmatter{}).FrontmatterEnd(content), markdownadapter.FrontmatterEnd(content); got != want {
				t.Errorf("%s: Frontmatter's end of %q = %d, want the parse's %d", tt.name, content, got, want)
			}
		}
	}
}
