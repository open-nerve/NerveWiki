package mcpadapter

import (
	"errors"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

var errLintProjection = errors.New("wiki lint: invalid MCP result")
var errNoLintItemFits = errors.New("wiki lint: first item does not fit in an answer")

type lintEnvelope struct {
	Mode       domain.Mode `json:"mode"`
	ComputedAt time.Time   `json:"computed_at"`
	StalePages int64       `json:"stale_pages"`
}

type lintSummary struct {
	lintEnvelope
	Rules []lintRuleSummary `json:"rules"`
}

type lintRuleSummary struct {
	Rule        domain.LintRule `json:"rule"`
	Count       int64           `json:"count"`
	CountCapped bool            `json:"count_capped"`
	Items       []lintWireItem  `json:"items"`
	NextCursor  *string         `json:"next_cursor"`
}

type lintPage struct {
	lintEnvelope
	Data       []lintWireItem `json:"data"`
	NextCursor *string        `json:"next_cursor"`
}

// All item variants are private value types, with no pointers into a caller's
// result. Identical page-only branches share their physical representation.
type lintWireItem interface{ continuation() string }
type lintIdentity struct {
	Rule    domain.LintRule `json:"rule"`
	ItemKey string          `json:"item_key"`
	Cursor  string          `json:"cursor"`
}

func (i lintIdentity) continuation() string { return i.Cursor }

type lintWanted struct {
	lintIdentity
	Target    string `json:"target"`
	TargetCut bool   `json:"target_cut"`
	Pages     int64  `json:"pages"`
}
type lintPageItem struct {
	lintIdentity
	PageID uuid.UUID `json:"page_id"`
	Path   string    `json:"path"`
}
type lintAmbiguous struct {
	lintPageItem
	Target    string `json:"target"`
	TargetCut bool   `json:"target_cut"`
}
type lintInvalidSource struct {
	lintAmbiguous
	NotLink bool `json:"not_link"`
}
type lintSource struct {
	lintIdentity
	NodeID    uuid.UUID       `json:"node_id"`
	Path      string          `json:"path"`
	Kind      domain.NodeKind `json:"kind"`
	Bytes     int64           `json:"bytes"`
	CreatedAt time.Time       `json:"created_at"`
}

type lintRuleView struct {
	rule        domain.LintRule
	count       domain.LintCount
	items       []lintWireItem
	next, start *string
}

// fitLint computes no rule twice. Every candidate has its own rule/item slices
// and continuation pointers; Fit never writes the input or an earlier candidate.
func fitLint(result domain.LintResult) (mcpserver.Result, error) {
	views := make([]lintRuleView, len(result.Rules))
	for i, rule := range result.Rules {
		view := lintRuleView{rule: rule.Rule, count: rule.Count, next: lintCopyCursor(rule.NextCursor), start: lintCopyCursor(rule.StartCursor), items: make([]lintWireItem, len(rule.Items))}
		for j, item := range rule.Items {
			if item.Rule != rule.Rule {
				return mcpserver.Result{}, errLintProjection
			}
			projected, err := lintProjectItem(item)
			if err != nil {
				return mcpserver.Result{}, err
			}
			view.items[j] = projected
		}
		views[i] = view
	}
	envelope := lintEnvelope{Mode: result.Mode, ComputedAt: result.ComputedAt, StalePages: result.StalePages}
	switch result.Kind {
	case domain.LintSummary:
		for _, view := range views {
			if view.count.Value > 0 && (view.start == nil || *view.start == "") {
				return mcpserver.Result{}, errLintProjection
			}
		}
		out, _, err := mcpserver.Fit(5, func(k int) any { return lintSummaryCandidate(envelope, views, k) })
		return out, err
	case domain.LintItems:
		if len(views) != 1 {
			return mcpserver.Result{}, errLintProjection
		}
		view := views[0]
		out, k, err := mcpserver.Fit(len(view.items), func(k int) any {
			return lintPage{lintEnvelope: envelope, Data: lintPrefix(view.items, k), NextCursor: lintAfter(view, k)}
		})
		if err != nil {
			return mcpserver.Result{}, err
		}
		if k == 0 && len(view.items) > 0 {
			return mcpserver.Result{}, errNoLintItemFits
		}
		return out, nil
	}
	return mcpserver.Result{}, errLintProjection
}

func lintSummaryCandidate(envelope lintEnvelope, views []lintRuleView, k int) lintSummary {
	out := lintSummary{lintEnvelope: envelope, Rules: make([]lintRuleSummary, len(views))}
	for i, view := range views {
		n := min(k, len(view.items))
		next := lintAfter(view, n)
		if k == 0 && view.count.Value > 0 {
			next = lintCopyCursor(view.start)
		}
		out.Rules[i] = lintRuleSummary{Rule: view.rule, Count: view.count.Value, CountCapped: view.count.Capped, Items: lintPrefix(view.items, n), NextCursor: next}
	}
	return out
}

func lintPrefix(items []lintWireItem, n int) []lintWireItem {
	out := make([]lintWireItem, n)
	copy(out, items[:n])
	return out
}

func lintAfter(view lintRuleView, n int) *string {
	if n == 0 && len(view.items) > 0 {
		return nil
	}
	if n < len(view.items) {
		return new(view.items[n-1].continuation())
	}
	return lintCopyCursor(view.next)
}

func lintCopyCursor(value *string) *string {
	if value == nil {
		return nil
	}
	return new(*value)
}

func lintProjectItem(item domain.LintItem) (lintWireItem, error) {
	identity := lintIdentity{Rule: item.Rule, ItemKey: item.ItemKey, Cursor: item.Cursor}
	if item.Rule == domain.RuleWanted {
		return lintWanted{lintIdentity: identity, Target: item.Target, TargetCut: item.TargetCut, Pages: item.Pages}, nil
	}
	if item.Path == nil {
		return nil, errLintProjection
	}
	if item.Rule == domain.RuleUndigestedSources {
		if item.NodeID == nil {
			return nil, errLintProjection
		}
		return lintSource{lintIdentity: identity, NodeID: *item.NodeID, Path: *item.Path, Kind: item.Kind, Bytes: item.Bytes, CreatedAt: item.CreatedAt}, nil
	}
	if item.PageID == nil {
		return nil, errLintProjection
	}
	page := lintPageItem{lintIdentity: identity, PageID: *item.PageID, Path: *item.Path}
	switch item.Rule {
	case domain.RuleOrphans, domain.RuleInvalidFrontmatter, domain.RuleMissingType, domain.RuleMissingSummary:
		return page, nil
	case domain.RuleAmbiguous:
		return lintAmbiguous{lintPageItem: page, Target: item.Target, TargetCut: item.TargetCut}, nil
	case domain.RuleInvalidSources:
		return lintInvalidSource{lintAmbiguous: lintAmbiguous{lintPageItem: page, Target: item.Target, TargetCut: item.TargetCut}, NotLink: item.NotLink}, nil
	}
	return nil, errLintProjection
}
