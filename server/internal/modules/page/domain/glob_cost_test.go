//go:build !race

package domain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// globTimeBound is how long a list's matching may take at MaxGlobWork:
// about three times the slowest walk here, the ligature tree's (M9/P1
// design 3.10.1; the numbers are in its section 9, and fix round 1 of S2's
// report).
const globTimeBound = 2 * time.Second

// guardKey is a key of about 255 bytes made of r repeated, ending in four
// ASCII letters that tell siblings apart.
func guardKey(r string, i int) string {
	const letters = "bcdefghijklmnopqrstuvwxyz"
	var b strings.Builder
	b.WriteString(strings.Repeat(r, (255-4)/len(r)))
	for range 4 {
		b.WriteByte(letters[i%len(letters)])
		i /= len(letters)
	}
	return b.String()
}

// guardTree is review B's guard tree over titles of r: 100,000 paths of
// 11 segments of about 255 bytes, sharing nothing, 1.1 million nodes:
// each root above a chain of 10 pages, the chains' nodes alike but for
// their children, whose keys the walk reads as it would any other's. A
// key is its title's: of և, which folds to եւ, twice its bytes (M9/P1 S2
// review, A-M2), the longest a title of 255 bytes has.
func guardTree(r string) []*domain.GlobNode {
	chain := make([]*domain.Node, 11)
	for d := 1; d < 11; d++ {
		k := guardKey(r, d)
		chain[d] = &domain.Node{Kind: domain.KindPage, Name: k, NameKey: shared.TitleKey(k)}
	}
	roots := make([]*domain.GlobNode, 100_000)
	for i := range roots {
		k := guardKey(r, i)
		root := &domain.GlobNode{Node: &domain.Node{Kind: domain.KindPage, Name: k, NameKey: shared.TitleKey(k)}}
		cur := root
		for d := 1; d < 11; d++ {
			c := &domain.GlobNode{Node: chain[d]}
			cur.Children = []*domain.GlobNode{c}
			cur = c
		}
		roots[i] = root
	}
	return roots
}

// balancedTree is a tree of fan children a page, depth levels deep, its
// keys of 255 bytes: 111,110 nodes for 10 and 5.
func balancedTree(fan, depth int) []*domain.GlobNode {
	if depth == 0 {
		return nil
	}
	out := make([]*domain.GlobNode, fan)
	for i := range out {
		k := guardKey("a", i)
		out[i] = &domain.GlobNode{Node: &domain.Node{Kind: domain.KindPage, Name: k, NameKey: k}, Children: balancedTree(fan, depth-1)}
	}
	return out
}

// piece is lits of r with qs ?s spread among them, and tail.
func piece(r string, lits, qs int, tail string) string {
	var b strings.Builder
	step := lits / (qs + 1)
	for range qs {
		b.WriteString(strings.Repeat(r, step))
		b.WriteString("?")
	}
	b.WriteString(strings.Repeat(r, lits-step*qs))
	b.WriteString(tail)
	return b.String()
}

// groups is n groups of *p* between **s.
func groups(n int, p string) string {
	var b strings.Builder
	b.WriteString("**")
	for range n {
		b.WriteString("/*" + p + "*/**")
	}
	return b.String()
}

// capped is p cut to MaxGlobBytes at a character's start.
func capped(p string) string {
	for len(p) > domain.MaxGlobBytes || !utf8.ValidString(p) {
		p = p[:len(p)-1]
	}
	return strings.TrimSuffix(p, "/")
}

// worstGlobs are the costly shapes of review B and the drafts, their
// literal characters r (M9/P1 design 3.10.1): long pieces with ?s between
// a leading **/* and a trailing */**, pieces at the bound's bytes,
// groups of *P* between **s, a long literal, a run of *a.
func worstGlobs(r string) []string {
	return []string{
		"**/*.md",
		"**/*u?u*/**",
		groups(3, "u"),
		groups(5, "u"),
		"**/*" + piece(r, 28, 28, "b") + "*/**",
		"**/*" + piece(r, 126, 28, "b") + "*/**",
		"**/*" + piece(r, 160, 28, "b") + "*/**",
		"**/*" + piece(r, 30, 30, "b"),
		"**/" + strings.Repeat(r+"*", 30) + "b/**",
		capped("**/*" + piece(r, 1000, 20, "b") + "*/**"),
		capped("**/*" + piece(r, 900, 26, "b") + "*/**"),
		capped(groups(4, piece(r, 240, 4, "b"))),
		capped("**/" + strings.Repeat(r, 1000) + "?/**"),
		capped("**/" + strings.Repeat("*"+r, 15) + strings.Repeat(r, 900) + "/**"),
		"**/*需求?*/**",
	}
}

// cursorAt is the cursor of the path down the chain of the root at in
// the walk's order, from the end when negative.
func cursorAt(roots []*domain.GlobNode, at int) string {
	sorted := slices.Clone(roots)
	slices.SortFunc(sorted, func(a, b *domain.GlobNode) int { return strings.Compare(a.Node.NameKey, b.Node.NameKey) })
	if at < 0 {
		at += len(sorted)
	}
	var keys []string
	for n := sorted[at]; n != nil; {
		keys = append(keys, n.Node.NameKey)
		if len(n.Children) == 0 {
			break
		}
		n = n.Children[0]
	}
	return strings.Join(keys, "/") + ".md"
}

// The worst globs over the guard trees (titles of a, é and 漢: one, two
// and three bytes a character; and of և, its keys twice its bytes) and a
// balanced tree stop at MaxGlobWork, or end before it, within their bound
// of work, MaxGlobWork and a parent's entries and two steps besides, and
// of time; so do calls resumed from a cursor in each guard tree, which
// re-enter its root's 200,000 entries: near the end, where little is left
// to match, and near the start, where all MaxGlobWork is (M9/P1 design
// 3.10.1, review B I5, N-1, N-2; S2 review A-M2). Each walk's time is
// logged, and each tree's slowest, for CI's log under -v. -short skips it:
// go test -short ./... runs the packages side by side, whose load sways a
// bound in time (CI's short leg took 2.6 s for a walk bound at 2; M9/P2 S1
// fix round 2), and make test-go-costs runs it alone.
func TestTheWorstGlobsStopWithinTheirWork(t *testing.T) {
	if testing.Short() {
		t.Skip("a bound in time, measured alone by make test-go-costs")
	}
	type subject struct {
		name   string
		roots  []*domain.GlobNode
		r      string // the literals' character, as its key has it
		resume bool   // from cursors in it too
	}
	for _, s := range []func() subject{
		func() subject { return subject{"balanced 10×5", balancedTree(10, 5), "a", false} },
		func() subject { return subject{"guard a", guardTree("a"), "a", true} },
		func() subject { return subject{"guard é", guardTree("é"), "é", true} },
		func() subject { return subject{"guard 漢", guardTree("漢"), "漢", true} },
		func() subject { return subject{"guard և", guardTree("և"), "եւ", true} },
	} {
		s := s()
		// A step runs each of the glob's 45 states at most over a key of
		// 510 bytes and ".md", at 17 words a segment at most.
		over := int64(domain.GlobEntryWork*2*len(s.roots)) + 2*45*17*513
		var slowest time.Duration
		var slowestOf string
		check := func(what, cursor string, g domain.Glob, ends bool) {
			start := time.Now()
			_, w := walk(t, g, s.roots, cursor, domain.MaxGlobWork)
			took := time.Since(start)
			t.Logf("%s: %s: %v, work %d, stopped %v", s.name, what, took, w.Work, w.Stopped)
			if took > slowest {
				slowest, slowestOf = took, what
			}
			if took > globTimeBound || w.Work > domain.MaxGlobWork+over || ends && w.Stopped {
				t.Errorf("%s: %s took %v and %d units of work, stopped %v", s.name, what, took, w.Work, w.Stopped)
			}
		}
		for _, pattern := range worstGlobs(s.r) {
			check(fmt.Sprintf("%.40q (%d bytes)", pattern, len(pattern)), "", mustGlob(t, pattern), false)
		}
		if s.resume {
			g := mustGlob(t, capped("**/*"+piece(s.r, 1000, 20, "b")+"*/**"))
			check("resumed at the 10th root from the end", cursorAt(s.roots, -10), g, true)
			check("resumed at the 10th root from the start", cursorAt(s.roots, 10), g, false)
		}
		t.Logf("%s: the slowest walk %v of %v, %s", s.name, slowest, globTimeBound, slowestOf)
	}
}
