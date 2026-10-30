package app

import (
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
)

// Rules consumes bounded index streams and static rules in a caller-owned
// snapshot. It is stateless between requests and never owns a transaction.
type Rules struct {
	reads  *LintReads
	store  LintRuleStore
	assets AssetSizes
}

// NewRules keeps the existing bounded stream implementation, including for
// tests that replace the database ports underneath it.
func NewRules(reads *LintReads, store LintRuleStore, assets AssetSizes) *Rules {
	return &Rules{reads: reads, store: store, assets: assets}
}

type lintPlan struct {
	rules []domain.Rule
	after *domain.Position
	limit int
}

func lintCopyQuery(q RuleQuery) RuleQuery {
	if q.Rule != nil {
		r := *q.Rule
		q.Rule = &r
	}
	q.After = lintCopyPosition(q.After)
	return q
}

func lintQuery(q RuleQuery, s *ReadScope) (lintPlan, error) {
	allowed := domain.ApplicableRules(s.mode)
	if q.Rule == nil {
		if q.After != nil || q.Limit != 0 {
			return lintPlan{}, errLintQuery
		}
		return lintPlan{rules: allowed, limit: 5}, nil
	}
	if !slices.Contains(allowed, *q.Rule) || q.Limit < 0 || q.Limit > 100 {
		return lintPlan{}, errLintQuery
	}
	limit := q.Limit
	if limit == 0 {
		limit = 50
	}
	if q.After != nil {
		// Validate a caller's decoded internal value too. No public API error
		// or authorization ordering is invented by this module-level entry.
		if _, err := domain.EncodeLintCursor(s.binding, *q.Rule, *q.After); err != nil {
			return lintPlan{}, errLintQuery
		}
		if q.After.Begin {
			q.After = nil
		}
	}
	return lintPlan{rules: []domain.Rule{*q.Rule}, after: q.After, limit: limit}, nil
}

// Prepare completes every requested count/page before publishing a result.
// Input ownership is taken before the first port call; errors discard all
// accumulated state. Prepare does not authenticate or start a snapshot.
func (r *Rules) Prepare(ctx context.Context, input ReadInput, query RuleQuery) (*Prepared, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || r.reads == nil || r.store == nil {
		return nil, errLintQuery
	}
	query = lintCopyQuery(query)
	in, err := lintCopyInput(ctx, input)
	if err != nil {
		return nil, err
	}
	heads, err := r.reads.Heads(ctx, in.NotebookID)
	if err != nil {
		return nil, err
	}
	scope, err := newReadScope(ctx, in, heads)
	if err != nil {
		return nil, err
	}
	plan, err := lintQuery(query, scope)
	if err != nil {
		return nil, err
	}
	out := &Prepared{stale: scope.stale, results: make(map[domain.Rule]preparedRule, len(plan.rules))}
	var sources *sourceResults
	if slices.Contains(plan.rules, domain.InvalidSources) || slices.Contains(plan.rules, domain.UndigestedSources) {
		started := time.Now()
		sources, err = r.prepareSources(ctx, scope, plan)
		if err != nil {
			return nil, err
		}
		elapsed := time.Since(started)
		out.timings.SharedSources = &elapsed
	}
	for _, rule := range plan.rules {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var result preparedRule
		started := time.Now()
		switch rule {
		case domain.Wanted:
			result, err = r.prepareWanted(ctx, scope, plan)
		case domain.Ambiguous:
			result, err = r.prepareAmbiguous(ctx, scope, plan)
		case domain.InvalidSources:
			result = sources.invalid
		case domain.UndigestedSources:
			result, err = r.preparePending(ctx, scope, plan, sources)
		default:
			result, err = r.preparePages(ctx, scope, plan, rule)
		}
		if err != nil {
			return nil, err
		}
		out.results[rule] = result
		out.timings.Rules = append(out.timings.Rules, RuleTiming{Rule: rule, Duration: time.Since(started)})
	}
	if err := r.projectAssets(ctx, scope, out); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Rules) projectAssets(ctx context.Context, s *ReadScope, p *Prepared) error {
	result, ok := p.results[domain.UndigestedSources]
	if !ok {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(result.items))
	seen := make(map[uuid.UUID]struct{})
	for _, item := range result.items {
		if item.Kind != domain.NodeAsset {
			continue
		}
		if item.NodeID == nil {
			return errLintRows
		}
		if _, duplicate := seen[*item.NodeID]; !duplicate {
			seen[*item.NodeID] = struct{}{}
			ids = append(ids, *item.NodeID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if r.assets == nil {
		return errLintRows
	}
	lintIDs(ids)
	sizes, err := r.assets.BytesOf(ctx, s.notebook, ids)
	if err != nil {
		return err
	}
	for i := range result.items {
		item := &result.items[i]
		if item.Kind != domain.NodeAsset {
			continue
		}
		size, found := sizes[*item.NodeID]
		if !found || size < 0 {
			return errLintRows
		}
		item.Bytes = size
	}
	p.results[domain.UndigestedSources] = result
	return nil
}

func (r *Rules) preparePages(ctx context.Context, s *ReadScope, plan lintPlan, rule domain.Rule) (preparedRule, error) {
	input := PageRuleInput{NotebookID: s.notebook}
	var count func(context.Context, PageRuleInput) (domain.Count, error)
	var items func(context.Context, PageRuleInput, *uuid.UUID, int) ([]uuid.UUID, error)
	switch rule {
	case domain.Orphans:
		input.CandidateIDs, input.ExcludedEmitters = s.orphans, s.excluded
		count, items = r.store.LintOrphanCount, r.store.LintOrphanItems
	case domain.InvalidFrontmatter:
		input.CandidateIDs = s.frontmatter
		count, items = r.store.LintFrontmatterCount, r.store.LintFrontmatterItems
	case domain.MissingType:
		input.CandidateIDs = s.missing
		count, items = r.store.LintMissingTypeCount, r.store.LintMissingTypeItems
	case domain.MissingSummary:
		input.CandidateIDs = s.missing
		count, items = r.store.LintMissingSummaryCount, r.store.LintMissingSummaryItems
	default:
		return preparedRule{}, errLintQuery
	}
	if len(input.CandidateIDs) == 0 {
		return preparedRule{}, nil
	}
	n, err := count(ctx, input)
	if err != nil {
		return preparedRule{}, err
	}
	if n.Value < 0 || n.Value > 10000 || n.Capped != (n.Value == 10000) {
		return preparedRule{}, errLintRows
	}
	if n.Value == 0 {
		return preparedRule{count: n}, ctx.Err()
	}
	var after *uuid.UUID
	if plan.after != nil {
		after = plan.after.ID
	}
	ids, err := items(ctx, input, after, plan.limit+1)
	if err != nil {
		return preparedRule{}, err
	}
	if len(ids) > plan.limit+1 || int64(len(ids)) > n.Value || after == nil && n.Value > 0 && len(ids) == 0 {
		return preparedRule{}, errLintRows
	}
	c := newLintCollector(rule, plan.after, plan.limit)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return preparedRule{}, err
		}
		_, member := slices.BinarySearchFunc(input.CandidateIDs, id, func(a, b uuid.UUID) int { return a.Compare(b) })
		if !member || i > 0 && ids[i-1].Compare(id) >= 0 || after != nil && id.Compare(*after) <= 0 {
			return preparedRule{}, errLintRows
		}
		item, pos, err := domain.PageItem(rule, id)
		if err != nil {
			return preparedRule{}, err
		}
		c.add(item, pos)
	}
	out, err := c.finish(ctx, s.binding)
	out.count = n
	return out, err
}
