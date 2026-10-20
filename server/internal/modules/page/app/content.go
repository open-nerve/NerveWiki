package app

import (
	"cmp"
	"context"
	"crypto/sha256"
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
	return decidedAll(ctx, c, contents, func(id uuid.UUID) uuid.UUID { return id }, uuid.UUID.Compare, tooLarge)
}

// DecidedContents is DecidedAll of contents by page and hash (M9/P3
// design 3.18.3, 3.18.4): a revert's, which may restore a page at its
// content in the trash and write another back on it, two contents of the
// page, each parsed and counted; those of a lone page, however many, are
// past no bound.
func (c *ContentParser) DecidedContents(ctx context.Context, contents map[domain.ContentRef]string, tooLarge error) (
	map[domain.ContentRef]Facts, func(), error,
) {
	return decidedAll(ctx, c, contents, func(r domain.ContentRef) uuid.UUID { return r.NodeID }, func(a, b domain.ContentRef) int {
		return cmp.Or(a.NodeID.Compare(b.NodeID), cmp.Compare(a.Hash, b.Hash))
	}, tooLarge)
}

// decidedAll is DecidedAll of contents by key, each the content of the
// page pageOf names, in compare's order among those of a size.
func decidedAll[K comparable](ctx context.Context, c *ContentParser, contents map[K]string, pageOf func(K) uuid.UUID,
	compare func(a, b K) int, tooLarge error,
) (map[K]Facts, func(), error) {
	keys := make([]K, 0, len(contents))
	pages := map[uuid.UUID]bool{}
	for k := range contents {
		keys, pages[pageOf(k)] = append(keys, k), true
	}
	slices.SortFunc(keys, func(a, b K) int { return cmp.Or(len(contents[b])-len(contents[a]), compare(a, b)) })
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
	out := make(map[K]Facts, len(contents))
	left, links := c.wait, 0
	for _, k := range keys {
		content := contents[k]
		if content == "" {
			out[k] = c.markdown.Facts(content)
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
		out[k] = facts
		if links += c.markdown.Links(facts); domain.UnitTooLarge(len(pages), len(contents), 0, links) {
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

// ParseSession is the parses of one request, a batch's (M9/P3 design
// 3.4.5): its plans' parses, each taking the budget as Decided does and
// keeping its facts' share alone, the facts kept by page and hash until
// the request ends, so that a content parsed once, in a plan made again
// too, is parsed no more; and its final contents' DecidedAll. Its takes
// wait, together, what is left of the budget's wait, once for the
// request. Before each take it gives back the shares it holds, their facts
// kept, uncounted: a parse never waits for the session's own facts.
type ParseSession struct {
	c     *ContentParser
	left  time.Duration
	facts map[parsedKey]Facts
	holds []BudgetHold
	// planned are the contents of the plan being made, each page and hash
	// once, and their links: a unit's bound counts them with the final
	// contents'.
	planned map[parsedKey]bool
	links   int
}

// parsedKey is a content the session parsed: its page and its hash.
type parsedKey struct {
	page uuid.UUID
	hash [sha256.Size]byte
}

// Planned is a parse session for a request: a batch's plans and its
// final contents.
func (c *ContentParser) Planned() *ParseSession {
	return &ParseSession{c: c, left: c.wait, facts: map[parsedKey]Facts{}, planned: map[parsedKey]bool{}}
}

// NewPlan starts the counting of a plan made again: its contents counted
// from none, those parsed before found as they are.
func (s *ParseSession) NewPlan() {
	s.planned, s.links = map[parsedKey]bool{}, 0
}

// Parse is the facts of content, the page id's, for the plan: kept ones
// when the session has them, else parsed within the budget, the shares the
// session holds given back first.
func (s *ParseSession) Parse(ctx context.Context, id uuid.UUID, content string) (Facts, error) {
	key := parsedKey{page: id, hash: sha256.Sum256([]byte(content))}
	f, err := s.factsOf(ctx, key, content, true)
	if err != nil {
		return nil, err
	}
	if !s.planned[key] {
		s.planned[key] = true
		s.links += s.c.markdown.Links(f)
	}
	return f, nil
}

// factsOf is the facts of content, keyed key: those the session kept, else
// parsed, taking its bytes within what is left of the wait, the shares the
// session holds given back first when giveBack, its own share then held.
func (s *ParseSession) factsOf(ctx context.Context, key parsedKey, content string, giveBack bool) (Facts, error) {
	if f, ok := s.facts[key]; ok {
		return f, nil
	}
	if content == "" {
		f := s.c.markdown.Facts(content)
		s.facts[key] = f
		return f, nil
	}
	if giveBack {
		s.giveBack()
	}
	started := time.Now()
	hold, err := s.c.take(ctx, len(content), s.left)
	s.left -= time.Since(started)
	if err != nil {
		return nil, err
	}
	s.holds = append(s.holds, hold) // given back by Release, on a panic of the parse too
	f := s.c.markdown.Facts(content)
	hold.KeepFacts(f)
	s.facts[key] = f
	return f, nil
}

// giveBack gives back the shares the session holds, their facts kept.
func (s *ParseSession) giveBack() {
	for _, h := range s.holds {
		h.Release()
	}
	s.holds = nil
}

// DecidedAll is the facts of contents, by page, the final ones of a batch,
// as ContentParser.DecidedAll takes them: the largest first, each share
// held until Release, within what is left of the session's wait, the
// shares of the plan's parses given back before the first; facts the
// session has are taken as they are. Unless they are a lone page, their
// links, with the plan's, each page and hash once, past the unit's bound
// are tooLarge's error, told how many.
func (s *ParseSession) DecidedAll(ctx context.Context, contents map[uuid.UUID]string, tooLarge func(links int) error) (
	map[uuid.UUID]Facts, error,
) {
	s.giveBack()
	ids := make([]uuid.UUID, 0, len(contents))
	for id := range contents {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return cmp.Or(len(contents[b])-len(contents[a]), a.Compare(b)) })
	out := make(map[uuid.UUID]Facts, len(contents))
	for _, id := range ids {
		key := parsedKey{page: id, hash: sha256.Sum256([]byte(contents[id]))}
		f, err := s.factsOf(ctx, key, contents[id], false)
		if err != nil {
			return nil, err
		}
		out[id] = f
		if !s.planned[key] {
			s.planned[key] = true
			if s.links += s.c.markdown.Links(f); domain.UnitTooLarge(len(contents), 0, 0, s.links) {
				return nil, tooLarge(s.links)
			}
		}
	}
	return out, nil
}

// Release gives back every share the session holds, its facts kept: before
// a batch's deferred rewrite, whose read backs it must not stand in the
// way of, and as the request ends.
func (s *ParseSession) Release() { s.giveBack() }
