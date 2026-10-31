package httpadapter

import (
	"errors"

	"github.com/oapi-codegen/nullable"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

var errLintProjection = errors.New("wiki lint: invalid HTTP result")

func lintNextCursor(value *string) nullable.Nullable[string] {
	if value == nil {
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(*value)
}

func lintSummary(rule domain.LintRuleResult) (gen.LintRuleSummary, error) {
	var out gen.LintRuleSummary
	var err error
	next := lintNextCursor(rule.NextCursor)
	switch rule.Rule {
	case domain.RuleWanted:
		err = out.FromLintWantedSummary(gen.LintWantedSummary{
			Rule: gen.LintWantedSummaryRuleWanted, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintWanted), NextCursor: next,
		})
	case domain.RuleOrphans:
		err = out.FromLintOrphansSummary(gen.LintOrphansSummary{
			Rule: gen.LintOrphansSummaryRuleOrphans, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintOrphan), NextCursor: next,
		})
	case domain.RuleAmbiguous:
		err = out.FromLintAmbiguousSummary(gen.LintAmbiguousSummary{
			Rule: gen.LintAmbiguousSummaryRuleAmbiguous, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintAmbiguous), NextCursor: next,
		})
	case domain.RuleInvalidFrontmatter:
		err = out.FromLintInvalidFrontmatterSummary(gen.LintInvalidFrontmatterSummary{
			Rule: gen.LintInvalidFrontmatterSummaryRuleInvalidFrontmatter, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintFrontmatter), NextCursor: next,
		})
	case domain.RuleMissingType:
		err = out.FromLintMissingTypeSummary(gen.LintMissingTypeSummary{
			Rule: gen.LintMissingTypeSummaryRuleMissingType, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintMissingType), NextCursor: next,
		})
	case domain.RuleMissingSummary:
		err = out.FromLintMissingSummarySummary(gen.LintMissingSummarySummary{
			Rule: gen.LintMissingSummarySummaryRuleMissingSummary, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintMissingSummary), NextCursor: next,
		})
	case domain.RuleUndigestedSources:
		err = out.FromLintUndigestedSourcesSummary(gen.LintUndigestedSourcesSummary{
			Rule: gen.LintUndigestedSourcesSummaryRuleUndigestedSources, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintUndigested), NextCursor: next,
		})
	case domain.RuleInvalidSources:
		err = out.FromLintInvalidSourcesSummary(gen.LintInvalidSourcesSummary{
			Rule: gen.LintInvalidSourcesSummaryRuleInvalidSources, Count: rule.Count.Value, CountCapped: rule.Count.Capped,
			Items: lintMapItems(rule.Items, lintInvalidSource), NextCursor: next,
		})
	default:
		err = errLintProjection
	}
	return out, err
}

func lintMapItems[T any](items []domain.LintItem, project func(domain.LintItem) T) []T {
	out := make([]T, len(items))
	for i, item := range items {
		out[i] = project(item)
	}
	return out
}

func lintItem(item domain.LintItem) (gen.LintItem, error) {
	var out gen.LintItem
	var err error
	switch item.Rule {
	case domain.RuleWanted:
		err = out.FromLintWantedItem(lintWanted(item))
	case domain.RuleOrphans:
		err = out.FromLintOrphansItem(lintOrphan(item))
	case domain.RuleAmbiguous:
		err = out.FromLintAmbiguousItem(lintAmbiguous(item))
	case domain.RuleInvalidFrontmatter:
		err = out.FromLintInvalidFrontmatterItem(lintFrontmatter(item))
	case domain.RuleMissingType:
		err = out.FromLintMissingTypeItem(lintMissingType(item))
	case domain.RuleMissingSummary:
		err = out.FromLintMissingSummaryItem(lintMissingSummary(item))
	case domain.RuleUndigestedSources:
		err = out.FromLintUndigestedSourcesItem(lintUndigested(item))
	case domain.RuleInvalidSources:
		err = out.FromLintInvalidSourcesItem(lintInvalidSource(item))
	default:
		err = errLintProjection
	}
	return out, err
}

func lintWanted(item domain.LintItem) gen.LintWantedItem {
	return gen.LintWantedItem{Rule: gen.LintWantedItemRuleWanted, ItemKey: item.ItemKey, Cursor: item.Cursor,
		Target: item.Target, TargetCut: item.TargetCut, Pages: item.Pages}
}

func lintOrphan(item domain.LintItem) gen.LintOrphansItem {
	return gen.LintOrphansItem{Rule: gen.LintOrphansItemRuleOrphans, ItemKey: item.ItemKey, Cursor: item.Cursor, PageID: *item.PageID}
}

func lintAmbiguous(item domain.LintItem) gen.LintAmbiguousItem {
	return gen.LintAmbiguousItem{Rule: gen.LintAmbiguousItemRuleAmbiguous, ItemKey: item.ItemKey, Cursor: item.Cursor,
		PageID: *item.PageID, Target: item.Target, TargetCut: item.TargetCut}
}

func lintFrontmatter(item domain.LintItem) gen.LintInvalidFrontmatterItem {
	return gen.LintInvalidFrontmatterItem{Rule: gen.LintInvalidFrontmatterItemRuleInvalidFrontmatter, ItemKey: item.ItemKey, Cursor: item.Cursor, PageID: *item.PageID}
}

func lintMissingType(item domain.LintItem) gen.LintMissingTypeItem {
	return gen.LintMissingTypeItem{Rule: gen.LintMissingTypeItemRuleMissingType, ItemKey: item.ItemKey, Cursor: item.Cursor, PageID: *item.PageID}
}

func lintMissingSummary(item domain.LintItem) gen.LintMissingSummaryItem {
	return gen.LintMissingSummaryItem{Rule: gen.LintMissingSummaryItemRuleMissingSummary, ItemKey: item.ItemKey, Cursor: item.Cursor, PageID: *item.PageID}
}

func lintUndigested(item domain.LintItem) gen.LintUndigestedSourcesItem {
	return gen.LintUndigestedSourcesItem{Rule: gen.LintUndigestedSourcesItemRuleUndigestedSources, ItemKey: item.ItemKey, Cursor: item.Cursor,
		NodeID: *item.NodeID, Kind: gen.LintNodeKind(item.Kind), Bytes: item.Bytes, CreatedAt: item.CreatedAt}
}

func lintInvalidSource(item domain.LintItem) gen.LintInvalidSourcesItem {
	return gen.LintInvalidSourcesItem{Rule: gen.LintInvalidSourcesItemRuleInvalidSources, ItemKey: item.ItemKey, Cursor: item.Cursor,
		PageID: *item.PageID, Target: item.Target, TargetCut: item.TargetCut, NotLink: item.NotLink}
}
