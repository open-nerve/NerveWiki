package domain

import (
	"math/rand/v2"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

// perSegmentNFC is NewGrepNFC as S1 first built it, for a line not in NFC
// (M9/P4 review A-I1): each NFC segment normalized by its own call, and
// the segments' starts in the NFC and in the line kept, then Text's end and
// the line's byte it ends at.
func perSegmentNFC(line string, cut int) (text string, isCut bool, nfc, at []int) {
	var out []byte
	pos := 0
	for pos < len(line) {
		n := norm.NFC.NextBoundaryInString(line[pos:], true)
		before := len(out)
		out = norm.NFC.AppendString(out, line[pos:pos+n])
		if len(out) > cut {
			out, isCut = out[:before], true
			break
		}
		nfc, at = append(nfc, before), append(at, pos)
		pos += n
	}
	return string(out), isCut, append(nfc, len(out)), append(at, pos)
}

// perSegmentSpan is S1's first Span over perSegmentNFC's tables.
func perSegmentSpan(nfc, at []int, start, end int) (int, int) {
	k := 0
	for k+1 < len(nfc) && nfc[k+1] <= start {
		k++
	}
	if end <= start {
		return at[k], at[k]
	}
	j := k + 1
	for nfc[j] < end {
		j++
	}
	return at[k], at[j]
}

// TestGrepNFCIsTheSegmentsMapping: NewGrepNFC's one pass of norm.Iter and
// Span's second give the text, the cut and every match's bytes in the
// line that S1's first form, a call and two offsets a segment, gave, on
// lines of runes NFC composes, reorders and splits with a CGJ (runs of 20
// to 50 marks), cut anywhere.
func TestGrepNFCIsTheSegmentsMapping(t *testing.T) {
	t.Parallel()
	trials := 20_000
	if testing.Short() {
		trials = 2_000
	}
	rng := rand.New(rand.NewPCG(2026, 1022))
	runes := []rune("ae\U00000301\U00000323\U00000300\U000000e9\U00001100\U00001161\U000011a8\U0000ac00\U00004e2d \U0000212bA" +
		"\U0000030a\U0001d160\U00000345\U000003b9\U00000f73\U00000f75\U00000f81\U00000344\U0000034f\U00000958\U0000fb1d" +
		"\U000005b4\U00000e48\U00003099\U0000304b\U000000df\U0000fb03x")
	marks := []rune("\U00000301\U00000323\U00000300\U0000030a\U00000345\U00000344\U000005b4\U00000e48\U00003099")
	for trial := range trials {
		var b strings.Builder
		for range rng.IntN(120) {
			if rng.IntN(20) == 0 {
				for range 20 + rng.IntN(31) {
					b.WriteRune(marks[rng.IntN(len(marks))])
				}
			}
			b.WriteRune(runes[rng.IntN(len(runes))])
		}
		line := b.String()
		if norm.NFC.IsNormalString(line) {
			continue
		}
		whole := norm.NFC.String(line)
		cut := rng.IntN(len(whole) + 10)
		g := NewGrepNFC(line, cut)
		text, isCut, nfc, at := perSegmentNFC(line, cut)
		if g.Text != text || g.Cut != isCut {
			t.Fatalf("trial %d: NewGrepNFC(%+q, %d) = %+q (cut %v), per segment %+q (cut %v)", trial, line, cut, g.Text, g.Cut,
				text, isCut)
		}
		for range 20 {
			start := rng.IntN(len(g.Text) + 1)
			end := start + rng.IntN(len(g.Text)-start+1)
			if rng.IntN(5) == 0 {
				end = start
			}
			a, z := g.Span(start, end)
			wa, wz := perSegmentSpan(nfc, at, start, end)
			if a != wa || z != wz {
				t.Fatalf("trial %d: Span(%d, %d) of %+q (cut %d) = %d, %d; per segment %d, %d", trial, start, end, line, cut, a, z, wa, wz)
			}
		}
	}
}
