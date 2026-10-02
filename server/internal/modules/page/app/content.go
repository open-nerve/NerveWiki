package app

import "context"

// ContentParser parses the content of a write before its unit (M4 design
// 4, "parse timing"), once the write is decided and within the parse
// budget (M4/P4 review P1, P2): only a writer makes the server parse, and
// the bytes parsed at once stay within the budget.
type ContentParser struct {
	writer   *Writer
	markdown Markdown
	budget   ParseBudget
}

// NewContentParser returns the parser.
func NewContentParser(writer *Writer, markdown Markdown, budget ParseBudget) *ContentParser {
	return &ContentParser{writer: writer, markdown: markdown, budget: budget}
}

// Parse parses content, which domain.CheckContent passed, for a write of
// spec. Unless the content is empty, the write is decided first, unlocked,
// with the unit's 404 and 403 (Writer.Allowed); then the budget holds the
// content's bytes until release, which the write calls once its unit is
// over, or until a panic of the parse leaves here. The unit decides again
// under its locks.
func (c *ContentParser) Parse(ctx context.Context, spec UnitSpec, content string) (Parsed, func(), error) {
	if content == "" {
		return c.markdown.Parse(content), func() {}, nil
	}
	if err := c.writer.Allowed(ctx, spec); err != nil {
		return nil, nil, err
	}
	release, err := c.budget.Take(ctx, len(content))
	if err != nil {
		return nil, nil, err
	}
	parsed := false
	defer func() {
		if !parsed {
			release()
		}
	}()
	p := c.markdown.Parse(content)
	parsed = true
	return p, release, nil
}
