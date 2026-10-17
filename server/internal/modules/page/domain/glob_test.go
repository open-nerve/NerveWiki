package domain_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

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
// S), and the dots and ".md" of the path keys. Long, they hold runs of 62
// to 65 and 126 to 129 characters too, of ASCII and of two bytes, ? and *
// among them: a segment's states in two and three words of the masks, and
// the carry between them (M9/P1 S2 review, A-I2).
func patternPieces(long bool) []string {
	short := []string{"a", "b", "é", "漢", "ß", "S", "*", "?", ".md", "x", ".", "-", "!"}
	if !long {
		return short
	}
	return append(short, strings.Repeat("a", 62), strings.Repeat("a", 63), strings.Repeat("a", 64), strings.Repeat("é", 65),
		strings.Repeat("a", 62)+"?", "?"+strings.Repeat("é", 63), strings.Repeat("a", 61)+"*b", strings.Repeat("a", 126),
		strings.Repeat("é", 127), strings.Repeat("a", 63)+"?"+strings.Repeat("a", 64), strings.Repeat("é", 64)+"*"+strings.Repeat("a", 65))
}

func titlePieces(long bool) []string {
	short := []string{"a", "b", "é", string(rune(0x04e9)), "漢", string(rune(0x6b22)), "x.md", ".", "-", "!", "md", "a.md", "ß", "SS", "A"}
	if !long {
		return short
	}
	return append(short, strings.Repeat("a", 62), strings.Repeat("a", 63), strings.Repeat("a", 64), strings.Repeat("a", 65),
		strings.Repeat("é", 63), strings.Repeat("é", 64), strings.Repeat("a", 63)+"b", strings.Repeat("a", 126),
		strings.Repeat("a", 127), strings.Repeat("é", 128), strings.Repeat("a", 129))
}

// randomPattern is a valid glob of up to five segments, of the long
// alphabet when long.
func randomPattern(rnd *rand.Rand, long bool) string {
	for {
		segs := make([]string, 1+rnd.IntN(5))
		for i := range segs {
			if rnd.IntN(4) == 0 {
				segs[i] = "**"
				continue
			}
			var b strings.Builder
			pieces := patternPieces(long)
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

// randomKey is the title key of a title of up to three pieces, of the long
// alphabet when long.
func randomKey(rnd *rand.Rand, long bool) string {
	var b strings.Builder
	pieces := titlePieces(long)
	for range 1 + rnd.IntN(3) {
		b.WriteString(pieces[rnd.IntN(len(pieces))])
	}
	return shared.TitleKey(b.String())
}

// The second oracle, a wildcard matcher of its own (M9/P1 S2 review A's):
// each segment's literal parts folded as the glob folds them, matched to
// a key rune by rune by the classic table of which pattern prefix matches
// which key prefix, and the segments to the keys by a memo of the same
// kind, ** taking any number of keys (one or more at the end).
type oracleToken struct {
	kind int // 0 a literal rune, 1 ?, 2 *
	r    rune
}

type oracleSegment struct {
	kind   int // 0 a segment's tokens, 1 **, 2 a trailing ** (one key or more)
	tokens []oracleToken
}

func oracleOf(pattern string) []oracleSegment {
	parts := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	out := make([]oracleSegment, 0, len(parts))
	for i, p := range parts {
		if p == "**" {
			kind := 1
			if i > 0 && i == len(parts)-1 {
				kind = 2
			}
			out = append(out, oracleSegment{kind: kind})
			continue
		}
		var tokens []oracleToken
		for {
			j := strings.IndexAny(p, "*?")
			if j < 0 {
				j = len(p)
			}
			for _, r := range shared.TitleKey(p[:j]) {
				tokens = append(tokens, oracleToken{0, r})
			}
			if j == len(p) {
				break
			}
			if p[j] == '*' {
				tokens = append(tokens, oracleToken{kind: 2})
			} else {
				tokens = append(tokens, oracleToken{kind: 1})
			}
			p = p[j+1:]
		}
		out = append(out, oracleSegment{tokens: tokens})
	}
	return out
}

// oracleSegmentMatches reports whether tokens match the key's runes s,
// prev and cur two rows of the table at least as long as s and its end.
func oracleSegmentMatches(tokens []oracleToken, s []rune, prev, cur []bool) bool {
	prev, cur = prev[:len(s)+1], cur[:len(s)+1]
	clear(prev)
	prev[0] = true // no token matches the empty prefix
	for _, tok := range tokens {
		cur[0] = tok.kind == 2 && prev[0]
		for j := 1; j <= len(s); j++ {
			switch tok.kind {
			case 2:
				cur[j] = prev[j] || cur[j-1]
			case 1:
				cur[j] = prev[j-1]
			default:
				cur[j] = prev[j-1] && s[j-1] == tok.r
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(s)]
}

// oracleMatches reports whether segments match the path of keys.
func oracleMatches(segments []oracleSegment, keys []string) bool {
	runes := make([][]rune, len(keys))
	longest := 0
	for i, k := range keys {
		runes[i] = []rune(k)
		longest = max(longest, len(runes[i]))
	}
	prev, cur := make([]bool, longest+1), make([]bool, longest+1)
	memo := map[[2]int]bool{}
	var match func(i, j int) bool
	match = func(i, j int) bool {
		if v, ok := memo[[2]int{i, j}]; ok {
			return v
		}
		var v bool
		switch {
		case i == len(segments):
			v = j == len(keys)
		case segments[i].kind == 1:
			v = match(i+1, j) || j < len(keys) && match(i, j+1)
		case segments[i].kind == 2:
			v = j < len(keys) && (match(i+1, j+1) || match(i, j+1))
		default:
			v = j < len(keys) && oracleSegmentMatches(segments[i].tokens, runes[j], prev, cur) && match(i+1, j+1)
		}
		memo[[2]int{i, j}] = v
		return v
	}
	return match(0, 0)
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

// The automata match what the regular expression does, and the second
// oracle, on random patterns and paths, of the short alphabet and of the
// long (M9/P1 design 3.10.1, 3.19).
func TestTheGlobMatchesAsItsRegexpDoes(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	differ, pairs, matched := 0, 0, 0
	for i := range 22000 {
		long := i >= 20000
		pattern := randomPattern(rnd, long)
		g, re, segments := mustGlob(t, pattern), regexpOf(pattern), oracleOf(pattern)
		for range 15 {
			keys := make([]string, 1+rnd.IntN(6))
			for i := range keys {
				keys[i] = randomKey(rnd, long)
			}
			pairs++
			want := re.MatchString("/" + strings.Join(keys, "/"))
			if oracle := oracleMatches(segments, keys); oracle != want {
				t.Fatalf("%q over %q: the regexp %v, the second oracle %v", pattern, keys, want, oracle)
			}
			if want {
				matched++
			}
			if got := matchPath(g, keys); got != want {
				if differ++; differ < 5 {
					t.Errorf("%q over %q: %v, the oracles %v", pattern, keys, got, want)
				}
			}
		}
	}
	if differ > 0 || matched < pairs/50 {
		t.Errorf("%d of %d pairs differ, %d match", differ, pairs, matched)
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
// distinct, no attachment's key ending with ".md", of the long alphabet
// when long.
func randomTree(rnd *rand.Rand, depth int, long bool) []*tree {
	seen := map[string]bool{}
	var out []*tree
	for range rnd.IntN(5) + 1 {
		key := randomKey(rnd, long)
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
			n.children = randomTree(rnd, depth-1, long)
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
// match of each path on its own, in the paths' joined byte order, of the
// short alphabet and of the long (M9/P1 design 3.10.1, 3.19).
func TestTheGlobWalkIsThePathsMatch(t *testing.T) {
	rnd := rand.New(rand.NewPCG(21, 23))
	for i := range 3300 {
		long, depth := i >= 3000, 1+rnd.IntN(11)
		if long {
			depth = 1 + rnd.IntN(4)
		}
		trees := randomTree(rnd, depth, long)
		var paths []string
		pathKeys(trees, "", &paths)
		slices.Sort(paths)
		roots := globNodes(trees)
		for range 10 {
			pattern := randomPattern(rnd, long)
			re, segments := regexpOf(pattern), oracleOf(pattern)
			var want []string
			for _, p := range paths {
				if re.MatchString("/" + p) {
					want = append(want, p)
				}
				if long && oracleMatches(segments, strings.Split(p, "/")) != re.MatchString("/"+p) {
					t.Fatalf("%q over %q: the oracles differ", pattern, p)
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
		trees := randomTree(rnd, 1+rnd.IntN(9), false)
		roots := globNodes(trees)
		pattern := randomPattern(rnd, false)
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
		trees := randomTree(rnd, 1+rnd.IntN(7), false)
		pattern := randomPattern(rnd, false)
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

// A glob without wildcards names one path, its keys, and lists it alone
// among siblings whose keys share all but its end, titles of 64 folded
// characters and more among them: a segment's states in two words of the
// masks and more (M9/P1 S2 review, A-I2).
func TestALiteralGlobNamesOnePath(t *testing.T) {
	if keys, ok := mustGlob(t, "/Guide/Straße").Literal(); !ok || !slices.Equal(keys, []string{"guide", "strasse"}) {
		t.Errorf("Literal() = %q, %v", keys, ok)
	}
	if _, ok := mustGlob(t, "Guide/*").Literal(); ok {
		t.Error("Guide/* is literal")
	}
	for _, title := range []string{strings.Repeat("a", 63), strings.Repeat("a", 64), strings.Repeat("a", 70), strings.Repeat("É", 64),
		strings.Repeat("a", 127) + "b", strings.Repeat("ab", 100) + "c", strings.Repeat("é", 127)} {
		key := shared.TitleKey(title)
		cut, _ := utf8.DecodeLastRuneInString(key)
		alike := []string{key, key[:len(key)-utf8.RuneLen(cut)], key + "a", key[:len(key)-utf8.RuneLen(cut)] + "z"}
		var under []*tree
		for _, k := range alike {
			under = append(under, &tree{key: k, page: true})
		}
		var roots []*tree
		for _, k := range alike {
			roots = append(roots, &tree{key: k, page: true, children: under})
		}
		for pattern, want := range map[string]string{title + ".md": key + ".md", title + "/" + title + ".md": key + "/" + key + ".md"} {
			if got, _ := walk(t, mustGlob(t, pattern), globNodes(roots), "", 1<<62); !slices.Equal(got, []string{want}) {
				t.Errorf("%.20q… of %d characters lists %q", pattern, utf8.RuneCountInString(title), got)
			}
		}
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
