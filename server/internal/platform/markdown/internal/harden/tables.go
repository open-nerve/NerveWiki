package harden

// Tables are adapted from goldmark's extension/table.go, v1.8.6
// (https://github.com/yuin/goldmark, MIT License, Copyright (c) 2019 Yusuke
// Inuzuka). goldmark makes every row as wide as the header, so a wide
// header over many short rows is cells by the square of its bytes; and for
// each text of a code span in a cell it looks at the escaped pipes of every
// cell of the document. A table here holds at most as many cells as it has
// bytes, and a text looks at its own cell's pipes.

import (
	"bytes"
	"sort"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// tableRows goes before goldmark's table paragraph transformer: a table
// whose cells, the ones goldmark adds to fill short rows counted, would
// outnumber its bytes stays a paragraph. The header and the delimiter row
// take two bytes a column, so a table whose rows fill about half their
// columns is still one.
type tableRows struct{ parser.ParagraphTransformer }

func (t tableRows) Transform(node *ast.Paragraph, reader text.Reader, pc parser.Context) {
	lines := node.Lines()
	for i := 1; i < lines.Len(); i++ {
		seg := lines.At(i)
		cols := delimiterColumns(seg.Value(reader.Source()))
		if cols == 0 {
			continue
		}
		size := lines.At(lines.Len()-1).Stop - lines.At(i-1).Start
		if (lines.Len()-i)*cols > size {
			return
		}
		break
	}
	t.ParagraphTransformer.Transform(node, reader, pc)
}

// delimiterColumns is how many columns line has as goldmark reads a table's
// delimiter row, 0 if it is not one. goldmark also turns down a line of
// dashes alone, which no paragraph holds: it underlines a heading.
func delimiterColumns(line []byte) int {
	if w, _ := util.IndentWidth(line, 0); w > 3 {
		return 0
	}
	for _, c := range line {
		if !util.IsSpace(c) && c != '-' && c != '|' && c != ':' {
			return 0
		}
	}
	cols := bytes.Split(line, []byte{'|'})
	if util.IsBlank(cols[0]) {
		cols = cols[1:]
	}
	if len(cols) > 0 && util.IsBlank(cols[len(cols)-1]) {
		cols = cols[:len(cols)-1]
	}
	for _, col := range cols {
		if !isDelimiterCell(col) {
			return 0
		}
	}
	return len(cols)
}

// isDelimiterCell is goldmark's `^\s*:?-+:?\s*$`, its four alignments' in
// one.
func isDelimiterCell(col []byte) bool {
	col = bytes.Trim(col, "\t\n\f\r ")
	col = bytes.TrimPrefix(col, []byte{':'})
	col = bytes.TrimSuffix(col, []byte{':'})
	return len(col) > 0 && len(bytes.Trim(col, "-")) == 0
}

// tableCells is goldmark's table AST transformer: in a code span of a cell,
// where the table's parser saw "\|" after a backtick, the backslash goes.
type tableCells struct{}

func (tableCells) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch {
		case !entering:
		case n.Kind() == east.KindTableCell:
			unescapePipes(n, escapedPipes(n, reader.Source()))
			return ast.WalkSkipChildren, nil
		case n.Type() == ast.TypeInline:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
}

// escapedPipes is where cell has "\|": the backslashes' positions, in
// order. goldmark's parseRow records only those after a backtick, but only
// those can be in a code span. Every '|' in a cell is escaped, or the cell
// would end there.
func escapedPipes(cell ast.Node, source []byte) []int {
	if cell.Lines().Len() == 0 {
		return nil
	}
	seg := cell.Lines().At(0)
	var at []int
	for i := seg.Start + 1; i < seg.Stop; i++ {
		if source[i] == '|' && source[i-1] == '\\' {
			at = append(at, i-1)
		}
	}
	return at
}

// unescapePipes splits each text of a code span in cell around the
// backslashes at, as goldmark does.
func unescapePipes(cell ast.Node, at []int) {
	if len(at) == 0 {
		return
	}
	_ = ast.Walk(cell, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Kind() != ast.KindCodeSpan {
			return ast.WalkContinue, nil
		}
		for c := n.FirstChild(); c != nil; {
			next := c.NextSibling()
			if c.Kind() == ast.KindText {
				split(n, c, at)
			}
			c = next
		}
		return ast.WalkContinue, nil
	})
}

// split replaces t, a text child of span, with raw texts that leave out
// the bytes at that it holds.
func split(span, t ast.Node, at []int) {
	seg := t.(*ast.Text).Segment
	cur := t
	for k := sort.SearchInts(at, seg.Start); k < len(at) && at[k] < seg.Stop; k++ {
		s := cur.(*ast.Text).Segment
		before := ast.NewRawTextSegment(s.WithStop(at[k]))
		after := ast.NewRawTextSegment(s.WithStart(at[k] + 1))
		span.InsertAfter(span, cur, before)
		span.InsertAfter(span, before, after)
		span.RemoveChild(span, cur)
		cur = after
	}
}
