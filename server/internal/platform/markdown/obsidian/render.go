package obsidian

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// nodeRenderer renders the dialect's nodes (M6/P1 design 3.10). A wikilink,
// an embed and a tag are spans with no state until P3 gives them their
// pages; each text is escaped, and no address is written.
type nodeRenderer struct{}

func (nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikilink, renderWikilink)
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

// renderWikilink writes the span of a wikilink or an embed around the text
// it shows, its child; data-nw-target is its target and anchor.
func renderWikilink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</span>")
		return ast.WalkContinue, nil
	}
	n := node.(*wikilink)
	_, _ = w.WriteString(`<span class="nw-wikilink`)
	if n.embed {
		_, _ = w.WriteString(" nw-embed")
	}
	_, _ = w.WriteString(`" data-nw-target="`)
	target := n.target
	if n.anchor != "" {
		target += "#" + n.anchor
	}
	escaped(w, []byte(target))
	_, _ = w.WriteString(`">`)
	return ast.WalkContinue, nil
}

// renderTag writes the span of a tag around its text, its child.
func renderTag(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</span>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<span class="nw-tag" data-nw-tag="`)
	escaped(w, []byte(node.(*tag).name))
	_, _ = w.WriteString(`">`)
	return ast.WalkContinue, nil
}

// renderMath writes an inline formula's TeX, for the front end to typeset:
// a display one ($$…$$) as a block's.
func renderMath(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if node.(*math).display {
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
func renderCalloutTitle(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
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
	if !n.HasChildren() {
		escaped(w, []byte(n.of.standing()))
	}
	return ast.WalkContinue, nil
}
