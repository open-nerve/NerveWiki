// Package harden assembles goldmark's parser for platform/markdown:
// CommonMark, GFM's tables, task lists, strikethrough and autolinks, and
// footnotes. Every part of goldmark whose cost grows faster than its input
// is replaced or guarded here (M4/P3 design 3.4), so a parse costs about its
// size, and so does the tree it gives: it is goldmark's, but for four
// limits. Block quotes, list items and footnote definitions nest at most
// MaxNesting deep; an inline link's destination opens at most
// MaxDestinationParens parentheses; a table holds at most as many cells as
// it has bytes; and reference links repeat at most the larger of the
// source's size and MinExpansion bytes of their definitions' destinations
// and titles.
//
// Delimiter syntax goes through this package's runs (emphasis.go), never
// goldmark's delimiter list: nothing processes that list here. Links are
// this package's (links.go): goldmark's Context.IsInLinkLabel is always
// false, and "a link may not contain a link" counts the links made here
// only. It imports goldmark only.
package harden

import (
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

const (
	// MaxNesting is how deep block quotes, list items and footnote
	// definitions nest: a line that would open one deeper is text.
	MaxNesting = 32
	// MaxDestinationParens is how many parentheses an inline link's
	// destination may open (CommonMark lets an implementation limit their
	// nesting).
	MaxDestinationParens = 32
	// MinExpansion is how many bytes of destinations and titles reference
	// links may repeat however short the source; past the larger of it and
	// the source's size a reference is text, as in cmark.
	MinExpansion = 100_000
)

// NewParser returns the parser with opts, the extensions' parsers, added.
func NewParser(opts ...parser.Option) parser.Parser {
	p := parser.NewParser(
		parser.WithBlockParsers(blockParsers()...),
		parser.WithInlineParsers(inlineParsers()...),
		parser.WithParagraphTransformers(
			util.Prioritized(refdefs{}, 100),
			util.Prioritized(tableRows{extension.NewTableParagraphTransformer()}, 200),
		),
		parser.WithASTTransformers(
			// Before every other transformer: goldmark pairs emphasis while it
			// parses a block, before any of them runs.
			util.Prioritized(emphasisPass{}, -1),
			util.Prioritized(tableCells{}, 0),
			util.Prioritized(footnoteList{}, 999),
		),
	)
	p.AddOptions(opts...)
	return p
}

// blockParsers are parser.DefaultBlockParsers and footnote definitions,
// block quotes, lists and footnotes nested at most MaxNesting deep.
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
		util.Prioritized(nested{footnoteBlock{}}, 999),
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
