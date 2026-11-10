package httpadapter

import (
	"context"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http/gen"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func (h handler) CheckUpgrade(ctx context.Context, req gen.CheckUpgradeRequestObject) (gen.CheckUpgradeResponseObject, error) {
	checked, err := h.upgrades.CheckUpgrade(ctx, req.NotebookID, app.LintQuery{})
	if err != nil {
		return nil, err
	}
	c := checked.Check
	if c == nil {
		return nil, errLintProjection
	}
	out := gen.CheckUpgrade200JSONResponse{ComputedAt: c.ComputedAt, StalePages: c.StalePages, Passed: c.Passed,
		Gates: upgradeGatesOf(c.Gates), Counts: upgradeCountsOf(c.Counts), Rules: make([]gen.LintRuleSummary, len(c.Rules))}
	for i, rule := range c.Rules {
		if out.Rules[i], err = lintSummary(rule); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (h handler) ListUpgradeCheckItems(ctx context.Context, req gen.ListUpgradeCheckItemsRequestObject) (gen.ListUpgradeCheckItemsResponseObject,
	error,
) {
	checked, err := h.upgrades.CheckUpgrade(ctx, req.NotebookID, app.LintQuery{
		Rule: &req.Rule, Limit: req.Params.Limit, Cursor: req.Params.Cursor,
	})
	if err != nil {
		return nil, err
	}
	result := checked.Items
	if result == nil || len(result.Rules) != 1 {
		return nil, errLintProjection
	}
	rule := result.Rules[0]
	out := gen.ListUpgradeCheckItems200JSONResponse{
		Mode: gen.LintMode(result.Mode), ComputedAt: result.ComputedAt, StalePages: result.StalePages,
		Data: make([]gen.LintItem, len(rule.Items)), NextCursor: lintNextCursor(rule.NextCursor),
	}
	for i, item := range rule.Items {
		if out.Data[i], err = lintItem(item); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// upgradeGatesOf is g as the contract's UpgradeGates: each gate's nodes a
// list, [] for none.
func upgradeGatesOf(g domain.UpgradeGates) gen.UpgradeGates {
	ids := func(in []uuid.UUID) []uuid.UUID { return append([]uuid.UUID{}, in...) }
	return gen.UpgradeGates{
		Agents: gen.UpgradeGate{Passed: g.Agents}, SourceRoot: gen.UpgradeGate{Passed: g.SourceRoot},
		ReservedTitles: gen.UpgradeTitlesGate{Passed: g.ReservedTitles.Passed, NodeIds: ids(g.ReservedTitles.NodeIDs)},
		Frontmatter:    gen.UpgradeGate{Passed: g.Frontmatter}, Sources: gen.UpgradeGate{Passed: g.Sources},
		Links: gen.UpgradeLinksGate{Passed: g.Links.Passed, PagesWorse: g.Links.PagesWorse, NodeIds: ids(g.Links.NodeIDs)},
		Preserved: gen.UpgradePreservedGate{Passed: g.Preserved.Passed, Nodes: g.Preserved.Nodes, NodesKept: g.Preserved.NodesKept,
			Pages: g.Preserved.Pages, PagesKept: g.Preserved.PagesKept, NodeIds: ids(g.Preserved.NodeIDs)},
		Index: gen.UpgradeGate{Passed: g.Index},
	}
}

// upgradeCountsOf is c as the contract's UpgradeCounts.
func upgradeCountsOf(c domain.UpgradeCounts) gen.UpgradeCounts {
	return gen.UpgradeCounts{Sources: c.Sources, MovedIntoSources: c.MovedIntoSources, WikiPages: c.WikiPages, NewPages: c.NewPages,
		Changesets:   c.Changesets,
		WantedBefore: gen.UpgradeWanted{Count: gen.LintCount(c.WantedBefore.Value), CountCapped: c.WantedBefore.Capped},
		WantedNow:    gen.UpgradeWanted{Count: gen.LintCount(c.WantedNow.Value), CountCapped: c.WantedNow.Capped}}
}
