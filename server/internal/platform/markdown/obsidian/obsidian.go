// Package obsidian is the extension of Obsidian's dialect (M6 design 4.1;
// M6/P1 design 3.5–3.11): wikilinks and embeds, tags, math, comments,
// callouts and highlights, parsed as the fixture set's rules have them
// (tools/md-fixtures), and rendered; and the links and tags of a page,
// taken from its wikilinks, its Markdown links and its frontmatter's
// properties as well.
package obsidian

import (
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"

	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown"
	"github.com/open-nerve/NerveWiki/server/internal/platform/markdown/internal/harden"
)

// Name is the extension's name: Document.Extracted(Name) is an Extracted.
const Name = "obsidian"

// Extension is the extension of Obsidian's dialect.
func Extension() markdown.Extension {
	// A property's value is parsed as the body is, to tell whether it is
	// one link and nothing else (rule 10).
	values := harden.NewParser(dialect()...)
	return markdown.Extension{
		Name:    Name,
		Parser:  dialect(),
		Extract: func(t markdown.Tree) any { return extract(t, values) },
		Renderer: func(any) []util.PrioritizedValue {
			return []util.PrioritizedValue{util.Prioritized(nodeRenderer{}, 500)}
		},
		Markup: markdown.Markup{
			Elements: map[string][]string{
				"span":    {"class", "data-nw-target", "data-nw-tag"},
				"mark":    nil,
				"div":     {"class", "data-callout"},
				"details": {"class", "data-callout", "open"},
				"summary": nil,
			},
			Classes: []string{
				"nw-wikilink", "nw-embed", "nw-tag", "nw-math", "nw-math-block", "nw-callout", "nw-callout-title",
			},
		},
	}
}

// dialect is the dialect's parsers and transformers. The priorities are
// goldmark's, a lower one first: a wikilink is tried after a footnote's
// reference and before a link, which '[' and '!' also start; a formula's
// block before a paragraph, which it interrupts; the callouts, the comments
// and then the tags' second look after the emphasis is paired. An address
// ends before a comment's "%%", so that a comment may end with one.
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
			util.Prioritized(tagsAfterText{}, 30),
		),
	}
}
