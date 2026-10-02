// Package harden assembles goldmark's parser for platform/markdown:
// CommonMark, GFM's tables, task lists, strikethrough and autolinks, and
// footnotes. Every part of goldmark whose cost grows faster than its input
// is replaced or guarded here (M4/P3 design 3.4), so a parse costs about its
// size; the tree it gives is goldmark's, but for two limits: block quotes
// and list items nest at most MaxNesting deep, and a link destination opens
// at most MaxDestinationParens parentheses.
//
// Delimiter syntax goes through this package's runs (emphasis.go), never
// goldmark's delimiter list: nothing processes that list here. It imports
// goldmark only.
package harden

import (
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

const (
	// MaxNesting is how deep block quotes and list items nest: a line that
	// would open one deeper is text.
	MaxNesting = 32
	// MaxDestinationParens is how many parentheses a link destination may
	// open (CommonMark lets an implementation limit their nesting).
	MaxDestinationParens = 32
)

// NewParser returns the parser with opts, the extensions' parsers, added.
func NewParser(opts ...parser.Option) parser.Parser {
	p := parser.NewParser(
		parser.WithBlockParsers(blockParsers()...),
		parser.WithInlineParsers(inlineParsers()...),
		parser.WithParagraphTransformers(
			util.Prioritized(refdefs{}, 100),
			util.Prioritized(extension.NewTableParagraphTransformer(), 200),
		),
		parser.WithASTTransformers(
			// Before every other transformer: goldmark pairs emphasis while it
			// parses a block, before any of them runs.
			util.Prioritized(emphasisPass{}, -1),
			util.Prioritized(extension.NewTableASTTransformer(), 0),
			util.Prioritized(footnoteList{}, 999),
		),
	)
	p.AddOptions(opts...)
	return p
}

// blockParsers are parser.DefaultBlockParsers, block quotes and lists nested
// at most MaxNesting deep, and footnote definitions.
func blockParsers() []util.PrioritizedValue {
	return []util.PrioritizedValue{
		util.Prioritized(parser.NewSetextHeadingParser(), 100),
		util.Prioritized(parser.NewThematicBreakParser(), 200),
		util.Prioritized(nested{parser.NewListParser()}, 300),
		util.Prioritized(nested{parser.NewListItemParser()}, 400),
		util.Prioritized(parser.NewCodeBlockParser(), 500),
		util.Prioritized(parser.NewATXHeadingParser(), 600),
		util.Prioritized(parser.NewFencedCodeBlockParser(), 700),
		util.Prioritized(nested{parser.NewBlockquoteParser()}, 800),
		util.Prioritized(parser.NewHTMLBlockParser(), 900),
		util.Prioritized(parser.NewParagraphParser(), 1000),
		util.Prioritized(footnoteBlock{}, 999),
	}
}

// inlineParsers are parser.DefaultInlineParsers with links and emphasis
// replaced and code spans and raw HTML guarded, and GFM's and footnotes'.
// The priorities are goldmark's: a lower one is tried first.
func inlineParsers() []util.PrioritizedValue {
	return []util.PrioritizedValue{
		util.Prioritized(extension.NewTaskCheckBoxParser(), 0),
		util.Prioritized(codeSpanGuard{}, 90),
		util.Prioritized(parser.NewCodeSpanParser(), 100),
		util.Prioritized(footnoteRef{}, 101),
		util.Prioritized(links{}, 200),
		util.Prioritized(parser.NewAutoLinkParser(), 300),
		util.Prioritized(rawHTMLGuard{}, 350),
		util.Prioritized(parser.NewRawHTMLParser(), 400),
		util.Prioritized(runs{}, 500),
		util.Prioritized(linkify{extension.NewLinkifyParser()}, 999),
	}
}
