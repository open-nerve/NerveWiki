package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// language is what a code block's info string may give its class.
var language = regexp.MustCompile(`^[a-z0-9_+#.-]{1,32}$`)

// marks renders what goldmark's HTML renderer would render unsafely, or
// not as a reading view needs (M4/P3 design 3.6): every address through
// SafeURL, an image as a link, a code block's language only if it is one,
// and raw HTML only after the sanitizer. A Render has its own.
type marks struct {
	links int // the links being rendered around the node
}

func (m *marks) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, m.link)
	reg.Register(ast.KindAutoLink, autoLink)
	reg.Register(ast.KindImage, m.image)
	reg.Register(ast.KindFencedCodeBlock, fencedCode)
	reg.Register(kindSafeHTML, safe)
	reg.Register(kindSafeBlock, safe)
}

func (m *marks) link(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if entering {
		m.links++
	} else {
		m.links--
	}
	href, ok := SafeURL(string(util.URLEscape(n.Destination, true)))
	if !ok {
		return ast.WalkContinue, nil // the text alone
	}
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML([]byte(href)))
	_ = w.WriteByte('"')
	if n.Title != nil {
		_, _ = w.WriteString(` title="`)
		gmhtml.DefaultWriter.Write(w, n.Title)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	return ast.WalkContinue, nil
}

func autoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}
	label := util.EscapeHTML(n.Label(source))
	url := util.URLEscape(n.URL(source), false)
	if n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(bytes.ToLower(url), []byte("mailto:")) {
		url = append([]byte("mailto:"), url...)
	}
	href, ok := SafeURL(string(url))
	if !ok {
		_, _ = w.Write(label)
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML([]byte(href)))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(label)
	_, _ = w.WriteString("</a>")
	return ast.WalkContinue, nil
}

// image is never an <img>: the reading view loads nothing from elsewhere,
// and nothing from here before M7's attachments (M4 design 4, "external
// images"). It is its text and a link to its address, or its text alone
// when the address is not allowed or the image is in a link already.
func (m *marks) image(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := util.EscapeHTML([]byte(plainText(n, source)))
	_, _ = w.WriteString(`<span class="nw-image">`)
	_, _ = w.Write(alt)
	href, ok := SafeURL(string(util.URLEscape(n.Destination, true)))
	if ok && m.links == 0 {
		if len(alt) > 0 {
			_ = w.WriteByte(' ')
		}
		esc := util.EscapeHTML([]byte(href))
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(esc)
		_, _ = w.WriteString(`">`)
		_, _ = w.Write(esc)
		_, _ = w.WriteString("</a>")
	}
	_, _ = w.WriteString("</span>")
	return ast.WalkSkipChildren, nil
}

// fencedCode gives the block its language's class when the info string
// starts with one; the highlighting is the front end's.
func fencedCode(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.FencedCodeBlock)
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<pre><code")
	if lang := bytes.ToLower(n.Language(source)); language.Match(lang) {
		_, _ = w.WriteString(` class="language-`)
		_, _ = w.Write(lang)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		gmhtml.DefaultWriter.RawWrite(w, line.Value(source))
	}
	return ast.WalkContinue, nil
}

func safe(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		switch n := node.(type) {
		case *safeHTML:
			_, _ = w.Write(n.html)
		case *safeBlock:
			_, _ = w.Write(n.html)
		}
	}
	return ast.WalkContinue, nil
}
