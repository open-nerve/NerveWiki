package domain

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxGlobBytes and MaxGlobWildcards bound a glob (M9 design 4.8);
// MaxGlobWork bounds the matching of one list call: when it runs out, list
// stops and answers next_cursor, as at the answer's bound (M9/P1 design
// 3.10.1). A unit of work is a mask's word stepped over a key's byte;
// GlobEntryWork is an entry's string, its place in the sort and its
// state's step, charged for each entry of a parent on entering it. Both
// are measured: 300 million units are about two thirds of a second here
// in the slowest shape, keys twice their titles' bytes (M9/P1 S2 review,
// A-M2), a third of the guard's bound.
const (
	MaxGlobBytes     = 1024
	MaxGlobWildcards = 32
	MaxGlobWork      = 300_000_000
	GlobEntryWork    = 256
)

// Glob is a pattern over the paths of a notebook's nodes (M9 design 4.8):
// * any characters within a segment, ? one, ** any segments. Compiled: a
// bit-parallel automaton per segment, and one over the segments.
type Glob struct {
	segments []globSegment
	// need is how many segments a path needs after the state j to reach
	// the end: the states that cannot within MaxPathSegments drop.
	need     []int
	words    int  // the most mask words of a segment
	literal  bool // no wildcard: it names one path
	keys     []string
	accepted GlobState
}

// The kinds of a glob's segments.
const (
	segmentLiteral = iota // a segment's pattern, with * and ? maybe
	segmentAny            // ** : no segment or any
	segmentOne            // the one segment at least of a trailing /**
)

// globSegment is a segment's automaton (Shift-And, Baeza-Yates and
// Gonnet): its positions are the folded characters and ?s of its pattern
// once the *s are out, a state the positions matched so far, a bit each
// after the first, the start's; it reads a key's character in one shift
// and a few ands and ors per mask word.
type globSegment struct {
	kind  int
	words int // mask words: ⌈(positions + 1) / 64⌉
	final int // the bit of the state that matched every position
	// ascii and others are the positions each character matches, which
	// include the ?s'; a character of neither matches the ?s alone.
	ascii  [utf8.RuneSelf][]uint64
	others map[rune][]uint64
	// sieve holds the low 10 bits of others' characters: a key's character
	// that is not in it matches the ?s alone, without a look in the map.
	sieve [1024 / 64]uint64
	any   []uint64 // the ?s' positions
	// stay are the states that read any character and stay: after a *.
	stay []uint64
}

// ParseGlob checks s as field, a glob, 422 on field when it breaks a rule:
// at most MaxGlobBytes; one leading "/" may go; no empty, "." or ".."
// segment; no [, ] or \, which would be a class or an escape; ** alone in
// its segment; at most MaxGlobWildcards of *, ? and **; at most
// MaxPathSegments segments besides the **s, more matching nothing; and
// the literal parts characters a title may hold, without white space at a
// segment's ends.
func ParseGlob(field, s string) (Glob, error) {
	if len(s) > MaxGlobBytes {
		return Glob{}, invalidPath(field, shared.FieldTooLong, fmt.Sprintf("must be at most %d bytes", MaxGlobBytes))
	}
	if !utf8.ValidString(s) {
		return Glob{}, invalidPath(field, shared.FieldInvalidFormat, "must be UTF-8")
	}
	segments := strings.Split(strings.TrimPrefix(s, "/"), "/")
	if !wellFormed(segments) {
		return Glob{}, invalidPath(field, shared.FieldInvalidFormat, "must not hold empty, . or .. segments, nor end with /")
	}
	wildcards, literals := 0, 0
	for i, seg := range segments {
		switch {
		case strings.ContainsAny(seg, `[]\`):
			return Glob{}, invalidPath(field, shared.FieldInvalidFormat,
				fmt.Sprintf(`segment %d holds [, ] or \, but a glob has no character classes or escapes`, i+1))
		case seg == "**":
			wildcards++
			continue
		case strings.Contains(seg, "**"):
			return Glob{}, invalidPath(field, shared.FieldInvalidFormat, fmt.Sprintf("segment %d holds ** beside other characters: ** is a segment of its own", i+1))
		}
		wildcards += strings.Count(seg, "*") + strings.Count(seg, "?")
		literals++
	}
	switch {
	case wildcards > MaxGlobWildcards:
		return Glob{}, invalidPath(field, shared.FieldInvalidFormat, fmt.Sprintf("holds more than %d wildcards", MaxGlobWildcards))
	case literals > MaxPathSegments:
		return Glob{}, invalidPath(field, shared.FieldTooLong, fmt.Sprintf("must hold at most %d segments besides **", MaxPathSegments))
	}
	for i, seg := range segments {
		if strings.ContainsFunc(seg, func(r rune) bool { return r != '*' && r != '?' && !shared.TitleHolds(r) }) {
			return Glob{}, invalidPath(field, shared.FieldInvalidFormat, fmt.Sprintf("segment %d holds a character no title can hold", i+1))
		}
		if strings.TrimSpace(seg) != seg {
			return Glob{}, invalidPath(field, shared.FieldInvalidFormat, fmt.Sprintf("segment %d must not start or end with white space", i+1))
		}
	}
	return compileGlob(segments, wildcards == 0), nil
}

// compileGlob is the automata of segments, checked.
func compileGlob(segments []string, literal bool) Glob {
	g := Glob{literal: literal, words: 1}
	for i, seg := range segments {
		switch {
		case seg == "**" && i > 0 && i == len(segments)-1:
			// A trailing /** is everything below: one segment at least.
			g.segments = append(g.segments, globSegment{kind: segmentOne}, globSegment{kind: segmentAny})
		case seg == "**":
			g.segments = append(g.segments, globSegment{kind: segmentAny})
		default:
			c := compileSegment(seg)
			g.words = max(g.words, c.words)
			g.segments = append(g.segments, c)
		}
		if literal {
			g.keys = append(g.keys, shared.TitleKey(seg))
		}
	}
	g.need = make([]int, len(g.segments)+1)
	for j := len(g.segments) - 1; j >= 0; j-- {
		g.need[j] = g.need[j+1]
		if g.segments[j].kind != segmentAny {
			g.need[j]++
		}
	}
	g.accepted = 1 << len(g.segments)
	return g
}

// compileSegment is seg's automaton: its literal parts each folded as
// shared.TitleKey folds a title, so a ? matches one folded character.
func compileSegment(seg string) globSegment {
	var positions []rune // a literal's character, or '?'
	var starBefore []bool
	star := false
	for len(seg) > 0 {
		i := strings.IndexAny(seg, "*?")
		if i < 0 {
			i = len(seg)
		}
		for _, r := range shared.TitleKey(seg[:i]) {
			positions, starBefore, star = append(positions, r), append(starBefore, star), false
		}
		if i == len(seg) {
			break
		}
		if seg[i] == '*' {
			star = true
		} else {
			positions, starBefore, star = append(positions, '?'), append(starBefore, star), false
		}
		seg = seg[i+1:]
	}
	m := len(positions)
	words := (m + 1 + 63) / 64
	c := globSegment{kind: segmentLiteral, words: words, final: m, others: map[rune][]uint64{},
		any: make([]uint64, words), stay: make([]uint64, words)}
	masks := map[rune][]uint64{}
	for i, r := range positions {
		if starBefore[i] {
			setBit(c.stay, i)
		}
		if r == '?' {
			setBit(c.any, i+1)
			continue
		}
		mask, ok := masks[r]
		if !ok {
			mask = make([]uint64, words)
			masks[r] = mask
		}
		setBit(mask, i+1)
	}
	if star {
		setBit(c.stay, m)
	}
	for r, mask := range masks {
		for k := range mask {
			mask[k] |= c.any[k]
		}
		if r < utf8.RuneSelf {
			c.ascii[r] = mask
		} else {
			c.others[r] = mask
			c.sieve[(r&1023)>>6] |= 1 << (r & 63)
		}
	}
	return c
}

func setBit(mask []uint64, i int) { mask[i/64] |= 1 << (i % 64) }

// match reports whether the segment matches key and suffix, read as one
// key (a page's own entry is its key and ".md"), adding to work a unit for
// each mask word and byte it reads: it stops at the first character after
// which no state is left. buf holds two masks' words at least.
func (c *globSegment) match(key, suffix string, work *int64, buf []uint64) bool {
	w := c.words
	d, next := buf[:w], buf[w:2*w]
	clear(d)
	d[0] = 1
	for _, s := range [2]string{key, suffix} {
		for i := 0; i < len(s); {
			r, n := rune(s[i]), 1
			if r >= utf8.RuneSelf {
				r, n = utf8.DecodeRuneInString(s[i:])
			}
			i += n
			var mask []uint64
			switch {
			case r < utf8.RuneSelf:
				mask = c.ascii[r]
			case c.sieve[(r&1023)>>6]&(1<<(r&63)) != 0:
				mask = c.others[r]
			}
			if mask == nil {
				mask = c.any
			}
			var carry, alive uint64
			for k := range w {
				x := d[k]
				next[k] = ((x<<1)|carry)&mask[k] | x&c.stay[k]
				carry, alive = x>>63, alive|next[k]
			}
			*work += int64(w * n)
			d, next = next, d
			if alive == 0 {
				return false
			}
		}
	}
	return d[c.final/64]&(1<<(c.final%64)) != 0
}

// GlobState is where a path's walk through the glob is: the set of the
// glob's segments matched so far. A node's state is its parent's stepped by
// its own key, so a walk of the tree steps each key once, not once a path.
type GlobState uint64

// Start is the state of a path of no segment.
func (g Glob) Start() GlobState { return g.closure(1) }

// Step is st after one more segment of key at depth (the segments before
// it), adding its work, the mask words × len(key), to work; states that
// cannot reach the end within MaxPathSegments drop, so 0 means nothing
// below can match.
func (g Glob) Step(st GlobState, key string, depth int, work *int64) GlobState {
	return g.step(st, key, "", depth, work, make([]uint64, 2*g.words))
}

// step is Step over key and suffix read as one, with buf for the masks.
func (g Glob) step(st GlobState, key, suffix string, depth int, work *int64, buf []uint64) GlobState {
	var out GlobState
	for j := range g.segments {
		if st&(1<<j) == 0 {
			continue
		}
		switch c := &g.segments[j]; c.kind {
		case segmentAny:
			out |= 1 << j
		case segmentOne:
			out |= 1 << (j + 1)
		default:
			if c.match(key, suffix, work, buf) {
				out |= 1 << (j + 1)
			}
		}
	}
	out = g.closure(out)
	for j := range g.need {
		if out&(1<<j) != 0 && depth+1+g.need[j] > MaxPathSegments {
			out &^= 1 << j
		}
	}
	return out
}

// closure is st with the state after each ** it holds: ** may match no
// segment.
func (g Glob) closure(st GlobState) GlobState {
	for j := range g.segments {
		if st&(1<<j) != 0 && g.segments[j].kind == segmentAny {
			st |= 1 << (j + 1)
		}
	}
	return st
}

// Accepts reports whether a path whose walk is at st matches the glob.
func (g Glob) Accepts(st GlobState) bool { return st&g.accepted != 0 }

// Literal reports whether the glob has no wildcard, and its segments'
// keys: it names one path at most.
func (g Glob) Literal() ([]string, bool) { return g.keys, g.literal }

// GlobNode is a node with its children, as a glob's walk reads the tree:
// the use case groups the nodes it read by parent, once.
type GlobNode struct {
	Node     *Node
	Children []*GlobNode
}

// GlobWalk is what a walk did: the path key of the last node it examined;
// whether it stopped because its work reached its bound; its work, and
// the work before it examined its first node, re-entering the cursor's
// ancestors and the parents on the way.
type GlobWalk struct {
	Last    string
	Stopped bool
	Work    int64
	Before  int64
}

// Walk walks the tree under roots in path-key order (each segment's
// NameKey, a page's last with ".md") after the path key after ("" from the
// start), each node's state stepped from its parent's: each(keys, n) for
// every node g accepts, its path's keys, until each answers false. It stops
// between nodes once its work reaches max, after at least one node was
// examined in this call, and answers the path key of the last node
// examined; resuming skips what sorts before after at each level and steps
// only after's ancestors. Every globCheck entries it looks at ctx, and
// answers its error once it ended.
//
// Under each parent the entries are a page's own ("key.md"), a page's
// children as one block ("key/"), and an attachment's ("key"), by their
// bytes: no title key holds a "/", so a block's paths come together in the
// joined order, and ordering each parent's entries orders the paths
// (M9/P1 design 3.10.1).
func (g Glob) Walk(ctx context.Context, roots []*GlobNode, after string, max int64, each func(keys []string, n *GlobNode) bool,
) (GlobWalk, error) {
	w := globWalker{g: g, max: max, each: each, buf: make([]uint64, 2*g.words), ctx: ctx}
	var cursor []string
	if after != "" {
		cursor = strings.Split(after, "/")
	}
	w.visit(roots, g.Start(), cursor)
	if w.err != nil {
		return GlobWalk{}, w.err
	}
	out := GlobWalk{Stopped: w.stopped, Work: w.work, Before: w.before}
	if w.examined > 0 {
		out.Last = strings.Join(append(w.lastPath, w.last.key()+w.last.suffix), "/")
	}
	return out, nil
}

// globCheck is how many entries a walk looks at between its looks at its
// ctx.
const globCheck = 1024

// globWalker is a walk's state: the keys of the blocks entered, as a
// stack; the last own entry examined and the keys above it.
type globWalker struct {
	g        Glob
	max      int64
	each     func(keys []string, n *GlobNode) bool
	buf      []uint64
	path     []string
	work     int64
	before   int64
	examined int
	entries  int // looked at
	last     globEntry
	lastPath []string
	stopped  bool
	ctx      context.Context
	err      error // ctx's, once it ended
}

// globEntry is an entry of a parent: a node's own, or a page's block of
// children; suffix is ".md" for a page's own, "/" for a block and "" for
// an attachment.
type globEntry struct {
	n      *GlobNode
	suffix string
}

func (e globEntry) key() string { return e.n.Node.NameKey }

// compareEntries orders two entries by their strings, the key and suffix
// joined.
func compareEntries(a, b globEntry) int {
	return compareJoined(a.key(), a.suffix, b.key(), b.suffix)
}

// compareJoined compares a1+a2 with b1+b2 without joining them.
func compareJoined(a1, a2, b1, b2 string) int {
	for {
		if a1 == "" {
			a1, a2 = a2, ""
		}
		if b1 == "" {
			b1, b2 = b2, ""
		}
		if a1 == "" || b1 == "" {
			return cmp.Compare(len(a1), len(b1))
		}
		n := min(len(a1), len(b1))
		if c := strings.Compare(a1[:n], b1[:n]); c != 0 {
			return c
		}
		a1, b1 = a1[n:], b1[n:]
	}
}

// visit walks the entries of the parent whose children are children and
// whose state is st, from the cursor's segments within it when there are
// any, and reports whether the walk goes on.
func (w *globWalker) visit(children []*GlobNode, st GlobState, cursor []string) bool {
	entries := make([]globEntry, 0, len(children))
	for _, c := range children {
		switch {
		case c.Node.Kind != KindPage:
			entries = append(entries, globEntry{n: c})
		case len(c.Children) > 0:
			entries = append(entries, globEntry{n: c, suffix: pageSuffix}, globEntry{n: c, suffix: "/"})
		default:
			entries = append(entries, globEntry{n: c, suffix: pageSuffix})
		}
	}
	w.work += int64(GlobEntryWork * len(entries))
	slices.SortFunc(entries, compareEntries)
	from, at := 0, ""
	if len(cursor) > 0 {
		// The cursor's own entry, or the block of its ancestor.
		at = cursor[0]
		if len(cursor) > 1 {
			at += "/"
		}
		from, _ = slices.BinarySearchFunc(entries, at, func(e globEntry, s string) int {
			return compareJoined(e.key(), e.suffix, s, "")
		})
	}
	depth := len(w.path)
	for i := from; i < len(entries); i++ {
		e := entries[i]
		if w.examined > 0 && w.work >= w.max {
			w.stopped = true
			return false
		}
		if w.entries++; w.entries%globCheck == 0 {
			if w.err = w.ctx.Err(); w.err != nil {
				return false
			}
		}
		resumed := len(cursor) > 0 && compareJoined(e.key(), e.suffix, at, "") == 0
		if e.suffix == "/" {
			// Matched in full, the block's path is no node: what its
			// children need is the states before the end.
			next := w.g.step(st, e.key(), "", depth, &w.work, w.buf) &^ w.g.accepted
			if next == 0 {
				continue
			}
			var below []string
			if resumed {
				below = cursor[1:]
			}
			w.path = append(w.path, e.key())
			goOn := w.visit(e.n.Children, next, below)
			w.path = w.path[:depth]
			if !goOn {
				return false
			}
			continue
		}
		if resumed {
			continue // examined by the call before
		}
		if w.examined == 0 {
			w.before = w.work
		}
		w.examined++
		w.last, w.lastPath = e, append(w.lastPath[:0], w.path...)
		if !w.g.Accepts(w.g.step(st, e.key(), e.suffix, depth, &w.work, w.buf)) {
			continue
		}
		keys := append(slices.Clip(w.path), e.key()+e.suffix)
		if !w.each(keys, e.n) {
			return false
		}
	}
	return true
}
