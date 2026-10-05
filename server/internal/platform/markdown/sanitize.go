package markdown

import (
	"bytes"
	"iter"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
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
// Markdown between too. A link holds no link (M6/P3B review L1): a user's
// <a> is dropped in a Markdown link, in a user's <a>, and where a node
// that renders a link comes before its end tag in its scope.
func sanitize(root ast.Node, source []byte) {
	links := 0                     // the links around n, n among them, a user's <a> open around them too
	var linked map[ast.Node]bool   // the nodes that render or hold a link, once raw HTML needs them
	inUsers := map[ast.Node]bool{} // the nodes a user's <a> is open around
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n.Kind() == ast.KindLink || inUsers[n] {
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
					if linked == nil {
						linked = linksIn(root)
					}
					s = &scope{inLink: links > 0, holding: holding(c, source, linked)}
				}
				n.ReplaceChild(n, c, &safeHTML{html: s.writeInline(c.Segments.Value(source), s.holding[c])})
			case *ast.HTMLBlock:
				raw := c.Lines().Value(source)
				if c.HasClosure() {
					raw = append(raw, c.ClosureLine.Value(source)...)
				}
				n.ReplaceChild(n, c, &safeBlock{html: (&scope{block: true}).writeBlock(raw)})
			default:
				switch {
				case s == nil:
				case s.skip != "":
					n.RemoveChild(n, c)
				case s.opened["a"] > 0:
					inUsers[c] = true
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

// rendersLink tells whether n renders as a link: a Markdown link, an
// autolink, an image (its address), a footnote's reference or back link, a
// Linker.
func rendersLink(n ast.Node) bool {
	switch n.Kind() {
	case ast.KindLink, ast.KindAutoLink, ast.KindImage, east.KindFootnoteLink, east.KindFootnoteBacklink:
		return true
	}
	_, ok := n.(Linker)
	return ok
}

// linksIn is the nodes of root's tree that render a link or hold one.
func linksIn(root ast.Node) map[ast.Node]bool {
	out := map[ast.Node]bool{}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering && (out[n] || rendersLink(n)) {
			out[n] = true
			if p := n.Parent(); p != nil {
				out[p] = true
			}
		}
		return ast.WalkContinue, nil
	})
	return out
}

// holding is, of the user's <a> start tags among first and the siblings
// after it, those after which a sibling renders or holds a link before an
// </a> that ends it: one look ahead over the tags, for the </a> a dropped
// element holds, which ends nothing (scope.start, scope.end; P3B fix
// check), and one look back over the siblings.
func holding(first ast.Node, source []byte, linked map[ast.Node]bool) map[ast.Node]bool {
	held := map[ast.Node]bool{} // the </a> a dropped element holds
	skip := ""
	for c := first; c != nil; c = c.NextSibling() {
		if raw, ok := c.(*ast.RawHTML); ok {
			switch name, end, _ := readTag(raw.Segments.Value(source)); {
			case skip == "" && !end && dropped[name]:
				skip = name
			case skip != "" && end && name == skip:
				skip = ""
			case skip != "" && end && name == "a":
				held[c] = true
			}
		}
	}
	out := map[ast.Node]bool{}
	link := false // a link comes before the next </a>
	for c := first.Parent().LastChild(); c != nil; c = c.PreviousSibling() {
		if raw, ok := c.(*ast.RawHTML); ok {
			switch name, end, _ := readTag(raw.Segments.Value(source)); {
			case name == "a" && end && !held[c]:
				link = false
			case name == "a" && link:
				out[c] = true
			}
		} else if linked[c] {
			link = true
		}
		if c == first {
			break
		}
	}
	return out
}

// scope is the elements open in one scope.
type scope struct {
	block   bool              // flow elements are allowed: an HTML block
	inLink  bool              // a user's <a> would nest in a link
	holding map[ast.Node]bool // the user's <a> start tags before whose end a link comes
	out     bytes.Buffer
	open    []string
	opened  map[string]int // how many of each name are open
	skip    string         // the dropped element whose content is being dropped
}

// writeInline is one inline raw HTML node as the allowlist keeps it. goldmark
// makes the node of one start or end tag, comment, processing instruction,
// declaration or CDATA section as CommonMark defines them, so it is read
// here rather than by a tokenizer: one for each of thousands of tags would
// cost far more than the tags.
// A start tag of a link that holds one is dropped.
func (s *scope) writeInline(raw []byte, holdsLink bool) []byte {
	s.out.Reset()
	if name, end, attrs := readTag(raw); name != "" {
		switch {
		case end:
			s.end(name)
		case name != "a" || !holdsLink:
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
	if tag == "a" && (s.inLink || s.opened["a"] > 0) {
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
