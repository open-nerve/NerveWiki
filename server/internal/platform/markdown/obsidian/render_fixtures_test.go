package obsidian_test

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/markdowntest"
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
// break, a line break at a block's end left out (it shows none), runs of
// white space one space.
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

// A render fixture's reading view is what its JSON says, as Obsidian 1.12.7
// shows it: a line break in a paragraph is shown as one (M6/P8 design 4).
func TestTheRenderFixturesShowWhatObsidianShows(t *testing.T) {
	m := newMarkdown(t)
	for _, f := range markdowntest.RenderCases(t) {
		t.Run(f.Name, func(t *testing.T) {
			got, err := m.Render(context.Background(), m.Parse(f.Content), markdown.Page{})
			if err != nil {
				t.Fatal(err)
			}
			var want struct {
				Rendered string `json:"rendered"`
			}
			if f.JSON == nil {
				t.Fatalf("no JSON; the reading view shows %q", shown(t, got))
			}
			if err := json.Unmarshal(f.JSON, &want); err != nil {
				t.Fatal(err)
			}
			if s := shown(t, got); s != want.Rendered {
				t.Errorf("the reading view shows %q, want %q\n%s", s, want.Rendered, got)
			}
		})
	}
}
