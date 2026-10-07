package markdown

import (
	"bytes"
	"context"
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

// Render is the HTML of d's reading view for page (M4 design 4,
// "rendering"; M4/P3 design 3.5–3.8): the frontmatter's properties as a
// table, in a region of its own that scrolls sideways (M6/P6 design 6),
// then the body. Each extension's Fetch gets its data for the page
// first, in order, and its error is Render's. The tree's raw HTML is
// replaced by what the sanitizer keeps of it, so d serves this one Render.
func (m *Markdown) Render(ctx context.Context, d *Document, page Page) (string, error) {
	footnotes := registered{}
	extension.NewFootnoteHTMLRenderer(extension.WithFootnoteIDPrefix(idPrefix)).RegisterFuncs(footnotes)
	links := &marks{destinations: d.destinations, footnoteLink: footnotes[east.KindFootnoteLink]}
	fm := d.facts.frontmatter
	props := table{scalars: fm.Scalars}
	nodes := []util.PrioritizedValue{
		// goldmark's renderer stays safe: a node that reached it unexpected
		// would be an omitted comment or a dropped address. A line break in a
		// paragraph is shown as one, as Obsidian's reading view shows it (its
		// strict line breaks off, the default; M6/P8 design 3).
		util.Prioritized(gmhtml.NewRenderer(gmhtml.WithHardWraps()), 1000),
		util.Prioritized(scrollingTables{extension.NewTableHTMLRenderer(
			extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute))}, 500),
		util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
		util.Prioritized(footnotes, 500),
		util.Prioritized(links, 100),
	}
	for _, e := range m.exts {
		var data any
		if e.Fetch != nil {
			var err error
			if data, err = e.Fetch(ctx, page, d.facts.Extracted(e.Name)); err != nil {
				return "", err
			}
		}
		if e.Renderer != nil {
			nodes = append(nodes, e.Renderer(data)...)
		}
		if e.Links != nil {
			links.written = append(links.written, e.Links(data))
		}
		if e.Properties != nil {
			props.links = append(props.links, e.Properties(data))
		}
	}
	sanitize(d.root, d.source)
	var out bytes.Buffer
	if fm.Valid && len(fm.Properties) > 0 {
		props.out = &out
		out.WriteString(`<div class="nw-scroll">`)
		props.write(fm.Properties)
		out.WriteString("</div>\n")
	}
	if err := renderer.NewRenderer(renderer.WithNodeRenderers(nodes...)).Render(&out, d.source, d.root); err != nil {
		return "", err
	}
	return out.String(), nil
}

// table writes the frontmatter's properties to out (M4/P3 design 3.6):
// its strings in the order the reader numbered them, each written on one
// line a scalar, which links may write as a link (M6/P6 design 4).
type table struct {
	out     *bytes.Buffer
	scalars []Scalar // those after the strings written, by ordinal
	links   []func(Scalar) ([]Attr, string, bool)
	strings int // how many strings it wrote
}

// write writes props as a table, a row each: the key in a th, the value in
// a td.
func (t *table) write(props []Property) {
	t.out.WriteString(`<table class="nw-props">`)
	for _, p := range props {
		t.out.WriteString("<tr><th>" + html.EscapeString(p.Key) + "</th><td>")
		t.value(p.Value)
		t.out.WriteString("</td></tr>")
	}
	t.out.WriteString("</table>")
}

// value writes a property's value: null as nothing, a list as a ul, a
// mapping as a table of its own, a number as JSON writes it, a string as
// a link if an extension writes it so.
func (t *table) value(v any) {
	switch v := v.(type) {
	case bool:
		t.out.WriteString(strconv.FormatBool(v))
	case int64:
		t.out.WriteString(strconv.FormatInt(v, 10))
	case float64:
		t.out.WriteString(jsonNumber(v))
	case string:
		if attrs, text, ok := t.link(); ok {
			t.out.WriteString("<a")
			WriteAttrs(t.out, attrs)
			t.out.WriteString(">" + html.EscapeString(text) + "</a>")
		} else {
			t.out.WriteString(html.EscapeString(v))
		}
	case []any:
		t.out.WriteString("<ul>")
		for _, item := range v {
			t.out.WriteString("<li>")
			t.value(item)
			t.out.WriteString("</li>")
		}
		t.out.WriteString("</ul>")
	case []Property:
		t.write(v)
	}
}

// link is how the next string is written as a link, if it is a scalar an
// extension writes so.
func (t *table) link() ([]Attr, string, bool) {
	ordinal := t.strings
	t.strings++
	for len(t.scalars) > 0 && t.scalars[0].ordinal < ordinal {
		t.scalars = t.scalars[1:]
	}
	if len(t.scalars) == 0 || t.scalars[0].ordinal != ordinal {
		return nil, "", false
	}
	for _, link := range t.links {
		if attrs, text, ok := link(t.scalars[0]); ok {
			return attrs, text, true
		}
	}
	return nil, "", false
}

// scrollingTables is goldmark's table renderer, each table in a region of
// its own that scrolls sideways, so that a wide table scrolls and not the
// whole reading view (M6/P1 design 3.4). The front end gives the region
// the focus while it is wider than it shows, so that the keyboard scrolls
// it (WCAG 2.1.1; M6/P6 design 6): the server cannot tell.
type scrollingTables struct{ inner renderer.NodeRenderer }

// registered is a registerer that keeps what it is given, and the
// renderer of what it keeps.
type registered map[ast.NodeKind]renderer.NodeRendererFunc

func (r registered) Register(kind ast.NodeKind, f renderer.NodeRendererFunc) { r[kind] = f }

func (r registered) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	for kind, f := range r {
		reg.Register(kind, f)
	}
}

func (s scrollingTables) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	funcs := registered{}
	s.inner.RegisterFuncs(funcs)
	for kind, f := range funcs {
		if kind != east.KindTable {
			reg.Register(kind, f)
			continue
		}
		reg.Register(kind, func(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				_, _ = w.WriteString(`<div class="nw-scroll">`)
			}
			status, err := f(w, source, n, entering)
			if !entering {
				_, _ = w.WriteString("</div>\n")
			}
			return status, err
		})
	}
}
