package harden

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
)

// linkArena holds only its current block. Every returned node keeps its
// block alive, and no slot is reused, even when a context parses again.
// Image temporaries use individual allocations, so a nearby retained Link
// never holds otherwise dead image temporaries in its block.
type linkArena struct {
	block []ast.Link
	next  int
}

const maxLinkBlock = 256

var linkArenaKey = parser.NewContextKey() //nolint:gochecknoglobals // a key is made once, as goldmark's are

// newLink is ast.NewLink's zero-valued node in a context's bounded block.
// Begin with one slot so a small document reserves no large tail. Once a
// block has returned a pointer, never grow or copy it: only replace the
// current block with a new one. The AST and destination records own the
// old pointers. Nothing is pooled or retained by the shared parser.
func newLink(pc parser.Context, image bool) *ast.Link {
	if image {
		return ast.NewLink()
	}
	a, _ := pc.Get(linkArenaKey).(*linkArena)
	if a == nil {
		a = &linkArena{}
		pc.Set(linkArenaKey, a)
	}
	if a.next == len(a.block) {
		a.block = make([]ast.Link, min(max(1, 2*len(a.block)), maxLinkBlock))
		a.next = 0
	}
	n := &a.block[a.next]
	a.next++
	return n
}
