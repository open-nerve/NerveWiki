package obsidian

import (
	"bytes"
	"cmp"
	"regexp"
	"slices"
	"strings"

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
	var ranges linkRanges
	return extractWithRanges(t, values, &ranges)
}

// extractWithRanges keeps the range state local to this extraction. The
// supplied state also lets tests observe whether the real walk needed a map.
func extractWithRanges(t markdown.Tree, values parser.Parser, ranges *linkRanges) Extracted {
	var out Extracted
	var blocks [][]Link
	linkCount := 0
	if !ranges.started && ranges.seen == nil {
		if count, ordered := orderedBodyLinks(t); ordered && count > 0 {
			out.Links = make([]Link, 0, count)
		}
	}
	add := func(l Link) {
		if ranges.first(l.Range, blocks, out.Links) {
			// Keep completed blocks instead of repeatedly copying a growing
			// full slice. The final facts below still own one exact slice.
			if len(out.Links) == cap(out.Links) {
				if len(out.Links) > 0 {
					blocks = append(blocks, out.Links)
				}
				out.Links = make([]Link, 0, min(max(1, 2*cap(out.Links)), 256))
			}
			out.Links = append(out.Links, l)
			linkCount++
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
		if p, ok := property(values, s); ok {
			add(p.Link)
		}
	}
	// The facts outlive the parse: retain no staging blocks or excess
	// capacity (M6/P2 fix check 3 M1). Preserve the original visit order
	// before the stable sort, including equal starts with different ends.
	if len(blocks) > 0 || len(out.Links) != cap(out.Links) {
		links := make([]Link, linkCount)
		at := 0
		for _, block := range blocks {
			at += copy(links[at:], block)
		}
		copy(links[at:], out.Links)
		out.Links = links
	}
	slices.SortStableFunc(out.Links, func(a, b Link) int { return cmp.Compare(a.Range.Start, b.Range.Start) })
	slices.SortStableFunc(out.Tags, func(a, b Tag) int { return cmp.Compare(a.Range.Start, b.Range.Start) })
	out.Tags = slices.Clone(out.Tags)
	return out
}

// orderedBodyLinks counts without constructing facts or decoding their targets.
// Properties and any unproven body retain the bounded staging path. Strictly
// increasing starts prove that none of these complete ranges can be repeated.
func orderedBodyLinks(t markdown.Tree) (int, bool) {
	if len(t.Frontmatter.Scalars) != 0 {
		return 0, false
	}
	count, last := 0, 0
	ordered := true
	_ = ast.Walk(t.Root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		status := ast.WalkContinue
		var start int
		switch n := n.(type) {
		case *wikilink:
			if n.target == "" {
				ordered = false
				return ast.WalkStop, nil
			}
			start, status = n.at.Start, ast.WalkSkipChildren
		case *tag:
			return ast.WalkSkipChildren, nil
		case *ast.Link, *ast.Image:
			at, found := t.Destination(n)
			if !found || at.Start < 0 || at.Stop < at.Start || at.Stop > len(t.Content) {
				ordered = false
				return ast.WalkStop, nil
			}
			written := t.Content[at.Start:at.Stop]
			var end int
			if bytes.IndexByte(written, ':') < 0 {
				end = markdownPathEnd(string(written))
			} else {
				end = markdownTargetEnd(string(written))
			}
			if end < 0 {
				ordered = false
				return ast.WalkStop, nil
			}
			start = at.Start
			if _, image := n.(*ast.Image); image {
				status = ast.WalkSkipChildren
			}
		default:
			return ast.WalkContinue, nil
		}
		if count > 0 && start <= last {
			ordered = false
			return ast.WalkStop, nil
		}
		count++
		last = start
		return status, nil
	})
	return count, ordered
}

// linkRanges accepts the first occurrence of each complete range. A strictly
// increasing start cannot repeat a previous range. On the first other start,
// all accepted ranges enter the map, which handles the rest of the extraction.
// Nothing assumes that an AST walk or frontmatter visits ranges in source order.
type linkRanges struct {
	seen    map[markdown.Span]bool
	last    int
	started bool
}

func (r *linkRanges) first(at markdown.Span, blocks [][]Link, tail []Link) bool {
	if r.seen == nil && (!r.started || at.Start > r.last) {
		r.last, r.started = at.Start, true
		return true
	}
	if r.seen == nil {
		r.seen = make(map[markdown.Span]bool)
		for _, block := range blocks {
			for _, l := range block {
				r.seen[l.Range] = true
			}
		}
		for _, l := range tail {
			r.seen[l.Range] = true
		}
	}
	if r.seen[at] {
		return false
	}
	r.seen[at] = true
	return true
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
	end := markdownTargetEnd(written)
	if end < 0 {
		return Link{}, false
	}
	target, anchor := written[:end], ""
	if end < len(written) {
		anchor = written[end+1:]
	}
	return Link{
		Kind: kind, Target: markdown.DecodeURI(target), Anchor: markdown.DecodeURI(anchor),
		Range: markdown.Span{Start: at, Stop: at + len(target)},
	}, true
}

// markdownTargetEnd is the shared qualification of a written destination,
// before URI decoding. A negative end means that it relates no notebook page.
func markdownTargetEnd(written string) int {
	if strings.Contains(written, ":") && scheme.MatchString(written) {
		return -1
	}
	return markdownPathEnd(written)
}

// markdownPathEnd qualifies the path part without retaining written. Callers
// either checked the scheme or proved that written contains no colon.
func markdownPathEnd(written string) int {
	if strings.HasPrefix(written, "//") {
		return -1
	}
	end := strings.IndexByte(written, '#')
	if end < 0 {
		end = len(written)
	}
	if end == 0 {
		return -1
	}
	return end
}

// propertyLink is a property link, its node and the value it was parsed
// from.
type propertyLink struct {
	Link
	node  ast.Node
	value []byte
}

// shown is the text the link shows (M6/P6 design 4): a wikilink's as the
// body's shows, a Markdown link's text.
func (p propertyLink) shown() string {
	if w, ok := p.node.(*wikilink); ok {
		return w.shown()
	}
	return markdown.ShownText(p.node, p.value)
}

// property is the link the scalar s is, if its whole value is one wikilink,
// not an embed, or one Markdown link, not an image (rule 10). values
// parses the value as the body is parsed; the link's range is where the
// content writes it, through s's offsets.
func property(values parser.Parser, s markdown.Scalar) (propertyLink, bool) {
	// Either link starts with '[': most values are parsed no further.
	if !strings.HasPrefix(s.Value, "[") || strings.TrimSpace(s.Value) != s.Value {
		return propertyLink{}, false
	}
	value := []byte(s.Value)
	pc := parser.NewContext()
	root := values.Parse(text.NewReader(value), parser.WithContext(pc))
	p := root.FirstChild()
	if p == nil || p != root.LastChild() || p.Kind() != ast.KindParagraph || p.FirstChild() != p.LastChild() {
		return propertyLink{}, false
	}
	var l Link
	var ok bool
	switch n := p.FirstChild().(type) {
	case *wikilink: // not an embed: the value starts with '['
		l, ok = n.link()
	case *ast.Link:
		at, found := harden.Destinations(pc)(n)
		if !found {
			return propertyLink{}, false
		}
		l, ok = markdownLink(KindLink, s.Value[at.Start:at.Stop], at.Start)
	}
	if !ok {
		return propertyLink{}, false
	}
	l.Key = s.Path
	l.Range = markdown.Span{Start: s.Offset(l.Range.Start), Stop: s.Offset(l.Range.Stop)}
	return propertyLink{Link: l, node: p.FirstChild(), value: value}, true
}
