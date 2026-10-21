package domain_test

import (
	"errors"
	"math/rand/v2"
	"regexp"
	"regexp/syntax"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fieldCode is the field code of err's one field, "" for none.
func fieldCode(t *testing.T, err error, field string) (string, string) {
	t.Helper()
	if err == nil {
		return "", ""
	}
	var se *shared.Error
	if !errors.As(err, &se) || len(se.Fields) != 1 || se.Fields[0].Field != field {
		t.Fatalf("err = %v, want a field error on %s", err, field)
	}
	return se.Fields[0].Code, se.Fields[0].Message
}

// TestParseGrepPattern checks ParseGrepPattern's refusals and estimates
// (M9/P4 design 3.6.1), review A's worst parses among them, each of which
// allocates 8 MB at most: TotalAlloc is the process's, so neither the test
// nor its rows run in parallel (review A2-n1).
func TestParseGrepPattern(t *testing.T) {
	for _, row := range []struct {
		name, pattern string
		ignoreCase    bool
		code          string // the field code on pattern, "" for none
		insts         int64
		keys          []string
	}{
		{"1,024 bytes", strings.Repeat("a", 1024), false, "", 1026, []string{"aaa"}},
		{"1,025 bytes", strings.Repeat("a", 1025), false, shared.FieldTooLong, 0, nil},
		{"a newline", "a\nb", false, shared.FieldInvalidFormat, 0, nil},
		{"the escape of a newline", `a\nb`, false, "", 5, nil},
		{"NUL", "a\x00b", false, "", 5, nil},
		{"a syntax error", `secret(word`, false, shared.FieldInvalidFormat, 0, nil},
		{"a repeat over 1,000", `x{1001}`, false, shared.FieldTooLong, 0, nil},
		{"repeats over 1,000 nested", `(?:x{1000}){2}`, false, shared.FieldTooLong, 0, nil},
		{"a repeat's counts out of order", `x{2,1}`, false, shared.FieldInvalidFormat, 0, nil},
		{"2,000 instructions", `(?:ab){999}`, false, "", 2000, nil},
		{"2,001 instructions", `(?:ab){999}c`, false, shared.FieldTooLong, 0, nil},
		{"without case", `FOO`, true, "", 5, []string{"foo"}},
		{"in NFC", "e\u0301t\u00e9", false, "", 5, []string{"\u00e9t\u00e9"}},
		// Review A's worst parses.
		{`\pL 341 times`, strings.Repeat(`\pL`, 341), false, "", 343, nil},
		{`\PL 341 times`, strings.Repeat(`\PL`, 341), false, "", 343, nil},
		{`(?i)\pL 340 times`, "(?i)" + strings.Repeat(`\pL`, 340), false, "", 342, nil},
		{`^\pL? 255 times`, "^" + strings.Repeat(`\pL?`, 255), false, "", 513, nil},
		{`[\pL0] 170 times`, strings.Repeat(`[\pL0]`, 170), false, "", 172, nil},
		{`(?i:[\x{100}-\x{10ffff}]) 40 times`, strings.Repeat(`(?i:[\x{100}-\x{10ffff}])`, 40), false, "", 42, nil},
		{`\pL{999} 113 times`, strings.Repeat(`\pL{999}`, 113), false, shared.FieldTooLong, 0, nil},
	} {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		p, err := domain.ParseGrepPattern(row.pattern, row.ignoreCase)
		runtime.ReadMemStats(&after)
		code, message := fieldCode(t, err, "pattern")
		switch {
		case code != row.code:
			t.Errorf("%s: code %q (%v), want %q", row.name, code, err, row.code)
		case code == "" && (p.Insts != row.insts || !slices.Equal(p.Keys, row.keys)):
			t.Errorf("%s: insts %d, keys %q, want %d, %q", row.name, p.Insts, p.Keys, row.insts, row.keys)
		case code != "" && len(row.pattern) <= domain.MaxGrepPatternBytes && strings.Contains(message, row.pattern):
			t.Errorf("%s: the message %q holds the pattern", row.name, message)
		}
		alloc := after.TotalAlloc - before.TotalAlloc
		if alloc > 8<<20 {
			t.Errorf("%s: the parse allocated %d bytes, more than 8 MB", row.name, alloc)
		}
		if alloc > 1<<20 {
			t.Logf("%s: %.2f MB allocated", row.name, float64(alloc)/(1<<20))
		}
	}
	// The messages: a syntax error's code alone; the estimate's.
	_, err := domain.ParseGrepPattern(`secret(word`, false)
	if _, message := fieldCode(t, err, "pattern"); message != "missing closing )" {
		t.Errorf("a syntax error's message %q, want %q", message, "missing closing )")
	}
	_, err = domain.ParseGrepPattern(`(?:ab){999}c`, false)
	if _, message := fieldCode(t, err, "pattern"); message != "too complex: about 2001 instructions, at most 2000; "+
		"large repeats such as [a-z]{999} cost the most" {
		t.Errorf("the estimate's message %q", message)
	}
	// A pattern taken compiles.
	p, err := domain.ParseGrepPattern(`(?:ab){999}`, true)
	if err != nil {
		t.Fatal(err)
	}
	if re, err := p.Compile(); err != nil || !re.MatchString(strings.Repeat("AB", 999)) {
		t.Errorf("Compile() = %v, %v", re, err)
	}
}

// TestGrepInstsIsNoLessThanTheProgram checks GrepInsts against the program
// syntax.Compile makes of the pattern simplified (M9/P4 design 3.6.1,
// review A-m6 = C-M1): never fewer, at most twice and 2, on 3.6.1's edge
// patterns and 20,000 random ones (2,000 under -short), exp/insts2's. The
// upper bound only catches a gross overcount: calcSize counts each of
// nested stars that Simplify folds into one, so lazy ones nested under a
// repeat, which that generator does not draw, reach 2.67 times.
func TestGrepInstsIsNoLessThanTheProgram(t *testing.T) {
	t.Parallel()
	check := func(pattern string) {
		re, err := syntax.Parse(pattern, syntax.Perl)
		if err != nil {
			return
		}
		prog, err := syntax.Compile(re.Simplify())
		if err != nil {
			t.Fatalf("%q: %v", pattern, err)
		}
		if n, got := int64(len(prog.Inst)), domain.GrepInsts(re); got < n || got > 2*n+2 {
			t.Errorf("GrepInsts(%q) = %d, the program %d", pattern, got, n)
		}
	}
	for _, p := range []string{`a*`, `a*b*c*d*`, `(a)`, `(?:a|b|c)`, `[ab]`, `a|b`, `abc|abd`, `x{0}`, `x{0,0}`, `(?:)`, `^$`,
		`\bfoo\b`, `TODO|FIXME`, `.*foo.*`, `a+`, `(?:ab)*`, `(?i)foo`, `[a-z]{3,}`, `(x*)*`, `a{2,5}`, `(?:a*){1000}`,
		`(?:a*b*){50}`, `(?:a?){1000}`, `(?:(?:a*){30}){30}`, `[a-z]{999}`, `\pL{999}`} {
		check(p)
	}
	trials := 20_000
	if testing.Short() {
		trials = 2_000
	}
	rng := rand.New(rand.NewPCG(3, 4))
	atoms := []string{"a", "b", "[a-z]", `\pL`, ".", "(?:ab|cd)", `\d`, "x"}
	reps := []string{"", "*", "+", "?", "{2}", "{1,5}", "{3,}", "{0,4}"}
	var gen func(d int) string
	gen = func(d int) string {
		if d == 0 {
			return atoms[rng.IntN(len(atoms))] + reps[rng.IntN(len(reps))]
		}
		var b strings.Builder
		for range 1 + rng.IntN(3) {
			b.WriteString("(?:" + gen(d-1) + ")" + reps[rng.IntN(len(reps))])
		}
		if rng.IntN(4) == 0 {
			return b.String() + "|" + gen(d-1)
		}
		return b.String()
	}
	for range trials {
		check(gen(rng.IntN(4)))
	}
}

func TestCheckGrepLimit(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		limit *int
		want  int
		code  string
	}{
		{nil, domain.DefaultGrepLimit, ""},
		{ptr(1), 1, ""},
		{ptr(domain.MaxGrepLimit), domain.MaxGrepLimit, ""},
		{ptr(0), 0, shared.FieldOutOfRange},
		{ptr(domain.MaxGrepLimit + 1), 0, shared.FieldOutOfRange},
	} {
		got, err := domain.CheckGrepLimit(row.limit)
		if code, _ := fieldCode(t, err, "limit"); code != row.code || got != row.want {
			t.Errorf("CheckGrepLimit(%v) = %d, %q, want %d, %q", row.limit, got, code, row.want, row.code)
		}
	}
}

// TestGrepCursor checks the cursor's round trip and its refusals: a
// negative line, and what does not decode, are 400 bad_request.
func TestGrepCursor(t *testing.T) {
	t.Parallel()
	for _, c := range []domain.GrepCursor{{K: "a\x00b", N: 0}, {K: "", N: 7}} {
		s, err := c.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if got, err := domain.DecodeGrepCursor(s); err != nil || got != c {
			t.Errorf("DecodeGrepCursor(%q) = %+v, %v, want %+v", s, got, err, c)
		}
	}
	negative, err := shared.EncodeCursor(domain.GrepCursor{K: "a", N: -1})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{negative, "not a cursor", ""} {
		var se *shared.Error
		if _, err := domain.DecodeGrepCursor(s); !errors.As(err, &se) || se.Code != shared.CodeBadRequest {
			t.Errorf("DecodeGrepCursor(%q) = %v, want bad_request", s, err)
		}
	}
}

// TestGrepLines checks the lines' numbers and texts against read's
// counting (M9/P4 design 3.7.4, review C-M12).
func TestGrepLines(t *testing.T) {
	t.Parallel()
	type line struct {
		n      int
		text   string
		column int
	}
	for _, row := range []struct {
		name, content string
		from          int
		want          []line
	}{
		{"CRLF", "a\r\nb\r\n", 1, []line{{1, "a", 0}, {2, "b", 0}}},
		{"a lone CR stays", "a\rb\n", 1, []line{{1, "a\rb", 0}}},
		{"one CR off the end", "a\r\r\n", 1, []line{{1, "a\r", 0}}},
		{"a BOM on line 1 alone", "\ufeffa\n\ufeffb", 1, []line{{1, "a", 3}, {2, "\ufeffb", 0}}},
		{"a BOM alone", "\ufeff", 1, []line{{1, "", 3}}},
		{"an empty content", "", 1, nil},
		{"the last newline ends the last line", "a\n", 1, []line{{1, "a", 0}}},
		{"an empty last line", "a\n\n", 1, []line{{1, "a", 0}, {2, "", 0}}},
		{"no last newline", "a\nb", 1, []line{{1, "a", 0}, {2, "b", 0}}},
		{"from line 2", "\ufeffa\nb\r\nc", 2, []line{{2, "b", 0}, {3, "c", 0}}},
		{"from past the end", "a\nb\n", 3, nil},
	} {
		var got []line
		domain.GrepLines(row.content, row.from, func(n int, text string, column int) bool {
			got = append(got, line{n, text, column})
			return true
		})
		if !slices.Equal(got, row.want) {
			t.Errorf("%s: %q from %d gives %+v, want %+v", row.name, row.content, row.from, got, row.want)
		}
	}
	calls := 0
	domain.GrepLines("a\nb\nc", 1, func(int, string, int) bool { calls++; return false })
	if calls != 1 {
		t.Errorf("GrepLines went on %d times after yield said stop", calls)
	}
}

// TestGrepHitOf checks a hit's text and column: the line whole up to
// MaxGrepLineText bytes, else that many about the match's middle, in at
// characters' boundaries; and, through MatchGrepLine, a match in a line
// not in NFC mapped back to the line's own bytes (mutant 162).
func TestGrepHitOf(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 1500) + "MATCH" + strings.Repeat("y", 1500)
	wide := strings.Repeat("\u00e9", 1500) // 2 bytes each
	for _, row := range []struct {
		name               string
		text               string
		column, start, end int
		want               domain.GrepHit
	}{
		{"a short line", "a needle", 2, 2, 8, domain.GrepHit{Text: "a needle", Column: 2}},
		{"1,000 bytes", strings.Repeat("a", 1000), 0, 0, 1, domain.GrepHit{Text: strings.Repeat("a", 1000)}},
		{"about the middle", long, 0, 1500, 1505, domain.GrepHit{Text: long[1002:2002], Column: 1002, Cut: true}},
		{"at the start", long, 3, 0, 1, domain.GrepHit{Text: long[:1000], Column: 3, Cut: true}},
		{"at the end", long, 0, 3004, 3005, domain.GrepHit{Text: long[2005:], Column: 2005, Cut: true}},
		{"in at characters' boundaries", wide, 0, 1000, 1002, domain.GrepHit{Text: wide[502:1500], Column: 502, Cut: true}},
	} {
		if got := domain.GrepHitOf(row.text, row.column, row.start, row.end); got != row.want {
			t.Errorf("%s: GrepHitOf = %q at %d (cut %v), want %q at %d", row.name, short(got.Text), got.Column, got.Cut,
				short(row.want.Text), row.want.Column)
		}
	}
	// A line not in NFC: 600 e and U+0301 (3 bytes each, 2 in NFC), then
	// the match. In NFC it is 600 bytes earlier than in the line.
	decomposed := strings.Repeat("e\u0301", 600) + "needle" + strings.Repeat("z", 600)
	m := domain.MatchGrepLine(regexp.MustCompile(`needle`), decomposed, 3, 1<<20)
	if m.Hit == nil || !strings.Contains(m.Hit.Text, "needle") || !strings.HasPrefix(decomposed[m.Hit.Column-3:], m.Hit.Text) {
		t.Errorf("the hit of a line not in NFC: %+v, want the line's own bytes about the needle", m.Hit)
	}
	// Spans: a match inside a composed segment covers the segment's bytes.
	g := domain.NewGrepNFC("ae\u0301b", 100)
	if g.Text != "a\u00e9b" {
		t.Fatalf("NFC %q", g.Text)
	}
	for _, s := range []struct{ start, end, wantStart, wantEnd int }{
		{1, 3, 1, 4}, // \u00e9: e and U+0301
		{0, 1, 0, 1},
		{3, 4, 4, 5},
		{4, 4, 5, 5}, // an empty match at the end
		{1, 1, 1, 1},
	} {
		if a, b := g.Span(s.start, s.end); a != s.wantStart || b != s.wantEnd {
			t.Errorf("Span(%d, %d) = %d, %d, want %d, %d", s.start, s.end, a, b, s.wantStart, s.wantEnd)
		}
	}
}

func short(s string) string {
	if len(s) > 20 {
		return s[:10] + "…" + s[len(s)-10:]
	}
	return s
}

// TestGrepLineCut checks the cut: the most bytes whose work is within the
// line's bound.
func TestGrepLineCut(t *testing.T) {
	t.Parallel()
	if got := domain.GrepLineCut(1999, domain.MaxGrepLineWork); got != 37_516 {
		t.Errorf("GrepLineCut(1999) = %d, want 37,516", got)
	}
	if got := domain.GrepLineCut(10, domain.MaxGrepLineWork); got != 7_499_998 {
		t.Errorf("GrepLineCut(10) = %d, want 7,499,998", got)
	}
	if got := domain.GrepLineCut(2000, 3000); got != 0 {
		t.Errorf("GrepLineCut(2000, 3000) = %d, want 0", got)
	}
	for _, insts := range []int64{3, 5, 22, 343, 1999, 2000} {
		for _, bound := range []int64{1e4, 1e6, 75e6} {
			cut := domain.GrepLineCut(insts, bound)
			if domain.GrepWork(cut, insts) > bound || cut > 0 && domain.GrepWork(cut+1, insts) <= bound {
				t.Errorf("GrepLineCut(%d, %d) = %d: work %d, and a byte more %d", insts, bound, cut, domain.GrepWork(cut, insts),
					domain.GrepWork(cut+1, insts))
			}
		}
	}
}

// TestGrepCutLines checks a cut line's match (M9/P4 design 3.6.2): a match
// ending at the cut is no hit and the line is partial (foo$ there: mutant
// 158), one ending before it is a hit; and only the start the cut reaches
// is normalized, a segment at a time, as the whole line would be.
func TestGrepCutLines(t *testing.T) {
	t.Parallel()
	line := strings.Repeat("a", 97) + "foo" + strings.Repeat("b", 50)
	for _, row := range []struct {
		name, pattern string
		cut           int
		hit, partial  bool
	}{
		{"foo$ at the cut", `foo$`, 100, false, true},
		{"foo at the cut", `foo`, 100, false, true},
		{"foo before the cut", `foo`, 101, true, true},
		{"foo$ in the whole line", `foo$`, 1000, false, false},
		{"nothing in a cut line", `zzz`, 100, false, true},
		{"nothing in a whole line", `zzz`, 1000, false, false},
		{"a hit in a whole line", `ob*$`, 1000, true, false},
	} {
		m := domain.MatchGrepLine(regexp.MustCompile(row.pattern), line, 0, row.cut)
		if (m.Hit != nil) != row.hit || m.Partial != (row.partial && m.Hit == nil) || m.Bytes != min(len(line), row.cut) {
			t.Errorf("%s: hit %v, partial %v, bytes %d", row.name, m.Hit, m.Partial, m.Bytes)
		}
	}
	// Random lines of runes NFC changes, joins and reorders, cut anywhere:
	// the text matched is the start of the line's NFC up to a boundary,
	// within the cut, and all of it when it fits.
	rng := rand.New(rand.NewPCG(5, 6))
	runes := []rune("ae\u0301\u0323\u0300\u00e9\u1100\u1161\u11a8\uac00\u4e2d \u212bA\u030a\U0001d160")
	for range 2000 {
		var b strings.Builder
		for range rng.IntN(200) {
			b.WriteRune(runes[rng.IntN(len(runes))])
		}
		text := b.String()
		whole := norm.NFC.String(text)
		cut := rng.IntN(len(whole) + 10)
		g := domain.NewGrepNFC(text, cut)
		if !strings.HasPrefix(whole, g.Text) || len(g.Text) > cut || g.Cut != (len(whole) > cut) ||
			!g.Cut && g.Text != whole {
			t.Fatalf("NewGrepNFC(%+q, %d) = %+q (cut %v), NFC %+q", text, cut, g.Text, g.Cut, whole)
		}
		if a, z := g.Span(0, len(g.Text)); a != 0 || norm.NFC.String(text[a:z]) != g.Text {
			t.Fatalf("Span of all of %+q (cut %d) = %d, %d", text, cut, a, z)
		}
	}
	// A long line not in NFC: its start alone is copied.
	long := strings.Repeat("e\u0301", 1<<20)
	if g := domain.NewGrepNFC(long, 100); len(g.Text) > 100 || len(g.Text) < 90 || !g.Cut {
		t.Errorf("a 3 MiB line cut at 100: %d bytes matched", len(g.Text))
	}
}
