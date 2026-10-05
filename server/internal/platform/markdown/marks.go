package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// language is what a code block's info string may give its class.
var language = regexp.MustCompile(`^[a-z0-9_+#.-]{1,32}$`)

// marks renders what goldmark's HTML renderer would render unsafely, or
// not as a reading view needs (M4/P3 design 3.6): every address through
// SafeURL, an image as a link, a code block's language only if it is one,
// and raw HTML only after the sanitizer; a link or an image an extension
// knows with the attributes its Links gives in place of its address. A
// Render has its own.
type marks struct {
	links int // the links being rendered around the node
	// destinations is where the parse found the destinations written, and
	// written the extensions' Links, in their order.
	destinations func(ast.Node) (text.Segment, bool)
	written      []func(start int) ([]Attr, bool)
}

// known is the attributes an extension's Links has n, a Markdown link or
// image, carry in place of its address, if one knows n.
func (m *marks) known(n ast.Node) ([]Attr, bool) {
	if len(m.written) == 0 || m.destinations == nil {
		return nil, false
	}
	at, ok := m.destinations(n)
	if !ok {
		return nil, false
	}
	for _, f := range m.written {
		if attrs, ok := f(at.Start); ok {
			return attrs, true
		}
	}
	return nil, false
}

// writeAttrs writes attrs, each value escaped.
func writeAttrs(w util.BufWriter, attrs []Attr) {
	for _, a := range attrs {
		_ = w.WriteByte(' ')
		_, _ = w.WriteString(a.Name)
		_, _ = w.WriteString(`="`)
		_, _ = w.Write(util.EscapeHTML([]byte(a.Value)))
		_ = w.WriteByte('"')
	}
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
	attrs, known := m.known(n)
	href, ok := SafeURL(string(util.URLEscape(n.Destination, true)))
	if !known && !ok {
		return ast.WalkContinue, nil // the text alone
	}
	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}
	if known {
		_, _ = w.WriteString("<a")
		writeAttrs(w, attrs)
	} else {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML([]byte(href)))
		_ = w.WriteByte('"')
	}
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
// when the address is not allowed or the image is in a link already. The
// link of one an extension knows carries the attributes its Links gives
// in place of the address, and shows the address.
func (m *marks) image(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	alt := util.EscapeHTML([]byte(plainText(n, source)))
	_, _ = w.WriteString(`<span class="nw-image">`)
	_, _ = w.Write(alt)
	if m.links == 0 {
		attrs, known := m.known(n)
		href, ok := SafeURL(string(util.URLEscape(n.Destination, true)))
		if (known || ok) && len(alt) > 0 {
			_ = w.WriteByte(' ')
		}
		switch {
		case known:
			_, _ = w.WriteString("<a")
			writeAttrs(w, attrs)
			_ = w.WriteByte('>')
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
			_, _ = w.WriteString("</a>")
		case ok:
			esc := util.EscapeHTML([]byte(href))
			_, _ = w.WriteString(`<a href="`)
			_, _ = w.Write(esc)
			_, _ = w.WriteString(`">`)
			_, _ = w.Write(esc)
			_, _ = w.WriteString("</a>")
		}
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
