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

// UpgradeGateKey names a gate of an upgrade's check (M11 design 4.7), in
// the order the check and the refusal list them.
type UpgradeGateKey string

// The gates' keys, in their order.
const (
	GateAgents         UpgradeGateKey = "agents"
	GateSourceRoot     UpgradeGateKey = "source_root"
	GateReservedTitles UpgradeGateKey = "reserved_titles"
	GateFrontmatter    UpgradeGateKey = "frontmatter"
	GateSources        UpgradeGateKey = "sources"
	GateLinks          UpgradeGateKey = "links"
	GatePreserved      UpgradeGateKey = "preserved"
	GateIndex          UpgradeGateKey = "index"
)

// Failed are the gates that did not pass, in their order.
func (g UpgradeGates) Failed() []UpgradeGateKey {
	var out []UpgradeGateKey
	for _, gate := range []struct {
		key    UpgradeGateKey
		passed bool
	}{
		{GateAgents, g.Agents}, {GateSourceRoot, g.SourceRoot}, {GateReservedTitles, g.ReservedTitles.Passed},
		{GateFrontmatter, g.Frontmatter}, {GateSources, g.Sources}, {GateLinks, g.Links.Passed},
		{GatePreserved, g.Preserved.Passed}, {GateIndex, g.Index},
	} {
		if !gate.passed {
			out = append(out, gate.key)
		}
	}
	return out
}

// UpgradeReport is a check a submission kept (M11 design 4.7): its rules
// with their count and first five items, no cursor; no paths.
type UpgradeReport struct {
	ComputedAt time.Time
	StalePages int64
	Passed     bool
	Gates      UpgradeGates
	Rules      []ReportRule
	Counts     UpgradeCounts
}

// ReportRule is a rule of a kept report: its count and its first five
// items, each without its cursor and path.
type ReportRule struct {
	Rule  LintRule
	Count LintCount
	Items []LintItem
}

// maxReportItems is how many items a kept report has of a rule, as the
// summary previews them.
const maxReportItems = 5

// ReportOf is the report a submission keeps of c, its own copy.
func ReportOf(c UpgradeCheck) UpgradeReport {
	kept := CloneUpgradeCheck(&c)
	out := UpgradeReport{ComputedAt: kept.ComputedAt, StalePages: kept.StalePages, Passed: kept.Passed, Gates: kept.Gates,
		Counts: kept.Counts, Rules: make([]ReportRule, len(kept.Rules))}
	for i, rule := range kept.Rules {
		items := rule.Items[:min(len(rule.Items), maxReportItems)]
		r := ReportRule{Rule: rule.Rule, Count: rule.Count, Items: make([]LintItem, len(items))}
		for j, item := range items {
			item.Cursor, item.Path = "", nil
			r.Items[j] = item
		}
		out.Rules[i] = r
	}
	return out
}

// UpgradeRoots are what gates 1 and 2 read (M11/P2 design 3.9): the root's
// AGENTS page, nil for none, whether its body is not empty; whether the
// proposed source root is a live page.
type UpgradeRoots struct {
	AgentsID   *uuid.UUID
	AgentsBody bool
	RootPage   bool
}

// BaselineNode is a baseline row: the node's kind and parent as the
// upgrade started, a page's distinct wanted targets then (0 for an
// attachment).
type BaselineNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     NodeKind
	Wanted   int
}

// Preserved is gate 7 as read: the baseline rows' nodes and pages, those
// kept, and the first five not, by id.
type Preserved struct {
	Nodes, NodesKept, Pages, PagesKept int
	NotKept                            []uuid.UUID
}

// GateNode is a live node as lint read it.
type GateNode struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Kind     NodeKind
	Key      string
}

// UpgradeRead is what one snapshot read for an upgrade's check: the
// upgrade's row, its gates' reads, lint's live nodes, the proposed root's
// strict descendants and its summary with the general emitters' wanted
// counts, and the root's reserved title keys.
type UpgradeRead struct {
	Upgrade      Upgrade
	Roots        UpgradeRoots
	Nodes        []GateNode
	Area         []uuid.UUID
	Lint         LintResult
	WantedByPage map[uuid.UUID]int
	Baseline     []BaselineNode
	Preserved    Preserved
	Changesets   int
	Reserved     []string
}

// maxGateNodes is how many nodes a gate names, the first by id.
const maxGateNodes = 5

// JudgeUpgrade is the check of r (M11 design 4.7), but its time and paths.
func JudgeUpgrade(r UpgradeRead) UpgradeCheck {
	counts := map[LintRule]LintCount{}
	for _, rule := range r.Lint.Rules {
		counts[rule.Rule] = rule.Count
	}
	zero := func(rules ...LintRule) bool {
		for _, rule := range rules {
			if c := counts[rule]; c.Value != 0 || c.Capped {
				return false
			}
		}
		return true
	}
	baseline := make(map[uuid.UUID]BaselineNode, len(r.Baseline))
	for _, b := range r.Baseline {
		baseline[b.ID] = b
	}
	var g UpgradeGates
	g.Agents = r.Roots.AgentsID != nil && r.Roots.AgentsBody
	g.SourceRoot = r.Roots.RootPage && (r.Roots.AgentsID == nil || *r.Roots.AgentsID != r.Upgrade.SourceRootID)
	for _, n := range r.Nodes {
		if n.ParentID == nil && slices.Contains(r.Reserved, n.Key) {
			g.ReservedTitles.NodeIDs = append(g.ReservedTitles.NodeIDs, n.ID)
		}
	}
	slices.SortFunc(g.ReservedTitles.NodeIDs, uuid.UUID.Compare)
	g.ReservedTitles.Passed = len(g.ReservedTitles.NodeIDs) == 0
	g.Frontmatter = zero(RuleInvalidFrontmatter, RuleMissingType, RuleMissingSummary)
	g.Sources = zero(RuleInvalidSources)
	var worse []uuid.UUID
	for id, now := range r.WantedByPage {
		if b, ok := baseline[id]; ok && b.Kind == NodePage && now > b.Wanted {
			worse = append(worse, id)
		}
	}
	slices.SortFunc(worse, uuid.UUID.Compare)
	g.Links = LinksGate{Passed: len(worse) == 0, PagesWorse: len(worse), NodeIDs: worse[:min(len(worse), maxGateNodes)]}
	p := r.Preserved
	g.Preserved = PreservedGate{
		Passed: p.Nodes == r.Upgrade.BaselineNodes && p.Pages == r.Upgrade.BaselinePages && p.NodesKept == p.Nodes &&
			p.PagesKept == p.Pages,
		Nodes: r.Upgrade.BaselineNodes, NodesKept: p.NodesKept, Pages: r.Upgrade.BaselinePages, PagesKept: p.PagesKept,
		NodeIDs: slices.Clone(p.NotKept),
	}
	g.Index = r.Lint.StalePages == 0
	return UpgradeCheck{
		StalePages: r.Lint.StalePages, Passed: len(g.Failed()) == 0, Gates: g, Rules: r.Lint.Rules,
		Counts: upgradeCounts(r, baseline, counts[RuleWanted]),
	}
}

// upgradeCounts are what r holds (M11 design 4.7): the area's nodes; the
// baseline nodes in it now that were not as the upgrade started, its tree
// then rebuilt from the baseline's parents (a seeded root has no row and
// held nothing); the pages outside it but the proposed root and the root's
// AGENTS; the live pages with no baseline row; the upgrade's changesets;
// the wanted pages then and now.
func upgradeCounts(r UpgradeRead, baseline map[uuid.UUID]BaselineNode, wanted LintCount) UpgradeCounts {
	area := make(map[uuid.UUID]bool, len(r.Area))
	for _, id := range r.Area {
		area[id] = true
	}
	root := r.Upgrade.SourceRootID
	// under says whether a baseline node was a strict descendant of the
	// proposed root as the upgrade started; memo keeps each answer.
	memo := map[uuid.UUID]bool{}
	var under func(id uuid.UUID, depth int) bool
	under = func(id uuid.UUID, depth int) bool {
		if done, ok := memo[id]; ok {
			return done
		}
		b, ok := baseline[id]
		was := false
		switch {
		case !ok || b.ParentID == nil || depth > len(baseline):
		case *b.ParentID == root:
			_, was = baseline[root]
		default:
			was = under(*b.ParentID, depth+1)
		}
		memo[id] = was
		return was
	}
	c := UpgradeCounts{Sources: len(r.Area), Changesets: r.Changesets,
		WantedBefore: LintCount{Value: int64(r.Upgrade.BaselineWanted), Capped: r.Upgrade.BaselineWantedCapped}, WantedNow: wanted}
	for _, b := range r.Baseline {
		if area[b.ID] && !under(b.ID, 0) {
			c.MovedIntoSources++
		}
	}
	for _, n := range r.Nodes {
		if n.Kind != NodePage {
			continue
		}
		if _, ok := baseline[n.ID]; !ok {
			c.NewPages++
		}
		if !area[n.ID] && n.ID != root && (r.Roots.AgentsID == nil || n.ID != *r.Roots.AgentsID) {
			c.WikiPages++
		}
	}
	return c
}
