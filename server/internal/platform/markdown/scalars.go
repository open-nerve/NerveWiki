package markdown

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// note notes the string scalar n, valued value, if it is written on one
// line: plain, in single quotes or in double quotes (M6/P1 design 3.2). A
// block scalar, a string over lines, or one whose bytes do not read back
// as its value is not noted.
func (r *reader) note(n *yaml.Node, value string) {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return
	}
	at, ok := r.offset(n.Line, n.Column)
	if !ok {
		return
	}
	at = pastProperties(r.src, at)
	s := Scalar{Path: strings.Join(r.path, "."), Value: value}
	switch {
	case n.Style&yaml.SingleQuotedStyle != 0:
		s.Quote = '\''
		s.start, s.offsets, ok = singleQuoted(r.src, at, value)
	case n.Style&yaml.DoubleQuotedStyle != 0:
		s.Quote = '"'
		s.start, s.offsets, ok = doubleQuoted(r.src, at, value)
	default:
		s.start = at
		ok = plain(r.src, at, value)
	}
	if !ok {
		return
	}
	s.start += r.at
	for i := range s.offsets {
		s.offsets[i] += r.at
	}
	r.scalars = append(r.scalars, s)
}

// offset is where line and column (from 1, in characters, as the YAML
// library counts them) are in src.
func (r *reader) offset(line, column int) (int, bool) {
	if r.lines == nil {
		r.lines = []int{0}
		for i, b := range r.src {
			if b == '\n' {
				r.lines = append(r.lines, i+1)
			}
		}
	}
	if line < 1 || line > len(r.lines) || column < 1 {
		return 0, false
	}
	i := r.lines[line-1]
	for range column - 1 {
		if i >= len(r.src) || r.src[i] == '\n' {
			return 0, false
		}
		_, size := utf8.DecodeRune(r.src[i:])
		i += size
	}
	return i, true
}

// pastProperties is where the scalar at at is written past its anchor and
// tag (&x, !!str), which the YAML library counts as its start.
func pastProperties(src []byte, at int) int {
	for at < len(src) && (src[at] == '&' || src[at] == '!') {
		for at < len(src) && src[at] != ' ' && src[at] != '\t' && src[at] != '\n' && src[at] != '\r' {
			at++
		}
		for at < len(src) && (src[at] == ' ' || src[at] == '\t') {
			at++
		}
	}
	return at
}

// plain tells whether src at at writes value as it is, on its line.
func plain(src []byte, at int, value string) bool {
	return at+len(value) <= len(src) && string(src[at:at+len(value)]) == value &&
		!strings.ContainsAny(value, "\r\n")
}

// singleQuoted reads the string in single quotes at at: where its value
// starts and where each of its bytes is written, two single quotes
// writing one.
func singleQuoted(src []byte, at int, value string) (int, []int, bool) {
	if at >= len(src) || src[at] != '\'' {
		return 0, nil, false
	}
	var got []byte
	var offsets []int
	for i := at + 1; i < len(src); i++ {
		switch src[i] {
		case '\n', '\r':
			return 0, nil, false
		case '\'':
			if i+1 < len(src) && src[i+1] == '\'' {
				offsets = append(offsets, i)
				got = append(got, '\'')
				i++
				continue
			}
			if string(got) != value {
				return 0, nil, false
			}
			return at + 1, append(offsets, i), true
		}
		offsets = append(offsets, i)
		got = append(got, src[i])
	}
	return 0, nil, false
}

// doubleQuoted reads the string in double quotes at at: where its value
// starts and where each of its bytes is written, an escape writing the
// bytes it stands for at the backslash.
func doubleQuoted(src []byte, at int, value string) (int, []int, bool) {
	if at >= len(src) || src[at] != '"' {
		return 0, nil, false
	}
	var got []byte
	var offsets []int
	for i := at + 1; i < len(src); {
		switch c := src[i]; c {
		case '\n', '\r':
			return 0, nil, false
		case '"':
			if string(got) != value {
				return 0, nil, false
			}
			return at + 1, append(offsets, i), true
		case '\\':
			b, n, ok := unescape(src[i:])
			if !ok {
				return 0, nil, false
			}
			for range b {
				offsets = append(offsets, i)
			}
			got = append(got, b...)
			i += n
		default:
			offsets = append(offsets, i)
			got = append(got, c)
			i++
		}
	}
	return 0, nil, false
}

// escapes are YAML's one-character escapes in double quotes.
var escapes = map[byte]string{ //nolint:gochecknoglobals // read only
	'0': "\x00", 'a': "\a", 'b': "\b", 't': "\t", '\t': "\t", 'n': "\n", 'v': "\v", 'f': "\f",
	'r': "\r", 'e': "\x1b", ' ': " ", '"': "\"", '/': "/", '\\': "\\",
	'N': "\u0085", '_': " ", 'L': " ", 'P': " ",
}

// unescape reads the escape at the head of b: the bytes it stands for and
// how many bytes it takes. An escaped line break is not on one line.
func unescape(b []byte) ([]byte, int, bool) {
	if len(b) < 2 {
		return nil, 0, false
	}
	if s, ok := escapes[b[1]]; ok {
		return []byte(s), 2, true
	}
	var digits int
	switch b[1] {
	case 'x':
		digits = 2
	case 'u':
		digits = 4
	case 'U':
		digits = 8
	default:
		return nil, 0, false
	}
	if len(b) < 2+digits {
		return nil, 0, false
	}
	v, err := strconv.ParseUint(string(b[2:2+digits]), 16, 32)
	if err != nil {
		return nil, 0, false
	}
	return utf8.AppendRune(nil, rune(v)), 2 + digits, true
}
