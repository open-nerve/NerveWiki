package domain

import (
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

//go:generate env NERVE_WRITE_GREP_TABLES=1 go test -run ^TestGrepTablesAreTheirDefinition$ .

// MaxGrepKeys bounds a pattern's keys: dropping the others leaves a
// superset (M9 design 4.13).
const MaxGrepKeys = 32

// maxGrepExact bounds the atoms of the exact text x{n} makes of an exact
// x: past it, x{n} tells x's facts alone.
const maxGrepExact = 1024

// A segment's atoms (M9/P4 design 3.5.2): a code point; one with foldAtom
// set, a literal under (?i), stands for each code point of its simple
// folding orbit (unicode.SimpleFold); edgeAtom, a boundary of the
// tokenizer's runs, a character of the class other or a line's start or
// end.
const (
	foldAtom rune = 1 << 24
	edgeAtom rune = -1
)

// grepFacts are what a node of a pattern's tree tells of each of its
// matches (M9/P4 design 3.5.2): exact, when isExact, the atoms each match
// is; pre and suf, atoms each match starts and ends with; must, segments
// each match holds.
type grepFacts struct {
	exact    []rune
	isExact  bool
	pre, suf []rune
	must     [][]rune
}

// exactFacts are the facts of a node each match of which is atoms.
func exactFacts(atoms []rune) grepFacts {
	return grepFacts{exact: atoms, isExact: true, pre: atoms, suf: atoms, must: [][]rune{atoms}}
}

// GrepKeys is the keys of re, a grep's pattern parsed in Go's NFC (Perl's
// flags, FoldCase with ignore_case): lexemes of the body index that every
// page holding a line re matches holds, unless the page is partial (M9/P4
// design 3.5); at most MaxGrepKeys of them, in the order of the segments
// they come from, each once; none for a pattern no key reads, as TODO|FIXME
// or \d{4}, whose grep walks every page of the notebook within its bounds.
// re is the tree before Simplify, which only expands repeats.
func GrepKeys(re *syntax.Regexp) []string {
	f := factsOf(re)
	segments := f.must
	if f.isExact {
		segments = [][]rune{f.exact}
	}
	var out []string
	seen := map[string]bool{}
	for _, seg := range segments {
		for _, k := range segmentKeys(seg) {
			if seen[k] {
				continue
			}
			if len(out) == MaxGrepKeys {
				return out
			}
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// factsOf is re's facts, by the kind of its node (M9/P4 design 3.5.2's
// table): those of RE2's semantics, each line matched alone.
func factsOf(re *syntax.Regexp) grepFacts {
	switch re.Op {
	case syntax.OpLiteral:
		atoms := slices.Clone(re.Rune)
		if re.Flags&syntax.FoldCase != 0 {
			for i := range atoms {
				atoms[i] |= foldAtom
			}
		}
		return exactFacts(atoms)
	case syntax.OpCharClass:
		if othersAlone(re.Rune) {
			return exactFacts([]rune{edgeAtom})
		}
	case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		// Zero wide: a line's start and end are a "\n" in the tokenizer's
		// text, the content's first line is after the name's "\n", and
		// "\r" and a BOM are of the class other.
		return exactFacts([]rune{edgeAtom})
	case syntax.OpWordBoundary, syntax.OpNoWordBoundary, syntax.OpEmptyMatch:
		// They take no character: the atoms on their two sides are next to
		// each other in the match. \b is no boundary of the tokenizer's
		// runs: \w is ASCII's, a run's are Unicode's.
		return exactFacts(nil)
	case syntax.OpCapture:
		return factsOf(re.Sub[0])
	case syntax.OpConcat:
		return concatFacts(re.Sub)
	case syntax.OpAlternate:
		return alternateFacts(re.Sub)
	case syntax.OpPlus:
		return onceAtLeast(factsOf(re.Sub[0]))
	case syntax.OpRepeat:
		if re.Min == 0 {
			break
		}
		x := factsOf(re.Sub[0])
		if x.isExact && re.Min == re.Max && re.Min*len(x.exact) <= maxGrepExact {
			atoms := make([]rune, 0, re.Min*len(x.exact))
			for range re.Min {
				atoms = append(atoms, x.exact...)
			}
			return exactFacts(atoms)
		}
		return onceAtLeast(x)
	}
	// OpStar, OpQuest, OpRepeat of at least 0, OpAnyChar, OpAnyCharNotNL,
	// OpNoMatch and a class not of others alone: they may match anything,
	// or nothing.
	return grepFacts{}
}

// onceAtLeast is the facts of x repeated once or more: the first starts
// with its pre, the last ends with its suf, each holds its must.
func onceAtLeast(x grepFacts) grepFacts {
	if x.isExact && len(x.exact) == 0 {
		return x
	}
	return grepFacts{pre: x.pre, suf: x.suf, must: x.must}
}

// concatFacts is the facts of subs matched one after another: the exact
// texts next to each other run together; the text before a child not
// exact runs on into its pre, a must (and the concat's pre, when it is the
// first such), its must are kept, and the text after it starts from its
// suf.
func concatFacts(subs []*syntax.Regexp) grepFacts {
	out := grepFacts{isExact: true}
	var cur []rune
	head := true // no child not exact yet: cur is the concat's start
	for _, s := range subs {
		x := factsOf(s)
		if x.isExact {
			cur = append(cur, x.exact...)
			if out.isExact {
				out.exact = append(out.exact, x.exact...)
			}
			continue
		}
		seg := append(slices.Clone(cur), x.pre...)
		if head {
			out.pre, head = seg, false
		}
		out.must = append(out.must, seg)
		out.must = append(out.must, x.must...)
		cur = slices.Clone(x.suf)
		out.isExact = false
	}
	if out.isExact {
		return exactFacts(out.exact)
	}
	out.suf = cur
	out.must = append(out.must, cur)
	return out
}

// alternateFacts is the facts of a match of one of subs: the exact text
// they all are, or their common start and end.
func alternateFacts(subs []*syntax.Regexp) grepFacts {
	all := make([]grepFacts, len(subs))
	for i, s := range subs {
		all[i] = factsOf(s)
	}
	pre, suf := all[0].pre, all[0].suf
	same := all[0].isExact
	for _, x := range all[1:] {
		pre = commonPrefix(pre, x.pre)
		suf = commonSuffix(suf, x.suf)
		same = same && x.isExact && slices.Equal(x.exact, all[0].exact)
	}
	if same {
		return all[0]
	}
	return grepFacts{pre: pre, suf: suf, must: [][]rune{pre, suf}}
}

func commonPrefix(a, b []rune) []rune {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return a[:n]
}

func commonSuffix(a, b []rune) []rune {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return a[len(a)-n:]
}

// othersAlone reports whether each code point of the class's ranges, pairs
// of a low and a high, is of the tokenizer's class other. A line in NFC
// holds none of the 14 others whose fold is of another class (U+2ADC and
// 13 musical symbols, which NFC takes apart), so what such a class matches
// folds to others too (M9/P4 design 3.5.3).
func othersAlone(ranges []rune) bool {
	cjk, word := searchClasses()
	for i := 0; i+1 < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		for r := lo; r <= min(hi, utf8.RuneSelf-1); r++ {
			if classOf(r) != classOther {
				return false
			}
		}
		if hi >= utf8.RuneSelf && (overlaps(cjk, max(lo, utf8.RuneSelf), hi) || overlaps(word, max(lo, utf8.RuneSelf), hi)) {
			return false
		}
	}
	return true
}

// overlaps reports whether the table t, of stride 1 (tableOf's), holds a
// code point from lo to hi.
func overlaps(t *unicode.RangeTable, lo, hi rune) bool {
	for _, g := range t.R16 {
		if rune(g.Lo) <= hi && rune(g.Hi) >= lo {
			return true
		}
	}
	for _, g := range t.R32 {
		if rune(g.Lo) <= hi && rune(g.Hi) >= lo {
			return true
		}
	}
	return false
}

// segmentKeys are the keys of a segment, its pieces' (M9/P4 design 3.5.3).
func segmentKeys(seg []rune) []string {
	var out []string
	for _, p := range pieces(seg) {
		out = append(out, pieceKeys(p)...)
	}
	return out
}

// piece is a segment's atoms between edges: left and right tell whether an
// edge is right before and after them.
type piece struct {
	atoms       []rune
	left, right bool
}

// pieces are seg's pieces, in its order.
func pieces(seg []rune) []piece {
	var out []piece
	start := 0
	for i := 0; i <= len(seg); i++ {
		if i < len(seg) && seg[i] != edgeAtom {
			continue
		}
		if i > start {
			out = append(out, piece{atoms: seg[start:i], left: start > 0, right: i < len(seg)})
		}
		start = i + 1
	}
	return out
}

// pieceKeys are the keys of a piece (M9/P4 design 3.5.3): its atoms
// trimmed, folded one by one and then in NFC, the text the piece is in the
// tokenizer's, and that text's lexemes.
func pieceKeys(p piece) []string {
	t, ok := trimPiece(p)
	if !ok {
		return nil
	}
	return foldedKeys(foldAtoms(t.atoms), t.left, t.right)
}

// trimPiece is p without what may join the unknown text around it (M9/P4
// design 3.5.3, steps 1 and 2); false when nothing is left.
func trimPiece(p piece) (piece, bool) {
	lo, hi := 0, len(p.atoms)
	// 1. From the first atom each code point of which has an NFC boundary
	// before it: one that has none joins the unknown text before it.
	for lo < hi && !startsSequence(p.atoms[lo]) {
		lo++
	}
	if lo > 0 {
		p.left = false
	}
	// 2. Not the last combining sequence, unless an edge follows it, when
	// it may join what follows it, folded.
	if !p.right {
		k := hi - 1
		for k >= lo && !startsSequence(p.atoms[k]) {
			k--
		}
		if k < lo {
			return piece{}, false
		}
		if riskyTailSequence(p.atoms[k:hi]) {
			hi = k
		}
	}
	p.atoms = p.atoms[lo:hi]
	return p, lo < hi
}

// foldAtoms is the atoms each folded alone, every code point of an orbit
// folding alike, then in NFC (M9/P4 design 3.5.3, step 3).
func foldAtoms(atoms []rune) string {
	var folded strings.Builder
	for _, a := range atoms {
		folded.WriteString(searchFold(string(a &^ foldAtom)))
	}
	return norm.NFC.String(folded.String())
}

// foldedKeys are the lexemes of f, a piece's text in the tokenizer's, by
// its rules (M9/P4 design 3.5.3, step 4): each CJK bigram; the trigrams of
// a run of three letters or more; a run of one or two where both its ends
// are known boundaries, an other or a CJK character of f, or the piece's
// edges, left and right.
func foldedKeys(f string, left, right bool) []string {
	type run struct {
		class      searchClass
		start, end int
		n          int // characters
	}
	var runs []run
	for i, r := range f {
		c := classOf(r)
		if n := len(runs); n > 0 && runs[n-1].class == c && c != classOther {
			runs[n-1].end = i + utf8.RuneLen(r)
			runs[n-1].n++
			continue
		}
		runs = append(runs, run{class: c, start: i, end: i + utf8.RuneLen(r), n: 1})
	}
	keys := lexemes{out: []string{}}
	for j, g := range runs {
		switch {
		case g.class == classCJK:
			prev := -1
			for i, r := range f[g.start:g.end] {
				if prev >= 0 {
					keys.add(f[g.start+prev : g.start+i+utf8.RuneLen(r)])
				}
				prev = i
			}
		case g.class == classWord && g.n >= 3:
			// A run past MaxSearchWordRunes of ASCII alone gives them too:
			// a page holding it is partial, a candidate by !cut anyway.
			keys.trigrams(f, g.start, g.end)
		case g.class == classWord && (j > 0 || left) && (j < len(runs)-1 || right):
			keys.add(f[g.start:g.end])
		}
	}
	return keys.out
}

// orbit is the code points the atom a stands for.
func orbit(a rune) []rune {
	r := a &^ foldAtom
	if a&foldAtom == 0 {
		return []rune{r}
	}
	out := []rune{r}
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		out = append(out, f)
	}
	return out
}

// boundaryBefore reports whether NFC never joins r to what precedes it.
func boundaryBefore(r rune) bool {
	return norm.NFC.PropertiesString(string(r)).BoundaryBefore()
}

// startsSequence reports whether each code point the atom a stands for
// has an NFC boundary before it: a combining sequence starts there.
func startsSequence(a rune) bool {
	for _, r := range orbit(a) {
		if !boundaryBefore(r) {
			return false
		}
	}
	return true
}

// riskyTailSequence reports whether a piece's last combining sequence s,
// its starter first, may join what follows it once folded, where in the
// line it does not (M9/P4 design 3.5.3, step 2): a code point of its
// starter's orbit is a risky tail, or folds to a text that holds a code
// point without an NFC boundary before it; or a code point of the orbit
// of an atom after it folds to another text (in NFC, U+0345 alone: it folds
// to ι, a starter that the next mark may join).
func riskyTailSequence(s []rune) bool {
	for _, r := range orbit(s[0]) {
		if riskyTail(r) {
			return true
		}
		for _, x := range searchFold(string(r)) {
			if !boundaryBefore(x) {
				return true
			}
		}
	}
	for _, a := range s[1:] {
		for _, r := range orbit(a) {
			if searchFold(string(r)) != string(r) {
				return true
			}
		}
	}
	return false
}
