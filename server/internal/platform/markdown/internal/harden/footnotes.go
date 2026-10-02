package harden

// Footnotes are adapted from goldmark's extension/footnote.go, v1.8.6
// (https://github.com/yuin/goldmark, MIT License, Copyright (c) 2019 Yusuke
// Inuzuka), with goldmark's nodes and renderer. goldmark finds each
// reference's definition by walking the list of them and sorts the list by
// insertion, both quadratic in the footnotes; this looks the label up in a
// map and sorts once.

import (
	"slices"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//nolint:gochecknoglobals // keys are made once, as goldmark's are
var (
	footnoteListKey  = parser.NewContextKey() // *ast.FootnoteList
	footnoteByRefKey = parser.NewContextKey() // map[string]*ast.Footnote
	footnoteRefsKey  = parser.NewContextKey() // []*ast.FootnoteLink
)

// footnoteBlock parses a footnote's definition, `[^label]: …`.
type footnoteBlock struct{}

func (footnoteBlock) Trigger() []byte { return []byte{'['} }

func (footnoteBlock) Open(_ gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || line[pos] != '[' {
		return nil, parser.NoChildren
	}
	pos++
	if pos > len(line)-1 || line[pos] != '^' {
		return nil, parser.NoChildren
	}
	open := pos + 1
	closure := util.FindClosure(line[pos+1:], '[', ']', false, false) //nolint:staticcheck // goldmark's own call
	closes := pos + 1 + closure
	next := closes + 1
	if closure < 0 || next >= len(line) || line[next] != ':' {
		return nil, parser.NoChildren
	}
	padding := segment.Padding
	label := reader.Value(text.NewSegment(segment.Start+open-padding, segment.Start+closes-padding))
	if util.IsBlank(label) {
		return nil, parser.NoChildren
	}
	item := ast.NewFootnote(label)
	pos = next + 1 - padding
	if pos >= len(line) {
		reader.Advance(pos)
		return item, parser.NoChildren
	}
	reader.AdvanceAndSetPadding(pos, padding)
	return item, parser.HasChildren
}

func (footnoteBlock) Continue(_ gast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	childpos, padding := util.IndentPosition(line, reader.LineOffset(), 4)
	if childpos < 0 {
		return parser.Close
	}
	reader.AdvanceAndSetPadding(childpos, padding)
	return parser.Continue | parser.HasChildren
}

func (footnoteBlock) Close(node gast.Node, _ text.Reader, pc parser.Context) {
	list, _ := pc.Get(footnoteListKey).(*ast.FootnoteList)
	if list == nil {
		list = ast.NewFootnoteList()
		pc.Set(footnoteListKey, list)
		node.Parent().InsertBefore(node.Parent(), node, list)
	}
	node.Parent().RemoveChild(node.Parent(), node)
	list.AppendChild(list, node)
}

func (footnoteBlock) CanInterruptParagraph() bool { return true }

func (footnoteBlock) CanAcceptIndentedLine() bool { return false }

// footnoteRef parses a reference to a footnote, `[^label]`.
type footnoteRef struct{}

// Trigger is '!' too: footnotes conflict with images.
func (footnoteRef) Trigger() []byte { return []byte{'!', '['} }

func (footnoteRef) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, segment := block.PeekLine()
	pos := 1
	if len(line) > 0 && line[0] == '!' {
		pos++
	}
	if pos >= len(line) || line[pos] != '^' {
		return nil
	}
	pos++
	if pos >= len(line) {
		return nil
	}
	open := pos
	closure := util.FindClosure(line[pos:], '[', ']', false, false) //nolint:staticcheck // goldmark's own call
	if closure < 0 {
		return nil
	}
	closes := pos + closure
	ref := value(parent, block.Source(), text.NewSegment(segment.Start+open, segment.Start+closes))
	block.Advance(closes + 1)

	list, _ := pc.Get(footnoteListKey).(*ast.FootnoteList)
	if list == nil {
		return nil
	}
	def := footnoteByRef(list, pc)[string(ref)]
	if def == nil {
		return nil
	}
	if def.Index < 0 {
		list.Count++
		def.Index = list.Count
	}
	link := ast.NewFootnoteLink(def.Index)
	refs, _ := pc.Get(footnoteRefsKey).([]*ast.FootnoteLink)
	pc.Set(footnoteRefsKey, append(refs, link))
	if line[0] == '!' {
		parent.AppendChild(parent, gast.NewTextSegment(text.NewSegment(segment.Start, segment.Start+1)))
	}
	return link
}

// footnoteByRef maps each label to its first definition: the list is whole
// once the blocks are parsed, before any reference is.
func footnoteByRef(list *ast.FootnoteList, pc parser.Context) map[string]*ast.Footnote {
	if m, ok := pc.Get(footnoteByRefKey).(map[string]*ast.Footnote); ok {
		return m
	}
	m := map[string]*ast.Footnote{}
	for n := list.FirstChild(); n != nil; n = n.NextSibling() {
		d := n.(*ast.Footnote)
		if _, ok := m[string(d.Ref)]; !ok {
			m[string(d.Ref)] = d
		}
	}
	pc.Set(footnoteByRefKey, m)
	return m
}

// footnoteList numbers the references, gives each used footnote its back
// links, drops the unused ones and puts the list, in reference order, at the
// end of the document.
type footnoteList struct{}

func (footnoteList) Transform(doc *gast.Document, _ text.Reader, pc parser.Context) {
	list, _ := pc.Get(footnoteListKey).(*ast.FootnoteList)
	refs, _ := pc.Get(footnoteRefsKey).([]*ast.FootnoteLink)
	pc.Set(footnoteListKey, nil)
	pc.Set(footnoteRefsKey, nil)
	pc.Set(footnoteByRefKey, nil)
	if list == nil {
		return
	}
	counter := map[int]int{}
	for _, l := range refs {
		if l.Index >= 0 {
			counter[l.Index]++
		}
	}
	refIndex := map[int]int{}
	for _, l := range refs {
		l.RefCount = counter[l.Index]
		l.RefIndex = refIndex[l.Index]
		refIndex[l.Index]++
	}
	var used []*ast.Footnote
	for n := list.FirstChild(); n != nil; {
		next := n.NextSibling()
		fn := n.(*ast.Footnote)
		if fn.Index < 0 {
			list.RemoveChild(list, n)
		} else {
			var container gast.Node = fn
			if fc := container.LastChild(); fc != nil && gast.IsParagraph(fc) {
				container = fc
			}
			for i := range max(counter[fn.Index], 1) {
				back := ast.NewFootnoteBacklink(fn.Index)
				back.RefCount = counter[fn.Index]
				back.RefIndex = i
				container.AppendChild(container, back)
			}
			used = append(used, fn)
		}
		n = next
	}
	if list.Count <= 0 {
		list.Parent().RemoveChild(list.Parent(), list)
		return
	}
	slices.SortFunc(used, func(a, b *ast.Footnote) int { return a.Index - b.Index })
	list.RemoveChildren(list)
	for _, fn := range used {
		list.AppendChild(list, fn)
	}
	doc.AppendChild(doc, list)
}
