package domain

import (
	"math/rand/v2"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// grepKeysSeed seeds TestGrepKeysAreASuperset; its expectations hold on
// any seed (seeds 1 to 10 and 100 checked when it was written).
const grepKeysSeed = 1

// TestGrepKeysAreASuperset is 3.5.4's claim (M9/P4 design 6.1), judged by
// Go's regexp and SearchTokens, not by the code under test: a page holding
// a line that a random pattern matches in NFC, when the page is not
// partial, holds each of the pattern's keys among its body's lexemes.
// Half the trials draw on a general alphabet, a quarter of their patterns
// without case; half on a directed one, of the runes 3.5.4's cases need,
// three quarters without case. The line sits between two lines, or first
// after a BOM, or between lines ended by CRLF, or last without a newline,
// a quarter each. Its floors count the trials where a mutant of 3.5.4
// could show (M9/P4 review C-I2), each apart: a tail (a starter whose fold
// joins the folds of the marks after it), an orbit (a literal under (?i)
// matched by another rune of its orbit, whose risk or NFC boundary is
// another), a fold (a piece's keys differ, folded atom by atom and whole);
// and the bigrams, trigrams and words among the keys, each at least 1% of
// the trials with keys. Partial pages, left out, are at most 1%.
func TestGrepKeysAreASuperset(t *testing.T) {
	t.Parallel()
	trials, tails, orbits, folds := 20_000, 150, 400, 1
	if testing.Short() {
		trials, tails, orbits, folds = 2_000, 10, 40, 0
	}
	rng := rand.New(rand.NewPCG(grepKeysSeed, 7))
	general, directed := []rune(grepGeneralRunes), []rune(grepDirectedRunes)
	var matched, keyed, partial int
	cases := map[string]int{}
	kinds := map[string]int{}
	for i := range trials {
		alphabet, without := general, 1
		if i%2 == 1 {
			alphabet, without = directed, 3
		}
		g := grepGen{rng: rng, alphabet: alphabet}
		pattern := g.pattern(1 + rng.IntN(4))
		if rng.IntN(4) < without {
			pattern = "(?i)" + pattern
		}
		pattern = norm.NFC.String(pattern)
		re, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			continue
		}
		var b strings.Builder
		b.WriteString(g.around())
		if !g.sample(re, &b) {
			continue
		}
		b.WriteString(g.around())
		line := b.String()
		if !utf8.ValidString(line) || strings.ContainsAny(line, "\n\r\x00\ufeff") {
			continue
		}
		if !regexp.MustCompile(pattern).MatchString(norm.NFC.String(line)) {
			continue
		}
		matched++
		keys := GrepKeys(re)
		if len(keys) == 0 {
			continue
		}
		keyed++
		var content string
		switch rng.IntN(4) {
		case 0:
			content = "x\n" + line + "\ny"
		case 1:
			content = "\ufeff" + line + "\ny"
		case 2:
			content = "x\r\n" + line + "\r\ny"
		default:
			content = "x\n" + line
		}
		_, body, cut := SearchTokens("n", &content)
		if cut {
			partial++
			continue
		}
		for name, in := range map[string]bool{"tail": grepTailCase(line), "orbit": grepOrbitCase(re, line), "fold": grepFoldCase(re)} {
			if in {
				cases[name]++
			}
		}
		for _, k := range keys {
			kinds[grepKeyKind(k)]++
			if !slices.Contains(body, k) {
				t.Errorf("trial %d: %q's key %q is not among the lexemes of a page holding %q (%U): keys %q", i, pattern, k, line,
					[]rune(line), keys)
			}
		}
	}
	t.Logf("%d trials, %d matched, %d with keys, %d partial; cases %v; keys %v", trials, matched, keyed, partial, cases, kinds)
	if partial*100 > keyed {
		t.Errorf("%d of %d pages with keys are partial, more than 1%%", partial, keyed)
	}
	for name, floor := range map[string]int{"tail": tails, "orbit": orbits, "fold": folds} {
		if cases[name] < floor {
			t.Errorf("%d trials of the case %s, want %d at least", cases[name], name, floor)
		}
	}
	for _, kind := range []string{"bigram", "trigram", "word"} {
		if kinds[kind]*100 < keyed {
			t.Errorf("%d %s keys in %d trials with keys, want 1%% at least", kinds[kind], kind, keyed)
		}
	}
}

// The alphabets of TestGrepKeysAreASuperset. The general: ASCII, marks,
// runes that fold to ASCII, Greek that folds and composes, CJK, Hangul and
// its jamo, Cherokee, titlecase, fullwidth, a soft hyphen. The directed:
// the runes whose fold is longer (the iota subscripts, ligatures, sharp
// s), the iotas' orbit, the marks their folds compose with, risky tails
// and their small letters, a few letters.
const (
	grepGeneralRunes = "abcAB1-. <" + "\u00e9e\u0301\u0323\u0338" +
		"\u00df\u1e9e\u017f\u212a\ufb03\u0130\u0131" +
		"\u0390\u03b1\u0345\u1fb4\u03a3\u03c2\u03c3" +
		"\u4e2d\u6587\u5b57\uac00\u1100\u1161\u11a8" +
		"\u13a0\uab70\u01c5\u01c6\uff21\u00ad"
	grepDirectedRunes = "\u1fbc\u1fb3\u1fb4\u1fc3\u1ffc\ufb06\ufb05\u00df\u1e9e\u0149\u01f0\u0390" +
		"\u0345\u0399\u03b9\u03b1\u03b7\u03c9\u0391\u0397\u038f\u0389" +
		"\u0342\u0301\u0300\u0313\u0314\u0308\u030a\u0307\u0331" +
		"\u017fHJTWY\u0130sjhtwy\u030c" +
		"ab\u1e22\uff21\uac00 "
)

// grepGen draws patterns and the lines they match from an alphabet.
type grepGen struct {
	rng      *rand.Rand
	alphabet []rune
}

func (g grepGen) pick() rune { return g.alphabet[g.rng.IntN(len(g.alphabet))] }

// around is up to two runes before or after a sample, two times in three.
func (g grepGen) around() string {
	if g.rng.IntN(3) == 0 {
		return ""
	}
	return string([]rune{g.pick(), g.pick()}[:g.rng.IntN(3)])
}

func (g grepGen) literal(n int) string {
	var b strings.Builder
	for range n {
		b.WriteString(regexp.QuoteMeta(string(g.pick())))
	}
	return b.String()
}

// pattern is a random pattern of depth at most d: concatenations,
// alternations, repeats, \b, \B, ^, $, groups, 13 classes, literals.
func (g grepGen) pattern(d int) string {
	classes := []string{`.`, `\s`, `\d`, `\w`, `\W`, `\pL`, `[ \-.]`, `[^ ]`, `[a-c]`, `[\x{4e00}-\x{9fff}]`, `[ab]`, `\PL`, `[[:punct:]]`}
	if d <= 0 {
		if g.rng.IntN(3) == 0 {
			return classes[g.rng.IntN(len(classes))]
		}
		return g.literal(1 + g.rng.IntN(4))
	}
	switch g.rng.IntN(10) {
	case 0, 1, 2:
		var b strings.Builder
		for range 2 + g.rng.IntN(3) {
			b.WriteString(g.pattern(d - 1))
		}
		return b.String()
	case 3:
		return "(?:" + g.pattern(d-1) + "|" + g.pattern(d-1) + ")"
	case 4:
		return "(?:" + g.pattern(d-1) + ")" + []string{"*", "+", "?", "{1,3}", "{2}", "{0,2}", "+?"}[g.rng.IntN(7)]
	case 5:
		return []string{`\b`, `\B`, `^`, `$`}[g.rng.IntN(4)] + g.pattern(d-1)
	case 6:
		return "(" + g.pattern(d-1) + ")"
	case 7:
		return g.pattern(d-1) + []string{`\b`, `$`}[g.rng.IntN(2)]
	default:
		return g.literal(1 + g.rng.IntN(5))
	}
}

// sample writes a text re may match, drawn at random; false when it gives
// up. A literal under (?i) takes another rune of its orbit half the time.
func (g grepGen) sample(re *syntax.Regexp, b *strings.Builder) bool {
	switch re.Op {
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			if re.Flags&syntax.FoldCase != 0 && g.rng.IntN(2) == 0 {
				orbit := []rune{r}
				for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
					orbit = append(orbit, f)
				}
				r = orbit[g.rng.IntN(len(orbit))]
			}
			b.WriteRune(r)
		}
	case syntax.OpCharClass:
		var in []rune
		for _, r := range g.alphabet {
			for i := 0; i+1 < len(re.Rune); i += 2 {
				if re.Rune[i] <= r && r <= re.Rune[i+1] {
					in = append(in, r)
					break
				}
			}
		}
		if len(in) > 0 {
			b.WriteRune(in[g.rng.IntN(len(in))])
			return true
		}
		if len(re.Rune) == 0 {
			return false
		}
		i := 2 * g.rng.IntN(len(re.Rune)/2)
		r := re.Rune[i] + rune(g.rng.IntN(int(min(re.Rune[i+1]-re.Rune[i], 50))+1))
		if r == '\n' {
			return false
		}
		b.WriteRune(r)
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		b.WriteRune(g.pick())
	case syntax.OpCapture:
		return g.sample(re.Sub[0], b)
	case syntax.OpConcat:
		for _, s := range re.Sub {
			if !g.sample(s, b) {
				return false
			}
		}
	case syntax.OpAlternate:
		return g.sample(re.Sub[g.rng.IntN(len(re.Sub))], b)
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		lo, hi := 0, 3
		switch re.Op {
		case syntax.OpPlus:
			lo = 1
		case syntax.OpQuest:
			hi = 1
		case syntax.OpRepeat:
			lo, hi = re.Min, re.Max
			if hi < 0 {
				hi = lo + 2
			}
		}
		for range lo + g.rng.IntN(hi-lo+1) {
			if !g.sample(re.Sub[0], b) {
				return false
			}
		}
	case syntax.OpNoMatch:
		return false
	}
	return true
}

func grepKeyKind(k string) string {
	r, _ := utf8.DecodeRuneInString(k)
	switch {
	case classOf(r) == classCJK:
		return "bigram"
	case utf8.RuneCountInString(k) >= 3:
		return "trigram"
	}
	return "word"
}

// grepTailCase: the line in NFC holds a starter followed by marks, and the
// starter's fold joins the marks' folds in NFC (mutants 137 and 139 show
// on such a line).
func grepTailCase(line string) bool {
	rs := []rune(norm.NFC.String(line))
	for i := range rs {
		if !boundaryBefore(rs[i]) {
			continue
		}
		j := i + 1
		for j < len(rs) && !boundaryBefore(rs[j]) {
			j++
		}
		if j > i+1 && searchFold(string(rs[i:j])) != searchFold(string(rs[i]))+searchFold(string(rs[i+1:j])) {
			return true
		}
	}
	return false
}

// grepOrbitCase: a rune r of a literal under (?i) is matched in the line
// by another rune x of its orbit whose risk is another: x is a risky tail
// and r not, or the other way, or their NFC boundaries differ (mutant 138
// shows on such a line).
func grepOrbitCase(re *syntax.Regexp, line string) bool {
	lits := map[rune]bool{}
	var walk func(re *syntax.Regexp)
	walk = func(re *syntax.Regexp) {
		if re.Op == syntax.OpLiteral && re.Flags&syntax.FoldCase != 0 {
			for _, r := range re.Rune {
				lits[r] = true
			}
		}
		for _, s := range re.Sub {
			walk(s)
		}
	}
	walk(re)
	for _, x := range norm.NFC.String(line) {
		for r := range lits {
			if x != r && slices.Contains(orbit(r|foldAtom), x) && (riskyTail(x) != riskyTail(r) || boundaryBefore(x) != boundaryBefore(r)) {
				return true
			}
		}
	}
	return false
}

// grepFoldCase: a piece of re's segments has other keys folded whole than
// atom by atom (mutant 140 shows on such a pattern).
func grepFoldCase(re *syntax.Regexp) bool {
	f := factsOf(re)
	segments := f.must
	if f.isExact {
		segments = [][]rune{f.exact}
	}
	for _, seg := range segments {
		for _, p := range pieces(seg) {
			t, ok := trimPiece(p)
			if !ok {
				continue
			}
			var raw strings.Builder
			for _, a := range t.atoms {
				raw.WriteRune(a &^ foldAtom)
			}
			if !slices.Equal(foldedKeys(foldAtoms(t.atoms), t.left, t.right), foldedKeys(searchFold(raw.String()), t.left, t.right)) {
				return true
			}
		}
	}
	return false
}
