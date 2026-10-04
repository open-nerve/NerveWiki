package harden

import (
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
	at   map[ast.Node]text.Segment // an inline link's or image's destination
	uses map[ast.Node]string       // a reference link's or image's definition, by its key
	defs map[string]text.Segment   // each definition's destination, the first of a key
}

var writtenKey = parser.NewContextKey() //nolint:gochecknoglobals // a key is made once, as goldmark's are

func writtenOf(pc parser.Context) *written {
	if w, ok := pc.Get(writtenKey).(*written); ok {
		return w
	}
	w := &written{at: map[ast.Node]text.Segment{}, uses: map[ast.Node]string{}, defs: map[string]text.Segment{}}
	pc.Set(writtenKey, w)
	return w
}

// inline records n's destination, written at dest; an empty one is not.
func (w *written) inline(n ast.Node, dest text.Segment) {
	if dest.Len() > 0 {
		w.at[n] = dest
	}
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
