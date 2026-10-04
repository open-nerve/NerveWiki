package markdown

import (
	"bytes"
	"context"
	"strconv"

	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

// Render is the HTML of d's reading view for page (M4 design 4,
// "rendering"; M4/P3 design 3.5–3.8): the frontmatter's properties as a
// table, then the body. Each extension's Fetch gets its data for the page
// first, in order, and its error is Render's. The tree's raw HTML is
// replaced by what the sanitizer keeps of it, so d serves this one Render.
func (m *Markdown) Render(ctx context.Context, d *Document, page Page) (string, error) {
	nodes := []util.PrioritizedValue{
		// goldmark's renderer stays safe: a node that reached it unexpected
		// would be an omitted comment or a dropped address.
		util.Prioritized(gmhtml.NewRenderer(), 1000),
		util.Prioritized(extension.NewTableHTMLRenderer(
			extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)), 500),
		util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
		util.Prioritized(extension.NewFootnoteHTMLRenderer(extension.WithFootnoteIDPrefix(idPrefix)), 500),
		util.Prioritized(&marks{}, 100),
	}
	for _, e := range m.exts {
		var data any
		if e.Fetch != nil {
			var err error
			if data, err = e.Fetch(ctx, page, d.extracted[e.Name]); err != nil {
				return "", err
			}
		}
		if e.Renderer != nil {
			nodes = append(nodes, e.Renderer(data)...)
		}
	}
	sanitize(d.root, d.source)
	var out bytes.Buffer
	if fm := d.frontmatter; fm.Valid && len(fm.Properties) > 0 {
		writeProperties(&out, fm.Properties)
		out.WriteByte('\n')
	}
	if err := renderer.NewRenderer(renderer.WithNodeRenderers(nodes...)).Render(&out, d.source, d.root); err != nil {
		return "", err
	}
	return out.String(), nil
}

// writeProperties writes props as a table, a row each: the key in a th,
// the value in a td (M4/P3 design 3.6).
func writeProperties(out *bytes.Buffer, props []Property) {
	out.WriteString(`<table class="nw-props">`)
	for _, p := range props {
		out.WriteString("<tr><th>" + html.EscapeString(p.Key) + "</th><td>")
		writeValue(out, p.Value)
		out.WriteString("</td></tr>")
	}
	out.WriteString("</table>")
}

// writeValue writes a property's value: null as nothing, a list as a ul, a
// mapping as a table of its own, a number as JSON writes it.
func writeValue(out *bytes.Buffer, v any) {
	switch v := v.(type) {
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case int64:
		out.WriteString(strconv.FormatInt(v, 10))
	case float64:
		out.WriteString(jsonNumber(v))
	case string:
		out.WriteString(html.EscapeString(v))
	case []any:
		out.WriteString("<ul>")
		for _, item := range v {
			out.WriteString("<li>")
			writeValue(out, item)
			out.WriteString("</li>")
		}
		out.WriteString("</ul>")
	case []Property:
		writeProperties(out, v)
	}
}
