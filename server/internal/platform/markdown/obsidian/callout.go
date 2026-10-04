package obsidian

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

//nolint:gochecknoglobals // kinds are made once, as goldmark's are
var (
	kindCallout      = ast.NewNodeKind("Callout")
	kindCalloutTitle = ast.NewNodeKind("CalloutTitle")
)

// callout is a block quote that starts with [!type], maybe folded: '-'
// folds it, '+' folds it open. Its first child is its title.
type callout struct {
	ast.BaseBlock
	kind string // in lower case
	fold byte   // '-', '+' or none
}

// Kind implements ast.Node.
func (c *callout) Kind() ast.NodeKind { return kindCallout }

// Dump implements ast.Node.
func (c *callout) Dump(source []byte, level int) {
	ast.DumpHelper(c, source, level, map[string]string{"Type": c.kind, "Fold": string(c.fold)}, nil)
}

// calloutTitle is the rest of a callout's first line.
type calloutTitle struct {
	ast.BaseBlock
	of *callout
}

// Kind implements ast.Node.
func (t *calloutTitle) Kind() ast.NodeKind { return kindCalloutTitle }

// Dump implements ast.Node.
func (t *calloutTitle) Dump(source []byte, level int) { ast.DumpHelper(t, source, level, nil, nil) }

// standing is the title a callout has when its line has none: its type,
// capitalized, as Obsidian has it.
func (c *callout) standing() string {
	r, size := utf8.DecodeRuneInString(c.kind)
	return string(unicode.ToUpper(r)) + c.kind[size:]
}

// calloutLine is how a callout's first line starts.
var calloutLine = regexp.MustCompile(`^\[!([^\[\]\s]+)\]([-+]?)`)

// callouts makes each block quote whose first block is a paragraph that
// starts with [!type] a callout (M6/P1 design 3.9). It runs after the
// emphasis is paired, so the nodes it moves are whole.
type callouts struct{}

func (callouts) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch {
		case !entering:
		case n.Type() == ast.TypeInline:
			return ast.WalkSkipChildren, nil
		case n.Kind() == ast.KindBlockquote:
			quotes = append(quotes, n.(*ast.Blockquote))
		}
		return ast.WalkContinue, nil
	})
	// An outer quote first: a quote in it moves with it, still a quote.
	for _, q := range quotes {
		toCallout(q, reader.Source())
	}
}

// toCallout makes q a callout if it is one.
func toCallout(q *ast.Blockquote, source []byte) {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return
	}
	first := p.Lines().At(0)
	m := calloutLine.FindSubmatchIndex(first.Value(source))
	if m == nil || !trimTexts(p, first.Start, m[1]) {
		return
	}
	c := &callout{kind: strings.ToLower(string(source[first.Start+m[2] : first.Start+m[3]]))}
	if m[5] > m[4] {
		c.fold = source[first.Start+m[4]]
	}
	title := &calloutTitle{of: c}
	for n := p.FirstChild(); n != nil; {
		next := n.NextSibling()
		title.AppendChild(title, n)
		if t, ok := n.(*ast.Text); ok && (t.SoftLineBreak() || t.HardLineBreak()) {
			t.SetSoftLineBreak(false)
			t.SetHardLineBreak(false)
			break
		}
		n = next
	}
	for t, ok := title.FirstChild().(*ast.Text); ok; t, ok = title.FirstChild().(*ast.Text) {
		if t.Segment = t.Segment.TrimLeftSpace(source); !t.Segment.IsEmpty() {
			break
		}
		title.RemoveChild(title, t)
	}
	if !p.HasChildren() {
		q.RemoveChild(q, p)
	}
	c.AppendChild(c, title)
	for n := q.FirstChild(); n != nil; {
		next := n.NextSibling()
		c.AppendChild(c, n)
		n = next
	}
	q.Parent().ReplaceChild(q.Parent(), q, c)
}

// trimTexts takes the n bytes written from at off the start of p's inline
// nodes, if text nodes and no others hold them. The text they end in stays,
// maybe empty, with its line break.
func trimTexts(p ast.Node, at, n int) bool {
	end := at + n
	pos := at
	for c := p.FirstChild(); pos < end; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok || t.Segment.Start != pos || t.Segment.Padding != 0 {
			return false
		}
		pos = t.Segment.Stop
	}
	for c := p.FirstChild(); ; {
		t := c.(*ast.Text)
		next := c.NextSibling()
		if t.Segment.Stop >= end {
			t.Segment = t.Segment.WithStart(end)
			return true
		}
		p.RemoveChild(p, t)
		c = next
	}
}
