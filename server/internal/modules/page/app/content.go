package app

import "context"

// ContentParser parses the content of a write before its unit (M4 design
// 4, "parse timing"), once the write is decided and within the parse
// budget (M4/P4 review P1, P2): only a writer makes the server parse, and
// the bytes parsed at once stay within the budget. The unit gets the
// content's facts, not its tree (M6 design 4.7), so once they are taken the
// write keeps only their share of the budget, a tenth of its content's
// bytes (M6/P2 review M1): a write that waits for its locks holds little.
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
// holds the content's bytes while it is parsed, and the facts' share of
// them from then until release, which the write calls once its unit is
// over; a panic of the parse gives them all back as it leaves here. The
// unit decides again under its locks.
func (c *ContentParser) Parse(ctx context.Context, spec UnitSpec, content string) (Facts, func(), error) {
	if content == "" {
		return c.markdown.Facts(content), func() {}, nil
	}
	if err := c.writer.Allowed(ctx, spec); err != nil {
		return nil, nil, err
	}
	return c.Decided(ctx, content)
}

// Decided is the facts of content, as Parse takes them, for a write its
// caller decided already.
func (c *ContentParser) Decided(ctx context.Context, content string) (Facts, func(), error) {
	if content == "" {
		return c.markdown.Facts(content), func() {}, nil
	}
	hold, err := c.budget.Take(ctx, len(content))
	if err != nil {
		return nil, nil, err
	}
	taken := false
	defer func() {
		if !taken {
			hold.Release()
		}
	}()
	facts := c.markdown.Facts(content)
	hold.KeepFacts()
	taken = true
	return facts, hold.Release, nil
}
