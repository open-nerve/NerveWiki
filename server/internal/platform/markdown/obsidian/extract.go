package obsidian

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Extracted is what a page's content says of its links and tags, in the
// content's order (M6/P1 design 3.11).
type Extracted struct {
	Links []Link
	Tags  []Tag
}

// Kind is how a link is written.
type Kind string

// The kinds of link.
const (
	KindWikilink Kind = "wikilink" // [[…]]
	KindEmbed    Kind = "embed"    // ![[…]]
	KindLink     Kind = "link"     // a Markdown link
	KindImage    Kind = "image"    // a Markdown image
)

// Link is one link of a page, as the fixture set writes it: Target is what
// resolves (a wikilink's trimmed, a Markdown link's decoded), Anchor what
// follows its '#', Display a wikilink's display text, Key the path of the
// property it is the value of, and Range where its target is written, the
// bytes a rename rewrites. An empty field is none. InTable tells a wikilink
// in a table's cell, where a display text a rename adds follows "\|", as
// Obsidian writes it there (M6/P4 design 3.1).
type Link struct {
	Kind    Kind
	Target  string
	Anchor  string
	Display string
	Key     string
	Range   markdown.Span
	InTable bool
}

// Tag is one tag of the body: its name as written, and where it is written,
// '#' and name.
type Tag struct {
	Name  string
	Range markdown.Span
}

// extract is the links and tags of t (rules 4, 7–10): those of the body,
// a Markdown image's caption left out, then those of the frontmatter's
// properties, each read by values. A reference definition two links use is
// one link.
func extract(t markdown.Tree, values parser.Parser) any {
	var out Extracted
	seen := map[markdown.Span]bool{}
	add := func(l Link) {
		if !seen[l.Range] {
			seen[l.Range] = true
			out.Links = append(out.Links, l)
		}
	}
	_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *wikilink:
			if l, ok := n.link(); ok {
				add(l)
			}
			return ast.WalkSkipChildren, nil
		case *tag:
			out.Tags = append(out.Tags, Tag{Name: n.name, Range: markdown.Span{Start: n.seg.Start, Stop: n.seg.Stop}})
			return ast.WalkSkipChildren, nil
		case *ast.Link:
			if l, ok := destination(t, n, KindLink); ok {
				add(l)
			}
		case *ast.Image:
			if l, ok := destination(t, n, KindImage); ok {
				add(l)
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	for _, s := range t.Frontmatter.Scalars {
		if l, _, ok := property(values, s); ok {
			add(l)
		}
	}
	slices.SortStableFunc(out.Links, func(a, b Link) int { return cmp.Compare(a.Range.Start, b.Range.Start) })
	slices.SortStableFunc(out.Tags, func(a, b Tag) int { return cmp.Compare(a.Range.Start, b.Range.Start) })
	// The facts outlive the parse: each slice as long as what it holds, not
	// the twice as long that appending may leave (M6/P2 fix check 3 M1).
	out.Links, out.Tags = slices.Clone(out.Links), slices.Clone(out.Tags)
	return out
}

// link is the link w makes, none if it has no target: one to its own page's
// anchor relates no pages (rule 7).
func (w *wikilink) link() (Link, bool) {
	if w.target == "" {
		return Link{}, false
	}
	kind := KindWikilink
	if w.embed {
		kind = KindEmbed
	}
	return Link{Kind: kind, Target: w.target, Anchor: w.anchor, Display: w.display, Range: w.at, InTable: w.inTable}, true
}

// destination is the link n, a Markdown link or image, makes: its
// destination where it is written.
func destination(t markdown.Tree, n ast.Node, kind Kind) (Link, bool) {
	at, ok := t.Destination(n)
	if !ok {
		return Link{}, false
	}
	return markdownLink(kind, string(t.Content[at.Start:at.Stop]), at.Start)
}

// scheme starts an address that is not a path: a URI's scheme.
var scheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// markdownLink is the link a Markdown link's destination makes, written at
// at (rule 8): its target and anchor around the first '#', each decoded as
// JavaScript's decodeURI does; none for an address elsewhere (a scheme, or
// "//") or an anchor alone.
func markdownLink(kind Kind, written string, at int) (Link, bool) {
	if scheme.MatchString(written) || strings.HasPrefix(written, "//") {
		return Link{}, false
	}
	target, anchor, _ := strings.Cut(written, "#")
	if target == "" {
		return Link{}, false
	}
	return Link{
		Kind: kind, Target: decodeURI(target), Anchor: decodeURI(anchor),
		Range: markdown.Span{Start: at, Stop: at + len(target)},
	}, true
}

// property is the link the scalar s is, if its whole value is one wikilink,
// not an embed, or one Markdown link, not an image (rule 10), and the text
// it shows: a wikilink's as the body's shows, a Markdown link's text.
// values parses the value as the body is parsed; the link's range is where
// the content writes it, through s's offsets.
func property(values parser.Parser, s markdown.Scalar) (Link, string, bool) {
	// Either link starts with '[': most values are parsed no further.
	if !strings.HasPrefix(s.Value, "[") || strings.TrimSpace(s.Value) != s.Value {
		return Link{}, "", false
	}
	value := []byte(s.Value)
	pc := parser.NewContext()
	root := values.Parse(text.NewReader(value), parser.WithContext(pc))
	p := root.FirstChild()
	if p == nil || p != root.LastChild() || p.Kind() != ast.KindParagraph || p.FirstChild() != p.LastChild() {
		return Link{}, "", false
	}
	var l Link
	var shown string
	var ok bool
	switch n := p.FirstChild().(type) {
	case *wikilink: // not an embed: the value starts with '['
		l, ok = n.link()
		shown = n.shown()
	case *ast.Link:
		at, found := harden.Destinations(pc)(n)
		if !found {
			return Link{}, "", false
		}
		l, ok = markdownLink(KindLink, s.Value[at.Start:at.Stop], at.Start)
		shown = markdown.PlainText(n, value)
	}
	if !ok {
		return Link{}, "", false
	}
	l.Key = s.Path
	l.Range = markdown.Span{Start: s.Offset(l.Range.Start), Stop: s.Offset(l.Range.Stop)}
	return l, shown, true
}

// reserved are the characters whose escapes decodeURI keeps.
const reserved = ";/?:@&=+$,#"

// decodeURI is JavaScript's decodeURI: each %XX decoded, an escaped
// reserved character's kept as it is; s as it is if an escape is malformed
// or its bytes are not UTF-8.
func decodeURI(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '%' {
			b.WriteByte(s[i])
			i++
			continue
		}
		c, ok := unhex(s, i)
		if !ok {
			return s
		}
		if c < utf8.RuneSelf {
			if strings.IndexByte(reserved, c) >= 0 {
				b.WriteString(s[i : i+3])
			} else {
				b.WriteByte(c)
			}
			i += 3
			continue
		}
		var size int
		switch {
		case c&0xE0 == 0xC0:
			size = 2
		case c&0xF0 == 0xE0:
			size = 3
		case c&0xF8 == 0xF0:
			size = 4
		default:
			return s
		}
		bs := []byte{c}
		for k := 1; k < size; k++ {
			c, ok := unhex(s, i+3*k)
			if !ok || c&0xC0 != 0x80 {
				return s
			}
			bs = append(bs, c)
		}
		if r, n := utf8.DecodeRune(bs); r == utf8.RuneError && n <= 1 {
			return s
		}
		b.Write(bs)
		i += 3 * size
	}
	return b.String()
}

// unhex is the byte the escape %XX at s[i] writes.
func unhex(s string, i int) (byte, bool) {
	if i+2 >= len(s) || s[i] != '%' {
		return 0, false
	}
	hi, ok1 := hexDigit(s[i+1])
	lo, ok2 := hexDigit(s[i+2])
	return hi<<4 | lo, ok1 && ok2
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
