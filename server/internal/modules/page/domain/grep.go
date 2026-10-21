package domain

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The bounds of a grep (M9 design 4.13, 4.14; M9/P4 design 3.6). The work
// ones were measured at full load, a burst of 9 greps and 9 other workers
// on 18 cores, in 4 KiB lines of the costliest instruction (M9/P4 S1, step
// 1): p99 23-25 ns a unit, 2 × 10⁸ within 5 s and a line's 0.75 × 10⁸
// within 2 s. Sustained, a laptop's cores slow to 34 ns a unit, which M12
// measures again on a server.
const (
	MaxGrepPatternBytes = 1024
	// MaxGrepInsts bounds a pattern's program as GrepInsts estimates it,
	// before it is compiled.
	MaxGrepInsts                   = 2000
	DefaultGrepLimit, MaxGrepLimit = 100, 500
	// MaxGrepLineText is the most bytes of a line an answer shows.
	MaxGrepLineText = 1000
	// MaxGrepRead is about the most bytes of contents a call reads, as
	// backlinks' (linking/domain/context.go), which page cannot import.
	MaxGrepRead = 32 << 20
	// GrepBatchBytes is about the most bytes of contents a statement reads.
	GrepBatchBytes = 4 << 20
	// MaxGrepLineWork bounds a line's work, MaxGrepWork a call's: a unit
	// is a byte of a line by an instruction of the program.
	MaxGrepLineWork int64 = 75_000_000
	MaxGrepWork     int64 = 200_000_000
	// GrepLineOverhead is a line's work beyond its bytes, by instruction:
	// the loop's and a match's fixed cost, about 1.4 bytes of the costliest
	// instruction at full load (2.3 at the 90th percentile).
	GrepLineOverhead = 2
	// GrepSoftMargin is how long before the call's deadline grep starts no
	// more lines; GrepUnitTime what a unit is thought to take before the
	// call has timed its own, and at least.
	GrepSoftMargin = 3 * time.Second
	GrepUnitTime   = 25 * time.Nanosecond
	// GrepSlotWait is how long a call waits for its slot before 503.
	GrepSlotWait = time.Second
)

// GrepPattern is a pattern grep took: its expression as it compiles, its
// program's instructions as estimated and its keys.
type GrepPattern struct {
	expr  string
	Insts int64
	Keys  []string
}

// ParseGrepPattern parses a grep's pattern (M9/P4 design 3.6.1): 422 on
// pattern when it is longer than MaxGrepPatternBytes (too_long), holds a
// newline (invalid_format: a line never does), does not parse
// (invalid_format, its syntax.ErrorCode alone: never the pattern's text),
// or is past Go's bounds (too_long: syntax.ErrLarge, ErrNestingDepth, a
// repeat over 1,000) or MaxGrepInsts as GrepInsts estimates it (too_long:
// too complex). It parses the pattern's NFC with regexp.Compile's flags,
// "(?i)" before it with ignoreCase, and compiles nothing.
func ParseGrepPattern(pattern string, ignoreCase bool) (GrepPattern, error) {
	switch {
	case len(pattern) > MaxGrepPatternBytes:
		return GrepPattern{}, invalidPattern(shared.FieldTooLong, fmt.Sprintf("must be at most %d bytes", MaxGrepPatternBytes))
	case strings.Contains(pattern, "\n"):
		return GrepPattern{}, invalidPattern(shared.FieldInvalidFormat, `must not hold a newline: grep matches each line alone (\n never matches)`)
	}
	expr := norm.NFC.String(pattern)
	if ignoreCase {
		expr = "(?i)" + expr
	}
	re, err := syntax.Parse(expr, syntax.Perl)
	if err != nil {
		var se *syntax.Error
		if !errors.As(err, &se) {
			return GrepPattern{}, invalidPattern(shared.FieldInvalidFormat, "does not parse")
		}
		if pastGoBounds(se) {
			return GrepPattern{}, invalidPattern(shared.FieldTooLong, "too complex: "+string(se.Code)+"; "+grepRepeatsCost)
		}
		return GrepPattern{}, invalidPattern(shared.FieldInvalidFormat, string(se.Code))
	}
	insts := GrepInsts(re)
	if insts > MaxGrepInsts {
		return GrepPattern{}, invalidPattern(shared.FieldTooLong, fmt.Sprintf("too complex: about %d instructions, at most %d; %s",
			insts, MaxGrepInsts, grepRepeatsCost))
	}
	return GrepPattern{expr: expr, Insts: insts, Keys: GrepKeys(re)}, nil
}

// grepRepeatsCost ends the message of a pattern too complex.
const grepRepeatsCost = "large repeats such as [a-z]{999} cost the most"

// pastGoBounds reports whether the parse failed at one of Go's bounds, not
// at the syntax: too large, too deep, or a repeat over 1,000 (one whose
// counts are in order: {2,1} is a syntax error).
func pastGoBounds(se *syntax.Error) bool {
	switch se.Code {
	case syntax.ErrLarge, syntax.ErrNestingDepth:
		return true
	case syntax.ErrInvalidRepeatSize:
		lo, hi, comma := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(se.Expr, "{"), "}"), ",")
		least, err := strconv.Atoi(lo)
		if err != nil {
			return false
		}
		if !comma || hi == "" {
			return true
		}
		most, err := strconv.Atoi(hi)
		return err == nil && least <= most
	}
	return false
}

// invalidPattern is 422 on pattern.
func invalidPattern(code, message string) error {
	return shared.Invalid(shared.FieldError{Field: "pattern", Code: code, Message: message})
}

// Compile is p's program, once the call holds its slot.
func (p GrepPattern) Compile() (*regexp.Regexp, error) {
	return regexp.Compile(p.expr)
}

// GrepInsts is the instructions of re's program as calcSize estimates them
// (regexp/syntax/parse.go:212 of Go 1.27.1, unexported, which the parser
// checks its maxSize with), and the program's start and match: never
// fewer than syntax.Compile makes of re simplified (M9/P4 design 3.6.1).
// A star counts 2 and the program needs 1 when its operand cannot be
// empty, so it may count up to about twice the program.
func GrepInsts(re *syntax.Regexp) int64 {
	return calcSize(re) + 2
}

// calcSize is regexp/syntax's: a literal its runes; a capture and a star 2
// and their operand; a plus and a quest 1 and theirs; a concatenation its
// parts; an alternation its branches and one between each two; a repeat
// as x{n,m} expands; at least 1.
func calcSize(re *syntax.Regexp) int64 {
	var size int64
	switch re.Op {
	case syntax.OpLiteral:
		size = int64(len(re.Rune))
	case syntax.OpCapture, syntax.OpStar:
		size = 2 + calcSize(re.Sub[0])
	case syntax.OpPlus, syntax.OpQuest:
		size = 1 + calcSize(re.Sub[0])
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			size += calcSize(sub)
		}
	case syntax.OpAlternate:
		for _, sub := range re.Sub {
			size += calcSize(sub)
		}
		if len(re.Sub) > 1 {
			size += int64(len(re.Sub)) - 1
		}
	case syntax.OpRepeat:
		sub := calcSize(re.Sub[0])
		if re.Max == -1 {
			if re.Min == 0 {
				size = 2 + sub // x*
			} else {
				size = 1 + int64(re.Min)*sub // xxx+
			}
			break
		}
		size = int64(re.Max)*sub + int64(re.Max-re.Min) // x{2,5} = xx(x(x(x)?)?)?
	}
	return max(1, size)
}

// CheckGrepLimit is the hits limit asks for: DefaultGrepLimit when it is
// nil; 422 out_of_range on limit outside 1–MaxGrepLimit.
func CheckGrepLimit(limit *int) (int, error) {
	switch {
	case limit == nil:
		return DefaultGrepLimit, nil
	case *limit < 1 || *limit > MaxGrepLimit:
		return 0, shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: fmt.Sprintf("must be between 1 and %d", MaxGrepLimit)})
	}
	return *limit, nil
}

// GrepCursor is where a grep goes on (M9/P4 design 3.7.1): from line N of
// the page at the path key K when N ≥ 1, after K when N is 0.
type GrepCursor struct {
	K string `json:"k"`
	N int    `json:"n"`
}

// DecodeGrepCursor is the cursor s: 400 bad_request (shared.InvalidCursor)
// when it does not decode or its line is negative.
func DecodeGrepCursor(s string) (GrepCursor, error) {
	var c GrepCursor
	if err := shared.DecodeCursor(s, &c); err != nil {
		return GrepCursor{}, err
	}
	if c.N < 0 {
		return GrepCursor{}, shared.InvalidCursor()
	}
	return c, nil
}

// Encode is c as a cursor.
func (c GrepCursor) Encode() (string, error) {
	return shared.EncodeCursor(c)
}

// GrepWork is a line's work: its bytes, and GrepLineOverhead, by the
// program's instructions.
func GrepWork(lineBytes int, insts int64) int64 {
	return (int64(lineBytes) + GrepLineOverhead) * insts
}

// GrepLineCut is the most bytes of a line grep matches within
// maxLineWork: a longer line is matched in its first so many bytes, cut
// at a character's boundary, and listed in partial_lines unless a match
// ends before the cut (M9 design 4.13).
func GrepLineCut(insts, maxLineWork int64) int {
	return int(max(0, maxLineWork/max(1, insts)-GrepLineOverhead))
}

// GrepLines yields the lines of content from line from on, numbered as
// read numbers them (M9 design 4.9): parted by "\n", the last "\n" ending
// the last line, an empty content no line; each without its trailing
// "\r", the first without its byte order mark, column the byte of the
// line it starts at (3 after the mark, else 0). It stops when yield does.
func GrepLines(content string, from int, yield func(line int, text string, column int) bool) {
	for n, start := 1, 0; start < len(content); n++ {
		end := len(content)
		next := end
		if i := strings.IndexByte(content[start:], '\n'); i >= 0 {
			end = start + i
			next = end + 1
		}
		if n >= from {
			text, column := strings.TrimSuffix(content[start:end], "\r"), 0
			if n == 1 && strings.HasPrefix(text, "\ufeff") {
				text, column = text[len("\ufeff"):], len("\ufeff")
			}
			if !yield(n, text, column) {
				return
			}
		}
		start = next
	}
}

// GrepHit is a line's hit as an answer shows it: the line's text, cut to
// MaxGrepLineText bytes centred on its first match when longer (Cut), and
// the byte of the line it starts at, as read's column (M9/P4 design
// 3.7.4).
type GrepHit struct {
	Text   string
	Column int
	Cut    bool
}

// GrepHitOf is the hit of the line text (GrepLines'), at column, whose
// first match is text[start:end]: the line whole, or MaxGrepLineText
// bytes of it about the match's middle, each end moved in to a
// character's boundary.
func GrepHitOf(text string, column, start, end int) GrepHit {
	if len(text) <= MaxGrepLineText {
		return GrepHit{Text: text, Column: column}
	}
	lo := min(max(0, (start+end)/2-MaxGrepLineText/2), len(text)-MaxGrepLineText)
	hi := lo + MaxGrepLineText
	for lo < hi && !utf8.RuneStart(text[lo]) {
		lo++
	}
	for hi > lo && hi < len(text) && !utf8.RuneStart(text[hi]) {
		hi--
	}
	return GrepHit{Text: text[lo:hi], Column: column + lo, Cut: true}
}

// GrepMatch is a line's match within the cut bytes of its NFC (M9/P4
// design 3.6.2, 3.7.4): Hit when re's leftmost match ends before the cut,
// anywhere in a line not cut, the hit's offsets back in the line's bytes;
// Partial when the line is cut and no match ends before the cut, a line of
// partial_lines; Bytes, the NFC's matched, which the line's work counts.
type GrepMatch struct {
	Hit     *GrepHit
	Partial bool
	Bytes   int
}

// MatchGrepLine is re's match of the line text (GrepLines'), at column,
// within cut bytes of its NFC. A match ending at the cut is no hit: a $ or
// \b there may not hold in the whole line.
func MatchGrepLine(re *regexp.Regexp, text string, column, cut int) GrepMatch {
	g := NewGrepNFC(text, cut)
	m := GrepMatch{Bytes: len(g.Text)}
	loc := re.FindStringIndex(g.Text)
	switch {
	case loc != nil && (!g.Cut || loc[1] < len(g.Text)):
		start, end := g.Span(loc[0], loc[1])
		hit := GrepHitOf(text, column, start, end)
		m.Hit = &hit
	case g.Cut:
		m.Partial = true
	}
	return m
}

// GrepNFC is a line as grep matches it (M9/P4 design 3.6.2, 3.7.4): Text,
// its NFC, or of its start alone when that is longer than the cut (Cut),
// up to the last NFC segment within it. A line in NFC already is Text
// itself, its offsets the same; Span takes a match in another back to the
// line's bytes.
type GrepNFC struct {
	Text string
	Cut  bool
	// line is the line when it is not in NFC, which Span normalizes again
	// up to a match's end; "" for a line in NFC.
	line string
}

// NewGrepNFC is line as grep matches it within cut bytes: normalized up to
// the cut in one pass of norm.Iter, a segment at a time, never the whole
// of a long line. It keeps nothing a segment: a call and two offsets a
// segment cost a long line in ASCII 100 bytes of memory a byte and 600 of
// allocation (review A-I1); Span finds a hit's offsets again.
func NewGrepNFC(line string, cut int) GrepNFC {
	if norm.NFC.IsNormalString(line) {
		if len(line) <= cut {
			return GrepNFC{Text: line}
		}
		end := 0
		for end < len(line) {
			n := norm.NFC.NextBoundaryInString(line[end:], true)
			if end+n > cut {
				break
			}
			end += n
		}
		return GrepNFC{Text: line[:end], Cut: true}
	}
	g := GrepNFC{line: line}
	var b strings.Builder
	b.Grow(max(0, min(len(line), cut)))
	var it norm.Iter
	it.InitString(norm.NFC, line)
	for !it.Done() {
		seg := it.Next()
		if b.Len()+len(seg) > cut {
			g.Cut = true
			break
		}
		b.Write(seg)
	}
	g.Text = b.String()
	return g
}

// Span is the bytes of the line Text[start:end] comes from: start back to
// its NFC segment's start, end on to its segment's end; an empty span at a
// segment's start. A line not in NFC is normalized again, in the same
// segments, up to the segment holding end.
func (g GrepNFC) Span(start, end int) (int, int) {
	if g.line == "" {
		return start, end
	}
	var it norm.Iter
	it.InitString(norm.NFC, g.line)
	// out and at are the segment's start in Text and in the line; lo, the
	// line's byte start's segment starts at, once found.
	out, at, lo := 0, 0, -1
	for out < len(g.Text) && !it.Done() {
		next := out + len(it.Next())
		if lo < 0 && start < next {
			if lo = at; end <= start {
				return lo, lo
			}
		}
		if lo >= 0 && end <= next {
			return lo, it.Pos()
		}
		out, at = next, it.Pos()
	}
	if lo < 0 {
		lo = at
	}
	return lo, at
}
