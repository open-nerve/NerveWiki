package harden

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindHighlight is the kind of a highlight, ==text== (M6/P1 design 3.3).
var KindHighlight = ast.NewNodeKind("Highlight") //nolint:gochecknoglobals // a kind is made once, as goldmark's are

// Highlight is ==text==: the runs of two '=' that the emphasis pass pairs,
// as it pairs '~~'.
type Highlight struct {
	ast.BaseInline
}

// Kind implements ast.Node.
func (h *Highlight) Kind() ast.NodeKind { return KindHighlight }

// Dump implements ast.Node.
func (h *Highlight) Dump(source []byte, level int) { ast.DumpHelper(h, source, level, nil, nil) }

// HighlightRuns is the inline parser of '=' runs, for a parser that
// highlights; NewParser has none, so its parse stays goldmark's. A run is
// two '=' exactly: a longer one, or one that a '=' comes before, is text.
func HighlightRuns() util.PrioritizedValue {
	return util.Prioritized(highlightRuns{}, 500)
}

type highlightRuns struct{}

func (highlightRuns) Trigger() []byte { return []byte{'='} }

func (highlightRuns) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	if before == '=' {
		return nil
	}
	line, segment := block.PeekLine()
	d := parser.ScanDelimiter(line, before, 1, scanOnly('='))
	if d == nil || d.OriginalLength != 2 {
		return nil
	}
	block.Advance(2)
	return &run{
		seg: segment.WithStop(segment.Start + 2), char: '=',
		length: 2, orig: 2, canOpen: d.CanOpen, canClose: d.CanClose,
	}
}
