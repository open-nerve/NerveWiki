package app

import (
	"container/heap"
	"context"
	"slices"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

type lintCandidate struct {
	item domain.Item
	pos  domain.Position
}

// lintHeap keeps only the best requested prefix plus one witness for more.
// The root is the last/worst retained position, not the first output item.
type lintHeap struct {
	rule domain.Rule
	rows []lintCandidate
}

func (h lintHeap) Len() int { return len(h.rows) }
func (h lintHeap) Less(i, j int) bool {
	return domain.ComparePosition(h.rule, h.rows[i].pos, h.rows[j].pos) > 0
}
func (h lintHeap) Swap(i, j int) { h.rows[i], h.rows[j] = h.rows[j], h.rows[i] }
func (h *lintHeap) Push(v any)   { h.rows = append(h.rows, v.(lintCandidate)) }
func (h *lintHeap) Pop() any {
	i := len(h.rows) - 1
	v := h.rows[i]
	h.rows[i] = lintCandidate{}
	h.rows = h.rows[:i]
	return v
}

type lintCollector struct {
	heap  lintHeap
	after *domain.Position
	limit int
	count int64
}

func newLintCollector(rule domain.Rule, after *domain.Position, limit int) *lintCollector {
	return &lintCollector{heap: lintHeap{rule: rule}, after: after, limit: limit}
}

func (c *lintCollector) add(item domain.Item, pos domain.Position) {
	if c.count < 10000 {
		c.count++
	}
	if c.after != nil && domain.ComparePosition(c.heap.rule, pos, *c.after) <= 0 {
		return
	}
	row := lintCandidate{item: item, pos: pos}
	if c.heap.Len() < c.limit+1 {
		heap.Push(&c.heap, row)
	} else if domain.ComparePosition(c.heap.rule, pos, c.heap.rows[0].pos) < 0 {
		c.heap.rows[0] = row
		heap.Fix(&c.heap, 0)
	}
}

type preparedRule struct {
	count domain.Count
	items []domain.Item
	more  bool
}

func (c *lintCollector) finish(ctx context.Context, binding domain.CursorBinding) (preparedRule, error) {
	slices.SortFunc(c.heap.rows, func(a, b lintCandidate) int {
		return domain.ComparePosition(c.heap.rule, a.pos, b.pos)
	})
	out := preparedRule{count: domain.CapCount(c.count), more: len(c.heap.rows) > c.limit}
	for _, row := range c.heap.rows[:min(c.limit, len(c.heap.rows))] {
		if err := ctx.Err(); err != nil {
			return preparedRule{}, err
		}
		cursor, err := domain.EncodeLintCursor(binding, c.heap.rule, row.pos)
		if err != nil {
			return preparedRule{}, err
		}
		row.item.Cursor = cursor
		out.items = append(out.items, row.item)
	}
	return out, nil
}

func lintCopyPosition(p *domain.Position) *domain.Position {
	if p == nil {
		return nil
	}
	out := *p
	out.ID = lintCopyID(p.ID)
	out.Prefix = slices.Clone(p.Prefix)
	return &out
}

func lintCopyItems(in []domain.Item) []domain.Item {
	out := slices.Clone(in)
	for i := range out {
		out[i].PageID = lintCopyID(out[i].PageID)
		out[i].NodeID = lintCopyID(out[i].NodeID)
	}
	return out
}

// Prepared is an immutable projection of exactly one preparation query.
// Sources and D are never re-read by its projection methods.
type Prepared struct {
	stale   int64
	results map[domain.Rule]preparedRule
	timings PreparationTimings
}

// StalePages counts each old indexed page once, across all selected rules.
func (p *Prepared) StalePages() int64 { return p.stale }

// RuleCount is a value copy. Rules not in the preparation are internal errors.
func (p *Prepared) RuleCount(ctx context.Context, rule domain.Rule) (domain.Count, error) {
	if err := ctx.Err(); err != nil {
		return domain.Count{}, err
	}
	if p == nil {
		return domain.Count{}, errLintQuery
	}
	out, ok := p.results[rule]
	if !ok {
		return domain.Count{}, errLintQuery
	}
	return out.count, nil
}

// RuleItems copies pointer fields as well as the slice. more means there is a
// following item; the next cursor is the last actually returned item's cursor.
func (p *Prepared) RuleItems(ctx context.Context, rule domain.Rule) ([]domain.Item, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if p == nil {
		return nil, false, errLintQuery
	}
	out, ok := p.results[rule]
	if !ok {
		return nil, false, errLintQuery
	}
	return lintCopyItems(out.items), out.more, nil
}
