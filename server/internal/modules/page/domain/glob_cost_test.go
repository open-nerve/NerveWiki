//go:build !race

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// globTimeBound is how long a list's matching may take at MaxGlobWork:
// about three times the slowest walk here (M9/P1 design 3.10.1; the
// numbers are in its section 9).
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

// guardTree is review B's guard tree over keys of r: 100,000 paths of 11
// segments of about 255 bytes, sharing nothing, 1.1 million nodes: each
// root above a chain of 10 pages, the chains' nodes alike but for their
// children, whose keys the walk reads as it would any other's.
func guardTree(r string) []*domain.GlobNode {
	chain := make([]*domain.Node, 11)
	for d := 1; d < 11; d++ {
		k := guardKey(r, d)
		chain[d] = &domain.Node{Kind: domain.KindPage, Name: k, NameKey: k}
	}
	roots := make([]*domain.GlobNode, 100_000)
	for i := range roots {
		k := guardKey(r, i)
		root := &domain.GlobNode{Node: &domain.Node{Kind: domain.KindPage, Name: k, NameKey: k}}
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

// The worst globs over the guard trees (keys of a, é and 漢: one, two and
// three bytes a character) and a balanced tree stop at MaxGlobWork, or
// end before it, within their bound of work, MaxGlobWork and a parent's
// entries and two steps besides, and of time; so does a call resumed from
// a cursor near the end of each guard tree, which re-enters its root's
// 200,000 entries (M9/P1 design 3.10.1, review B I5, N-1, N-2).
func TestTheWorstGlobsStopWithinTheirWork(t *testing.T) {
	type subject struct {
		name   string
		roots  []*domain.GlobNode
		r      string
		resume bool // from a cursor near the end too
	}
	for _, s := range []func() subject{
		func() subject { return subject{"balanced 10×5", balancedTree(10, 5), "a", false} },
		func() subject { return subject{"guard a", guardTree("a"), "a", true} },
		func() subject { return subject{"guard é", guardTree("é"), "é", true} },
		func() subject { return subject{"guard 漢", guardTree("漢"), "漢", true} },
	} {
		s := s()
		entries := 2 * len(s.roots)
		for _, pattern := range worstGlobs(s.r) {
			g := mustGlob(t, pattern)
			start := time.Now()
			_, w := walk(t, g, s.roots, "", domain.MaxGlobWork)
			took := time.Since(start)
			// A step runs each of the glob's 45 states at most over a key
			// of 255 bytes, at 17 words a segment at most.
			over := int64(domain.GlobEntryWork*entries) + 2*45*17*258
			t.Logf("%s: %.40q (%d bytes): %v, work %d, stopped %v", s.name, pattern, len(pattern), took, w.Work, w.Stopped)
			if took > globTimeBound || w.Work > domain.MaxGlobWork+over {
				t.Errorf("%s: %.40q took %v and %d units of work", s.name, pattern, took, w.Work)
			}
		}
		if !s.resume {
			continue
		}
		pattern := capped("**/*" + piece(s.r, 1000, 20, "b") + "*/**")
		g := mustGlob(t, pattern)
		// The root tenth from the end in the walk's order.
		last := slices.Clone(s.roots)
		slices.SortFunc(last, func(a, b *domain.GlobNode) int { return strings.Compare(a.Node.NameKey, b.Node.NameKey) })
		var keys []string
		for n := last[len(last)-10]; n != nil; {
			keys = append(keys, n.Node.NameKey)
			if len(n.Children) == 0 {
				break
			}
			n = n.Children[0]
		}
		cursor := strings.Join(keys, "/") + ".md"
		start := time.Now()
		_, w := walk(t, g, s.roots, cursor, domain.MaxGlobWork)
		took := time.Since(start)
		t.Logf("%s: resumed near the end: %v, work %d, stopped %v", s.name, took, w.Work, w.Stopped)
		if took > globTimeBound || w.Stopped {
			t.Errorf("%s: resumed near the end, it took %v and stopped %v", s.name, took, w.Stopped)
		}
	}
}
