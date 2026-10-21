package domain

import (
	"testing"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The threshold of migration 00046's first condition, BoundFreeContentBytes,
// is under the bound it stands for (M9/P4 design 3.2.2): a body's text is a
// name of MaxTitleBytes at most, "\n" and the content, and each of its bytes
// gives at most so many bytes of lexemes and prefixes. Over every code
// point, the CGJ among them: (3F + 7n) / r, F the bytes of its fold's
// characters of a run, word or CJK, n how many they are, r its own bytes (a
// character is in three trigrams or two bigrams at most, each a lexeme
// behind its SearchPrefixBytes); and Go's NFC writes a CGJ, a word
// character, after 30 non-starters, which costs a trigram and its prefix
// more, so the most non-starters a byte's fold holds counts too. Not timed:
// it runs under -short (about half a second; M9/P4 review C-N5).
func TestTheThresholdIsUnderTheBound(t *testing.T) {
	var worst, nonStarters float64
	var worstAt, nonStartersAt rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if utf16.IsSurrogate(r) {
			continue
		}
		s := string(r)
		var bytes, runes, marks int
		for _, x := range searchFold(s) {
			if classOf(x) != classOther {
				bytes += utf8.RuneLen(x)
				runes++
			}
			if !norm.NFC.PropertiesString(string(x)).BoundaryBefore() {
				marks++
			}
		}
		if q := float64(3*bytes+SearchPrefixBytes*runes) / float64(len(s)); q > worst {
			worst, worstAt = q, r
		}
		if q := float64(marks) / float64(len(s)); q > nonStarters {
			nonStarters, nonStartersAt = q, r
		}
	}
	const cgj = "\u034f"
	if classOf([]rune(cgj)[0]) != classWord {
		t.Fatalf("the CGJ is not a word character: its trigram is not counted")
	}
	perByte := worst + float64(3*len(cgj)+SearchPrefixBytes)*nonStarters/30
	text := BoundFreeContentBytes + shared.MaxTitleBytes + 1
	if got := float64(text) * perByte; got > MaxSearchLexemeBytes {
		t.Errorf("a body of %d bytes gives %.0f bytes of lexemes at most (%.4f a byte: %.4f at %U, %.4f non-starters a byte at %U), "+
			"over MaxSearchLexemeBytes, %d: BoundFreeContentBytes %d is past its proof", text, got, perByte, worst, worstAt,
			nonStarters, nonStartersAt, MaxSearchLexemeBytes, BoundFreeContentBytes)
	}
	t.Logf("%.4f bytes of lexemes a byte at most (%.4f at %U, %.4f non-starters a byte at %U); %d bytes of text give %.0f",
		perByte, worst, worstAt, nonStarters, nonStartersAt, text, float64(text)*perByte)
}
