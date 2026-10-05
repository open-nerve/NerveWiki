package domain_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// A link's context is its line, whole when it is short; of a long line, a
// piece with the link's start, cut on characters, an ellipsis where it is
// cut: from the line's start when the link ends in its first 240 bytes,
// else from 80 bytes before it, though no later than 240 bytes before the
// line's end (M6/P5 design 3).
func TestALinksContextIsItsLineOrAPieceOfItWithTheLink(t *testing.T) {
	han := func(n int) string { return strings.Repeat("中", n) }
	y := func(n int) string { return strings.Repeat("y", n) }
	tests := []struct {
		name    string
		content string
		link    string // the link's text: its target starts 2 bytes in, and is 1 byte long
		want    string
	}{
		{"a line between two", "a\nsee [[x]] here\nb", "[[x]]", "see [[x]] here"},
		{"the first line", "see [[x]]\nb", "[[x]]", "see [[x]]"},
		{"the last line", "a\nsee [[x]]", "[[x]]", "see [[x]]"},
		{"the only line", "[[x]]", "[[x]]", "[[x]]"},
		{"CRLF", "a\r\nsee [[x]]\r\nb", "[[x]]", "see [[x]]"},
		{"a lone CR", "a\rsee [[x]]\rb", "[[x]]", "see [[x]]"},
		{"a line of 240 bytes", y(235) + "[[x]]\nb", "[[x]]", y(235) + "[[x]]"},
		{"a link at a long line's start", "[[x]]" + y(300), "[[x]]", "[[x]]" + y(235) + "…"},
		{"a link ending at byte 240", y(237) + "[[x]]" + y(10), "[[x]]", y(237) + "[[x" + "…"},
		{"a link ending at byte 241", y(238) + "[[x]]" + y(10), "[[x]]", "…" + y(225) + "[[x]]" + y(10)},
		{"a link near a long line's end", y(300) + "[[x]]zz", "[[x]]", "…" + y(233) + "[[x]]zz"},
		{"a link in a long line's middle", y(300) + "[[x]]" + y(300), "[[x]]", "…" + y(78) + "[[x]]" + y(157) + "…"},
		// The piece's start would be in a character, and moves on to the next.
		{"a start in a character", han(200) + "a[[x]]" + han(200), "a[[x]]", "…" + han(25) + "a[[x]]" + han(53) + "…"},
		// The piece's end would be in a character, and moves back.
		{"an end in a character", "a" + han(200) + "[[x]]" + han(200), "[[x]]", "…" + han(26) + "[[x]]" + han(52) + "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at := strings.Index(tt.content, tt.link)
			start := at + strings.Index(tt.link, "[[") + 2
			if got := domain.Context(tt.content, start, start+1); got != tt.want {
				t.Errorf("got\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// Of random contents of short and long lines, ends of lines of the three
// kinds, and characters of one to four bytes, the context of a link at any
// character is its line when that is at most 240 bytes; otherwise a piece
// of the line of at most 240 bytes with the link's start, an ellipsis
// before it just when it does not start the line and after it just when it
// does not end it. It is valid UTF-8 and holds none of the content's bytes.
func TestAContextIsAPieceOfItsLineWithItsLink(t *testing.T) {
	pieces := []string{"a", "y", " ", "中", "é", "😀", "…", "\n", "\r", "\r\n", "[[x]]"}
	for seed := range uint64(20000) {
		r := rand.New(rand.NewPCG(seed, 5))
		var b strings.Builder
		for range r.IntN(700) {
			p := pieces[r.IntN(len(pieces))]
			if p == "\n" || p == "\r" || p == "\r\n" {
				if r.IntN(8) > 0 { // long lines are the point
					p = "y"
				}
			}
			b.WriteString(p)
		}
		content := b.String()
		if content == "" {
			continue
		}
		var starts []int
		for i := range content {
			if c := content[i]; c != '\n' && c != '\r' {
				starts = append(starts, i)
			}
		}
		if len(starts) == 0 {
			continue
		}
		start := starts[r.IntN(len(starts))]
		_, size := utf8.DecodeRuneInString(content[start:])
		got := domain.Context(content, start, start+size)
		if why := notAPiece(content, start, got); why != "" {
			t.Fatalf("seed %d: the context of %q at %d is %q: %s", seed, content, start, got, why)
		}
	}
}

// notAPiece is why got is no context of the link at start of content.
func notAPiece(content string, start int, got string) string {
	from := strings.LastIndexAny(content[:start], "\r\n") + 1
	to := len(content)
	if i := strings.IndexAny(content[start:], "\r\n"); i >= 0 {
		to = start + i
	}
	line, at := content[from:to], start-from
	switch {
	case !utf8.ValidString(got):
		return "not UTF-8"
	case len(got) > 0 && sharesMemory(content, got):
		return "it holds the content's bytes"
	case len(line) <= domain.MaxContext && got != line:
		return "not the whole of a short line"
	case len(line) <= domain.MaxContext:
		return ""
	}
	for _, before := range []bool{false, true} {
		for _, after := range []bool{false, true} {
			piece := got
			if before && !strings.HasPrefix(piece, "…") || after && !strings.HasSuffix(piece, "…") {
				continue
			}
			if before {
				piece = piece[len("…"):]
			}
			if after {
				piece = piece[:len(piece)-len("…")]
			}
			if len(piece) > domain.MaxContext {
				continue
			}
			for s := 0; s <= at; s++ {
				e := s + len(piece)
				if e > at && e <= len(line) && line[s:e] == piece && before == (s > 0) && after == (e < len(line)) {
					return ""
				}
			}
		}
	}
	return "no piece of its line of at most 240 bytes with the link, marked where cut"
}

// sharesMemory tells whether s's bytes are in content's memory.
func sharesMemory(content, s string) bool {
	c, p := uintptr(unsafe.Pointer(unsafe.StringData(content))), uintptr(unsafe.Pointer(unsafe.StringData(s)))
	return p >= c && p < c+uintptr(len(content))
}

// A page's contexts are its links', by start, one a line, the first link's
// on it; a range past the content, or of no bytes, is none.
func TestAPagesContextsAreOneALine(t *testing.T) {
	content := "a [[x]] b [[x]]\r\nc [[x]]\n\n[[x]]"
	var ranges []domain.Range
	for i := range len(content) {
		if strings.HasPrefix(content[i:], "[[") {
			ranges = append(ranges, domain.Range{Start: i + 2, End: i + 3})
		}
	}
	ranges = append(ranges, domain.Range{Start: len(content) - 1, End: len(content) + 1}, domain.Range{Start: 3, End: 3})
	got := domain.Contexts(content, ranges)
	if want := []string{"a [[x]] b [[x]]", "c [[x]]", "[[x]]"}; !slices.Equal(got, want) {
		t.Errorf("Contexts = %q, want %q", got, want)
	}
	if got := domain.Contexts(content, nil); got == nil || len(got) != 0 {
		t.Errorf("no ranges: %#v", got)
	}
}
