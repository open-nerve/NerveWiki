package obsidian_test

import (
	"context"
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
	"uuid"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/obsidian"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/tasks"
)

// blocks are the elements a reading view shows as blocks, as the render
// fixtures' text reads them (tools/md-fixtures/obsidian/verify-render.mjs).
//
//nolint:gochecknoglobals // read only
var blocks = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Li: true, atom.Ul: true, atom.Ol: true, atom.Blockquote: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true, atom.Pre: true,
	atom.Table: true, atom.Thead: true, atom.Tbody: true, atom.Tr: true, atom.Td: true, atom.Th: true,
	atom.Hr: true, atom.Details: true, atom.Summary: true, atom.Section: true,
}

//nolint:gochecknoglobals // read only
var (
	spaces     = regexp.MustCompile(`\s+`)
	marksSpace = regexp.MustCompile(` ?([¶⏎]) ?`)
	breaks     = regexp.MustCompile(`¶+`)
)

// shown is a reading view's HTML as text: "¶" between blocks, "⏎" a line
// break, the one line break at a block's end left out (it shows none), runs
// of ASCII white space (RE2's \s) one space; an attachment's image, audio or
// video as "⟨img text size⟩", "⟨audio text⟩", "⟨video text size⟩", its text
// an image's alt, a control's label (M7/P3 design 5.8). The properties'
// table is left out, as the script leaves out Obsidian's, and so are
// formulas, which Obsidian typesets with MathJax and the front end with
// KaTeX.
func shown(t *testing.T, s string) string {
	t.Helper()
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type != html.ElementNode:
		case n.DataAtom == atom.Br:
			b.WriteString("⏎")
		case n.DataAtom == atom.Img:
			b.WriteString(media("img", attr(n, "alt"), n))
		case n.DataAtom == atom.Audio || n.DataAtom == atom.Video:
			b.WriteString(media(n.Data, attr(n, "aria-label"), n))
		case slices.ContainsFunc(strings.Fields(attr(n, "class")), func(c string) bool { return c == "nw-props" || c == "nw-math" }):
		default:
			if blocks[n.DataAtom] {
				b.WriteString("¶")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if blocks[n.DataAtom] {
				b.WriteString("¶")
			}
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	out := spaces.ReplaceAllString(b.String(), " ")
	out = breaks.ReplaceAllString(marksSpace.ReplaceAllString(out, "$1"), "¶")
	out = strings.ReplaceAll(out, "⏎¶", "¶")
	return strings.TrimSuffix(strings.TrimPrefix(out, "¶"), "¶")
}

// media is an attachment's element as the render fixtures write it: its
// kind, its text and the size it is written with, each but the kind left
// out when it has none.
func media(kind, text string, n *html.Node) string {
	parts := []string{kind}
	if text != "" {
		parts = append(parts, text)
	}
	size := attr(n, "width")
	if h := attr(n, "height"); h != "" {
		size += "×" + h
	}
	if size != "" {
		parts = append(parts, size)
	}
	return "⟨" + strings.Join(parts, " ") + "⟩"
}

// fixtureAssets is the Markdown whose links lead to a render fixture's
// attachments, by their names, whatever their case, as the index has them:
// each of the type its extension tells, of no known size, at /a/ and its
// name.
func fixtureAssets(t *testing.T, names []string) *markdown.Markdown {
	t.Helper()
	ids := map[string]uuid.UUID{}
	for i, name := range names {
		ids[strings.ToLower(name)] = uuid.UUID{15: byte(i + 1)}
	}
	types := map[string]string{".png": "image/png", ".mp3": "audio/mpeg", ".webm": "video/webm"}
	return newMarkdownWith(t, obsidian.Options{
		Resolve: func(_ context.Context, _ markdown.Page, links []obsidian.Link) (map[int]obsidian.Target, error) {
			to := map[int]obsidian.Target{}
			for _, l := range links {
				if id, ok := ids[strings.ToLower(l.Target)]; ok {
					to[l.Range.Start] = obsidian.Target{Node: id, Asset: true}
				}
			}
			return to, nil
		},
		Assets: func(_ context.Context, _ uuid.UUID, asked []uuid.UUID) (map[uuid.UUID]obsidian.Asset, error) {
			out := map[uuid.UUID]obsidian.Asset{}
			for name, id := range ids {
				if slices.Contains(asked, id) {
					out[id] = obsidian.Asset{MIME: types[path.Ext(name)], Bytes: 1, URL: "/a/" + name}
				}
			}
			return out, nil
		},
	})
}

// attr is n's attribute key, "" when it has none.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// A render fixture's reading view is what its JSON says: as Obsidian 1.12.7
// shows it, or as its note says it differs; a line break in a paragraph is
// shown as one (M6/P8 design 4); its links to its attachments lead to them
// (M7/P3 design 5.8).
func TestTheRenderFixturesShowWhatTheirJSONSays(t *testing.T) {
	for _, f := range markdowntest.RenderCases(t) {
		t.Run(f.Name, func(t *testing.T) {
			var want struct {
				Rendered string   `json:"rendered"`
				Assets   []string `json:"assets"`
			}
			if f.JSON != nil {
				if err := json.Unmarshal(f.JSON, &want); err != nil {
					t.Fatal(err)
				}
			}
			m := fixtureAssets(t, want.Assets)
			view, err := m.Render(context.Background(), m.Parse(f.Content), markdown.Page{})
			if err != nil {
				t.Fatal(err)
			}
			got := view.HTML
			if f.JSON == nil {
				t.Fatalf("no JSON; the reading view shows %q", shown(t, got))
			}
			if s := shown(t, got); s != want.Rendered {
				t.Errorf("the reading view shows %q, want %q\n%s", s, want.Rendered, got)
			}
			if err := markdowntest.CheckHTML(got, tasks.Extension(), obsidian.Extension(obsidian.Options{})); err != nil {
				t.Error(err)
			}
		})
	}
}
