package obsidian

import (
	"context"
	"strings"
	"time"
	"uuid"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
)

// view is where the links of a page lead, as Fetch found for its reading
// view (M6/P3 design 6.4): by where each link's target starts, the link,
// and the node it resolves to; what the attachments among them show
// (M7/P3 design 5.3), and when the earliest of their addresses expires;
// and how many audio and video elements the view has written, which its
// copies share.
type view struct {
	links   map[int]Link
	to      map[int]Target
	assets  map[uuid.UUID]Asset
	expires time.Time
	played  *int
}

// fetch is the view of the page's links, the extracted ones, from Resolve,
// and of the attachments they lead to, each once, from Assets: none is
// read for a page without links, or without Resolve, nor are attachments
// for one whose links lead to none, or without Assets.
func (o Options) fetch(ctx context.Context, page markdown.Page, extracted any) (any, error) {
	ex, _ := extracted.(Extracted)
	v := view{links: make(map[int]Link, len(ex.Links)), played: new(int)}
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
	var ids []uuid.UUID
	seen := map[uuid.UUID]bool{}
	for _, l := range ex.Links {
		if t, ok := v.target(l.Range.Start); ok && t.Asset && !seen[t.Node] {
			seen[t.Node] = true
			ids = append(ids, t.Node)
		}
	}
	if o.Assets == nil || len(ids) == 0 {
		return v, nil
	}
	if v.assets, err = o.Assets(ctx, page.NotebookID, ids); err != nil {
		return nil, err
	}
	for _, a := range v.assets {
		if v.expires.IsZero() || a.Expires.Before(v.expires) {
			v.expires = a.Expires
		}
	}
	return v, nil
}

// target is the node the link whose target starts at start leads to, if
// it leads to one.
func (v view) target(start int) (Target, bool) {
	t, ok := v.to[start]
	return t, ok && t.Node != uuid.Nil()
}

// lead is the attributes of a link to target#anchor whose target starts at
// start, and the class that tells where it leads (M6/P3 design 6.2; M7/P3
// design 5.5): to a page, the page and the heading its anchor leads to,
// and none; to an attachment, its content's address, its id and its size,
// and nw-asset, the class alone for one Assets did not answer, which is
// text; to none, its target, and nw-unresolved. An attachment is never a
// page's data-nw-node.
func (v view) lead(start int, target, anchor string) ([]markdown.Attr, string) {
	t, ok := v.target(start)
	switch {
	case !ok:
		return []markdown.Attr{{Name: "data-nw-target", Value: target}}, "nw-unresolved"
	case t.Asset:
		return v.assetLink(t.Node), "nw-asset"
	}
	attrs := []markdown.Attr{{Name: "data-nw-node", Value: t.Node.String()}}
	if heading, ok := markdown.AnchorID(anchor); ok {
		attrs = append(attrs, markdown.Attr{Name: "data-nw-anchor", Value: heading})
	}
	return attrs, ""
}

// markdownAttrs is how the Markdown link or image whose destination starts
// at start is written, if it is one of the page's links: its state in place
// of its address, which the front end gives, or an attachment's.
func (v view) markdownAttrs(start int) ([]markdown.Attr, bool) {
	l, ok := v.links[start]
	if !ok {
		return nil, false
	}
	attrs, class := v.lead(start, l.Target, l.Anchor)
	if class != "" {
		attrs = append([]markdown.Attr{{Name: "class", Value: class}}, attrs...)
	}
	return attrs, true
}

// property is how the property table writes the frontmatter's string s
// (M6/P6 design 4): the property link it is, if it is one, as the body
// writes its kind, a wikilink or a Markdown link, showing what the link
// shows; values parses s as the extraction did. One to an attachment is a
// link to it, whatever its type (M7/P3 design 5.6).
func (v view) property(values parser.Parser) func(s markdown.Scalar) ([]markdown.Attr, string, bool) {
	return func(s markdown.Scalar) ([]markdown.Attr, string, bool) {
		p, ok := property(values, s)
		if !ok {
			return nil, "", false
		}
		attrs, leads := v.lead(p.Range.Start, p.Target, p.Anchor)
		var class []string
		if p.Kind == KindWikilink {
			class = append(class, "nw-wikilink")
		}
		if leads != "" {
			class = append(class, leads)
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
