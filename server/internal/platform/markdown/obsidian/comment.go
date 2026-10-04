package obsidian

import (
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//nolint:gochecknoglobals // kinds are made once, as goldmark's are
var (
	kindMarker       = ast.NewNodeKind("CommentMarker")
	kindHidden       = ast.NewNodeKind("Hidden")
	kindHiddenBlocks = ast.NewNodeKind("HiddenBlocks")
)

// marker is a comment's "%%" (rule 6). Comments change what the reading
// view shows, not what the page holds: what lies between two markers is
// parsed as the rest is, and its links and tags are the page's.
type marker struct {
	ast.BaseInline
	seg   text.Segment
	block ast.Node // the block it was parsed in
	line  int      // and the line of the block
	// first: only spaces are before it on a paragraph's line; last: only
	// spaces are after it on the content's line
	first, last bool
}

// Kind implements ast.Node.
func (m *marker) Kind() ast.NodeKind { return kindMarker }

// Dump implements ast.Node.
func (m *marker) Dump(source []byte, level int) { ast.DumpHelper(m, source, level, nil, nil) }

// hidden is what a comment hides in a block: inline nodes.
type hidden struct{ ast.BaseInline }

// Kind implements ast.Node.
func (h *hidden) Kind() ast.NodeKind { return kindHidden }

// Dump implements ast.Node.
func (h *hidden) Dump(source []byte, level int) { ast.DumpHelper(h, source, level, nil, nil) }

// Hides implements markdown.Hider.
func (h *hidden) Hides() {}

// hiddenBlocks is what a comment hides of blocks: blocks.
type hiddenBlocks struct{ ast.BaseBlock }

// Kind implements ast.Node.
func (h *hiddenBlocks) Kind() ast.NodeKind { return kindHiddenBlocks }

// Dump implements ast.Node.
func (h *hiddenBlocks) Dump(source []byte, level int) { ast.DumpHelper(h, source, level, nil, nil) }

// Hides implements markdown.Hider.
func (h *hiddenBlocks) Hides() {}

// markerParser parses "%%". Raw content (code, math, HTML, angle
// autolinks, a link's destination) is parsed first and keeps its own; a
// literal autolink ends before a marker, and an image's markers are made
// text when the markers pair.
type markerParser struct{}

func (markerParser) Trigger() []byte { return []byte{'%'} }

func (markerParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) < 2 || line[1] != '%' {
		return nil
	}
	l, _ := block.Position()
	m := &marker{seg: seg.WithStop(seg.Start + 2), block: parent, line: l}
	source := block.Source()
	if lines := parent.Lines(); l < lines.Len() && starts(parent) {
		m.first = util.IsBlank(source[lines.At(l).Start:seg.Start])
	}
	m.last = endsLine(source, seg.Start+2)
	block.Advance(2)
	return m
}

// starts tells whether a line of block may start a block comment: a
// paragraph's line, which starts where its block quote's, list item's or
// footnote definition's marker leaves it; not a heading's, after its '#',
// nor a cell's.
func starts(block ast.Node) bool {
	k := block.Kind()
	return k == ast.KindParagraph || k == ast.KindTextBlock
}

// endsLine tells whether only spaces and tabs are between at and the end of
// its line in source.
func endsLine(source []byte, at int) bool {
	for ; at < len(source); at++ {
		switch source[at] {
		case ' ', '\t':
		case '\n', '\r':
			return true
		default:
			return false
		}
	}
	return true
}

// comments pairs the markers in the tree's order (rule 6), which is the
// content's but for footnotes' definitions, gathered where the first is:
//   - a line whose one marker starts it opens a block comment, which the
//     first marker to end its line closes, or the end of the document;
//   - the other markers of a line pair from left to right;
//   - a marker left alone is text.
//
// What a comment spans is hidden.
type comments struct{}

func (comments) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var all, raw []*marker
	images := 0 // the images around the node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Image:
			if entering {
				images++
			} else {
				images--
			}
		case *marker:
			switch {
			case !entering:
			case images > 0:
				raw = append(raw, n)
			default:
				all = append(all, n)
			}
		}
		return ast.WalkContinue, nil
	})
	// An image's text is raw (rule 4): its markers are text.
	for _, m := range raw {
		ast.MergeOrReplaceTextSegment(m.Parent(), m, m.seg)
	}
	var open *marker // a block comment's
	for i := 0; i < len(all); {
		j := i + 1
		for j < len(all) && all[j].block == all[i].block && all[j].line == all[i].line {
			j++
		}
		line := all[i:j]
		i = j
		switch last := line[len(line)-1]; {
		case open != nil:
			if last.last {
				hide(open, last)
				open = nil
			}
		case len(line) == 1 && last.first:
			open = last
		default:
			for k := 0; k+1 < len(line); k += 2 {
				hide(line[k], line[k+1])
			}
			if len(line)%2 == 1 {
				ast.MergeOrReplaceTextSegment(last.Parent(), last, last.seg)
			}
		}
	}
	if open != nil {
		hide(open, nil)
	}
}

// hide hides what lies from marker a through marker b, b nil for the end of
// the document: at each level of the tree, the run of nodes it spans is
// moved into a hidden node, and a node it spans whole is hidden whole where
// it may be. Its steps are about the nodes it hides and the levels between
// a and b, not the depth of the tree, and comments do not overlap, so the
// pairing is linear.
func hide(a, b ast.Node) {
	top := commonAncestor(a, b)
	x, xWhole := a, true
	for x.Parent() != top {
		p := x.Parent()
		if xWhole && x.PreviousSibling() == nil && raisable(p) {
			x = p
			continue
		}
		from := x
		if !xWhole {
			from = x.NextSibling()
		}
		wrap(p, from, p.LastChild())
		x, xWhole = p, false
	}
	y, yWhole := top.LastChild(), true
	if b != nil {
		y = b
		for y.Parent() != top {
			p := y.Parent()
			if yWhole && y.NextSibling() == nil && raisable(p) {
				y = p
				continue
			}
			to := y
			if !yWhole {
				to = y.PreviousSibling()
			}
			wrap(p, p.FirstChild(), to)
			y, yWhole = p, false
		}
	}
	for {
		if !xWhole && !yWhole && x.NextSibling() == y {
			return
		}
		from, to := x, y
		if !xWhole {
			from = x.NextSibling()
		}
		if !yWhole {
			to = y.PreviousSibling()
		}
		if from == top.FirstChild() && to == top.LastChild() && raisable(top) {
			x, y, xWhole, yWhole = top, top, true, true
			top = top.Parent()
			continue
		}
		wrap(top, from, to)
		return
	}
}

// commonAncestor is the nearest node that holds both a and b, the root for
// b nil. Its steps are about a's and b's distance to it.
func commonAncestor(a, b ast.Node) ast.Node {
	if b == nil {
		for a.Parent() != nil {
			a = a.Parent()
		}
		return a
	}
	if a.Parent() == b.Parent() {
		return a.Parent()
	}
	seen := map[ast.Node]bool{}
	for x, y := a, b; ; {
		if x != nil {
			if seen[x] {
				return x
			}
			seen[x] = true
			x = x.Parent()
		}
		if y != nil {
			if seen[y] {
				return y
			}
			seen[y] = true
			y = y.Parent()
		}
	}
}

// raisable tells whether a comment that spans the whole of n hides n, not
// what it holds: an inline node, a paragraph or a heading. A container
// stays, its contents hidden.
func raisable(n ast.Node) bool {
	if n.Parent() == nil {
		return false
	}
	switch n.Kind() {
	case ast.KindParagraph, ast.KindTextBlock, ast.KindHeading:
		return true
	}
	return n.Type() == ast.TypeInline
}

// wrap moves from through to, siblings under parent, into a hidden node,
// unless a later transformer or the renderer counts parent's children: a
// table's rows and cells, the list of footnotes.
func wrap(parent, from, to ast.Node) {
	if from == nil || to == nil {
		return
	}
	switch parent.Kind() {
	case east.KindTable, east.KindTableHeader, east.KindTableRow, east.KindFootnoteList:
		return
	}
	var h ast.Node = &hiddenBlocks{}
	if from.Type() == ast.TypeInline {
		h = &hidden{}
	}
	parent.InsertBefore(parent, from, h)
	for c := from; ; {
		next := c.NextSibling()
		h.AppendChild(h, c)
		if c == to {
			return
		}
		c = next
	}
}
