package harden

import (
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// run is a delimiter run of '*', '_' or '~' waiting for the emphasis pass.
// goldmark pairs its delimiters with no openers_bottom, each closer looking
// back over every delimiter before it; the pass pairs the runs with
// CommonMark's algorithm instead, and the same rules as goldmark.
type run struct {
	ast.BaseInline
	seg               text.Segment // the characters left
	char              byte
	length, orig      int
	canOpen, canClose bool

	// The delimiter stack of the scope while the pass pairs it.
	idx        int
	prev, next *run
}

var kindRun = ast.NewNodeKind("DelimiterRun") //nolint:gochecknoglobals // a kind is made once, as goldmark's are

// Kind implements ast.Node.
func (r *run) Kind() ast.NodeKind { return kindRun }

// Dump implements ast.Node.
func (r *run) Dump(source []byte, level int) { ast.DumpHelper(r, source, level, nil, nil) }

// scanOnly makes parser.ScanDelimiter tell a run's sides; it never pairs.
type scanOnly byte

func (s scanOnly) IsDelimiter(b byte) bool                   { return b == byte(s) }
func (s scanOnly) CanOpenCloser(_, _ *parser.Delimiter) bool { return false }
func (s scanOnly) OnMatch(int) ast.Node                      { return nil }

// runs parses the delimiter runs of emphasis ('*', '_') and strikethrough
// ('~'), as goldmark's emphasis and strikethrough parsers do, but leaves
// them out of goldmark's delimiter list.
type runs struct{}

func (runs) Trigger() []byte { return []byte{'*', '_', '~'} }

func (runs) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	c := line[0]
	d := parser.ScanDelimiter(line, before, 1, scanOnly(c))
	if d == nil {
		return nil
	}
	if c == '~' && (d.OriginalLength > 2 || before == '~') {
		return nil
	}
	block.Advance(d.OriginalLength)
	return &run{
		seg: segment.WithStop(segment.Start + d.OriginalLength), char: c,
		length: d.OriginalLength, orig: d.OriginalLength, canOpen: d.CanOpen, canClose: d.CanClose,
	}
}

// emphasisPass pairs the runs of each scope: the children of a node. A
// link's or an image's text is a scope of its own, so emphasis never
// crosses its brackets, as CommonMark's processing at the closing bracket
// gives. The runs left over are text.
type emphasisPass struct{}

func (emphasisPass) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.HasChildren() {
			pair(n)
		}
		return ast.WalkContinue, nil
	})
}

// consumption is goldmark's Delimiter.CalcComsumption: how many characters
// opener and closer give to one emphasis, 0 when the rule of three forbids
// it.
func consumption(opener, closer *run) int {
	if (opener.canClose || closer.canOpen) && (opener.orig+closer.orig)%3 == 0 && closer.orig%3 != 0 {
		return 0
	}
	if opener.length >= 2 && closer.length >= 2 {
		return 2
	}
	return 1
}

// bottomOf keys openers_bottom: the closer's character, whether it can open
// and its original length modulo 3 (CommonMark 0.31, appendix).
func bottomOf(closer *run) int {
	c := 0
	switch closer.char {
	case '_':
		c = 1
	case '~':
		c = 2
	}
	o := 0
	if closer.canOpen {
		o = 1
	}
	return (c*2+o)*3 + closer.orig%3
}

func unlink(r *run) {
	if r.prev != nil {
		r.prev.next = r.next
	}
	if r.next != nil {
		r.next.prev = r.prev
	}
	r.prev, r.next = nil, nil
}

// pair runs CommonMark's "process emphasis" over parent's runs. Each closer
// looks back only to the openers_bottom of its kind, so the pass is linear
// in the runs: an opener a failed look passed is not looked at again by a
// closer of the same kind, and the delimiters between a pair leave the
// stack.
func pair(parent ast.Node) {
	var stack []*run
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if r, ok := c.(*run); ok {
			r.idx = len(stack)
			if r.idx > 0 {
				stack[r.idx-1].next, r.prev = r, stack[r.idx-1]
			}
			stack = append(stack, r)
		}
	}
	if len(stack) == 0 {
		return
	}
	var bottoms [18]int
	for i := range bottoms {
		bottoms[i] = -1
	}
	for closer := stack[0]; closer != nil; {
		if !closer.canClose {
			closer = closer.next
			continue
		}
		k := bottomOf(closer)
		var opener *run
		consume := 0
		for o := closer.prev; o != nil && o.idx > bottoms[k]; o = o.prev {
			if o.canOpen && o.char == closer.char {
				if n := consumption(o, closer); n > 0 {
					opener, consume = o, n
					break
				}
			}
		}
		if opener == nil {
			bottoms[k] = -1
			if closer.prev != nil {
				bottoms[k] = closer.prev.idx
			}
			next := closer.next
			if !closer.canOpen {
				unlink(closer)
			}
			closer = next
			continue
		}
		closer = match(parent, opener, closer, consume)
	}
	for _, r := range stack {
		if p := r.Parent(); p != nil {
			ast.MergeOrReplaceTextSegment(p, r, r.seg)
		}
	}
}

// match wraps what lies between opener and closer in an emphasis (or a
// strikethrough) of consume characters and returns the closer to go on
// from.
func match(parent ast.Node, opener, closer *run, consume int) *run {
	opener.length -= consume
	opener.seg = opener.seg.WithStop(opener.seg.Stop - consume)
	closer.length -= consume
	closer.seg = closer.seg.WithStart(closer.seg.Start + consume)
	var node ast.Node
	if closer.char == '~' {
		node = east.NewStrikethrough()
	} else {
		node = ast.NewEmphasis(consume)
	}
	node.SetPos(opener.seg.Start)
	for c := opener.NextSibling(); c != nil && c != ast.Node(closer); {
		next := c.NextSibling()
		node.AppendChild(node, c)
		c = next
	}
	parent.InsertAfter(parent, opener, node)
	for o := opener.next; o != nil && o != closer; {
		next := o.next
		unlink(o)
		o = next
	}
	if opener.length == 0 {
		unlink(opener)
		parent.RemoveChild(parent, opener)
	}
	if closer.length == 0 {
		next := closer.next
		unlink(closer)
		parent.RemoveChild(parent, closer)
		return next
	}
	return closer
}
