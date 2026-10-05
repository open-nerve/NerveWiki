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
	if heading, ok := markdown.AnchorID(anchor); ok {
		attrs = append(attrs, markdown.Attr{Name: "data-nw-anchor", Value: heading})
	}
	return attrs, true
}

// markdownAttrs is how the Markdown link or image whose destination starts
// at start is written, if it is one of the page's links: its state in place
// of its address, which the front end gives.
func (v view) markdownAttrs(start int) ([]markdown.Attr, bool) {
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

// property is how the property table writes the frontmatter's string s
// (M6/P6 design 4): the property link it is, if it is one, as the body
// writes its kind, a wikilink or a Markdown link, showing what the link
// shows; values parses s as the extraction did.
func (v view) property(values parser.Parser) func(s markdown.Scalar) ([]markdown.Attr, string, bool) {
	return func(s markdown.Scalar) ([]markdown.Attr, string, bool) {
		p, ok := property(values, s)
		if !ok {
			return nil, "", false
		}
		attrs, resolved := v.lead(p.Range.Start, p.Target, p.Anchor)
		var class []string
		if p.Kind == KindWikilink {
			class = append(class, "nw-wikilink")
		}
		if !resolved {
			class = append(class, "nw-unresolved")
		}
		if len(class) > 0 {
			attrs = append([]markdown.Attr{{Name: "class", Value: strings.Join(class, " ")}}, attrs...)
		}
		return attrs, p.shown(), true
	}
}

// inLinks marks the wikilinks and the tags in a Markdown link's text, which
// render as the text they show: a link holds no link (M6/P3 design 6.2,
// M6/P6 design 3). The parse's own "within a link's brackets" would not
// do: the brackets may turn out to be no link. A Markdown link whose
// address is not let through is its text alone, its wikilinks and tags
// too (P3B review L4).
type inLinks struct{}

func (inLinks) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
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
		case *tag:
			if entering && links > 0 {
				n.inLink = true
			}
		}
		return ast.WalkContinue, nil
	})
}
