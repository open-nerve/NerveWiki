// Package obsidian is the extension of Obsidian's dialect (M6 design 4.1;
// M6/P1 design 3.5–3.11): wikilinks and embeds, tags, math, comments,
// callouts and highlights, parsed as the fixture set's rules have them
// (tools/md-fixtures), and rendered, an attachment's embed as its image,
// audio or video (M7/P3 design 5.5); and the links and tags of a page,
// taken from its wikilinks, its Markdown links and its frontmatter's
// properties as well.
package obsidian

import (
	"context"
	"time"
	"uuid"

	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Name is the extension's name: Document.Extracted(Name) is an Extracted.
const Name = "obsidian"

// Target is where a link leads (M7/P3 design 5.3): a node, an
// attachment's or a page's.
type Target struct {
	Node  uuid.UUID
	Asset bool
}

// Resolve is where the links of a page lead, for its reading view (M6/P3
// design 6.4): by where each link's target starts (Link.Range.Start), the
// node it resolves to; a link it leaves out resolves to none. links are
// what the extension took of the page's content at page.Revision.
type Resolve func(ctx context.Context, page markdown.Page, links []Link) (map[int]Target, error)

// Asset is what a reading view shows of an attachment (M7/P3 design 5.3):
// its type and size, an image's width and height in pixels (0 when not
// known), its content's address, signed, a path of this site, and when
// that expires.
type Asset struct {
	MIME          string
	Bytes         int64
	Width, Height int
	URL           string
	Expires       time.Time
}

// Assets is what a reading view shows of the attachments ids of the
// notebook notebookID, by id: those not deleted of it. One it leaves out
// shows as text.
type Assets func(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]Asset, error)

// Options are what the extension takes from the composition root.
type Options struct {
	// Resolve is where a page's links lead; nil, none leads anywhere.
	Resolve Resolve
	// Assets is what the attachments the links lead to show; nil, each
	// shows as text.
	Assets Assets
}

// Extension is the extension of Obsidian's dialect.
func Extension(o Options) markdown.Extension {
	// A property's value is parsed as the body is, to tell whether it is
	// one link and nothing else (rule 10).
	values := harden.NewParser(dialect()...)
	return markdown.Extension{
		Name:    Name,
		Parser:  dialect(),
		Extract: func(t markdown.Tree) any { return extract(t, values) },
		Fetch:   o.fetch,
		Links: func(data any) func(int) ([]markdown.Attr, bool) {
			v, _ := data.(view)
			return v.markdownAttrs
		},
		Images: func(data any) func(int) (markdown.Image, bool) {
			v, _ := data.(view)
			return v.image
		},
		Expires: func(data any) time.Time {
			v, _ := data.(view)
			return v.expires
		},
		Renderer: func(data any) []util.PrioritizedValue {
			v, _ := data.(view)
			return []util.PrioritizedValue{util.Prioritized(nodeRenderer{v}, 500)}
		},
		Properties: func(data any) func(markdown.Scalar) ([]markdown.Attr, string, bool) {
			v, _ := data.(view)
			return v.property(values)
		},
		Markup: markdown.Markup{
			Elements: map[string][]string{
				"a":       {"class", "href", "data-nw-node", "data-nw-anchor", "data-nw-target", "data-nw-tag", "data-nw-size"},
				"span":    {"class", "data-nw-tag"},
				"img":     {"class", "src", "alt", "width", "height", "loading"},
				"audio":   {"class", "controls", "preload", "src", "aria-label"},
				"video":   {"class", "controls", "preload", "src", "aria-label", "width", "height"},
				"mark":    nil,
				"div":     {"class", "data-callout"},
				"details": {"class", "data-callout", "open"},
				"summary": nil,
			},
			URLs: []string{"src"},
			Classes: []string{
				"nw-wikilink", "nw-embed", "nw-unresolved", "nw-tag", "nw-math", "nw-math-block", "nw-callout",
				"nw-callout-title", "nw-asset",
			},
		},
	}
}

// dialect is the dialect's parsers and transformers. The priorities are
// goldmark's, a lower one first: a wikilink is tried after a footnote's
// reference and before a link, which '[' and '!' also start; a formula's
// block before a paragraph, which it interrupts; the callouts, the comments,
// the line breaks after formulas, the tags' second look after the emphasis
// is paired, and then the marks of the wikilinks and tags in a link's text.
// An address ends before a comment's "%%", so that a comment may end with
// one.
func dialect() []parser.Option {
	return []parser.Option{
		harden.AddressesEndBefore("%%"),
		parser.WithBlockParsers(util.Prioritized(mathBlockParser{}, 750)),
		parser.WithInlineParsers(
			util.Prioritized(wikilinkParser{}, 150),
			util.Prioritized(mathParser{}, 151),
			util.Prioritized(markerParser{}, 152),
			util.Prioritized(tagParser{}, 153),
			harden.HighlightRuns(),
		),
		parser.WithASTTransformers(
			util.Prioritized(callouts{}, 10),
			util.Prioritized(comments{}, 20),
			util.Prioritized(formulaLines{}, 25),
			util.Prioritized(tagsAfterText{}, 30),
			util.Prioritized(inLinks{}, 40),
		),
	}
}
