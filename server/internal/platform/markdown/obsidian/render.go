package obsidian

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// nodeRenderer renders the dialect's nodes (M6/P1 design 3.10), a
// wikilink and an embed as a link with the state view has of it (M6/P3
// design 6.2); each text is escaped.
type nodeRenderer struct{ view view }

func (r nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikilink, r.renderWikilink)
	reg.Register(kindTag, renderTag)
	reg.Register(harden.KindHighlight, element("<mark>", "</mark>"))
	reg.Register(kindMath, renderMath)
	reg.Register(kindMathBlock, renderMathBlock)
	reg.Register(kindMarker, nothing)
	reg.Register(kindHidden, nothing)
	reg.Register(kindHiddenBlocks, nothing)
	reg.Register(kindCallout, renderCallout)
	reg.Register(kindCalloutTitle, renderCalloutTitle)
}

// escaped writes b, HTML's special characters escaped.
func escaped(w util.BufWriter, b []byte) { _, _ = w.Write(util.EscapeHTML(b)) }

// element writes open before a node's children and closeTag after them.
func element(open, closeTag string) renderer.NodeRendererFunc {
	return func(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString(open)
		} else {
			_, _ = w.WriteString(closeTag)
		}
		return ast.WalkContinue, nil
	}
}

// nothing writes nothing of a node and its children: a comment's marker, or
// what a comment hides.
func nothing(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}

// renderWikilink writes a wikilink or an embed around the text it shows,
// its child (M6/P3 design 6.2): a link to the page it resolves to, with
// the heading its anchor leads to; one unresolved, with its target; one
// to its own page's heading, to the heading's id. One in a Markdown
// link's text, or to its own page's block, is a span.
func (r nodeRenderer) renderWikilink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*wikilink)
	heading, own := "", n.target == ""
	if own {
		heading, own = anchorID(n.anchor)
	}
	link := !n.inLink && (n.target != "" || own)
	if !entering {
		if link {
			_, _ = w.WriteString("</a>")
		} else {
			_, _ = w.WriteString("</span>")
		}
		return ast.WalkContinue, nil
	}
	class := "nw-wikilink"
	if n.embed {
		class += " nw-embed"
	}
	var attrs []markdown.Attr
	switch {
	case !link:
	case own:
		attrs = []markdown.Attr{{Name: "href", Value: "#" + heading}}
	default:
		var resolved bool
		attrs, resolved = r.view.lead(n.at.Start, n.target, n.anchor)
		if !resolved {
			class += " nw-unresolved"
		}
	}
	if link {
		_, _ = w.WriteString(`<a class="`)
	} else {
		_, _ = w.WriteString(`<span class="`)
	}
	_, _ = w.WriteString(class)
	_ = w.WriteByte('"')
	markdown.WriteAttrs(w, attrs)
	_ = w.WriteByte('>')
	return ast.WalkContinue, nil
}

// renderTag writes a tag around its text, its child (M6/P6 design 3): one
// Obsidian's tag pane counts, as a link to the pages with it, by the name
// the pane counts it as; one it does not count, or one in a Markdown
// link's text, as a span with its name.
func renderTag(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	t := node.(*tag)
	counted, ok := CountedTag(t.name)
	link := ok && !t.inLink
	switch {
	case !entering && link:
		_, _ = w.WriteString("</a>")
	case !entering:
		_, _ = w.WriteString("</span>")
	case link:
		_, _ = w.WriteString(`<a class="nw-tag" data-nw-tag="`)
		escaped(w, []byte(counted))
		_, _ = w.WriteString(`">`)
	default:
		_, _ = w.WriteString(`<span class="nw-tag" data-nw-tag="`)
		escaped(w, []byte(t.name))
		_, _ = w.WriteString(`">`)
	}
	return ast.WalkContinue, nil
}

// renderMath writes an inline formula's TeX, for the front end to typeset:
// a display one ($$…$$) as a block's.
func renderMath(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if node.(*inlineMath).display {
		_, _ = w.WriteString(`<span class="nw-math nw-math-block">`)
	} else {
		_, _ = w.WriteString(`<span class="nw-math">`)
	}
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			escaped(w, t.Segment.Value(source))
		}
	}
	_, _ = w.WriteString("</span>")
	return ast.WalkSkipChildren, nil
}

// renderMathBlock writes a block formula's TeX.
func renderMathBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<div class="nw-math nw-math-block">`)
	lines := node.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		escaped(w, seg.Value(source))
	}
	_, _ = w.WriteString("</div>\n")
	return ast.WalkSkipChildren, nil
}

// renderCallout writes a callout: a details element when it folds, open
// if '+' folds it, else a div.
func renderCallout(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*callout)
	if !entering {
		if n.fold != 0 {
			_, _ = w.WriteString("</details>\n")
		} else {
			_, _ = w.WriteString("</div>\n")
		}
		return ast.WalkContinue, nil
	}
	if n.fold != 0 {
		_, _ = w.WriteString(`<details class="nw-callout" data-callout="`)
	} else {
		_, _ = w.WriteString(`<div class="nw-callout" data-callout="`)
	}
	escaped(w, []byte(n.kind))
	_ = w.WriteByte('"')
	if n.fold == '+' {
		_, _ = w.WriteString(` open=""`)
	}
	_, _ = w.WriteString(">\n")
	return ast.WalkContinue, nil
}

// renderCalloutTitle writes a callout's title, its type's when its line
// has none: the summary of a details element, else a div.
func renderCalloutTitle(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*calloutTitle)
	folds := n.of.fold != 0
	if !entering {
		if folds {
			_, _ = w.WriteString("</summary>\n")
		} else {
			_, _ = w.WriteString("</div>\n")
		}
		return ast.WalkContinue, nil
	}
	if folds {
		_, _ = w.WriteString("<summary>")
	} else {
		_, _ = w.WriteString(`<div class="nw-callout-title">`)
	}
	if !shows(n, source) {
		escaped(w, []byte(n.of.standing()))
	}
	return ast.WalkContinue, nil
}

// shows tells whether n holds something that shows: neither what hides
// nor a blank text, such as the one that ended a callout's line or the
// spaces between two comments.
func shows(n ast.Node, source []byte) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case markdown.Hider:
		case *ast.Text:
			if !util.IsBlank(c.Segment.Value(source)) {
				return true
			}
		default:
			return true
		}
	}
	return false
}
