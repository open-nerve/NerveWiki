package markdown

import (
	"bytes"
	"iter"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// The typographic allowlist of a user's raw HTML (M4 design 4, "rendering";
// M4/P3 design 3.7). Inline raw HTML may hold the phrasing elements; an HTML
// block, the flow ones too.
//
//nolint:gochecknoglobals // read only
var (
	phrasing = set("a", "abbr", "b", "bdi", "bdo", "br", "cite", "code", "del", "dfn", "em", "i", "ins", "kbd",
		"mark", "q", "rp", "rt", "ruby", "s", "samp", "small", "span", "strong", "sub", "sup", "u", "var", "wbr")
	flow = set("blockquote", "caption", "dd", "details", "div", "dl", "dt", "figcaption", "figure", "h1", "h2",
		"h3", "h4", "h5", "h6", "hr", "li", "ol", "p", "pre", "summary", "table", "tbody", "td", "tfoot", "th",
		"thead", "tr", "ul")
	void = set("br", "hr", "wbr")
	// dropped go with all they hold: what is inside is script, style or
	// text the browser would not show.
	dropped = set("script", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes", "noscript",
		"plaintext")
	number   = regexp.MustCompile(`^-?[0-9]{1,6}$`)
	positive = regexp.MustCompile(`^[0-9]{1,4}$`)
)

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// attribute is an attribute's value as the allowlist takes it, whether it
// takes it, and whether the element may carry the attribute at all.
func attribute(element, name, value string) (kept string, ok, known bool) {
	switch element + " " + name {
	case "a href":
		kept, ok = SafeURL(value)
		return kept, ok, true
	case "a title", "abbr title":
		return value, true, true
	case "bdo dir":
		return value, value == "ltr" || value == "rtl", true
	case "ol start":
		return value, number.MatchString(value), true
	case "td colspan", "td rowspan", "th colspan", "th rowspan":
		return value, positive.MatchString(value), true
	case "ol reversed", "details open":
		return "", true, true
	}
	return "", false, false
}

// safeHTML is raw HTML after the sanitizer, written as it is.
type safeHTML struct {
	ast.BaseInline
	html []byte
}

// safeBlock is an HTML block after the sanitizer.
type safeBlock struct {
	ast.BaseBlock
	html []byte
}

//nolint:gochecknoglobals // kinds are made once, as goldmark's are
var (
	kindSafeHTML  = ast.NewNodeKind("SafeHTML")
	kindSafeBlock = ast.NewNodeKind("SafeHTMLBlock")
)

// Kind implements ast.Node.
func (n *safeHTML) Kind() ast.NodeKind { return kindSafeHTML }

// Dump implements ast.Node.
func (n *safeHTML) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// Kind implements ast.Node.
func (n *safeBlock) Kind() ast.NodeKind { return kindSafeBlock }

// Dump implements ast.Node.
func (n *safeBlock) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// sanitize replaces every raw HTML node of the tree by what the allowlist
// keeps of it. The elements a node opens are closed at the end of the
// innermost node around it, its scope: a paragraph, a heading, a cell, an
// emphasis, a link… (an HTML block is a scope of its own). An end tag with
// no element of its name open in the scope is dropped, and one that has
// closes what was opened after it first. So a user's elements nest in the
// output as they nest in the tree, and none spills into what follows. What
// a dropped element holds is dropped up to its end tag in the scope, the
// Markdown between too.
func sanitize(root ast.Node, source []byte) {
	links := 0 // the links around n, n among them
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n.Kind() == ast.KindLink {
			if entering {
				links++
			} else {
				links--
			}
		}
		if !entering || !n.HasChildren() {
			return ast.WalkContinue, nil
		}
		var s *scope
		for c := n.FirstChild(); c != nil; {
			next := c.NextSibling()
			switch c := c.(type) {
			case *ast.RawHTML:
				if s == nil {
					s = &scope{inLink: links > 0}
				}
				n.ReplaceChild(n, c, &safeHTML{html: s.writeInline(c.Segments.Value(source))})
			case *ast.HTMLBlock:
				raw := c.Lines().Value(source)
				if c.HasClosure() {
					raw = append(raw, c.ClosureLine.Value(source)...)
				}
				n.ReplaceChild(n, c, &safeBlock{html: (&scope{block: true}).writeBlock(raw)})
			default:
				if s != nil && s.skip != "" {
					n.RemoveChild(n, c)
				}
			}
			c = next
		}
		if s != nil && len(s.open) > 0 {
			n.AppendChild(n, &safeHTML{html: s.close()})
		}
		return ast.WalkContinue, nil
	})
}

// scope is the elements open in one scope.
type scope struct {
	block  bool // flow elements are allowed: an HTML block
	inLink bool // a user's <a> would nest in a link
	out    bytes.Buffer
	open   []string
	opened map[string]int // how many of each name are open
	skip   string         // the dropped element whose content is being dropped
}

// writeInline is one inline raw HTML node as the allowlist keeps it. goldmark
// makes the node of one start or end tag, comment, processing instruction,
// declaration or CDATA section as CommonMark defines them, so it is read
// here rather than by a tokenizer: one for each of thousands of tags would
// cost far more than the tags.
func (s *scope) writeInline(raw []byte) []byte {
	s.out.Reset()
	if name, end, attrs := readTag(raw); name != "" {
		if end {
			s.end(name)
		} else {
			s.start(name, attrs)
		}
	}
	return bytes.Clone(s.out.Bytes())
}

// writeBlock is an HTML block as the allowlist keeps it, what it leaves
// open closed.
func (s *scope) writeBlock(raw []byte) []byte {
	z := html.NewTokenizer(bytes.NewReader(raw))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return append(bytes.Clone(s.out.Bytes()), s.close()...)
		case html.TextToken:
			if s.skip == "" {
				s.out.WriteString(html.EscapeString(string(z.Text())))
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, more := z.TagName()
			s.start(string(name), func(yield func(string, string) bool) {
				for more {
					var k, v []byte
					k, v, more = z.TagAttr()
					if !yield(string(k), string(v)) {
						return
					}
				}
			})
		case html.EndTagToken:
			name, _ := z.TagName()
			s.end(string(name))
		}
	}
}

func (s *scope) allows(tag string) bool {
	if tag == "a" && s.inLink {
		return false
	}
	return phrasing[tag] || s.block && flow[tag]
}

// start writes tag with the attributes the allowlist keeps, or starts
// dropping what follows. An attribute counts once, its first value, as a
// browser takes it.
func (s *scope) start(tag string, attrs iter.Seq2[string, string]) {
	switch {
	case s.skip != "":
		return
	case dropped[tag]:
		s.skip = tag
		return
	case !s.allows(tag):
		return
	}
	s.out.WriteString("<" + tag)
	var seen []string // of the known ones: an element has a few
	for name, value := range attrs {
		kept, ok, known := attribute(tag, name, value)
		if !known || slices.Contains(seen, name) {
			continue
		}
		seen = append(seen, name)
		switch {
		case !ok:
		case kept == "" && (name == "reversed" || name == "open"):
			s.out.WriteString(" " + name)
		default:
			s.out.WriteString(" " + name + `="` + html.EscapeString(kept) + `"`)
		}
	}
	s.out.WriteString(">")
	if !void[tag] {
		if s.opened == nil {
			s.opened = map[string]int{}
		}
		s.open = append(s.open, tag)
		s.opened[tag]++
	}
}

// end closes tag, and what was opened after it, if it is open here; or it
// ends the dropping. What it looks through it closes, so each element costs
// one look.
func (s *scope) end(tag string) {
	switch {
	case s.skip == tag:
		s.skip = ""
		return
	case s.skip != "" || s.opened[tag] == 0:
		return
	}
	for {
		last := s.open[len(s.open)-1]
		s.open = s.open[:len(s.open)-1]
		s.opened[last]--
		s.out.WriteString("</" + last + ">")
		if last == tag {
			return
		}
	}
}

// close is the end tags of what is still open, the last opened first.
func (s *scope) close() []byte {
	var b strings.Builder
	for j := len(s.open) - 1; j >= 0; j-- {
		b.WriteString("</" + s.open[j] + ">")
	}
	s.open, s.opened = nil, nil
	return []byte(b.String())
}

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

// attrValue reads an attribute's value after its '=': quoted or not, its
// references resolved, a NUL made U+FFFD as a tokenizer makes it.
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
	return strings.ReplaceAll(html.UnescapeString(string(v)), "\x00", "�"), r
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
func isNameStart(c byte) bool { return isLetter(c) || c == '_' || c == ':' }
func isNameChar(c byte) bool  { return isNameStart(c) || isDigit(c) || c == '.' || c == '-' }
