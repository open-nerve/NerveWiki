package domain_test

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
)

// The messages of Replace's refusals.
const (
	notInPage    = "validation_failed old_string not_allowed: is not in the page: copy it from what read gave, byte for byte"
	differsInNFC = "; the page's text and old_string differ in Unicode normalization (NFC)"
	pageTooLong  = "validation_failed new_string too_long: would make the page longer than 5 MiB (5,242,880 bytes)"
)

// foundTimes is the refusal of an old_string found n times without all.
func foundTimes(n int) string {
	return fmt.Sprintf("validation_failed old_string not_allowed: is found %d times; add context to make it unique, or set replace_all", n)
}

// replaced is Replace's answer in one line: the content, the count and
// whether CRLF matched; or its refusal.
func replaced(content, old, new string, all bool) string {
	r, err := domain.Replace(content, old, new, all)
	if err != nil {
		return said(err)
	}
	return fmt.Sprintf("%q %d %v", r.Content, r.Count, r.CRLF)
}

// An edit's two strings are checked before anything is read (M9/P2 design
// 3.9): old_string of 1 to 1 MiB, new_string of at most 5 MiB, both UTF-8
// without NUL, and different; each refusal names its field and repeats no
// value.
func TestCheckEdit(t *testing.T) {
	for _, tt := range []struct{ old, new, want string }{
		{"a", "b", ""},
		{"a", "", ""},
		{strings.Repeat("a", domain.MaxEditOldBytes), "b", ""},
		{"a", strings.Repeat("b", domain.MaxContentBytes), ""},
		{"", "b", "validation_failed old_string too_short: must not be empty"},
		{strings.Repeat("a", domain.MaxEditOldBytes+1), "b", "validation_failed old_string too_long: must be at most 1 MiB (1,048,576 bytes)"},
		{"a\x00", "b", "validation_failed old_string invalid_format: must be UTF-8 text without NUL characters"},
		{"a\xff", "b", "validation_failed old_string invalid_format: must be UTF-8 text without NUL characters"},
		{"a", strings.Repeat("b", domain.MaxContentBytes+1), "validation_failed new_string too_long: must be at most 5 MiB (5,242,880 bytes)"},
		{"a", "b\x00", "validation_failed new_string invalid_format: must be UTF-8 text without NUL characters"},
		{"a", "\xc3", "validation_failed new_string invalid_format: must be UTF-8 text without NUL characters"},
		{"same", "same", "validation_failed new_string not_allowed: must differ from old_string"},
	} {
		got := ""
		if err := domain.CheckEdit(tt.old, tt.new); err != nil {
			got = said(err)
		}
		if got != tt.want {
			t.Errorf("CheckEdit(%.20q, %.20q) = %q, want %q", tt.old, tt.new, got, tt.want)
		}
	}
}

// Replace replaces byte for byte (M9/P2 design 3.9): once, when old occurs
// once, a second occurrence overlapping the first making it ambiguous as
// one apart does (aa is twice in aaa); each, left to right, with all. A
// page whose line breaks are all CRLF matches an old_string of bare LFs as
// CRLF, and writes new_string's bare LFs as CRLF, also an old_string whose
// one LF starts it, which as it is matches inside a CRLF (M9/P2 S1 review
// B-M1); a page of both kinds, or an old_string with a CR, is matched as
// it is. A byte order mark is the content's.
func TestReplace(t *testing.T) {
	for _, tt := range []struct {
		content, old, new string
		all               bool
		want              string
	}{
		{"hello world", "world", "there", false, `"hello there" 1 false`},
		{"hello world", "World", "there", false, notInPage},
		{"aaa", "aa", "b", false, foundTimes(2)},
		{"abab ab", "ab", "x", false, foundTimes(3)},
		{"aaaaa", "aa", "X", true, `"XXa" 2 false`},
		{"a b a", "a", "", true, `" b " 2 false`},
		{"one\r\ntwo\r\nthree\r\n", "two\nthree", "2\n3", false, `"one\r\n2\r\n3\r\n" 1 true`},
		{"a\r\nb\r\na\r\nb", "a\nb", "c\nd", true, `"c\r\nd\r\nc\r\nd" 2 true`},
		{"a\r\nb", "a\nb", "c\r\nd\ne", false, `"c\r\nd\r\ne" 1 true`},
		{"a\r\nb\r\na\r\nb", "a\nb", "c", false, foundTimes(2)},
		// The CRLF rule makes the two strings one: the content stays as it is.
		{"x\r\na\r\ny", "a\n", "a\r\n", false, `"x\r\na\r\ny" 1 true`},
		// An old_string that starts with its one LF is matched as CRLF, not
		// inside a CRLF: its occurrences are the same, each a CR longer.
		{"a\r\nb\r\n", "\nb", "\nc\nd", false, `"a\r\nc\r\nd\r\n" 1 true`},
		{"a\r\nb\r\n", "\nb", " b", false, `"a b\r\n" 1 true`},
		{"a\r\nb\r\n", "\n", "x", false, foundTimes(2)},
		{"a\r\nb\r\n", "\n", "\n\n", true, `"a\r\n\r\nb\r\n\r\n" 2 true`},
		// A page of both kinds of line breaks, one without any, an
		// old_string with a CR: no CRLF rule.
		{"one\r\ntwo\nthree\r\n", "one\ntwo", "x", false, notInPage},
		{"one\ntwo", "one\ntwo", "x", false, `"x" 1 false`},
		{"no breaks", "a\nb", "x", false, notInPage},
		{"a\r\nb\r\n", "\r\na\nb", "x", false, notInPage},
		{"\ufeffhello", "hello", "bye", false, `"\ufeffbye" 1 false`},
		{"\ufeffhello", "\ufeffh", "H", false, `"Hello" 1 false`},
	} {
		if got := replaced(tt.content, tt.old, tt.new, tt.all); got != tt.want {
			t.Errorf("Replace(%q, %q, %q, %v) = %s, want %s", tt.content, tt.old, tt.new, tt.all, got, tt.want)
		}
	}
}

// An old_string not in the page is said to differ from it in Unicode
// normalization when the page or old_string is not in NFC and their NFC
// forms match; never otherwise (M9/P2 design 3.9).
func TestReplaceHintsAtNFC(t *testing.T) {
	composed, decomposed := "caf\u00e9", "cafe\u0301"
	for _, tt := range []struct {
		content, old string
		hinted       bool
	}{
		{decomposed + " au lait", composed, true},
		{composed + " au lait", decomposed + " au", true},
		{decomposed + " au lait", "caf\u00e8", false},
		{composed + " au lait", "cafe", false},
	} {
		want := notInPage
		if tt.hinted {
			want += differsInNFC
		}
		if got := replaced(tt.content, tt.old, "x", false); got != want {
			t.Errorf("Replace(%q, %q) = %s, want %s", tt.content, tt.old, got, want)
		}
	}
}

// The size of what Replace would write is reckoned before it is built, and
// under the CRLF rule with the strings as converted: a new_string whose
// bare LFs CRLF makes longer passes 5 MiB where it would not as given
// (M9/P2 design 3.9; review B, BC-N5).
func TestReplaceReckonsTheSize(t *testing.T) {
	short := "b" + strings.Repeat("c", domain.MaxContentBytes-2)
	if got := replaced(short, "b", "xy", false); got == pageTooLong {
		t.Errorf("a content of exactly 5 MiB is refused: %s", got)
	}
	if got := replaced(short, "b", "xyz", false); got != pageTooLong {
		t.Errorf("a content past 5 MiB = %.60s, want %s", got, pageTooLong)
	}
	// All of n bytes, all CRLF, a\r\nb in it once.
	page := func(n int) string { return "a\r\nb\r\n" + strings.Repeat("c", n-6) }
	old, new := "a\nb", "x\ny\nz\n" // as CRLF, 4 and 9 bytes: the content grows by 5, by 3 as given
	if got := replaced(page(domain.MaxContentBytes-3), old, new, false); got != pageTooLong {
		t.Errorf("a new_string past 5 MiB as CRLF = %.60s, want %s", got, pageTooLong)
	}
	r, err := domain.Replace(page(domain.MaxContentBytes-5), old, new, false)
	if err != nil || len(r.Content) != domain.MaxContentBytes || !r.CRLF {
		t.Errorf("a new_string of exactly 5 MiB as CRLF = %d bytes, CRLF %v, %v", len(r.Content), r.CRLF, err)
	}
	// Refused for its size, a content of 8 MiB is never built: Replace
	// allocates a few bytes. Not parallel: TotalAlloc counts the whole
	// process.
	as, bs := strings.Repeat("a", 4096), strings.Repeat("b", 2048)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = domain.Replace(as, "a", bs, true)
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; said(err) != pageTooLong || got > 1<<20 {
		t.Errorf("a content of 8 MiB refused for its size: %v, after %d KiB allocated", err, got>>10)
	}
}

// occurrences is how many times p occurs in s, compared at each position:
// overlapping ones too.
func occurrences(s, p string) int {
	n := 0
	for i := 0; i+len(p) <= len(s); i++ {
		if s[i:i+len(p)] == p {
			n++
		}
	}
	return n
}

// The matcher finds what strings.Index finds, from the start and from any
// byte; Replace answers as an oracle that compares old at each position,
// for one occurrence, several, overlapping or not, the CRLF rule and the
// NFC hint, and with all as strings.Count and strings.ReplaceAll count and
// replace (M9/P2 design 3.9): the strings functions are oracles in a test
// alone. 100,000 cases, of pages of at most 40 bytes and patterns of at
// most 6, their bytes picked from ab, abc, a and CRLF, a and é, e and
// U+0301: some are no UTF-8, some not in NFC.
func TestTheMatcherIsStrings(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 17))
	alphabets := []string{"ab", "abc", "a\r\n", "a\u00e9", "ae\u0301"}
	for cases := 0; cases < 100_000; {
		al := alphabets[r.IntN(len(alphabets))]
		gen := func(most int) string {
			b := make([]byte, r.IntN(most+1))
			for i := range b {
				b[i] = al[r.IntN(len(al))]
			}
			return string(b)
		}
		s, p := gen(40), gen(6)
		if p == "" {
			continue
		}
		cases++
		if got, want := domain.MatchIndex(s, p, 0), strings.Index(s, p); got != want {
			t.Fatalf("MatchIndex(%q, %q, 0) = %d, want %d", s, p, got, want)
		}
		from := r.IntN(len(s) + 1)
		want := strings.Index(s[from:], p)
		if want >= 0 {
			want += from
		}
		if got := domain.MatchIndex(s, p, from); got != want {
			t.Fatalf("MatchIndex(%q, %q, %d) = %d, want %d", s, p, from, got, want)
		}

		q, crlf, newAs := p, false, "X\n"
		if strings.Contains(p, "\n") && !strings.Contains(p, "\r") && strings.Contains(s, "\r\n") &&
			strings.Count(s, "\n") == strings.Count(s, "\r\n") {
			q, crlf, newAs = strings.ReplaceAll(p, "\n", "\r\n"), true, "X\r\n"
		}
		occ := occurrences(s, q)
		missing := notInPage
		if (!norm.NFC.IsNormalString(s) || !norm.NFC.IsNormalString(q)) && strings.Contains(norm.NFC.String(s), norm.NFC.String(q)) {
			missing += differsInNFC
		}
		one := fmt.Sprintf("%q 1 %v", strings.Replace(s, q, newAs, 1), crlf)
		switch {
		case occ == 0:
			one = missing
		case occ > 1:
			one = foundTimes(max(strings.Count(s, q), 2))
		}
		if got := replaced(s, p, "X\n", false); got != one {
			t.Fatalf("Replace(%q, %q) = %s, want %s", s, p, got, one)
		}
		all := fmt.Sprintf("%q %d %v", strings.ReplaceAll(s, q, newAs), strings.Count(s, q), crlf)
		if occ == 0 {
			all = missing
		}
		if got := replaced(s, p, "X\n", true); got != all {
			t.Fatalf("Replace(%q, %q, all) = %s, want %s", s, p, got, all)
		}
	}
}
