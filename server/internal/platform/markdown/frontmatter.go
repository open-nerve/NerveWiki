package markdown

import "bytes"

const bom = "\xef\xbb\xbf"

// Frontmatter is what the YAML at the head of a page holds (overall design
// 4.2; the fixtures' rule 1).
type Frontmatter struct {
	// Present tells that the content has a frontmatter, written well or not.
	Present bool
	// Valid tells that it is a mapping within the limits of the YAML.
	Valid bool
	// Properties are its keys and values in the order they are written.
	Properties []Property
	// Scalars are its strings written on one line, where they are written
	// (M6/P1 design 3.2): a property link is one. A value an alias repeats
	// is not among them.
	Scalars []Scalar
}

// Scalar is a string of a frontmatter written on one line.
type Scalar struct {
	// Path is its property's: the keys and the list indexes down to it,
	// joined by '.', as "sources.0".
	Path string
	// Value is the string.
	Value string
	// Quote is how it is written: 0 plain, '\'' or '"'.
	Quote byte

	start   int   // where Value is written in the content, past its quote
	offsets []int // where each byte of Value is written, and its end; nil when at start+i
}

// Offset is where byte i of the value is written in the content, for i
// from 0 to len(Value): a quoted string may write a byte with more (two
// single quotes for one in single quotes, an escape in double quotes).
func (s Scalar) Offset(i int) int {
	if s.offsets == nil {
		return s.start + i
	}
	return s.offsets[i]
}

// Property is one key of a frontmatter. Its value is nil, a bool, an int64,
// a float64, a string, a []any of values or a []Property.
type Property struct {
	Key   string
	Value any
}

// span is where the frontmatter is: the YAML between its delimiters, and
// the end of the closing delimiter's line.
type span struct {
	from, to, end int
}

// frontmatterSpan finds the frontmatter as the fixtures' check.mjs does: at
// the head of the content, after a byte order mark maybe, a line that is
// "---" up to a later line that is "---" without its "\r". Without the
// closing line there is none.
func frontmatterSpan(src []byte) (span, bool) {
	start := 0
	if bytes.HasPrefix(src, []byte(bom)) {
		start = len(bom)
	}
	var open int
	switch {
	case bytes.HasPrefix(src[start:], []byte("---\n")):
		open = 4
	case bytes.HasPrefix(src[start:], []byte("---\r\n")):
		open = 5
	default:
		return span{}, false
	}
	for pos := start + open; pos < len(src); {
		end, next := len(src), len(src)
		if nl := bytes.IndexByte(src[pos:], '\n'); nl >= 0 {
			end, next = pos+nl, pos+nl+1
		}
		if string(bytes.TrimSuffix(src[pos:end], []byte("\r"))) == "---" {
			return span{from: start + open, to: pos, end: next}, true
		}
		pos = next
	}
	return span{}, false
}

// frontmatterOf reads the frontmatter of src, and where it ends: 0 without
// one.
func frontmatterOf(src []byte) (Frontmatter, int) {
	s, ok := frontmatterSpan(src)
	if !ok {
		return Frontmatter{}, 0
	}
	props, scalars, valid := properties(src[s.from:s.to], s.from)
	return Frontmatter{Present: true, Valid: valid, Properties: props, Scalars: scalars}, s.end
}

// blank is src as the parser reads it: the byte order mark made line
// breaks, which a document may start with to no effect (spaces would
// indent its first line), and the frontmatter (up to end) made spaces,
// keeping each '\r' and '\n'; so every offset stays where it is (overall
// design 4.3). It is src itself when there is nothing to blank.
func blank(src []byte, end int) []byte {
	start := 0
	if bytes.HasPrefix(src, []byte(bom)) {
		start = len(bom)
	}
	if start == 0 && end == 0 {
		return src
	}
	out := bytes.Clone(src)
	for i := range start {
		out[i] = '\n'
	}
	for i := start; i < end; i++ {
		if out[i] != '\r' && out[i] != '\n' {
			out[i] = ' '
		}
	}
	return out
}
