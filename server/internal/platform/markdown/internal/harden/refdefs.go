package harden

// The link reference definitions are adapted from goldmark's
// parser/link_ref.go, v1.8.6 (https://github.com/yuin/goldmark, MIT License,
// Copyright (c) 2019 Yusuke Inuzuka). goldmark copies the paragraph's
// remaining lines after each definition and finds a label's or a title's
// first line by walking back from the last, both quadratic in the lines;
// this removes the lines with the same arithmetic from a linked list, and
// finds the line by a binary search.

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// refdefs takes the link reference definitions off the head of a paragraph.
type refdefs struct{}

func (refdefs) Transform(node *ast.Paragraph, reader text.Reader, pc parser.Context) {
	lines := node.Lines()
	block := text.NewBlockReader(reader.Source(), lines)
	var removes [][2]int
	for {
		ref, start, end := definition(node, block, pc)
		if start < 0 {
			break
		}
		if start == 0 {
			ref.SetBlankPreviousLines(node.HasBlankPreviousLines())
		}
		node.Parent().InsertBefore(node.Parent(), node, ref)
		for i := start + 1; i < end; i++ {
			ref.Lines().Append(lines.At(i))
		}
		last := ref.Lines().Len() - 1
		seg := ref.Lines().At(last)
		ref.Lines().Set(last, seg.TrimRightSpace(reader.Source()))
		if start == end {
			end++
		}
		removes = append(removes, [2]int{start, end})
	}
	if len(removes) == 0 {
		return
	}
	rest := remove(lines, removes)
	if rest.Len() == 0 {
		node.Parent().RemoveChild(node.Parent(), node)
		return
	}
	node.SetLines(rest)
}

// remove is goldmark's removal of the definitions' lines: for each, keep
// what lies before its start and what lies from its end, both counted from
// the end of the one before it. That is how goldmark counts even when it
// kept lines between the two, so the lines it keeps are not always the ones
// between definitions. Each removal cuts a run out of what is left, here
// out of a linked list: the walks to the runs add up to the last end.
func remove(lines *text.Segments, removes [][2]int) *text.Segments {
	// Node k+1 is lines.At(k), node 0 comes before the first line and
	// lines.Len()+1 after the last; next[k] is the node kept after node k.
	next := make([]int, lines.Len()+1)
	for i := range next {
		next[i] = i + 1
	}
	offset := 0
	for _, r := range removes {
		if next[0] > lines.Len() {
			break
		}
		before := 0
		for range r[0] - offset {
			before = next[before]
		}
		cut := next[before]
		for range r[1] - r[0] {
			cut = next[cut]
		}
		next[before] = cut
		offset = r[1]
	}
	rest := text.NewSegments()
	for i := next[0]; i <= lines.Len(); i = next[i] {
		rest.Append(lines.At(i - 1))
	}
	return rest
}

// definition is goldmark's parseLinkReferenceDefinition: a definition at
// the reader's position in paragraph p, and the lines it takes.
func definition(p ast.Node, block text.Reader, pc parser.Context) (*ast.LinkReferenceDefinition, int, int) {
	block.SkipSpaces()
	line, _ := block.PeekLine()
	if line == nil {
		return nil, -1, -1
	}
	startLine, _ := block.Position()
	width, pos := util.IndentWidth(line, 0)
	if width > 3 {
		return nil, -1, -1
	}
	if width != 0 {
		pos++
	}
	if line[pos] != '[' {
		return nil, -1, -1
	}
	_, startPos := block.Position()
	block.Advance(pos + 1)
	segments, found := block.FindClosure('[', ']', closure)
	if !found {
		return nil, -1, -1
	}
	label := values(p, block.Source(), segments)
	if util.IsBlank(label) {
		return nil, -1, -1
	}
	if block.Peek() != ':' {
		return nil, -1, -1
	}
	block.Advance(1)
	block.SkipSpaces()
	dest, ok := destination(block)
	if !ok {
		return nil, -1, -1
	}
	line, _ = block.PeekLine()
	isNewLine := line == nil || util.IsBlank(line)

	endLine, _ := block.Position()
	_, spaces, _ := block.SkipSpaces()
	opener := block.Peek()
	if opener != '"' && opener != '\'' && opener != '(' {
		if !isNewLine {
			return nil, -1, -1
		}
		return define(pc, label, dest, nil, startPos), startLine, endLine + 1
	}
	if spaces == 0 {
		return nil, -1, -1
	}
	block.Advance(1)
	closer := opener
	if opener == '(' {
		closer = ')'
	}
	segments, found = block.FindClosure(opener, closer, closure)
	if !found {
		if !isNewLine {
			return nil, -1, -1
		}
		block.AdvanceLine()
		return define(pc, label, dest, nil, startPos), startLine, endLine + 1
	}
	title := values(p, block.Source(), segments)

	line, _ = block.PeekLine()
	if line != nil && !util.IsBlank(line) {
		if !isNewLine {
			return nil, -1, -1
		}
		return define(pc, label, dest, title, startPos), startLine, endLine
	}
	endLine, _ = block.Position()
	return define(pc, label, dest, title, startPos), startLine, endLine + 1
}

func define(pc parser.Context, label, dest, title []byte, at text.Segment) *ast.LinkReferenceDefinition {
	ref := ast.NewLinkReferenceDefinition(label, dest, title)
	ref.Lines().Append(at)
	pc.AddReference(parser.NewReference(label, dest, title))
	return ref
}
