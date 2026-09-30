package bodyshape

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A document that json.Valid accepts can still be read two ways. Check reads
// its objects as maps, where the last of two members with one name wins,
// while the generated handler decodes them into Go structs, which merge the
// two: a member Check never saw would reach the domain. And a string that is
// not valid Unicode decodes to U+FFFD, a text the client never sent. So
// Check first requires what I-JSON (RFC 7493) requires of both: each member
// name at most once in its object, compared as decoded, so that "a" and its
// \u escape are one name; each string, name or value, valid Unicode: UTF-8
// bytes, and a \u escape of a surrogate only as one half of a pair. Then
// every decoder reads the one document the client wrote.

// ambiguities reports to p what lets value, one valid JSON value, be read two
// ways: a member name repeated in its object (duplicate, once per name) and a
// string that is not valid Unicode (invalid_format), each at its JSON path.
// It stops when p is full.
func ambiguities(value []byte, p *problems) {
	s := scanner{data: value, p: p}
	s.value()
}

// scanner reads a valid JSON value byte by byte; it does not check the
// syntax again. It decodes member names only: a value's string is checked on
// its bytes.
type scanner struct {
	data []byte
	pos  int
	p    *problems
}

func (s *scanner) value() {
	s.space()
	switch s.data[s.pos] {
	case '{':
		s.object()
	case '[':
		s.array()
	case '"':
		if !validUnicode(s.str()) {
			s.p.report(codeInvalidFormat)
		}
	default: // a number, true, false or null
		for s.pos < len(s.data) && !isEnd(s.data[s.pos]) {
			s.pos++
		}
	}
}

func (s *scanner) object() {
	seen := map[string]int{}
	s.pos++ // {
	if s.space(); s.data[s.pos] == '}' {
		s.pos++
		return
	}
	for {
		s.space()
		raw := s.str()
		var name string
		_ = json.Unmarshal(raw, &name) // the whole document is valid JSON
		seen[name]++
		s.p.enter(name)
		switch {
		case !validUnicode(raw):
			s.p.report(codeInvalidFormat)
		case seen[name] == 2:
			s.p.report(codeDuplicate)
		}
		s.space()
		s.pos++ // :
		s.value()
		s.p.leave()
		if s.p.full() {
			return
		}
		s.space()
		s.pos++ // , or }
		if s.data[s.pos-1] == '}' {
			return
		}
	}
}

func (s *scanner) array() {
	s.pos++ // [
	if s.space(); s.data[s.pos] == ']' {
		s.pos++
		return
	}
	for i := 0; ; i++ {
		s.p.enterItem(i)
		s.value()
		s.p.leave()
		if s.p.full() {
			return
		}
		s.space()
		s.pos++ // , or ]
		if s.data[s.pos-1] == ']' {
			return
		}
	}
}

// str reads the string at s.pos and returns it as written, with its quotes.
func (s *scanner) str() []byte {
	start := s.pos
	for s.pos++; s.data[s.pos] != '"'; s.pos++ {
		if s.data[s.pos] == '\\' {
			s.pos++
		}
	}
	s.pos++
	return s.data[start:s.pos]
}

// validUnicode reports whether raw, a valid JSON string with its quotes,
// decodes to the text it spells: UTF-8 bytes, and every \u escape of a
// surrogate one half of a pair.
func validUnicode(raw []byte) bool { return utf8.Valid(raw) && pairedSurrogates(raw) }

// pairedSurrogates reports whether every \u escape of a surrogate in raw, a
// valid JSON string with its quotes, is one half of a pair: a high one right
// before a low one. A lone half decodes to U+FFFD. The closing quote is read
// like any other character, so a high half at the end of the string fails
// the same check as a high half before any other character.
func pairedSurrogates(raw []byte) bool {
	high := false // the escape just read is a high surrogate
	for i := 0; i < len(raw); {
		unit := -1
		switch {
		case raw[i] != '\\':
			i++
		case raw[i+1] != 'u':
			i += 2
		default:
			n, _ := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
			unit = int(n)
			i += 6
		}
		if low := unit >= 0xDC00 && unit <= 0xDFFF; low != high {
			return false // a high half without its low one, or a low half without its high one
		}
		high = unit >= 0xD800 && unit <= 0xDBFF
	}
	return true
}

func (s *scanner) space() {
	for s.pos < len(s.data) && isSpace(s.data[s.pos]) {
		s.pos++
	}
}

func isSpace(c byte) bool { return strings.IndexByte(jsonSpace, c) >= 0 }

// isEnd reports whether c ends a number or a literal.
func isEnd(c byte) bool { return c == ',' || c == ']' || c == '}' || isSpace(c) }
