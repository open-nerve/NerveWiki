package markdown

import (
	"bytes"
	"iter"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// readTag reads one inline raw HTML node: a tag's name in lower case,
// whether it is an end tag, and a start tag's attributes, their names in
// lower case and their values' references resolved. A comment, processing
// instruction, declaration or CDATA section has no name. It follows
// goldmark's grammar of the node (parser/raw_html.go).
func readTag(raw []byte) (name string, end bool, attrs iter.Seq2[string, string]) {
	i := 1
	if len(raw) > 1 && raw[1] == '/' {
		end, i = true, 2
	}
	j := i
	for j < len(raw) && (isLetter(raw[j]) || j > i && (isDigit(raw[j]) || raw[j] == '-')) {
		j++
	}
	if j == i {
		return "", false, nil
	}
	rest := raw[j:]
	return strings.ToLower(string(raw[i:j])), end, func(yield func(string, string) bool) {
		for r := rest; ; {
			k := skipSpace(r)
			if k == 0 || k == len(r) || !isNameStart(r[k]) {
				return
			}
			n := k + 1
			for n < len(r) && isNameChar(r[n]) {
				n++
			}
			key := strings.ToLower(string(r[k:n]))
			value, r2 := "", r[n:]
			if v := skipSpace(r2); v < len(r2) && r2[v] == '=' {
				value, r2 = attrValue(r2[v+1:])
			}
			if !yield(key, value) {
				return
			}
			r = r2
		}
	}
}

// attrValue reads an attribute's value after its '=', quoted or not, as a
// tokenizer reads it: its line breaks made '\n', its references resolved,
// a NUL made U+FFFD.
func attrValue(r []byte) (string, []byte) {
	r = r[skipSpace(r):]
	if len(r) == 0 {
		return "", r
	}
	var v []byte
	switch q := r[0]; q {
	case '"', '\'':
		e := bytes.IndexByte(r[1:], q)
		if e < 0 {
			return "", nil
		}
		v, r = r[1:1+e], r[2+e:]
	default:
		e := bytes.IndexAny(r, " \t\r\n\"'=<>`")
		if e < 0 {
			e = len(r)
		}
		v, r = r[:e], r[e:]
	}
	value := strings.ReplaceAll(strings.ReplaceAll(string(v), "\r\n", "\n"), "\r", "\n")
	return strings.ReplaceAll(unescapeAttr(value), "\x00", "�"), r
}

// unescapeAttr resolves the character references of an attribute's value
// as x/net/html's tokenizer does in an attribute, and an HTML block's
// attributes are read: a named reference is resolved whole or not at all,
// and one without its ';' before a '=' is not ("for historical reasons", in
// the HTML standard's words). html.UnescapeString reads text, where
// "&section=" is "§ion=".
func unescapeAttr(v string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(v, '&')
		if i < 0 {
			return b.String() + v
		}
		b.WriteString(v[:i])
		n := referenceLen(v[i:])
		b.WriteString(resolveAttr(v[i:i+n], v[i+n:]))
		v = v[i+n:]
	}
}

// referenceLen is how long the reference at the head of v is, as the
// tokenizer consumes it: '&', then '#' and an 'x' maybe and digits, or
// letters and digits, then a ';' maybe.
func referenceLen(v string) int {
	i := 1
	digit := isAlnum
	if i < len(v) && v[i] == '#' {
		i++
		digit = isDigit
		if i < len(v) && (v[i] == 'x' || v[i] == 'X') {
			i++
			digit = isHexDigit
		}
	}
	for i < len(v) && digit(v[i]) {
		i++
	}
	if i < len(v) && v[i] == ';' {
		i++
	}
	return i
}

// resolveAttr is the reference ref, followed by rest, resolved as in an
// attribute. A number is resolved as in text. A name text does not know it
// resolves by its head, if the head is a name that needs no ';', and keeps
// the rest of the name after it; an attribute keeps the whole.
func resolveAttr(ref, rest string) string {
	if len(ref) > 1 && ref[1] == '#' {
		return html.UnescapeString(ref)
	}
	if !strings.HasSuffix(ref, ";") && strings.HasPrefix(rest, "=") {
		return ref
	}
	text := html.UnescapeString(ref)
	_, size := utf8.DecodeRuneInString(text)
	if after := text[size:]; text == ref || after != "" && strings.HasSuffix(ref, after) {
		return ref
	}
	return text
}

func skipSpace(r []byte) int {
	i := 0
	for i < len(r) && (r[i] == ' ' || r[i] == '\t' || r[i] == '\n' || r[i] == '\r') {
		i++
	}
	return i
}

func isLetter(c byte) bool    { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
func isDigit(c byte) bool     { return '0' <= c && c <= '9' }
func isAlnum(c byte) bool     { return isLetter(c) || isDigit(c) }
func isHexDigit(c byte) bool  { return isDigit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' }
func isNameStart(c byte) bool { return isLetter(c) || c == '_' || c == ':' }
func isNameChar(c byte) bool  { return isNameStart(c) || isDigit(c) || c == '.' || c == '-' }
