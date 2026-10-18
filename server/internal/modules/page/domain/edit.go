package domain

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// MaxEditOldBytes bounds an edit's old_string: the 1 MiB besides a page's
// content that a call's body has room for (bootstrap's mcpBodyLimit; M9/P2
// design 2.1 item 5).
const MaxEditOldBytes = 1 << 20

// CheckEdit checks an edit's two strings, given (M9 design 4.10):
// old_string of 1 to MaxEditOldBytes bytes, new_string of at most
// MaxContentBytes, both UTF-8 without NUL, and different; 422 on the one
// that is not. A value is never repeated: it may be megabytes.
func CheckEdit(old, new string) error {
	switch {
	case old == "":
		return shared.Invalid(shared.FieldError{Field: "old_string", Code: shared.FieldTooShort, Message: "must not be empty"})
	case len(old) > MaxEditOldBytes:
		return shared.Invalid(shared.FieldError{Field: "old_string", Code: shared.FieldTooLong,
			Message: "must be at most 1 MiB (1,048,576 bytes)"})
	}
	// Within the bound, CheckContent answers old_string's format alone.
	if err := CheckContent("old_string", old); err != nil {
		return err
	}
	if err := CheckContent("new_string", new); err != nil {
		return err
	}
	if old == new {
		return NotAllowed("new_string", "must differ from old_string")
	}
	return nil
}

// Replacement is an edit's content: the page's with old_string replaced,
// how many times, and whether the two were matched as CRLF.
type Replacement struct {
	Content string
	Count   int
	CRLF    bool
}

// Replace replaces old with new in content, byte for byte (M9 design
// 4.10): once, when old occurs once (a second occurrence, overlapping or
// not, makes it ambiguous), or each occurrence with all, left to right as
// strings.ReplaceAll does. When old does not occur, content's line breaks
// are all CRLF and old's are bare LF, both are matched and written with
// each bare LF made CRLF. 422 on old_string when old does not occur
// (hinting when it would under NFC) or occurs more than once without all;
// on new_string when the content would pass MaxContentBytes, which it
// knows before it builds it. Every search is the matcher's: linear in the
// content and old, whatever their bytes. old is not empty: CheckEdit
// checks it first.
func Replace(content, old, new string, all bool) (Replacement, error) {
	m, crlf := newMatcher(old), false
	i := m.index(content, 0)
	if i < 0 && strings.IndexByte(old, '\n') >= 0 && strings.IndexByte(old, '\r') < 0 && allCRLF(content) {
		old, new, crlf = bareLFsAsCRLF(old), bareLFsAsCRLF(new), true
		m = newMatcher(old)
		i = m.index(content, 0)
	}
	if i < 0 {
		return Replacement{}, notInPage(content, old)
	}
	count := 1
	switch {
	case all:
		count = m.count(content)
	case m.index(content, i+1) >= 0:
		return Replacement{}, NotAllowed("old_string",
			fmt.Sprintf("is found %d times; add context to make it unique, or set replace_all", max(m.count(content), 2)))
	}
	if len(content)+count*(len(new)-len(old)) > MaxContentBytes {
		return Replacement{}, shared.Invalid(shared.FieldError{Field: "new_string", Code: shared.FieldTooLong,
			Message: "would make the page longer than 5 MiB (5,242,880 bytes)"})
	}
	if !all {
		return Replacement{Content: content[:i] + new + content[i+len(old):], Count: 1, CRLF: crlf}, nil
	}
	var b strings.Builder
	b.Grow(len(content) + count*(len(new)-len(old)))
	last := 0
	for j := i; j >= 0; j = m.index(content, j+len(old)) {
		b.WriteString(content[last:j])
		b.WriteString(new)
		last = j + len(old)
	}
	b.WriteString(content[last:])
	return Replacement{Content: b.String(), Count: count, CRLF: crlf}, nil
}

// notInPage is 422 on old_string, which content does not hold: with the
// hint that it would under NFC when content or old is not in NFC and
// their NFC forms match, as the matcher finds them too.
func notInPage(content, old string) error {
	message := "is not in the page: copy it from what read gave, byte for byte"
	if !norm.NFC.IsNormalString(content) || !norm.NFC.IsNormalString(old) {
		if newMatcher(norm.NFC.String(old)).index(norm.NFC.String(content), 0) >= 0 {
			message += "; the page's text and old_string differ in Unicode normalization (NFC)"
		}
	}
	return NotAllowed("old_string", message)
}

// allCRLF reports whether s's line breaks are all CRLF: it has one, and
// each LF follows a CR.
func allCRLF(s string) bool {
	seen := false
	for i := strings.IndexByte(s, '\n'); i >= 0; {
		if i == 0 || s[i-1] != '\r' {
			return false
		}
		seen = true
		j := strings.IndexByte(s[i+1:], '\n')
		if j < 0 {
			break
		}
		i += 1 + j
	}
	return seen
}

// bareLFsAsCRLF is s with each LF that follows no CR made CRLF.
func bareLFsAsCRLF(s string) string {
	var b strings.Builder
	b.Grow(len(s) + strings.Count(s, "\n"))
	last := 0
	for i := range len(s) {
		if s[i] == '\n' && (i == 0 || s[i-1] != '\r') {
			b.WriteString(s[last:i])
			b.WriteString("\r\n")
			last = i + 1
		}
	}
	b.WriteString(s[last:])
	return b.String()
}

// matcher finds a pattern in a text in time linear in both, whatever their
// bytes (Knuth–Morris–Pratt): the one search Replace makes, for finding,
// counting and replacing, and for the CRLF and NFC retries.
// strings.Index is never given an agent's pattern: past bytealg.MaxLen it
// is Rabin–Karp, quadratic for a crafted one (M9/P2 design review B-I2).
type matcher struct {
	p    string
	fail []int32 // fail[i]: the longest proper border of p[:i+1]
}

// newMatcher is the matcher of p, which is not empty.
func newMatcher(p string) matcher {
	fail := make([]int32, len(p))
	for i, k := 1, int32(0); i < len(p); i++ {
		for k > 0 && p[i] != p[k] {
			k = fail[k-1]
		}
		if p[i] == p[k] {
			k++
		}
		fail[i] = k
	}
	return matcher{p: p, fail: fail}
}

// index is the start of the first occurrence of m.p in s at or after from,
// -1 for none: one scan of s[from:] up to the occurrence's end, at most two
// comparisons a byte scanned in all; while no prefix of p is matched, the
// bytes that cannot start p are skipped with IndexByte.
func (m matcher) index(s string, from int) int {
	p, n := m.p, int32(len(m.p))
	k := int32(0) // how much of p the bytes before i match
	for i := from; i < len(s); i++ {
		if k == 0 {
			j := strings.IndexByte(s[i:], p[0])
			if j < 0 {
				return -1
			}
			i += j
		}
		for k > 0 && s[i] != p[k] {
			k = m.fail[k-1]
		}
		if s[i] == p[k] {
			k++
		}
		if k == n {
			return i - int(n) + 1
		}
	}
	return -1
}

// count is how many times m.p occurs in s, left to right without
// overlapping, as strings.Count counts: each scan goes on from the end of
// the occurrence before.
func (m matcher) count(s string) int {
	c := 0
	for j := m.index(s, 0); j >= 0; j = m.index(s, j+len(m.p)) {
		c++
	}
	return c
}
