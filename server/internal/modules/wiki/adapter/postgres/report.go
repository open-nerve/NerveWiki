package postgresadapter

import (
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// The report a submission keeps (M11/P2 design 3.12, 3.13), as
// notebook_upgrades.report holds it: its members named as the contract's
// UpgradeReport names them, an item's those of its rule's item, the
// members a rule's item lacks left out. One encoding stores and reads it;
// a gate's no nodes read back as none (nil).

type reportJSON struct {
	ComputedAt time.Time        `json:"computed_at"`
	StalePages int64            `json:"stale_pages"`
	Passed     bool             `json:"passed"`
	Gates      reportGatesJSON  `json:"gates"`
	Rules      []reportRuleJSON `json:"rules"`
	Counts     reportCountsJSON `json:"counts"`
}

type reportGateJSON struct {
	Passed bool `json:"passed"`
}

type reportNodesGateJSON struct {
	Passed  bool        `json:"passed"`
	NodeIDs []uuid.UUID `json:"node_ids"`
}

type reportLinksGateJSON struct {
	Passed     bool        `json:"passed"`
	PagesWorse int         `json:"pages_worse"`
	NodeIDs    []uuid.UUID `json:"node_ids"`
}

type reportPreservedGateJSON struct {
	Passed    bool        `json:"passed"`
	Nodes     int         `json:"nodes"`
	NodesKept int         `json:"nodes_kept"`
	Pages     int         `json:"pages"`
	PagesKept int         `json:"pages_kept"`
	NodeIDs   []uuid.UUID `json:"node_ids"`
}

type reportGatesJSON struct {
	Agents         reportGateJSON          `json:"agents"`
	SourceRoot     reportGateJSON          `json:"source_root"`
	ReservedTitles reportNodesGateJSON     `json:"reserved_titles"`
	Frontmatter    reportGateJSON          `json:"frontmatter"`
	Sources        reportGateJSON          `json:"sources"`
	Links          reportLinksGateJSON     `json:"links"`
	Preserved      reportPreservedGateJSON `json:"preserved"`
	Index          reportGateJSON          `json:"index"`
}

type reportWantedJSON struct {
	Count       int64 `json:"count"`
	CountCapped bool  `json:"count_capped"`
}

type reportCountsJSON struct {
	Sources          int              `json:"sources"`
	MovedIntoSources int              `json:"moved_into_sources"`
	WikiPages        int              `json:"wiki_pages"`
	NewPages         int              `json:"new_pages"`
	Changesets       int              `json:"changesets"`
	WantedBefore     reportWantedJSON `json:"wanted_before"`
	WantedNow        reportWantedJSON `json:"wanted_now"`
}

type reportRuleJSON struct {
	Rule        domain.LintRule  `json:"rule"`
	Count       int64            `json:"count"`
	CountCapped bool             `json:"count_capped"`
	Items       []reportItemJSON `json:"items"`
}

// reportItemJSON is an item of any rule: the members its rule's item has,
// the others zero and left out.
type reportItemJSON struct {
	Rule      domain.LintRule `json:"rule"`
	ItemKey   string          `json:"item_key"`
	PageID    *uuid.UUID      `json:"page_id,omitempty"`
	NodeID    *uuid.UUID      `json:"node_id,omitempty"`
	Target    string          `json:"target,omitzero"`
	TargetCut bool            `json:"target_cut,omitzero"`
	Pages     int64           `json:"pages,omitzero"`
	NotLink   bool            `json:"not_link,omitzero"`
	Kind      domain.NodeKind `json:"kind,omitzero"`
	Bytes     int64           `json:"bytes,omitzero"`
	CreatedAt time.Time       `json:"created_at,omitzero"`
}

// encodeReport is r as notebook_upgrades.report holds it.
func encodeReport(r domain.UpgradeReport) ([]byte, error) {
	g := r.Gates
	ids := func(in []uuid.UUID) []uuid.UUID { return append([]uuid.UUID{}, in...) }
	out := reportJSON{
		ComputedAt: r.ComputedAt, StalePages: r.StalePages, Passed: r.Passed,
		Gates: reportGatesJSON{
			Agents: reportGateJSON{g.Agents}, SourceRoot: reportGateJSON{g.SourceRoot},
			ReservedTitles: reportNodesGateJSON{g.ReservedTitles.Passed, ids(g.ReservedTitles.NodeIDs)},
			Frontmatter:    reportGateJSON{g.Frontmatter}, Sources: reportGateJSON{g.Sources},
			Links: reportLinksGateJSON{g.Links.Passed, g.Links.PagesWorse, ids(g.Links.NodeIDs)},
			Preserved: reportPreservedGateJSON{g.Preserved.Passed, g.Preserved.Nodes, g.Preserved.NodesKept, g.Preserved.Pages,
				g.Preserved.PagesKept, ids(g.Preserved.NodeIDs)},
			Index: reportGateJSON{g.Index},
		},
		Counts: reportCountsJSON{
			Sources: r.Counts.Sources, MovedIntoSources: r.Counts.MovedIntoSources, WikiPages: r.Counts.WikiPages,
			NewPages: r.Counts.NewPages, Changesets: r.Counts.Changesets,
			WantedBefore: reportWantedJSON{r.Counts.WantedBefore.Value, r.Counts.WantedBefore.Capped},
			WantedNow:    reportWantedJSON{r.Counts.WantedNow.Value, r.Counts.WantedNow.Capped},
		},
		Rules: make([]reportRuleJSON, len(r.Rules)),
	}
	for i, rule := range r.Rules {
		items := make([]reportItemJSON, len(rule.Items))
		for j, it := range rule.Items {
			items[j] = reportItemJSON{Rule: it.Rule, ItemKey: it.ItemKey, PageID: it.PageID, NodeID: it.NodeID, Target: it.Target,
				TargetCut: it.TargetCut, Pages: it.Pages, NotLink: it.NotLink, Kind: it.Kind, Bytes: it.Bytes, CreatedAt: it.CreatedAt}
		}
		out.Rules[i] = reportRuleJSON{Rule: rule.Rule, Count: rule.Count.Value, CountCapped: rule.Count.Capped, Items: items}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("encode an upgrade's report: %w", err)
	}
	return b, nil
}

// decodeReport is the report raw holds, as encodeReport wrote it.
func decodeReport(raw []byte) (*domain.UpgradeReport, error) {
	var in reportJSON
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode an upgrade's report: %w", err)
	}
	ids := func(in []uuid.UUID) []uuid.UUID {
		if len(in) == 0 {
			return nil
		}
		return in
	}
	g := in.Gates
	out := &domain.UpgradeReport{
		ComputedAt: in.ComputedAt, StalePages: in.StalePages, Passed: in.Passed,
		Gates: domain.UpgradeGates{
			Agents: g.Agents.Passed, SourceRoot: g.SourceRoot.Passed, Frontmatter: g.Frontmatter.Passed, Sources: g.Sources.Passed,
			Index:          g.Index.Passed,
			ReservedTitles: domain.NodesGate{Passed: g.ReservedTitles.Passed, NodeIDs: ids(g.ReservedTitles.NodeIDs)},
			Links:          domain.LinksGate{Passed: g.Links.Passed, PagesWorse: g.Links.PagesWorse, NodeIDs: ids(g.Links.NodeIDs)},
			Preserved: domain.PreservedGate{Passed: g.Preserved.Passed, Nodes: g.Preserved.Nodes, NodesKept: g.Preserved.NodesKept,
				Pages: g.Preserved.Pages, PagesKept: g.Preserved.PagesKept, NodeIDs: ids(g.Preserved.NodeIDs)},
		},
		Counts: domain.UpgradeCounts{
			Sources: in.Counts.Sources, MovedIntoSources: in.Counts.MovedIntoSources, WikiPages: in.Counts.WikiPages,
			NewPages: in.Counts.NewPages, Changesets: in.Counts.Changesets,
			WantedBefore: domain.LintCount{Value: in.Counts.WantedBefore.Count, Capped: in.Counts.WantedBefore.CountCapped},
			WantedNow:    domain.LintCount{Value: in.Counts.WantedNow.Count, Capped: in.Counts.WantedNow.CountCapped},
		},
		Rules: make([]domain.ReportRule, len(in.Rules)),
	}
	for i, rule := range in.Rules {
		items := make([]domain.LintItem, len(rule.Items))
		for j, it := range rule.Items {
			items[j] = domain.LintItem{Rule: it.Rule, ItemKey: it.ItemKey, PageID: it.PageID, NodeID: it.NodeID, Target: it.Target,
				TargetCut: it.TargetCut, Pages: it.Pages, NotLink: it.NotLink, Kind: it.Kind, Bytes: it.Bytes, CreatedAt: it.CreatedAt}
		}
		out.Rules[i] = domain.ReportRule{Rule: rule.Rule, Count: domain.LintCount{Value: rule.Count, Capped: rule.CountCapped},
			Items: items}
	}
	return out, nil
}
