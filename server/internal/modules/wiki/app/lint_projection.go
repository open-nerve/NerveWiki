package app

import (
	"context"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func (l *Lint) project(ctx context.Context, scope LintScope, query RuleQuery, prepared PreparedRules, result *domain.LintResult, stats *RunStats) error {
	rules, limit := domain.LintRules(result.Mode), 5
	if query.Rule != nil {
		rules, limit = []domain.LintRule{*query.Rule}, query.Limit
	}
	timings := prepared.Timings()
	stats.SharedSources = lintCopy(timings.SharedSources)
	if stats.SharedSources != nil && *stats.SharedSources < 0 {
		return errLintInvariant
	}
	durations := make(map[domain.LintRule]time.Duration, len(timings.Rules))
	for _, timing := range timings.Rules {
		if _, duplicate := durations[timing.Rule]; duplicate || timing.Duration < 0 {
			return errLintInvariant
		}
		durations[timing.Rule] = timing.Duration
	}
	result.Rules = make([]domain.LintRuleResult, 0, len(rules))
	var ids []uuid.UUID
	seen := make(map[uuid.UUID]struct{})
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := prepared.RuleCount(ctx, rule)
		if err != nil {
			return err
		}
		if count.Value < 0 || count.Value > 10000 || count.Capped != (count.Value == 10000) {
			return errLintInvariant
		}
		if duration, ok := durations[rule]; ok {
			stats.Rules = append(stats.Rules, RuleRunStat{Rule: rule, Duration: duration, Count: count})
		}
		items, more, err := prepared.RuleItems(ctx, rule)
		if err != nil {
			return err
		}
		if len(items) > limit || int64(len(items)) > count.Value || (more && len(items) == 0) {
			return errLintInvariant
		}
		row := domain.LintRuleResult{Rule: rule, Count: count, Items: make([]domain.LintItem, len(items))}
		for i, item := range items {
			id, err := lintItemID(rule, item)
			if err != nil {
				return err
			}
			item.PageID, item.NodeID, item.Path = lintCopy(item.PageID), lintCopy(item.NodeID), nil
			row.Items[i] = item
			if id != nil {
				if _, exists := seen[*id]; !exists {
					seen[*id] = struct{}{}
					ids = append(ids, *id)
				}
			}
		}
		if more {
			row.NextCursor = lintCopy(&row.Items[len(row.Items)-1].Cursor)
		}
		if count.Value > 0 {
			start, err := l.deps.Cursors.Encode(scope, rule, domain.LintPosition{Begin: true})
			if err != nil {
				return err
			}
			row.StartCursor = &start
		}
		result.Rules = append(result.Rules, row)
	}
	if len(ids) == 0 {
		return nil
	}
	paths, err := l.deps.Paths.Paths(ctx, scope.NotebookID, ids)
	if err != nil {
		return err
	}
	for i := range result.Rules {
		for j := range result.Rules[i].Items {
			item := &result.Rules[i].Items[j]
			id := item.PageID
			if id == nil {
				id = item.NodeID
			}
			if id != nil {
				path, ok := paths[*id]
				if !ok {
					return errLintInvariant
				}
				item.Path = &path
			}
		}
	}
	return nil
}

func lintItemID(rule domain.LintRule, item domain.LintItem) (*uuid.UUID, error) {
	if item.Rule != rule || item.ItemKey == "" || item.Cursor == "" {
		return nil, errLintInvariant
	}
	switch rule {
	case domain.RuleWanted:
		if item.PageID == nil && item.NodeID == nil {
			return nil, nil
		}
	case domain.RuleUndigestedSources:
		if item.NodeID != nil && item.PageID == nil && item.Bytes >= 0 &&
			(item.Kind == domain.NodePage || item.Kind == domain.NodeAsset) {
			return item.NodeID, nil
		}
	default:
		if item.PageID != nil && item.NodeID == nil {
			return item.PageID, nil
		}
	}
	return nil, errLintInvariant
}
