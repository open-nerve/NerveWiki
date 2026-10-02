package markdown

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Parse parses content, any bytes: its frontmatter is read and then made
// blank, so it never touches the Markdown and every offset in the tree is
// the content's (overall design 4.3). It is the one parse: the reading view
// and every extension's extraction start from it.
func (m *Markdown) Parse(content []byte) *Document {
	fm, end := frontmatterOf(content)
	source := blank(content, end)
	root := m.parser.Parse(text.NewReader(source))
	d := &Document{content: content, source: source, root: root, frontmatter: fm, extracted: map[string]any{}}
	for _, e := range m.exts {
		if e.Extract != nil {
			d.extracted[e.Name] = e.Extract(root, content)
		}
	}
	return d
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
// dropped; "section" for none; "-1", "-2"… after one already given.
type headingIDs struct{}

func (headingIDs) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	used := map[string]bool{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		base := idPrefix + slug(plainText(h, reader.Source()))
		id := base
		for i := 1; used[id]; i++ {
			id = base + "-" + strconv.Itoa(i)
		}
		used[id] = true
		h.SetAttributeString("id", []byte(id))
		return ast.WalkSkipChildren, nil
	})
}

// plainText is the text of n's descendants.
func plainText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
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
