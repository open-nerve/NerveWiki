package mcpadapter

import (
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// An upgrade's check as upgrade_verify answers it (M11/P2 design 3.14):
// REST's UpgradeCheck, its gates naming their nodes by path (null for one
// not in the live tree), its rules lint's summary's, items with paths.

type upgradeCheckAnswer struct {
	ComputedAt time.Time           `json:"computed_at"`
	StalePages int64               `json:"stale_pages"`
	Passed     bool                `json:"passed"`
	Gates      upgradeGatesAnswer  `json:"gates"`
	Rules      []lintRuleSummary   `json:"rules"`
	Counts     upgradeCountsAnswer `json:"counts"`
}

type upgradeGate struct {
	Passed bool `json:"passed"`
}

type upgradePathsGate struct {
	Passed bool      `json:"passed"`
	Paths  []*string `json:"paths"`
}

type upgradeLinksGate struct {
	Passed     bool      `json:"passed"`
	PagesWorse int       `json:"pages_worse"`
	Paths      []*string `json:"paths"`
}

type upgradePreservedGate struct {
	Passed    bool      `json:"passed"`
	Nodes     int       `json:"nodes"`
	NodesKept int       `json:"nodes_kept"`
	Pages     int       `json:"pages"`
	PagesKept int       `json:"pages_kept"`
	Paths     []*string `json:"paths"`
}

type upgradeGatesAnswer struct {
	Agents         upgradeGate          `json:"agents"`
	SourceRoot     upgradeGate          `json:"source_root"`
	ReservedTitles upgradePathsGate     `json:"reserved_titles"`
	Frontmatter    upgradeGate          `json:"frontmatter"`
	Sources        upgradeGate          `json:"sources"`
	Links          upgradeLinksGate     `json:"links"`
	Preserved      upgradePreservedGate `json:"preserved"`
	Index          upgradeGate          `json:"index"`
}

type upgradeWanted struct {
	Count       int64 `json:"count"`
	CountCapped bool  `json:"count_capped"`
}

type upgradeCountsAnswer struct {
	Sources          int           `json:"sources"`
	MovedIntoSources int           `json:"moved_into_sources"`
	WikiPages        int           `json:"wiki_pages"`
	NewPages         int           `json:"new_pages"`
	Changesets       int           `json:"changesets"`
	WantedBefore     upgradeWanted `json:"wanted_before"`
	WantedNow        upgradeWanted `json:"wanted_now"`
}

// maxGatePaths is the most nodes a gate names (M11 design 4.7).
const maxGatePaths = 5

// fitUpgradeCheck is c within MaxAnswerBytes: first the most items of
// each rule, five to none, that fit, as lint's summary; when none fits,
// none of each rule's and the most paths of each gate that fit, the
// gates' counts and passed whole. Neither fitting is a defect: three
// gates name at most 14 nodes, each path at most MaxDepth titles.
func fitUpgradeCheck(c domain.UpgradeCheck) (mcpserver.Result, error) {
	views, err := lintViews(c.Rules)
	if err != nil {
		return mcpserver.Result{}, err
	}
	if err := lintStarts(views); err != nil {
		return mcpserver.Result{}, err
	}
	envelope := lintEnvelope{Mode: domain.ModeLLMWiki, ComputedAt: c.ComputedAt, StalePages: c.StalePages}
	paths := func(ids []uuid.UUID, m int) []*string {
		out := make([]*string, 0, min(len(ids), m))
		for _, id := range ids[:min(len(ids), m)] {
			if p, ok := c.Paths[id]; ok {
				out = append(out, &p)
			} else {
				out = append(out, nil)
			}
		}
		return out
	}
	g := c.Gates
	candidate := func(k, m int) any {
		return upgradeCheckAnswer{
			ComputedAt: c.ComputedAt, StalePages: c.StalePages, Passed: c.Passed,
			Gates: upgradeGatesAnswer{
				Agents: upgradeGate{g.Agents}, SourceRoot: upgradeGate{g.SourceRoot},
				ReservedTitles: upgradePathsGate{g.ReservedTitles.Passed, paths(g.ReservedTitles.NodeIDs, m)},
				Frontmatter:    upgradeGate{g.Frontmatter}, Sources: upgradeGate{g.Sources},
				Links: upgradeLinksGate{g.Links.Passed, g.Links.PagesWorse, paths(g.Links.NodeIDs, m)},
				Preserved: upgradePreservedGate{g.Preserved.Passed, g.Preserved.Nodes, g.Preserved.NodesKept, g.Preserved.Pages,
					g.Preserved.PagesKept, paths(g.Preserved.NodeIDs, m)},
				Index: upgradeGate{g.Index},
			},
			Rules: lintSummaryCandidate(envelope, views, k).Rules,
			Counts: upgradeCountsAnswer{Sources: c.Counts.Sources, MovedIntoSources: c.Counts.MovedIntoSources,
				WikiPages: c.Counts.WikiPages, NewPages: c.Counts.NewPages, Changesets: c.Counts.Changesets,
				WantedBefore: upgradeWanted{c.Counts.WantedBefore.Value, c.Counts.WantedBefore.Capped},
				WantedNow:    upgradeWanted{c.Counts.WantedNow.Value, c.Counts.WantedNow.Capped}},
		}
	}
	if out, _, err := mcpserver.Fit(5, func(k int) any { return candidate(k, maxGatePaths) }); err == nil {
		return out, nil
	}
	out, _, err := mcpserver.Fit(maxGatePaths, func(m int) any { return candidate(0, m) })
	return out, err
}
