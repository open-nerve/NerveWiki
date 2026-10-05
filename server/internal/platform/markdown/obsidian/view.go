package obsidian

import (
	"context"
	"strings"
	"uuid"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// view is where the links of a page lead, as Fetch found for its reading
// view (M6/P3 design 6.4): by where each link's target starts, the link,
// and the page it resolves to.
type view struct {
	links map[int]Link
	to    map[int]uuid.UUID
}

// fetch is the view of the page's links, the extracted ones, from Resolve:
// none is read for a page without links, or without Resolve.
func (o Options) fetch(ctx context.Context, page markdown.Page, extracted any) (any, error) {
	ex, _ := extracted.(Extracted)
	v := view{links: make(map[int]Link, len(ex.Links))}
	for _, l := range ex.Links {
		v.links[l.Range.Start] = l
	}
	if o.Resolve == nil || len(ex.Links) == 0 {
		return v, nil
	}
	to, err := o.Resolve(ctx, page, ex.Links)
	if err != nil {
		return nil, err
	}
	v.to = to
	return v, nil
}

// lead is the attributes of a link to target#anchor whose target starts at
// start (M6/P3 design 6.2): the page it resolves to and the heading its
// anchor leads to, and true; or its target, and false for none.
func (v view) lead(start int, target, anchor string) ([]markdown.Attr, bool) {
	id, ok := v.to[start]
	if !ok || id == uuid.Nil() {
		return []markdown.Attr{{Name: "data-nw-target", Value: target}}, false
	}
	attrs := []markdown.Attr{{Name: "data-nw-node", Value: id.String()}}
	if heading, ok := anchorID(anchor); ok {
		attrs = append(attrs, markdown.Attr{Name: "data-nw-anchor", Value: heading})
	}
	return attrs, true
}

// markdownLink is how the Markdown link or image whose destination starts
// at start is written, if it is one of the page's links: its state in place
// of its address, which the front end gives.
func (v view) markdownLink(start int) ([]markdown.Attr, bool) {
	l, ok := v.links[start]
	if !ok {
		return nil, false
	}
	attrs, resolved := v.lead(start, l.Target, l.Anchor)
	if !resolved {
		attrs = append([]markdown.Attr{{Name: "class", Value: "nw-unresolved"}}, attrs...)
	}
	return attrs, true
}

// anchorID is the id of the heading an anchor leads to (M6/P3 design 6.2):
// that of the first heading of its last part's text, H2 of H1#H2. A block's
// anchor (^…) has none, v0.1 giving blocks no id, and an empty one none.
func anchorID(anchor string) (string, bool) {
	if k := strings.LastIndexByte(anchor, '#'); k >= 0 {
		anchor = anchor[k+1:]
	}
	anchor = strings.Trim(anchor, " \t")
	if anchor == "" || anchor[0] == '^' {
		return "", false
	}
	return markdown.HeadingID(anchor), true
}

// linkedWikilinks marks the wikilinks in a Markdown link's text, which
// render as the text they show: a link holds no link (M6/P3 design 6.2).
// The parse's own "within a link's brackets" would not do: the brackets
// may turn out to be no link.
type linkedWikilinks struct{}

func (linkedWikilinks) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	links := 0
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Link:
			if entering {
				links++
			} else {
				links--
			}
		case *wikilink:
			if entering && links > 0 {
				n.inLink = true
			}
		}
		return ast.WalkContinue, nil
	})
}
