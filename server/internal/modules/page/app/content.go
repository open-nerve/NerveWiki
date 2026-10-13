package app

import (
	"cmp"
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

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
	// wait is the longest the takes of one call of DecidedAll wait,
	// together: the budget's for one take.
	wait time.Duration
}

// NewContentParser returns the parser; wait is the budget's wait,
// page.parse_max_wait.
func NewContentParser(writer *Writer, markdown Markdown, budget ParseBudget, wait time.Duration) *ContentParser {
	return &ContentParser{writer: writer, markdown: markdown, budget: budget, wait: wait}
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
	hold.KeepFacts(facts)
	taken = true
	return facts, hold.Release, nil
}

// busyAfter is the Retry-After of the 503 when DecidedAll's waits run out.
const busyAfter = time.Second

// DecidedAll is the facts of contents, by page, as Decided takes them for a
// unit its caller decided already, which writes them all (M8/P2 design
// 3.5): the largest first, so that the smaller ones after it always find
// room, each kept until release; the waits of their takes, together, at
// most the budget's wait, as a request has the time of one: a take that
// runs out of what is left of it while the request has time is 503
// server_busy. Unless they are a lone page, it counts their links as it
// goes, and past the unit's bound gives back what it holds and answers
// tooLarge: their number is known only once they are parsed.
func (c *ContentParser) DecidedAll(ctx context.Context, contents map[uuid.UUID]string, tooLarge error) (
	map[uuid.UUID]Facts, func(), error,
) {
	ids := make([]uuid.UUID, 0, len(contents))
	for id := range contents {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return cmp.Or(len(contents[b])-len(contents[a]), a.Compare(b)) })
	var holds []BudgetHold
	release := func() {
		for _, h := range holds {
			h.Release()
		}
	}
	done := false
	defer func() {
		if !done {
			release()
		}
	}()
	out := make(map[uuid.UUID]Facts, len(contents))
	left, links := c.wait, 0
	for _, id := range ids {
		content := contents[id]
		if content == "" {
			out[id] = c.markdown.Facts(content)
			continue
		}
		started := time.Now()
		hold, err := c.take(ctx, len(content), left)
		left -= time.Since(started)
		if err != nil {
			return nil, nil, err
		}
		holds = append(holds, hold)
		facts := c.markdown.Facts(content)
		hold.KeepFacts(facts)
		out[id] = facts
		if links += c.markdown.Links(facts); domain.UnitTooLarge(len(contents), 0, links) {
			return nil, nil, tooLarge
		}
	}
	done = true
	return out, release, nil
}

// take takes n bytes of the budget, waiting at most left: when that runs
// out first, while the request has time, the server is busy.
func (c *ContentParser) take(ctx context.Context, n int, left time.Duration) (BudgetHold, error) {
	waited, cancel := context.WithTimeout(ctx, max(left, 0))
	defer cancel()
	hold, err := c.budget.Take(waited, n)
	if err != nil && ctx.Err() == nil && waited.Err() != nil {
		return nil, shared.ServerBusy(busyAfter)
	}
	return hold, err
}
