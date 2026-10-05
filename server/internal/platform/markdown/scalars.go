package markdown

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// note notes the string scalar n, valued value, if it is written on one
// line: plain, in single quotes or in double quotes (M6/P1 design 3.2). A
// block scalar, a string over lines, or one whose bytes do not read back
// as its value is not noted. Its path past what the paths may take is not
// valid.
func (r *reader) note(n *yaml.Node, value string) error {
	if n.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil
	}
	at, ok := r.offset(n.Line, n.Column)
	if !ok {
		return nil
	}
	if at, ok = pastProperties(r.src, at); !ok {
		return nil
	}
	s := Scalar{Value: value}
	switch {
	case n.Style&yaml.SingleQuotedStyle != 0:
		s.Quote = '\''
		s.start, s.offsets, ok = singleQuoted(r.src, at, value)
	case n.Style&yaml.DoubleQuotedStyle != 0:
		s.Quote = '"'
		s.start, s.offsets, ok = doubleQuoted(r.src, at, value)
	default:
		// An empty one is written nowhere: what is at at is the next node.
		s.start = at
		ok = value != "" && plain(r.src, at, value)
	}
	if !ok {
		return nil
	}
	if r.paths += pathBytes(r.path); r.paths > r.pathsBudget {
		return errInvalid
	}
	s.Path, s.Depth = strings.Join(r.path, "."), len(r.path)
	s.start += r.at
	for i := range s.offsets {
		s.offsets[i] += r.at
	}
	r.scalars = append(r.scalars, s)
	return nil
}

// pathBytes is how many bytes path takes joined by '.'.
func pathBytes(path []string) int {
	n := max(len(path)-1, 0)
	for _, key := range path {
		n += len(key)
	}
	return n
}

// offset is where line and column (from 1, in characters, as the YAML
// library counts them) are in src. The scalars come in the order they are
// written, so it goes on from the last one on the same line: a line of a
// thousand scalars is read once, not once for each.
func (r *reader) offset(line, column int) (int, bool) {
	if r.lines == nil {
		r.lines = lineStarts(r.src)
	}
	if line < 1 || line > len(r.lines) || column < 1 {
		return 0, false
	}
	i, col := r.lines[line-1], 1
	if c := r.last; c.line == line && c.column <= column {
		i, col = c.at, c.column
	}
	for ; col < column; col++ {
		if i >= len(r.src) || lineBreak(r.src[i:]) > 0 {
			return 0, false
		}
		_, size := utf8.DecodeRune(r.src[i:])
		i += size
	}
	r.last = position{line: line, column: column, at: i}
	return i, true
}

// position is a line and column of the YAML and where it is.
type position struct{ line, column, at int }

// lineStarts is where each line of src starts, a line ending at a break as
// the YAML library counts them.
func lineStarts(src []byte) []int {
	starts := []int{0}
	for i := 0; i < len(src); {
		if n := lineBreak(src[i:]); n > 0 {
			i += n
			starts = append(starts, i)
			continue
		}
		i++
	}
	return starts
}

// lineBreak is how many bytes the line break at the head of b takes, 0 if
// none: "\r\n", '\r', '\n', or NEL, LS and PS, which the YAML library breaks
// lines at too.
func lineBreak(b []byte) int {
	switch {
	case len(b) == 0:
		return 0
	case b[0] == '\n':
		return 1
	case b[0] == '\r':
		if len(b) > 1 && b[1] == '\n' {
			return 2
		}
		return 1
	}
	for _, br := range []string{"\u0085", "\u2028", "\u2029"} {
		if bytes.HasPrefix(b, []byte(br)) {
			return len(br)
		}
	}
	return 0
}

// pastProperties is where the scalar at at is written past its anchor and
// tag (&x, !!str), which the YAML library counts as its start: past the
// spaces, line breaks and comments after them too, the scalar maybe on a
// later line. An anchor ends at a character other than a letter, a digit,
// '_' or '-', as in the YAML library; one that a character other than a
// space, a tab or a line break ends is refused, as the code cannot tell
// where what follows it starts.
func pastProperties(src []byte, at int) (int, bool) {
	for at < len(src) && (src[at] == '&' || src[at] == '!') {
		anchor := src[at] == '&'
		for at++; at < len(src) && !isBlank(src, at) && (!anchor || isAnchorChar(src[at])); at++ {
		}
		if at < len(src) && !isBlank(src, at) {
			return 0, false
		}
		at = pastSeparation(src, at)
	}
	return at, true
}

// isBlank tells whether src at at is a space, a tab or a line break.
func isBlank(src []byte, at int) bool {
	return src[at] == ' ' || src[at] == '\t' || lineBreak(src[at:]) > 0
}

// isAnchorChar tells whether c may be in an anchor's name.
func isAnchorChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || c == '-'
}

// pastSeparation is where something starts past spaces, tabs, line breaks
// and comments from at: a '#' there follows a space or a line break.
func pastSeparation(src []byte, at int) int {
	for at < len(src) {
		switch {
		case src[at] == ' ' || src[at] == '\t':
			at++
		case src[at] == '#':
			for at < len(src) && lineBreak(src[at:]) == 0 {
				at++
			}
		case lineBreak(src[at:]) > 0:
			at += lineBreak(src[at:])
		default:
			return at
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
	'N': "\u0085", '_': " ", 'L': "\u2028", 'P': "\u2029",
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
