package mcpserver

import (
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MaxAnswerBytes bounds a tool's answer: the sum, over its text blocks,
// of TextBytes. For a structured tool the one text block is the result's
// JSON text, whose strings are thus escaped twice. Images are bounded on
// their own (read's MaxInlineImageBytes) and a link by its parts (M9
// design 4.7). An error's structuredContent.error counts as a text block
// too: a client may hand it to the model.
const MaxAnswerBytes = 48 << 10

// TextBytes is s's bytes in a text block as the SDK writes it:
// encoding/json's string escapes (" \ \n \r \t \b \f two bytes; another
// C0 control, U+2028 and U+2029 six; U+007F one; a C1 control two) and,
// since TextContent marshals with json.Marshal, <, > and & six. Without
// the surrounding quotes. A byte that is no UTF-8 is written as U+FFFD,
// three bytes.
func TextBytes(s string) int {
	n := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			n += asciiBytes(b)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			n += 3
		case r == '\u2028' || r == '\u2029':
			n += 6
		default:
			n += size
		}
		i += size
	}
	return n
}

// asciiBytes is TextBytes of one byte below utf8.RuneSelf.
func asciiBytes(b byte) int {
	switch b {
	case '"', '\\', '\b', '\f', '\n', '\r', '\t':
		return 2
	case '<', '>', '&':
		return 6
	}
	if b < 0x20 {
		return 6
	}
	return 1
}

// MemberBytes is s's bytes as a string member of a structured tool's
// answer: TextBytes of s as mcpserver's encoder writes it (" and \ four,
// the five short escapes three, another control, U+2028 and U+2029 seven,
// <, > and & six), without the surrounding quotes.
func MemberBytes(s string) int {
	n := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			switch {
			case b == '"' || b == '\\':
				n += 4 // \" or \\, each byte of it escaped again
			case b == '\b' || b == '\f' || b == '\n' || b == '\r' || b == '\t':
				n += 3 // \n: the backslash escaped again
			case b < 0x20:
				n += 7 // \u00XX: the backslash escaped again
			default:
				n += asciiBytes(b)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			n += 3
		case r == '\u2028' || r == '\u2029':
			n += 7
		default:
			n += size
		}
		i += size
	}
	return n
}

// answerBytes is res's bytes as MaxAnswerBytes counts them: TextBytes of
// each text block, and of an error's structuredContent, which errorAnswer
// encoded.
func answerBytes(res *mcp.CallToolResult) int {
	n := 0
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			n += TextBytes(t.Text)
		}
	}
	if structured, ok := res.StructuredContent.(json.RawMessage); res.IsError && ok {
		n += TextBytes(string(structured))
	}
	return n
}

// errNothingFits is a Fit whose answer of no item is over the bound.
var errNothingFits = errors.New("mcpserver: the answer of no item is over the answer's bound")

// Fit is the answer of k ≤ n items that MaxAnswerBytes holds, where k is
// n, or k items fit and k+1 do not: build(k) is the answer with the first
// k items, which marks the others as left out (a next_cursor, or
// truncated) when k < n. The candidates gallop (1, 2, 4, … at most n)
// until one is over, then bisect between it and the last that fit; each
// is encoded as the answer is and measured exactly, so no builder counts
// on its own. A cursor's length follows the last item, so the measure is
// not monotone in k, and k need not be the largest that fits.
// Result.Structured is the encoded JSON. build(0) over the bound is an
// error: nothing the tool can answer fits.
func Fit(n int, build func(k int) any) (Result, int, error) {
	text, k, ok, err := fitItems(n, func(k int) (json.RawMessage, bool, error) {
		text, err := encode(build(k))
		if err != nil {
			return nil, false, err
		}
		return text, TextBytes(string(text)) <= MaxAnswerBytes, nil
	})
	switch {
	case err != nil:
		return Result{}, 0, err
	case !ok:
		return Result{}, 0, errNothingFits
	}
	return Result{Structured: text}, k, nil
}

// fitItems is Fit's search, which a tool's error of items shares (M9/P2
// design 3.10.2): measure(k) is the answer of the first k of n items and
// whether it is within the bound. It is the answer of k items, where k is
// n or k fit and k+1 do not, and true; when not even the answer of no item
// is within the bound, that answer, 0 and false.
func fitItems[T any](n int, measure func(k int) (T, bool, error)) (T, int, bool, error) {
	var zero, fit T // fit: the answer of lo items, once measured
	lo, over, fitted := 0, 0, false
	for k := 1; k <= n; k = min(2*k, n) {
		a, ok, err := measure(k)
		switch {
		case err != nil:
			return zero, 0, false, err
		case !ok:
			over = k
		case k == n:
			return a, n, true, nil
		default:
			lo, fit, fitted = k, a, true
			continue
		}
		break
	}
	for over-lo > 1 {
		mid := lo + (over-lo)/2
		a, ok, err := measure(mid)
		switch {
		case err != nil:
			return zero, 0, false, err
		case ok:
			lo, fit, fitted = mid, a, true
		default:
			over = mid
		}
	}
	if !fitted { // no item fits, or there are none
		a, ok, err := measure(0)
		if err != nil {
			return zero, 0, false, err
		}
		return a, 0, ok, nil
	}
	return fit, lo, true, nil
}
