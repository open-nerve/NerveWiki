package app

import "context"

// ContentParser parses the content of a write before its unit (M4 design
// 4, "parse timing"), once the write is decided and within the parse
// budget (M4/P4 review P1, P2): only a writer makes the server parse, and
// the bytes parsed at once stay within the budget. The unit gets the
// content's facts, not its tree, so the budget is back once they are taken
// (M6 design 4.7): a write that waits for its locks holds none of it.
type ContentParser struct {
	writer   *Writer
	markdown Markdown
	budget   ParseBudget
}

// NewContentParser returns the parser.
func NewContentParser(writer *Writer, markdown Markdown, budget ParseBudget) *ContentParser {
	return &ContentParser{writer: writer, markdown: markdown, budget: budget}
}

// Parse is the facts of content, which domain.CheckContent passed, for a
// write of spec. Unless the content is empty, the write is decided first,
// unlocked, with the unit's 404 and 403 (Writer.Allowed); then the budget
// holds the content's bytes while it is parsed, and has them back once the
// facts are taken, or a panic of the parse leaves here. The unit decides
// again under its locks.
func (c *ContentParser) Parse(ctx context.Context, spec UnitSpec, content string) (Facts, error) {
	if content == "" {
		return c.markdown.Facts(content), nil
	}
	if err := c.writer.Allowed(ctx, spec); err != nil {
		return nil, err
	}
	return c.Decided(ctx, content)
}

// Decided is the facts of content, as Parse takes them, for a write its
// caller decided already.
func (c *ContentParser) Decided(ctx context.Context, content string) (Facts, error) {
	if content == "" {
		return c.markdown.Facts(content), nil
	}
	release, err := c.budget.Take(ctx, len(content))
	if err != nil {
		return nil, err
	}
	defer release()
	return c.markdown.Facts(content), nil
}
