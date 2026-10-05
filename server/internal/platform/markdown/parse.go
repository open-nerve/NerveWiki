package markdown

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Parse parses content, any bytes: its frontmatter is read and then made
// blank, so it never touches the Markdown and every offset in the tree is
// the content's (overall design 4.3). It is the one parse: the reading view
// and every extension's extraction start from it.
func (m *Markdown) Parse(content []byte) *Document {
	fm, end := frontmatterOf(content)
	source := blank(content, end)
	pc := parser.NewContext()
	root := m.parser.Parse(text.NewReader(source), parser.WithContext(pc))
	facts := Facts{frontmatter: fm, extracted: map[string]any{}}
	tree := Tree{Root: root, Content: content, Frontmatter: fm, destinations: harden.Destinations(pc)}
	for _, e := range m.exts {
		if e.Extract != nil {
			facts.extracted[e.Name] = e.Extract(tree)
		}
	}
	return &Document{source: source, root: root, facts: facts, destinations: tree.destinations}
}

const (
	// idPrefix starts every id the renderer gives, so none clobbers the
	// application's (M4 design 4, "rendering").
	idPrefix = "nw-"
	// maxSlug is how many characters of a heading its id keeps.
	maxSlug = 64
)

// headingIDs gives each heading an id: its text in lower case, letters,
// digits and marks kept, spaces, '-' and '_' made one '-', the rest
// dropped; "section" for none; "-1", "-2"… after one already given. The
// suffixes of a text go on from the last one it took, so a thousand
// headings alike cost a thousand ids.
type headingIDs struct{}

func (headingIDs) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	used := map[string]bool{}
	next := map[string]int{} // the suffix a text tries next
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, hidden := n.(Hider); hidden {
			return ast.WalkSkipChildren, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			if c := n.FirstChild(); c != nil && c.Type() == ast.TypeInline {
				return ast.WalkSkipChildren, nil // a paragraph or the like: no heading in it
			}
			return ast.WalkContinue, nil
		}
		base := HeadingID(PlainText(h, reader.Source()))
		id := base
		for used[id] {
			next[base]++
			id = base + "-" + strconv.Itoa(next[base])
		}
		used[id] = true
		h.SetAttributeString("id", []byte(id))
		return ast.WalkSkipChildren, nil
	})
}

// PlainText is the text of n's descendants, from source, but for what a
// Hider hides: a heading's for its id, an image's for its text, a property
// link's for what it shows (M6/P6 design 4).
func PlainText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case Hider:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			b.Write(c.Segment.Value(source))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// HeadingID is the id the first heading of text takes, when no heading
// before it took that id: the id a link's anchor of text leads to (M6/P3
// design 6.2). A heading whose id another took gets a suffix.
func HeadingID(text string) string { return idPrefix + slug(text) }

// AnchorID is the id of the heading an anchor leads to (M6/P3 design 6.2,
// M6/P6 design 5): that of the first heading of its last part's text, H2
// of H1#H2. A block's anchor (^…) has none, v0.1 giving blocks no id, and
// an empty one none.
func AnchorID(anchor string) (string, bool) {
	if k := strings.LastIndexByte(anchor, '#'); k >= 0 {
		anchor = anchor[k+1:]
	}
	anchor = strings.Trim(anchor, " \t")
	if anchor == "" || anchor[0] == '^' {
		return "", false
	}
	return HeadingID(anchor), true
}

func slug(s string) string {
	var b strings.Builder
	n := 0 // characters written
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsMark(r):
			sep := dash && n > 0
			if sep && n+2 > maxSlug || n+1 > maxSlug {
				return b.String()
			}
			if sep {
				b.WriteByte('-')
				n++
			}
			dash = false
			b.WriteRune(r)
			n++
		case unicode.IsSpace(r), r == '-', r == '_':
			dash = true
		}
	}
	if n == 0 {
		return "section"
	}
	return b.String()
}
