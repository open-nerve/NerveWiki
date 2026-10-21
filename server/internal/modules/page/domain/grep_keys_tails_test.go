package domain

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// TestGrepKeysOverTheRiskyTails tries every pattern "ab" X [M] (M9/P4
// design 3.5.4, review C's enumeration), with and without (?i), X a risky
// tail or one of a few Greek letters and iotas, M nothing, a mark of
// U+0300-U+036F, U+1DC0, U+1DC4, U+20D0 or U+0345, against each line
// "ab" X' [M] F that it matches in NFC, X' each rune of X's orbit under
// (?i), F such a mark, an iota or a few letters: each key is among the
// lexemes of a page holding the line, over 306,951 lines. It catches
// mutants 137 to 139 (6.3). About a second; not under -short.
func TestGrepKeysOverTheRiskyTails(t *testing.T) {
	if testing.Short() {
		t.Skip("the risky tails' enumeration takes about a second")
	}
	t.Parallel()
	var marks []rune
	for m := rune(0x300); m <= 0x36F; m++ {
		marks = append(marks, m)
	}
	marks = append(marks, 0x1DC0, 0x1DC4, 0x20D0)
	follow := append(slices.Clone(marks), []rune("ae\u03b9\u0399\u1fbe x\u03b1\u4e2d")...)
	var starters []rune
	for r := range unicode.MaxRune + 1 {
		if riskyTail(r) {
			starters = append(starters, r)
		}
	}
	starters = append(starters, []rune("ea\u03b1\u03b7\u03c9\u0391\u0397\u03a9\u03b9\u0399\u1fbe")...)
	seconds := append([]rune{0}, append(slices.Clone(marks), 0x345)...)
	lines := 0
	for _, fold := range []bool{false, true} {
		for _, x := range starters {
			for _, m := range seconds {
				literal := "ab" + string(x)
				if m != 0 {
					literal += string(m)
				}
				literal = norm.NFC.String(literal)
				pattern := regexp.QuoteMeta(literal)
				if fold {
					pattern = "(?i)" + pattern
				}
				re, err := syntax.Parse(pattern, syntax.Perl)
				if err != nil {
					t.Fatal(err)
				}
				keys := GrepKeys(re)
				if len(keys) == 0 {
					continue
				}
				compiled := regexp.MustCompile(pattern)
				// The literal, and under (?i) each with one rune swapped for
				// another of its orbit.
				variants := []string{literal}
				if fold {
					rs := []rune(literal)
					for i, r := range rs {
						for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
							v := slices.Clone(rs)
							v[i] = f
							variants = append(variants, string(v))
						}
					}
				}
				for _, v := range variants {
					for _, f := range follow {
						line := v + string(f)
						if !compiled.MatchString(norm.NFC.String(line)) {
							continue
						}
						lines++
						content := "x\n" + line + "\ny"
						_, body, _ := SearchTokens("n", &content)
						for _, k := range keys {
							if !slices.Contains(body, k) {
								t.Errorf("%q's key %q is not among the lexemes of a page holding %q (%U): keys %q", pattern, k, line,
									[]rune(line), keys)
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%d lines", lines)
	if lines < 300_000 {
		t.Errorf("%d lines enumerated, want 306,951 when it was written", lines)
	}
}
