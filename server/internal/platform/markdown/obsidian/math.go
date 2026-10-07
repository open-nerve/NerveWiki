package obsidian

import (
	"bytes"
	"sort"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//nolint:gochecknoglobals // kinds are made once, as goldmark's are
var (
	kindMath      = ast.NewNodeKind("Math")
	kindMathBlock = ast.NewNodeKind("MathBlock")
)

// inlineMath is an inline formula, $…$ or $$…$$ (display). Its children
// are its raw lines, as a code span's are.
type inlineMath struct {
	ast.BaseInline
	display bool
}

// Kind implements ast.Node.
func (m *inlineMath) Kind() ast.NodeKind { return kindMath }

// Dump implements ast.Node.
func (m *inlineMath) Dump(source []byte, level int) {
	ast.DumpHelper(m, source, level, map[string]string{"Display": strconv.FormatBool(m.display)}, nil)
}

// mathBlock is a block formula: its lines between the lines of "$$".
type mathBlock struct {
	ast.BaseBlock
}

// Kind implements ast.Node.
func (m *mathBlock) Kind() ast.NodeKind { return kindMathBlock }

// IsRaw implements ast.Node: its lines are not parsed.
func (m *mathBlock) IsRaw() bool { return true }

// Dump implements ast.Node.
func (m *mathBlock) Dump(source []byte, level int) { ast.DumpHelper(m, source, level, nil, nil) }

// mathBlockParser parses block formulas (rule 5): a line that starts with
// "$$" and has no other after it opens one, which the first line ending
// in "$$" closes, or the end of the document. It interrupts a paragraph.
type mathBlockParser struct{}

func (mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (mathBlockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || !bytes.HasPrefix(line[pos:], []byte("$$")) || bytes.Contains(line[pos+2:], []byte("$$")) {
		return nil, parser.NoChildren
	}
	node := &mathBlock{}
	// What follows the opening "$$" is the formula's first line.
	if rest := line[pos+2:]; !util.IsBlank(rest) {
		node.Lines().Append(text.NewSegment(seg.Start-seg.Padding+pos+2, seg.Stop))
	}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (mathBlockParser) Continue(node ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, seg := reader.PeekLine()
	trimmed := util.TrimRightSpace(line)
	if !bytes.HasSuffix(trimmed, []byte("$$")) {
		node.Lines().Append(seg)
		reader.AdvanceToEOL()
		return parser.Continue | parser.NoChildren
	}
	// What comes before the closing "$$" is the formula's last line.
	if before := trimmed[:len(trimmed)-2]; !util.IsBlank(before) {
		node.Lines().Append(seg.WithStop(seg.Start - seg.Padding + len(before)))
	}
	reader.AdvanceToEOL()
	return parser.Close
}

func (mathBlockParser) Close(ast.Node, text.Reader, parser.Context) {}

func (mathBlockParser) CanInterruptParagraph() bool { return true }

func (mathBlockParser) CanAcceptIndentedLine() bool { return false }

// mathIndex is where, in the block being parsed, a formula may end: each
// '$' that may close a $…$, and each "$$". A '$' a backslash escapes is
// neither. Positions are in the source.
type mathIndex struct {
	block   ast.Node
	singles []int // after no space, before no digit
	doubles []int // where the "$$" starts
}

var mathIndexKey = parser.NewContextKey() //nolint:gochecknoglobals // a key is made once, as goldmark's are

// mathIndexOf is the index of block, made the first time a '$' needs it,
// so a '$' looks for its end by a binary search, not a scan.
func mathIndexOf(block ast.Node, source []byte, pc parser.Context) *mathIndex {
	if ix, ok := pc.Get(mathIndexKey).(*mathIndex); ok && ix.block == block {
		return ix
	}
	ix := &mathIndex{block: block}
	lines := block.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		line := source[seg.Start:seg.Stop]
		backslashes := 0
		for j, c := range line {
			if c == '\\' {
				backslashes++
				continue
			}
			if c == '$' && backslashes%2 == 0 {
				if closes(line, j) {
					ix.singles = append(ix.singles, seg.Start+j)
				}
				if j+1 < len(line) && line[j+1] == '$' {
					ix.doubles = append(ix.doubles, seg.Start+j)
				}
			}
			backslashes = 0
		}
	}
	pc.Set(mathIndexKey, ix)
	return ix
}

// closes tells whether line's '$' at j may close a $…$: no space before
// it, and no digit after it.
func closes(line []byte, j int) bool {
	if j == 0 {
		return false
	}
	if r, _ := utf8.DecodeLastRune(line[:j]); unicode.IsSpace(r) {
		return false
	}
	return j+1 >= len(line) || line[j+1] < '0' || line[j+1] > '9'
}

// mathParser parses inline formulas (rule 5): $…$, whose '$' opens before
// no space; and $$…$$, which may hold spaces at its ends. Either may run
// over lines of its block. A "$$" that nothing closes is text.
type mathParser struct{}

func (mathParser) Trigger() []byte { return []byte{'$'} }

func (mathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	display := len(line) > 1 && line[1] == '$'
	if !display {
		if len(line) < 2 {
			return nil
		}
		if r, _ := utf8.DecodeRune(line[1:]); unicode.IsSpace(r) {
			return nil
		}
	}
	ix := mathIndexOf(parent, block.Source(), pc)
	open, ends := 1, ix.singles
	if display {
		open, ends = 2, ix.doubles
	}
	k := sort.SearchInts(ends, seg.Start+open)
	if k == len(ends) {
		if display {
			block.Advance(2)
			return ast.NewTextSegment(seg.WithStop(seg.Start + 2))
		}
		return nil
	}
	block.Advance(open)
	return formula(block, &inlineMath{display: display}, ends[k], open)
}

// formula reads m's lines from the reader up to end, where its closing
// delimiter of n bytes starts, and the delimiter.
func formula(block text.Reader, m *inlineMath, end, n int) ast.Node {
	for {
		line, seg := block.PeekLine()
		if line == nil {
			return m
		}
		if end < seg.Stop {
			if s := seg.WithStop(end); s.Start < s.Stop || s.Padding > 0 {
				m.AppendChild(m, ast.NewRawTextSegment(s))
			}
			block.Advance(seg.Padding + end - seg.Start + n)
			return m
		}
		m.AppendChild(m, ast.NewRawTextSegment(seg))
		block.AdvanceLine()
	}
}

// formulaLines takes away the line break after a $$…$$ formula that ends a
// paragraph's line: the reading view shows the formula as a block of its
// own, after which a <br> would show an empty line, and Obsidian's reading
// view shows none (M6/P8 design 3). A heading's line keeps it, for its id.
type formulaLines struct{}

// Transform implements parser.ASTTransformer.
func (formulaLines) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		m, ok := n.(*inlineMath)
		if !ok {
			return ast.WalkContinue, nil
		}
		if entering && m.display && starts(m.Parent()) {
			noBreak(m.NextSibling(), reader.Source())
		}
		return ast.WalkSkipChildren, nil
	})
}
