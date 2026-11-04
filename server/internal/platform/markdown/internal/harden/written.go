package harden

import (
	"math"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// written is where the parse found the links' destinations written (M6/P1
// design 3.2): the extraction of links needs their bytes, and a node
// attribute would show in goldmark's rendering of a link. It lives in the
// parse's context.
type written struct {
	links  map[*ast.Link]writtenRange // the usual inline destinations, by node identity
	images map[*ast.Image]writtenRange
	at     map[ast.Node]text.Segment // full segments for other nodes or uncommon ranges
	uses   map[ast.Node]string       // a reference link's or image's definition, by its key
	defs   map[string]text.Segment   // each definition's destination, the first of a key
}

// writtenRange keeps byte offsets only. The parser's inlineDestination makes
// plain segments; other inputs keep every field through at instead.
// The narrower representation never limits the number of destinations.
type writtenRange struct {
	start, stop uint32
}

func compactWrittenRange(s text.Segment) (writtenRange, bool) {
	if s.Padding != 0 || s.ForceNewline || s.Start < 0 || s.Stop < 0 ||
		uint64(s.Start) > math.MaxUint32 || uint64(s.Stop) > math.MaxUint32 {
		return writtenRange{}, false
	}
	return writtenRange{start: uint32(s.Start), stop: uint32(s.Stop)}, true
}

func (r writtenRange) segment() text.Segment {
	return text.Segment{Start: int(r.start), Stop: int(r.stop)}
}

var writtenKey = parser.NewContextKey() //nolint:gochecknoglobals // a key is made once, as goldmark's are

func writtenOf(pc parser.Context) *written {
	if w, ok := pc.Get(writtenKey).(*written); ok {
		return w
	}
	w := &written{uses: map[ast.Node]string{}, defs: map[string]text.Segment{}}
	pc.Set(writtenKey, w)
	return w
}

// inline records n's destination, written at dest; an empty one is not.
func (w *written) inline(n ast.Node, dest text.Segment) {
	if dest.Len() <= 0 {
		return
	}
	if at, ok := compactWrittenRange(dest); ok {
		switch n := n.(type) {
		case *ast.Link:
			if w.links == nil {
				w.links = make(map[*ast.Link]writtenRange)
			}
			w.links[n] = at
			delete(w.at, n)
			return
		case *ast.Image:
			if w.images == nil {
				w.images = make(map[*ast.Image]writtenRange)
			}
			w.images[n] = at
			delete(w.at, n)
			return
		}
	}
	// A node has only its latest record, whichever representation it needs.
	switch n := n.(type) {
	case *ast.Link:
		delete(w.links, n)
	case *ast.Image:
		delete(w.images, n)
	}
	if w.at == nil {
		w.at = make(map[ast.Node]text.Segment)
	}
	w.at[n] = dest
}

// reference records that n uses the definition labeled ref.
func (w *written) reference(n ast.Node, ref []byte) {
	w.uses[n] = util.ToLinkReference(ref)
}

// define records the destination of the definition labeled label, unless
// one came before it: the first definition of a label is the one links use
// (CommonMark; goldmark's Context.AddReference).
func (w *written) define(label []byte, dest text.Segment) {
	key := util.ToLinkReference(label)
	if _, ok := w.defs[key]; !ok {
		w.defs[key] = dest
	}
}

// Destinations tells where the parse with pc found a link's or an image's
// destination written: its own, or its definition's for a reference link.
// An empty destination has none.
func Destinations(pc parser.Context) func(n ast.Node) (text.Segment, bool) {
	w := writtenOf(pc)
	return func(n ast.Node) (text.Segment, bool) {
		switch n := n.(type) {
		case *ast.Link:
			if at, ok := w.links[n]; ok {
				return at.segment(), true
			}
		case *ast.Image:
			if at, ok := w.images[n]; ok {
				return at.segment(), true
			}
		}
		if s, ok := w.at[n]; ok {
			return s, true
		}
		if key, ok := w.uses[n]; ok {
			s, ok := w.defs[key]
			return s, ok && s.Len() > 0
		}
		return text.Segment{}, false
	}
}
