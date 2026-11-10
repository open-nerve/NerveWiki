package domain

import (
	"maps"
	"slices"
	"time"
	"uuid"
)

// UpgradeGates are a check's gates (M11 design 4.7): the seven of overall
// design 5.8 and whether the link index is fresh.
type UpgradeGates struct {
	Agents, SourceRoot, Frontmatter, Sources, Index bool
	ReservedTitles                                  NodesGate     // at most the four reserved keys' nodes
	Links                                           LinksGate     // the first five worse pages, by id
	Preserved                                       PreservedGate // the first five not kept, by id
}

// NodesGate is a gate that names its nodes.
type NodesGate struct {
	Passed  bool
	NodeIDs []uuid.UUID
}

// LinksGate is gate 6: how many baseline pages lead to more wanted pages
// than as the upgrade started, and the first five, by id.
type LinksGate struct {
	Passed     bool
	PagesWorse int
	NodeIDs    []uuid.UUID
}

// PreservedGate is gate 7: the baseline's nodes and pages, those kept, and
// the first five not, by id.
type PreservedGate struct {
	Passed                             bool
	Nodes, NodesKept, Pages, PagesKept int
	NodeIDs                            []uuid.UUID
}

// UpgradeCounts are what the notebook holds as checked (M11 design 4.7).
type UpgradeCounts struct {
	Sources, MovedIntoSources, WikiPages, NewPages, Changesets int
	WantedBefore, WantedNow                                    LintCount
}

// UpgradeCheck is an upgrade's check (M11 design 4.7): its time, stale
// pages, gates, the eight rules' summaries of an LLM Wiki's explicit
// contract with the proposed source root, and its counts; Paths, for MCP,
// the gates' nodes' paths, those in the live tree.
type UpgradeCheck struct {
	ComputedAt time.Time
	StalePages int64
	Passed     bool
	Gates      UpgradeGates
	Rules      []LintRuleResult
	Counts     UpgradeCounts
	Paths      map[uuid.UUID]string
}

// CloneUpgradeCheck gives a caller of a run its own check, nil for nil:
// independent slices, map and pointer values (M11/P2 design 3.8).
func CloneUpgradeCheck(c *UpgradeCheck) *UpgradeCheck {
	if c == nil {
		return nil
	}
	out := *c
	out.Rules = CloneLintResult(LintResult{Rules: c.Rules}).Rules
	out.Gates.ReservedTitles.NodeIDs = slices.Clone(c.Gates.ReservedTitles.NodeIDs)
	out.Gates.Links.NodeIDs = slices.Clone(c.Gates.Links.NodeIDs)
	out.Gates.Preserved.NodeIDs = slices.Clone(c.Gates.Preserved.NodeIDs)
	out.Paths = maps.Clone(c.Paths)
	return &out
}
