package harden

// The link parser is adapted from goldmark's parser/link.go, v1.8.6
// (https://github.com/yuin/goldmark, MIT License, Copyright (c) 2019 Yusuke
// Inuzuka). Its results are goldmark's; its costs are linear: a destination
// in angle brackets finds its end in the block's index, a bare one gives up
// past MaxDestinationParens open parentheses, a label's value finds its
// first line by a binary search, and "a link may not contain a link" counts
// the links made instead of walking the text. It processes no delimiters
// when a link closes: emphasis is paired after the parse (emphasis.go).
// Reference links repeat their definitions' destinations and titles up to a
// budget (MinExpansion).
//
// It stays one file, longer than the others, as upstream's is: a goldmark
// upgrade is compared with it part by part.

import (
	"sort"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// label is an open bracket, '[' or '![', waiting for its ']'. The open
// labels of a block are a list whose head holds First and Last.
type label struct {
	ast.BaseInline
	seg                     text.Segment
	isImage                 bool
	prev, next, first, last *label
	links                   int // links made before it opened
}

var kindLabel = ast.NewNodeKind("LinkLabel") //nolint:gochecknoglobals // a kind is made once, as goldmark's are

// Kind implements ast.Node.
func (l *label) Kind() ast.NodeKind { return kindLabel }

// Dump implements ast.Node.
func (l *label) Dump(source []byte, level int) { ast.DumpHelper(l, source, level, nil, nil) }

//nolint:gochecknoglobals // keys are made once, as goldmark's are
var (
	labelsKey   = parser.NewContextKey() // *label: the head of the open labels
	linksKey    = parser.NewContextKey() // int: the links made so far
	repeatedKey = parser.NewContextKey() // int: the bytes reference links have repeated
)

func labelsOf(pc parser.Context) *label {
	v, _ := pc.Get(labelsKey).(*label)
	return v
}

// inLinkLabel tells whether the parse is inside a link's or an image's
// brackets, as goldmark's Context.IsInLinkLabel does for its own parser.
func inLinkLabel(pc parser.Context) bool { return labelsOf(pc) != nil }

func linksMade(pc parser.Context) int {
	v, _ := pc.Get(linksKey).(int)
	return v
}

// reference is the definition of the reference link labeled ref, if
// reference links have not yet repeated more than the larger of the
// source's size and MinExpansion bytes of destinations and titles with it.
func reference(pc parser.Context, source, ref []byte) (parser.Reference, bool) {
	r, ok := pc.Reference(util.ToLinkReference(ref))
	if !ok {
		return nil, false
	}
	repeated, _ := pc.Get(repeatedKey).(int)
	repeated += len(r.Destination()) + len(r.Title())
	if repeated > max(len(source), MinExpansion) {
		return nil, false
	}
	pc.Set(repeatedKey, repeated)
	return r, true
}

func pushLabel(pc parser.Context, v *label) {
	list := labelsOf(pc)
	if list == nil {
		v.first, v.last = v, v
		pc.Set(labelsKey, v)
		return
	}
	l := list.last
	list.last = v
	l.next = v
	v.prev = l
}

func removeLabel(pc parser.Context, d *label) {
	list := labelsOf(pc)
	if list == nil {
		return
	}
	if d.prev == nil {
		list = d.next
		if list != nil {
			list.first = d
			list.last = d.last
			list.prev = nil
			pc.Set(labelsKey, list)
		} else {
			pc.Set(labelsKey, nil)
		}
	} else {
		d.prev.next = d.next
		if d.next != nil {
			d.next.prev = d.prev
		}
	}
	if list != nil && d.next == nil {
		list.last = d.prev
	}
	d.next, d.prev, d.first, d.last = nil, nil, nil, nil
}

// span is goldmark's linkLabelStateLength: from the first open label to the
// last one.
func span(head *label) int {
	if head == nil || head.last == nil || head.first == nil {
		return 0
	}
	return head.last.seg.Stop - head.first.seg.Start
}

// value is the block reader's Value: seg's bytes across the lines of the
// block, with their padding. goldmark walks back from the last line to the
// first one seg is in, a walk per label; this finds it by a binary search.
func value(block ast.Node, source []byte, seg text.Segment) []byte {
	v, _ := valueUpTo(block, source, seg, -1)
	return v
}

// valueUpTo is value, but stops past limit bytes (none for a negative
// limit) and tells whether it took them all: a label longer than a link's
// may be is not copied whole, once for each bracket that closes over it.
func valueUpTo(block ast.Node, source []byte, seg text.Segment, limit int) ([]byte, bool) {
	lines := block.Lines()
	n := lines.Len()
	line := max(sort.Search(n, func(i int) bool { return lines.At(i).Start > seg.Start })-1, 0)
	size := seg.Stop - seg.Start + 1
	if limit >= 0 {
		size = min(size, limit+1)
	}
	ret := make([]byte, 0, max(size, 0))
	i := seg.Start
	for ; line < n; line++ {
		s := lines.At(line)
		if i < 0 {
			i = s.Start
		}
		ret = s.ConcatPadding(ret)
		for ; i < seg.Stop && i < s.Stop; i++ {
			if limit >= 0 && len(ret) > limit {
				return ret, false
			}
			ret = append(ret, source[i])
		}
		i = -1
		if s.Stop > seg.Stop {
			break
		}
	}
	return ret, limit < 0 || len(ret) <= limit
}

// values is the value of a closure's segments as goldmark takes it: one
// segment's alone, several appended to nothing.
func values(block ast.Node, source []byte, segments *text.Segments) []byte {
	if segments.Len() == 1 {
		return value(block, source, segments.At(0))
	}
	var v []byte
	for i := range segments.Len() {
		v = append(v, value(block, source, segments.At(i))...)
	}
	return v
}

// links parses links and images.
type links struct{}

func (links) Trigger() []byte { return []byte{'!', '[', ']'} }

func toText(l *label) {
	ast.MergeOrReplaceTextSegment(l.Parent(), l, l.seg)
}

func (links) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if line[0] == '!' {
		if len(line) > 1 && line[1] == '[' {
			block.Advance(1)
			return openLabel(block, segment.Start+1, true, pc)
		}
		return nil
	}
	if line[0] == '[' {
		return openLabel(block, segment.Start, false, pc)
	}

	// line[0] == ']'
	head := labelsOf(pc)
	if head == nil {
		return nil
	}
	last := head.last
	if last == nil {
		return nil
	}
	block.Advance(1)
	removeLabel(pc, last)
	// CommonMark: a link label has at most 999 characters inside its
	// brackets (goldmark measures the open labels from the first).
	if span(head) > 998 {
		toText(last)
		return nil
	}
	if !last.isImage && linksMade(pc) > last.links { // a link in a link's text
		toText(last)
		return nil
	}

	c := block.Peek()
	l, pos := block.Position()
	var link *ast.Link
	var hasValue bool
	var dest text.Segment // an inline link's destination
	switch c {
	case '(':
		link, dest = inlineLink(parent, last, block, pc)
	case '[':
		link, hasValue = referenceLink(parent, last, block, pc)
		if link == nil && hasValue {
			toText(last)
			return nil
		}
	}
	if link == nil {
		// maybe a shortcut reference link
		block.SetPosition(l, pos)
		ref, whole := valueUpTo(parent, block.Source(), text.NewSegment(last.seg.Stop, segment.Start), 999)
		if !whole {
			toText(last)
			return nil
		}
		r, ok := reference(pc, block.Source(), ref)
		if !ok {
			toText(last)
			return nil
		}
		link = ast.NewLink()
		takeText(parent, link, last)
		link.Title = r.Title()
		link.Destination = r.Destination()
		link.Reference = ast.NewReferenceLink(ast.ReferenceLinkShortcut, ref)
	}
	last.Parent().RemoveChild(last.Parent(), last)
	var n ast.Node = link
	if last.isImage {
		n = ast.NewImage(link)
	} else {
		pc.Set(linksKey, linksMade(pc)+1)
	}
	n.SetPos(last.seg.Start)
	if link.Reference != nil {
		writtenOf(pc).reference(n, link.Reference.Value)
	} else {
		writtenOf(pc).inline(n, dest)
	}
	return n
}

func openLabel(block text.Reader, pos int, isImage bool, pc parser.Context) *label {
	start := pos
	if isImage {
		start--
	}
	l := &label{seg: text.NewSegment(start, pos+1), isImage: isImage, links: linksMade(pc)}
	pushLabel(pc, l)
	block.Advance(1)
	return l
}

// takeText moves the nodes after last, its text, into link.
func takeText(parent ast.Node, link *ast.Link, last *label) {
	for c := last.NextSibling(); c != nil; {
		next := c.NextSibling()
		parent.RemoveChild(parent, c)
		link.AppendChild(link, c)
		c = next
	}
}

// closure is how a label, a destination or a title is looked for, as
// goldmark's link parser looks for them.
var closure = text.FindClosureOptions{Newline: true, Advance: true} //nolint:gochecknoglobals // read only, as goldmark's is

func referenceLink(parent ast.Node, last *label, block text.Reader, pc parser.Context) (*ast.Link, bool) {
	_, orgpos := block.Position()
	block.Advance(1) // skip '['
	segments, found := block.FindClosure('[', ']', closure)
	if !found {
		return nil, false
	}
	ref := []byte{}
	for i := range segments.Len() {
		ref = append(ref, value(parent, block.Source(), segments.At(i))...)
	}
	refType := ast.ReferenceLinkFull
	if util.IsBlank(ref) { // a collapsed reference link
		var whole bool
		ref, whole = valueUpTo(parent, block.Source(), text.NewSegment(last.seg.Stop, orgpos.Start-1), 999)
		if !whole {
			return nil, true
		}
		refType = ast.ReferenceLinkCollapsed
	}
	if len(ref) > 999 {
		return nil, true
	}
	r, ok := reference(pc, block.Source(), ref)
	if !ok {
		return nil, true
	}
	link := ast.NewLink()
	takeText(parent, link, last)
	link.Title = r.Title()
	link.Destination = r.Destination()
	link.Reference = ast.NewReferenceLink(refType, ref)
	return link, true
}

// inlineLink is the link at the reader's '(', and where its destination is
// written: nowhere for an empty one.
func inlineLink(parent ast.Node, last *label, block text.Reader, pc parser.Context) (*ast.Link, text.Segment) {
	block.Advance(1) // skip '('
	block.SkipSpaces()
	var title, destination []byte
	var at text.Segment
	if block.Peek() == ')' { // an empty link like '[link]()'
		block.Advance(1)
	} else {
		var ok bool
		destination, at, ok = inlineDestination(parent, block, pc)
		if !ok {
			return nil, text.Segment{}
		}
		block.SkipSpaces()
		if block.Peek() == ')' {
			block.Advance(1)
		} else {
			title, ok = linkTitle(parent, block)
			if !ok {
				return nil, text.Segment{}
			}
			block.SkipSpaces()
			if block.Peek() != ')' {
				return nil, text.Segment{}
			}
			block.Advance(1)
		}
	}
	link := ast.NewLink()
	takeText(parent, link, last)
	link.Destination = destination
	link.Title = title
	return link, at
}

// inlineDestination is goldmark's parseLinkDestination on the line from the
// reader's position, the end of a destination in angle brackets taken from
// the block's index rather than a scan to the end of the line. It tells
// where the destination is written, inside the brackets.
func inlineDestination(parent ast.Node, block text.Reader, pc parser.Context) ([]byte, text.Segment, bool) {
	block.SkipSpaces()
	line, seg := block.PeekLine()
	if block.Peek() != '<' {
		n, ok := bareDestination(line, MaxDestinationParens)
		block.Advance(n)
		return line[:n], text.NewSegment(seg.Start, seg.Start+n), ok
	}
	ends := indexOf(parent, block.Source(), pc).angles
	k := sort.SearchInts(ends, seg.Start+1)
	if k == len(ends) || ends[k] >= seg.Start+len(line) {
		return nil, text.Segment{}, false
	}
	i := ends[k] - seg.Start
	block.Advance(i + 1)
	return line[1:i], text.NewSegment(seg.Start+1, seg.Start+i), true
}

// destination is goldmark's parseLinkDestination on line, for a link
// reference definition: its line is read once, so the scan stays linear
// and its parentheses need no limit. It tells where the destination is
// written, inside the brackets.
func destination(block text.Reader) ([]byte, text.Segment, bool) {
	block.SkipSpaces()
	line, seg := block.PeekLine()
	if block.Peek() != '<' {
		n, ok := bareDestination(line, len(line))
		block.Advance(n)
		return line[:n], text.NewSegment(seg.Start, seg.Start+n), ok
	}
	for i := 1; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\\' && i < len(line)-1 && util.IsPunct(line[i+1]):
			i++
		case c == '>':
			block.Advance(i + 1)
			return line[1:i], text.NewSegment(seg.Start+1, seg.Start+i), true
		}
	}
	return nil, text.Segment{}, false
}

// bareDestination is how many bytes of line a destination outside angle
// brackets takes, and whether it is one: up to a space or the ')' that
// closes the link, its parentheses balanced, opening at most maxParens.
func bareDestination(line []byte, maxParens int) (int, bool) {
	opened := 0
	i := 0
	for i < len(line) {
		c := line[i]
		if c == '\\' && i < len(line)-1 && util.IsPunct(line[i+1]) {
			i += 2
			continue
		}
		if c == '(' {
			opened++
			if opened > maxParens {
				return 0, false
			}
		} else if c == ')' {
			opened--
			if opened < 0 {
				break
			}
		} else if util.IsSpace(c) {
			break
		}
		i++
	}
	return i, i != 0
}

func linkTitle(parent ast.Node, block text.Reader) ([]byte, bool) {
	block.SkipSpaces()
	opener := block.Peek()
	if opener != '"' && opener != '\'' && opener != '(' {
		return nil, false
	}
	closer := opener
	if opener == '(' {
		closer = ')'
	}
	block.Advance(1)
	segments, found := block.FindClosure(opener, closer, closure)
	if !found {
		return nil, false
	}
	return values(parent, block.Source(), segments), true
}

// CloseBlock turns the labels still open at the end of a block into text.
func (links) CloseBlock(_ ast.Node, _ text.Reader, pc parser.Context) {
	for l := labelsOf(pc); l != nil; {
		next := l.next
		removeLabel(pc, l)
		l.Parent().ReplaceChild(l.Parent(), l, ast.NewTextSegment(l.seg))
		l = next
	}
	pc.Set(labelsKey, nil)
}
