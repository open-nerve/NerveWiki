package domain_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// mustGlob is pattern, parsed.
func mustGlob(t testing.TB, pattern string) domain.Glob {
	t.Helper()
	g, err := domain.ParseGlob("pattern", pattern)
	if err != nil {
		t.Fatalf("ParseGlob(%q): %v", pattern, err)
	}
	return g
}

func TestParseGlob(t *testing.T) {
	for _, tt := range []struct{ pattern, want string }{
		{"**/*.md", ""},
		{"/Guide/*.PNG", ""},
		{"A/**", ""},
		{"**", ""},
		{"a{b,c}!", ""}, // { } , ! are a title's characters
		{"*.md/**/x?", ""},
		{strings.Repeat("?", 32), ""},
		{strings.Repeat("a/", 10) + "a/**/**", ""}, // 11 segments besides the **s
		{strings.Repeat("a", 1024), ""},
		{strings.Repeat("a", 1025), "validation_failed pattern too_long"},
		{"A/", "validation_failed pattern invalid_format"},
		{"", "validation_failed pattern invalid_format"},
		{"A//B", "validation_failed pattern invalid_format"},
		{"./A", "validation_failed pattern invalid_format"},
		{"A/../B", "validation_failed pattern invalid_format"},
		{"[ab].md", "validation_failed pattern invalid_format"},
		{"a]b", "validation_failed pattern invalid_format"},
		{`a\*`, "validation_failed pattern invalid_format"},
		{"a**b", "validation_failed pattern invalid_format"},
		{"**.md", "validation_failed pattern invalid_format"},
		{strings.Repeat("?", 33), "validation_failed pattern invalid_format"},
		{strings.Repeat("a/", 11) + "a", "validation_failed pattern too_long"},
		{"A/a:b", "validation_failed pattern invalid_format"},
		{`A/a"b`, "validation_failed pattern invalid_format"},
		{"A/a#b", "validation_failed pattern invalid_format"},
		{"A/a\x00b", "validation_failed pattern invalid_format"},
		{"A/a\tb", "validation_failed pattern invalid_format"},
		{"A/a" + string(rune(0x202e)) + "b", "validation_failed pattern invalid_format"},
		{"A/ *.md", "validation_failed pattern invalid_format"},
		{"A/*.md ", "validation_failed pattern invalid_format"},
		{"\xff", "validation_failed pattern invalid_format"},
	} {
		_, err := domain.ParseGlob("pattern", tt.pattern)
		got := ""
		if err != nil {
			got = problemOf(err)
		}
		if got != tt.want {
			t.Errorf("ParseGlob(%.40q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

// regexpOf is pattern as a regular expression over a path of keys joined
// by "/" after a leading "/": the oracle the automata are tested against
// (M9/P1 design 3.10.1), with the literal parts folded as the glob folds
// them.
func regexpOf(pattern string) *regexp.Regexp {
	parts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	var b strings.Builder
	b.WriteString("^")
	for i, s := range parts {
		if s == "**" {
			if i > 0 && i == len(parts)-1 {
				b.WriteString("(?:/[^/]+)+")
			} else {
				b.WriteString("(?:/[^/]+)*")
			}
			continue
		}
		b.WriteString("/")
		for {
			j := strings.IndexAny(s, "*?")
			if j < 0 {
				b.WriteString(regexp.QuoteMeta(shared.TitleKey(s)))
				break
			}
			b.WriteString(regexp.QuoteMeta(shared.TitleKey(s[:j])))
			if s[j] == '*' {
				b.WriteString("[^/]*")
			} else {
				b.WriteString("[^/]")
			}
			s = s[j+1:]
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// patternPieces and titlePieces are the alphabets of the random patterns
// and titles: runes beyond ASCII, of two and three bytes, some sharing the
// sieve's low 10 bits (U+04E9 with é, U+6B22 with 漢), folding ones (ß,
// S), and the dots and ".md" of the path keys.
func patternPieces() []string {
	return []string{"a", "b", "é", "漢", "ß", "S", "*", "?", ".md", "x", ".", "-", "!"}
}

func titlePieces() []string {
	return []string{"a", "b", "é", string(rune(0x04e9)), "漢", string(rune(0x6b22)), "x.md", ".", "-", "!", "md", "a.md", "ß", "SS", "A"}
}

// randomPattern is a valid glob of up to five segments.
func randomPattern(rnd *rand.Rand) string {
	for {
		segs := make([]string, 1+rnd.IntN(5))
		for i := range segs {
			if rnd.IntN(4) == 0 {
				segs[i] = "**"
				continue
			}
			var b strings.Builder
			pieces := patternPieces()
			for range 1 + rnd.IntN(4) {
				b.WriteString(pieces[rnd.IntN(len(pieces))])
			}
			segs[i] = strings.ReplaceAll(b.String(), "**", "*")
		}
		p := strings.Join(segs, "/")
		if _, err := domain.ParseGlob("pattern", p); err == nil {
			return p
		}
	}
}

// randomKey is the title key of a title of up to three pieces.
func randomKey(rnd *rand.Rand) string {
	var b strings.Builder
	pieces := titlePieces()
	for range 1 + rnd.IntN(3) {
		b.WriteString(pieces[rnd.IntN(len(pieces))])
	}
	return shared.TitleKey(b.String())
}

// matchPath is the glob's match of a path of keys, segment by segment.
func matchPath(g domain.Glob, keys []string) bool {
	var work int64
	st := g.Start()
	for depth, k := range keys {
		if st = g.Step(st, k, depth, &work); st == 0 {
			return false
		}
	}
	return g.Accepts(st)
}

// The automata match what the regular expression does, on random patterns
// and paths (M9/P1 design 3.10.1, 3.19).
func TestTheGlobMatchesAsItsRegexpDoes(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	differ, pairs := 0, 0
	for range 20000 {
		pattern := randomPattern(rnd)
		g, re := mustGlob(t, pattern), regexpOf(pattern)
		for range 15 {
			keys := make([]string, 1+rnd.IntN(6))
			for i := range keys {
				keys[i] = randomKey(rnd)
			}
			pairs++
			if got, want := matchPath(g, keys), re.MatchString("/"+strings.Join(keys, "/")); got != want {
				if differ++; differ < 5 {
					t.Errorf("%q over %q: %v, the regexp %v", pattern, keys, got, want)
				}
			}
		}
	}
	if differ > 0 {
		t.Errorf("%d of %d pairs differ", differ, pairs)
	}
}

// tree is a test's tree of nodes by key, made into GlobNodes.
type tree struct {
	key      string
	page     bool
	children []*tree
}

// globNodes are trees as a glob walks them.
func globNodes(trees []*tree) []*domain.GlobNode {
	out := make([]*domain.GlobNode, len(trees))
	for i, t := range trees {
		kind := domain.KindAsset
		if t.page {
			kind = domain.KindPage
		}
		out[i] = &domain.GlobNode{Node: &domain.Node{Kind: kind, Name: t.key, NameKey: t.key}, Children: globNodes(t.children)}
	}
	return out
}

// randomTree is a tree of up to depth levels, its siblings' keys
// distinct, no attachment's key ending with ".md".
func randomTree(rnd *rand.Rand, depth int) []*tree {
	seen := map[string]bool{}
	var out []*tree
	for range rnd.IntN(5) + 1 {
		key := randomKey(rnd)
		page := rnd.IntN(3) != 0
		if !page && strings.HasSuffix(key, ".md") {
			key += "x"
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		n := &tree{key: key, page: page}
		if page && depth > 1 && rnd.IntN(3) != 0 {
			n.children = randomTree(rnd, depth-1)
		}
		out = append(out, n)
	}
	return out
}

// pathKeys are the path keys of every node of trees, under prefix.
func pathKeys(trees []*tree, prefix string, out *[]string) {
	for _, n := range trees {
		if !n.page {
			*out = append(*out, prefix+n.key)
			continue
		}
		*out = append(*out, prefix+n.key+".md")
		pathKeys(n.children, prefix+n.key+"/", out)
	}
}

// walk is the paths a walk of g over roots from after accepts, joined,
// with what it did.
func walk(t testing.TB, g domain.Glob, roots []*domain.GlobNode, after string, maxWork int64) ([]string, domain.GlobWalk) {
	t.Helper()
	var got []string
	w, err := g.Walk(context.Background(), roots, after, maxWork, func(keys []string, _ *domain.GlobNode) bool {
		got = append(got, strings.Join(keys, "/"))
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, w
}

// The walk of a tree, each node's state its parent's stepped, is the
// match of each path on its own, in the paths' joined byte order
// (M9/P1 design 3.10.1, 3.19).
func TestTheGlobWalkIsThePathsMatch(t *testing.T) {
	rnd := rand.New(rand.NewPCG(21, 23))
	for range 3000 {
		trees := randomTree(rnd, 1+rnd.IntN(11))
		var paths []string
		pathKeys(trees, "", &paths)
		slices.Sort(paths)
		roots := globNodes(trees)
		for range 10 {
			pattern := randomPattern(rnd)
			re := regexpOf(pattern)
			var want []string
			for _, p := range paths {
				if re.MatchString("/" + p) {
					want = append(want, p)
				}
			}
			got, w := walk(t, mustGlob(t, pattern), roots, "", 1<<62)
			if !slices.Equal(got, want) || w.Stopped {
				t.Fatalf("%q over %q:\n got %q\nwant %q", pattern, paths, got, want)
			}
		}
	}
}

// A walk stops when each answers false.
func TestTheGlobWalkStopsWhenEachDoes(t *testing.T) {
	roots := globNodes([]*tree{{key: "a", page: true}, {key: "b", page: true}, {key: "c", page: true}})
	var got []string
	w, err := mustGlob(t, "*.md").Walk(context.Background(), roots, "", 1<<62, func(keys []string, _ *domain.GlobNode) bool {
		got = append(got, keys[0])
		return len(got) < 2
	})
	if err != nil || !slices.Equal(got, []string{"a.md", "b.md"}) || w.Stopped || w.Last != "b.md" {
		t.Errorf("the walk gave %q, %+v, %v", got, w, err)
	}
}

// A unit of work is a mask's word over a key's byte, not its character:
// é counts two, 漢 three; and a parent's entries count GlobEntryWork each
// on entering it, all of them, though the walk stops after one (M9/P1
// design 3.10.1, review B N-1, N-8).
func TestTheGlobCountsItsWork(t *testing.T) {
	for _, tt := range []struct {
		pattern string
		key     string
		page    bool
		want    int64
	}{
		{"*", strings.Repeat("a", 300), false, domain.GlobEntryWork + 300},
		{"*", strings.Repeat("é", 100), false, domain.GlobEntryWork + 200},
		{"*", strings.Repeat("漢", 100), false, domain.GlobEntryWork + 300},
		{"*.md", "abc", true, domain.GlobEntryWork + 6},                                                      // the page's own entry, abc.md
		{"*" + strings.Repeat("a", 64) + "*", strings.Repeat("a", 100), false, domain.GlobEntryWork + 2*100}, // 65 states: two words
	} {
		_, w := walk(t, mustGlob(t, tt.pattern), globNodes([]*tree{{key: tt.key, page: tt.page}}), "", 1<<62)
		if w.Work != tt.want {
			t.Errorf("%q over %.20q counted %d, want %d", tt.pattern, tt.key, w.Work, tt.want)
		}
	}
	const n = 100_000
	trees := make([]*tree, n)
	for i := range trees {
		trees[i] = &tree{key: string(rune('a'+i%26)) + strings.Repeat("x", i/26%3) + string(rune(0x4e00+i))}
	}
	_, w := walk(t, mustGlob(t, "*"), globNodes(trees), "", 1)
	if !w.Stopped || w.Work < n*domain.GlobEntryWork || w.Work > n*domain.GlobEntryWork+64 || w.Before != n*domain.GlobEntryWork {
		t.Errorf("a parent of %d entries stopped after one: %+v", n, w)
	}
	// Resumed near its end, the walk re-enters the parent and steps none
	// of the entries before its cursor: their work is not done again.
	keys := make([]string, n)
	for i, tr := range trees {
		keys[i] = tr.key
	}
	slices.Sort(keys)
	cursor := keys[n*9/10]
	_, w = walk(t, mustGlob(t, "*"), globNodes(trees), cursor, 1)
	if w.Before != n*domain.GlobEntryWork || w.Last != keys[n*9/10+1] {
		t.Errorf("resumed after %q in a parent of %d entries: %+v; want its entries' work alone before %q", cursor, n, w, keys[n*9/10+1])
	}
}

// maxEntries is the most entries of a parent of trees: a page with
// children is two.
func maxEntries(trees []*tree) int {
	n := 0
	for _, c := range trees {
		n++
		if c.page && len(c.children) > 0 {
			n++
		}
	}
	for _, c := range trees {
		n = max(n, maxEntries(c.children))
	}
	return n
}

// Resumed from its cursor with a small bound, call after call, a walk is
// the walk without one: nothing repeated or skipped, the cursor strictly
// after the last, each call within its bound (the bound, or the work of
// re-entering the cursor's ancestors, and one parent's entries and two
// steps besides), the new work of the calls together no more than one
// whole walk's (M9/P1 design 3.10.1, review B N-2).
func TestTheGlobResumesFromItsCursor(t *testing.T) {
	rnd := rand.New(rand.NewPCG(31, 37))
	for range 3000 {
		trees := randomTree(rnd, 1+rnd.IntN(9))
		roots := globNodes(trees)
		pattern := randomPattern(rnd)
		g := mustGlob(t, pattern)
		whole, all := walk(t, g, roots, "", 1<<62)
		bound := int64(1 + rnd.IntN(5000))
		// A step runs each of the glob's states over a key of three pieces
		// of 4 bytes at most, a page's ".md" after it.
		step := int64(len(strings.Split(pattern, "/"))+1) * 17 * (3*4 + 3)
		over := int64(domain.GlobEntryWork*maxEntries(trees)) + 2*step
		var got []string
		var fresh int64
		cursor := ""
		for calls := 0; ; calls++ {
			if calls > 10000 {
				t.Fatalf("%q: no end", pattern)
			}
			matched, w := walk(t, g, roots, cursor, bound)
			got = append(got, matched...)
			before := w.Work
			if w.Last != "" {
				before, fresh = w.Before, fresh+w.Work-w.Before
			}
			if limit := max(bound, before) + over; w.Work > limit {
				t.Fatalf("%q: a call's work %d is over %d (bound %d, re-entry %d)", pattern, w.Work, limit, bound, before)
			}
			if !w.Stopped {
				break
			}
			if w.Last <= cursor {
				t.Fatalf("%q: the cursor %q is not after %q", pattern, w.Last, cursor)
			}
			cursor = w.Last
		}
		if fresh > all.Work {
			t.Fatalf("%q: the calls' new work %d is over one whole walk's %d", pattern, fresh, all.Work)
		}
		if !slices.Equal(got, whole) {
			t.Fatalf("%q resumed by %d:\n got %q\nwant %q", pattern, bound, got, whole)
		}
	}
}

// remove deletes the node at the path key p, with what is below it.
func remove(trees *[]*tree, p string) bool {
	parts := strings.Split(p, "/")
	level := trees
	for i, s := range parts {
		last := i == len(parts)-1
		j := slices.IndexFunc(*level, func(n *tree) bool {
			own := n.key
			if n.page && last {
				own += ".md"
			}
			return own == s && (last || n.page)
		})
		if j < 0 {
			return false
		}
		if last {
			*level = slices.Delete(*level, j, j+1)
			return true
		}
		level = &(*level)[j].children
	}
	return false
}

// A walk resumed after its cursor's node went, or a node came, lands
// after it: each node there throughout comes once, none twice
// (M9/P1 design 3.10.1).
func TestTheGlobResumesAfterADeletedCursor(t *testing.T) {
	rnd := rand.New(rand.NewPCG(41, 43))
	for range 3000 {
		trees := randomTree(rnd, 1+rnd.IntN(7))
		pattern := randomPattern(rnd)
		g, re := mustGlob(t, pattern), regexpOf(pattern)
		var initial []string
		pathKeys(trees, "", &initial)
		gone := map[string]bool{}
		seen := map[string]int{}
		cursor := ""
		for calls := 0; ; calls++ {
			if calls > 10000 {
				t.Fatalf("%q: no end", pattern)
			}
			matched, w := walk(t, g, globNodes(trees), cursor, int64(1+rnd.IntN(3000)))
			for _, m := range matched {
				seen[m]++
			}
			if !w.Stopped {
				break
			}
			cursor = w.Last
			switch rnd.IntN(3) {
			case 0:
				if remove(&trees, cursor) {
					for _, p := range initial {
						if p == cursor || strings.HasPrefix(p, strings.TrimSuffix(cursor, ".md")+"/") {
							gone[p] = true
						}
					}
				}
			case 1:
				key := "new" + string(rune('a'+rnd.IntN(26))) + string(rune('a'+calls%26))
				if !slices.ContainsFunc(trees, func(n *tree) bool { return n.key == key }) {
					trees = append(trees, &tree{key: key})
				}
			}
		}
		for p, n := range seen {
			if n > 1 {
				t.Fatalf("%q: %q came %d times", pattern, p, n)
			}
		}
		for _, p := range initial {
			if !gone[p] && re.MatchString("/"+p) && seen[p] != 1 {
				t.Fatalf("%q: %q came %d times", pattern, p, seen[p])
			}
		}
	}
}

// A glob without wildcards names one path: its keys.
func TestALiteralGlobNamesOnePath(t *testing.T) {
	if keys, ok := mustGlob(t, "/Guide/Straße").Literal(); !ok || !slices.Equal(keys, []string{"guide", "strasse"}) {
		t.Errorf("Literal() = %q, %v", keys, ok)
	}
	if _, ok := mustGlob(t, "Guide/*").Literal(); ok {
		t.Error("Guide/* is literal")
	}
}

// A walk whose ctx ended answers its error.
func TestTheGlobWalkStopsWithItsContext(t *testing.T) {
	trees := make([]*tree, 5000)
	for i := range trees {
		trees[i] = &tree{key: strings.Repeat("a", 1+i%7) + string(rune(0x4e00+i))}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mustGlob(t, "*").Walk(ctx, globNodes(trees), "", 1<<62, func([]string, *domain.GlobNode) bool { return true }); !errors.Is(err, context.Canceled) {
		t.Errorf("the walk answered %v", err)
	}
}
