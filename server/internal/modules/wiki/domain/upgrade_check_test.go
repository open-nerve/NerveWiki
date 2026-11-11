package domain_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func checkID(n byte) uuid.UUID { return uuid.UUID{15: n} }

func checkPtr(id uuid.UUID) *uuid.UUID { return &id }

// The ids of the passing read: raw (10), the proposed root, held src (12)
// as the upgrade started; Plan (20) and Notes (21) at the root; AGENTS
// (30) and Made (31) written since.

// passingRead is an upgrade's read whose every gate passes.
func passingRead() domain.UpgradeRead {
	rules := make([]domain.LintRuleResult, 0, 8)
	for _, rule := range domain.LintRules(domain.ModeLLMWiki) {
		r := domain.LintRuleResult{Rule: rule}
		if rule == domain.RuleWanted {
			r.Count = domain.LintCount{Value: 1}
		}
		rules = append(rules, r)
	}
	page := domain.NodePage
	return domain.UpgradeRead{
		Upgrade: domain.Upgrade{ID: checkID(1), SourceRootID: checkID(10), BaselineNodes: 4, BaselinePages: 4, BaselineWanted: 2},
		Roots:   domain.UpgradeRoots{AgentsID: checkPtr(checkID(30)), AgentsBody: true, RootPage: true},
		Nodes: []domain.GateNode{{ID: checkID(10), Kind: page, Key: "raw"}, {ID: checkID(12), ParentID: checkPtr(checkID(10)), Kind: page, Key: "src"},
			{ID: checkID(20), Kind: page, Key: "plan"}, {ID: checkID(21), Kind: page, Key: "notes"}, {ID: checkID(30), Kind: page, Key: "agents"},
			{ID: checkID(31), Kind: page, Key: "made"}},
		Area: []uuid.UUID{checkID(12)},
		Lint: domain.LintResult{Kind: domain.LintSummary, Mode: domain.ModeLLMWiki, Rules: rules},
		// Plan as many as it started with, Made's not counted.
		WantedByPage: map[uuid.UUID]int{checkID(20): 1, checkID(31): 7},
		Baseline: []domain.BaselineNode{{ID: checkID(10), Kind: page}, {ID: checkID(12), ParentID: checkPtr(checkID(10)), Kind: page},
			{ID: checkID(20), Kind: page, Wanted: 1}, {ID: checkID(21), Kind: page, Wanted: 1}},
		Preserved:  domain.Preserved{Nodes: 4, NodesKept: 4, Pages: 4, PagesKept: 4, NotKept: []uuid.UUID{}},
		Changesets: 3,
		Reserved:   []string{"index", "log", "index.md", "log.md"},
	}
}

// withAttachment adds to r's baseline an attachment below Plan, Pic (25),
// the upgrade's nodes one more than its pages.
func withAttachment(r *domain.UpgradeRead) {
	r.Upgrade.BaselineNodes = 5
	r.Baseline = append(r.Baseline, domain.BaselineNode{ID: checkID(25), ParentID: checkPtr(checkID(20)), Kind: domain.NodeAsset})
}

// count sets rule's count in r's summary.
func count(r *domain.UpgradeRead, rule domain.LintRule, value int64) {
	rules := slices.Clone(r.Lint.Rules)
	for i := range rules {
		if rules[i].Rule == rule {
			rules[i].Count = domain.LintCount{Value: value, Capped: value == 10000}
		}
	}
	r.Lint.Rules = rules
}

// Each gate of an upgrade's check passes and fails by one change of a
// passing read (M11/P2 design 3.10's table): a gate fails alone, the
// check with it; a gate's nodes are the first five by id.
func TestJudgeUpgrade(t *testing.T) {
	if c := domain.JudgeUpgrade(passingRead()); !c.Passed || len(c.Gates.Failed()) != 0 {
		t.Fatalf("the passing read = %+v, failed %v", c, c.Gates.Failed())
	}
	asset := domain.NodeAsset
	for _, tc := range []struct {
		name   string
		change func(*domain.UpgradeRead)
		fails  domain.UpgradeGateKey // "" for a read that still passes
		check  func(*testing.T, domain.UpgradeCheck)
	}{
		{"no AGENTS (none, or an attachment)", func(r *domain.UpgradeRead) { r.Roots.AgentsID, r.Roots.AgentsBody = nil, false },
			domain.GateAgents, nil},
		{"AGENTS whose body is only frontmatter or white space", func(r *domain.UpgradeRead) { r.Roots.AgentsBody = false },
			domain.GateAgents, nil},
		{"the source root renamed AGENTS", func(r *domain.UpgradeRead) { r.Roots.AgentsID = checkPtr(checkID(10)) }, domain.GateSourceRoot, nil},
		{"the source root no live page", func(r *domain.UpgradeRead) { r.Roots.RootPage = false }, domain.GateSourceRoot, nil},
		{"an index page and a log.md attachment at the root", func(r *domain.UpgradeRead) {
			r.Nodes = append(r.Nodes, domain.GateNode{ID: checkID(41), Kind: domain.NodePage, Key: "index"},
				domain.GateNode{ID: checkID(40), Kind: asset, Key: "log.md"},
				domain.GateNode{ID: checkID(42), ParentID: checkPtr(checkID(20)), Kind: domain.NodePage, Key: "index"})
		}, domain.GateReservedTitles, func(t *testing.T, c domain.UpgradeCheck) {
			if !slices.Equal(c.Gates.ReservedTitles.NodeIDs, []uuid.UUID{checkID(40), checkID(41)}) {
				t.Errorf("reserved titles name %v, want the two at the root, by id", c.Gates.ReservedTitles.NodeIDs)
			}
		}},
		{"an index page below the root", func(r *domain.UpgradeRead) {
			r.Nodes = append(r.Nodes, domain.GateNode{ID: checkID(42), ParentID: checkPtr(checkID(20)), Kind: domain.NodePage, Key: "index"})
		}, "", nil},
		{"invalid_frontmatter", func(r *domain.UpgradeRead) { count(r, domain.RuleInvalidFrontmatter, 1) }, domain.GateFrontmatter, nil},
		{"missing_type", func(r *domain.UpgradeRead) { count(r, domain.RuleMissingType, 1) }, domain.GateFrontmatter, nil},
		{"missing_summary", func(r *domain.UpgradeRead) { count(r, domain.RuleMissingSummary, 1) }, domain.GateFrontmatter, nil},
		{"invalid_sources", func(r *domain.UpgradeRead) { count(r, domain.RuleInvalidSources, 1) }, domain.GateSources, nil},
		{"the hints, not gates", func(r *domain.UpgradeRead) {
			for _, rule := range []domain.LintRule{domain.RuleWanted, domain.RuleOrphans, domain.RuleAmbiguous, domain.RuleUndigestedSources} {
				count(r, rule, 10000)
			}
		}, "", nil},
		// Decision 180: a page one more, another one less, is no pass.
		{"one page worse, another better", func(r *domain.UpgradeRead) { r.WantedByPage = map[uuid.UUID]int{checkID(20): 2} },
			domain.GateLinks, func(t *testing.T, c domain.UpgradeCheck) {
				if c.Gates.Links.PagesWorse != 1 || !slices.Equal(c.Gates.Links.NodeIDs, []uuid.UUID{checkID(20)}) {
					t.Errorf("links = %+v, want Plan worse", c.Gates.Links)
				}
			}},
		{"three wanted, four now", func(r *domain.UpgradeRead) {
			r.Baseline[2].Wanted, r.WantedByPage = 3, map[uuid.UUID]int{checkID(20): 4}
		}, domain.GateLinks, nil},
		{"10,000 wanted then and now, uncapped (decision 188)", func(r *domain.UpgradeRead) {
			r.Baseline[2].Wanted, r.WantedByPage = 10000, map[uuid.UUID]int{checkID(20): 10000}
		}, "", nil},
		{"a page written since, wanting many", func(r *domain.UpgradeRead) { r.WantedByPage = map[uuid.UUID]int{checkID(31): 9} }, "", nil},
		{"a baseline page wanting none now", func(r *domain.UpgradeRead) { r.WantedByPage = map[uuid.UUID]int{} }, "", nil},
		// B2-I1: a page in raw as the upgrade started, its second pass's 1.
		{"a page moved out of raw, as many as its second pass counted", func(r *domain.UpgradeRead) {
			r.Area, r.WantedByPage = nil, map[uuid.UUID]int{checkID(12): 1}
			r.Baseline[1].Wanted = 1
		}, "", nil},
		{"more than five pages worse", func(r *domain.UpgradeRead) {
			r.WantedByPage = map[uuid.UUID]int{}
			for i := byte(50); i < 57; i++ {
				r.Baseline = append(r.Baseline, domain.BaselineNode{ID: checkID(i), Kind: domain.NodePage})
				r.WantedByPage[checkID(i)] = 1
			}
		}, domain.GateLinks, func(t *testing.T, c domain.UpgradeCheck) {
			want := []uuid.UUID{checkID(50), checkID(51), checkID(52), checkID(53), checkID(54)}
			if c.Gates.Links.PagesWorse != 7 || !slices.Equal(c.Gates.Links.NodeIDs, want) {
				t.Errorf("links = %+v, want 7 worse, the first five by id", c.Gates.Links)
			}
		}},
		{"a node deleted", func(r *domain.UpgradeRead) {
			r.Preserved = domain.Preserved{Nodes: 4, NodesKept: 3, Pages: 4, PagesKept: 3, NotKept: []uuid.UUID{checkID(21)}}
		}, domain.GatePreserved, func(t *testing.T, c domain.UpgradeCheck) {
			if p := c.Gates.Preserved; p.Nodes != 4 || p.NodesKept != 3 || p.Pages != 4 || p.PagesKept != 3 ||
				!slices.Equal(p.NodeIDs, []uuid.UUID{checkID(21)}) {
				t.Errorf("preserved = %+v", p)
			}
		}},
		// An attachment of the baseline (review A-M2): kept, a pass; deleted,
		// a node not kept alone; its row gone, the nodes' denominator alone.
		{"a baseline attachment kept", func(r *domain.UpgradeRead) {
			withAttachment(r)
			r.Nodes = append(r.Nodes, domain.GateNode{ID: checkID(25), ParentID: checkPtr(checkID(20)), Kind: asset, Key: "pic.png"})
			r.Preserved = domain.Preserved{Nodes: 5, NodesKept: 5, Pages: 4, PagesKept: 4, NotKept: []uuid.UUID{}}
		}, "", nil},
		{"a baseline attachment deleted", func(r *domain.UpgradeRead) {
			withAttachment(r)
			r.Preserved = domain.Preserved{Nodes: 5, NodesKept: 4, Pages: 4, PagesKept: 4, NotKept: []uuid.UUID{checkID(25)}}
		}, domain.GatePreserved, func(t *testing.T, c domain.UpgradeCheck) {
			if p := c.Gates.Preserved; p.Nodes != 5 || p.NodesKept != 4 || p.Pages != 4 || p.PagesKept != 4 ||
				!slices.Equal(p.NodeIDs, []uuid.UUID{checkID(25)}) {
				t.Errorf("preserved = %+v, want Pic not kept", p)
			}
		}},
		{"an attachment's baseline row gone", func(r *domain.UpgradeRead) {
			withAttachment(r)
			r.Nodes = append(r.Nodes, domain.GateNode{ID: checkID(25), ParentID: checkPtr(checkID(20)), Kind: asset, Key: "pic.png"})
			r.Preserved = domain.Preserved{Nodes: 4, NodesKept: 4, Pages: 4, PagesKept: 4, NotKept: []uuid.UUID{}}
		}, domain.GatePreserved, func(t *testing.T, c domain.UpgradeCheck) {
			if p := c.Gates.Preserved; p.Nodes != 5 || p.NodesKept != 4 || p.Pages != 4 || p.PagesKept != 4 {
				t.Errorf("preserved = %+v, want the wiki row's 5 nodes over the 4 kept", p)
			}
		}},
		{"a page's content not the ledger's", func(r *domain.UpgradeRead) {
			r.Preserved = domain.Preserved{Nodes: 4, NodesKept: 4, Pages: 4, PagesKept: 3, NotKept: []uuid.UUID{checkID(20)}}
		}, domain.GatePreserved, nil},
		// The denominators are the wiki row's: a baseline row gone is no
		// pass, all the rows read kept.
		{"a baseline row fewer than the upgrade's counts", func(r *domain.UpgradeRead) {
			r.Preserved = domain.Preserved{Nodes: 3, NodesKept: 3, Pages: 3, PagesKept: 3, NotKept: []uuid.UUID{}}
		}, domain.GatePreserved, func(t *testing.T, c domain.UpgradeCheck) {
			if p := c.Gates.Preserved; p.Nodes != 4 || p.Pages != 4 || p.NodesKept != 3 || p.PagesKept != 3 {
				t.Errorf("preserved = %+v, want the wiki row's 4 and 4 over the 3 and 3 kept", p)
			}
		}},
		{"a stale page", func(r *domain.UpgradeRead) { r.Lint.StalePages = 1 }, domain.GateIndex, func(t *testing.T, c domain.UpgradeCheck) {
			if c.StalePages != 1 {
				t.Errorf("stale pages = %d", c.StalePages)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := passingRead()
			tc.change(&r)
			c := domain.JudgeUpgrade(r)
			failed := c.Gates.Failed()
			switch {
			case tc.fails == "" && (!c.Passed || len(failed) != 0):
				t.Errorf("failed %v, want a pass", failed)
			case tc.fails != "" && (c.Passed || !slices.Equal(failed, []domain.UpgradeGateKey{tc.fails})):
				t.Errorf("failed %v (passed %t), want %s alone", failed, c.Passed, tc.fails)
			}
			if tc.check != nil {
				tc.check(t, c)
			}
		})
	}
}

// The counts (M11 design 4.7, B-N8): the area's nodes; the baseline nodes
// in it now that were not as the upgrade started (its tree rebuilt from
// the baseline's parents; a seeded root held none); the pages outside it
// but the root and AGENTS; the pages with no baseline row; the changesets;
// the wanted pages then and now.
func TestJudgeUpgradesCounts(t *testing.T) {
	page := domain.NodePage
	t.Run("raw there, two pages under it as the upgrade started, one moved in", func(t *testing.T) {
		r := passingRead()
		deep := checkID(13)
		r.Baseline = append(r.Baseline, domain.BaselineNode{ID: deep, ParentID: checkPtr(checkID(12)), Kind: page})
		r.Nodes = append(r.Nodes, domain.GateNode{ID: deep, ParentID: checkPtr(checkID(12)), Kind: page, Key: "deep"})
		r.Upgrade.BaselineNodes, r.Upgrade.BaselinePages = 5, 5
		r.Preserved = domain.Preserved{Nodes: 5, NodesKept: 5, Pages: 5, PagesKept: 5}
		r.Area = []uuid.UUID{checkID(12), deep, checkID(21)}
		count(&r, domain.RuleWanted, 4)
		want := domain.UpgradeCounts{Sources: 3, MovedIntoSources: 1, WikiPages: 2, NewPages: 2, Changesets: 3,
			WantedBefore: domain.LintCount{Value: 2}, WantedNow: domain.LintCount{Value: 4}}
		if c := domain.JudgeUpgrade(r); c.Counts != want {
			t.Errorf("counts = %+v, want %+v", c.Counts, want)
		}
	})
	// Each count its own value, so that none is another's (review
	// P2S2C-M5): a page made under the sources since, more changesets, more
	// wanted pages before and now.
	t.Run("each count its own", func(t *testing.T) {
		r := passingRead()
		deep, made := checkID(13), checkID(14)
		r.Baseline = append(r.Baseline, domain.BaselineNode{ID: deep, ParentID: checkPtr(checkID(12)), Kind: page})
		r.Nodes = append(r.Nodes, domain.GateNode{ID: deep, ParentID: checkPtr(checkID(12)), Kind: page, Key: "deep"},
			domain.GateNode{ID: made, ParentID: checkPtr(checkID(12)), Kind: page, Key: "since"})
		r.Area = []uuid.UUID{checkID(12), deep, made, checkID(21)}
		r.Changesets, r.Upgrade.BaselineWanted = 5, 6
		count(&r, domain.RuleWanted, 7)
		want := domain.UpgradeCounts{Sources: 4, MovedIntoSources: 1, WikiPages: 2, NewPages: 3, Changesets: 5,
			WantedBefore: domain.LintCount{Value: 6}, WantedNow: domain.LintCount{Value: 7}}
		if c := domain.JudgeUpgrade(r); c.Counts != want {
			t.Errorf("counts = %+v, want %+v", c.Counts, want)
		}
	})
	t.Run("raw seeded, two pages moved in", func(t *testing.T) {
		r := passingRead()
		seeded := checkID(11)
		r.Upgrade.SourceRootID = seeded
		r.Upgrade.BaselineWanted, r.Upgrade.BaselineWantedCapped = 10000, true
		r.Nodes = append(r.Nodes, domain.GateNode{ID: seeded, Kind: page, Key: "sources"})
		r.Area = []uuid.UUID{checkID(20), checkID(21)}
		want := domain.UpgradeCounts{Sources: 2, MovedIntoSources: 2, WikiPages: 3, NewPages: 3, Changesets: 3,
			WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: domain.LintCount{Value: 1}}
		if c := domain.JudgeUpgrade(r); c.Counts != want {
			t.Errorf("counts = %+v, want %+v", c.Counts, want)
		}
	})
	// The wanted pages now are the summary's, its cap too (review C-M2).
	t.Run("the wanted pages now capped", func(t *testing.T) {
		r := passingRead()
		count(&r, domain.RuleWanted, 10000)
		if c := domain.JudgeUpgrade(r); c.Counts.WantedNow != (domain.LintCount{Value: 10000, Capped: true}) {
			t.Errorf("wanted now = %+v, want 10,000 capped", c.Counts.WantedNow)
		}
	})
}

// The gates that did not pass, in their order: the check's and the
// refusal's (M11 design 4.7).
func TestTheGatesFailInTheirOrder(t *testing.T) {
	all := []domain.UpgradeGateKey{domain.GateAgents, domain.GateSourceRoot, domain.GateReservedTitles, domain.GateFrontmatter,
		domain.GateSources, domain.GateLinks, domain.GatePreserved, domain.GateIndex}
	if got := (domain.UpgradeGates{}).Failed(); !slices.Equal(got, all) {
		t.Errorf("none passed: %v, want %v", got, all)
	}
	g := domain.UpgradeGates{Agents: true, SourceRoot: true, ReservedTitles: domain.NodesGate{Passed: true}, Sources: true,
		Preserved: domain.PreservedGate{Passed: true}}
	if got := g.Failed(); !slices.Equal(got, []domain.UpgradeGateKey{domain.GateFrontmatter, domain.GateLinks, domain.GateIndex}) {
		t.Errorf("failed %v, want frontmatter, links, index", got)
	}
	if got := passingGates().Failed(); len(got) != 0 {
		t.Errorf("all passed: %v", got)
	}
}

// Gates 1 to 3 as the check and the approval judge them (M11/P3 design
// 3.2, decision 284): each fails alone by one change of roots that pass,
// the others still passing; the nodes keyed a reserved title, given out of
// order, come back by id, a copy of their own; Failed lists the gates in
// their order.
func TestJudgeRoots(t *testing.T) {
	agents, root := checkID(30), checkID(10)
	passing := domain.UpgradeRoots{AgentsID: &agents, AgentsBody: true, RootPage: true}
	if g := domain.JudgeRoots(passing, root, nil); !g.Agents || !g.SourceRoot || !g.ReservedTitles.Passed ||
		g.ReservedTitles.NodeIDs != nil || len(g.Failed()) != 0 {
		t.Errorf("passing roots = %+v, failed %v", g, g.Failed())
	}
	for _, tc := range []struct {
		name     string
		roots    domain.UpgradeRoots
		root     uuid.UUID
		reserved []uuid.UUID
		fails    []domain.UpgradeGateKey
	}{
		{"no AGENTS", domain.UpgradeRoots{RootPage: true, AgentsBody: true}, root, nil, []domain.UpgradeGateKey{domain.GateAgents}},
		{"AGENTS without a body", domain.UpgradeRoots{AgentsID: &agents, RootPage: true}, root, nil,
			[]domain.UpgradeGateKey{domain.GateAgents}},
		{"the source root the root's AGENTS", passing, agents, nil, []domain.UpgradeGateKey{domain.GateSourceRoot}},
		{"the source root no live page", domain.UpgradeRoots{AgentsID: &agents, AgentsBody: true}, root, nil,
			[]domain.UpgradeGateKey{domain.GateSourceRoot}},
		{"a node at the root keyed a reserved title", passing, root, []uuid.UUID{checkID(41)},
			[]domain.UpgradeGateKey{domain.GateReservedTitles}},
		{"all three", domain.UpgradeRoots{}, root, []uuid.UUID{checkID(41)},
			[]domain.UpgradeGateKey{domain.GateAgents, domain.GateSourceRoot, domain.GateReservedTitles}},
	} {
		if got := domain.JudgeRoots(tc.roots, tc.root, tc.reserved).Failed(); !slices.Equal(got, tc.fails) {
			t.Errorf("%s: failed %v, want %v", tc.name, got, tc.fails)
		}
	}
	reserved := []uuid.UUID{checkID(43), checkID(41), checkID(42)}
	g := domain.JudgeRoots(passing, root, reserved)
	reserved[0] = checkID(99)
	if g.ReservedTitles.Passed || !slices.Equal(g.ReservedTitles.NodeIDs, []uuid.UUID{checkID(41), checkID(42), checkID(43)}) {
		t.Errorf("reserved titles = %+v, want the three by id, not passed, a copy", g.ReservedTitles)
	}
	if !slices.Equal(reserved, []uuid.UUID{checkID(99), checkID(41), checkID(42)}) {
		t.Errorf("JudgeRoots changed the ids it was given: %v", reserved)
	}
}

func passingGates() domain.UpgradeGates {
	return domain.UpgradeGates{Agents: true, SourceRoot: true, Frontmatter: true, Sources: true, Index: true,
		ReservedTitles: domain.NodesGate{Passed: true}, Links: domain.LinksGate{Passed: true}, Preserved: domain.PreservedGate{Passed: true}}
}

// The report a submission keeps (M11 design 4.7), whole: the check's
// time, stale pages, passed, gates and counts as they are, each rule's
// count and first five items without their cursors and paths; its own
// copy. Each number its own, each rule's count not its items' number,
// so that none is kept as another (review P2S2C-M6); a check that did
// not pass and one that did.
func TestReportOf(t *testing.T) {
	path, next := "Plan.md", "after:5"
	pages := make([]uuid.UUID, 7)
	items := make([]domain.LintItem, 7)
	for i := range items {
		pages[i] = checkID(byte(40 + i))
		items[i] = domain.LintItem{Rule: domain.RuleOrphans, ItemKey: fmt.Sprintf("o:%d", i), Cursor: fmt.Sprintf("c:%d", i),
			PageID: &pages[i], Path: &path}
	}
	gates := func(passed bool) domain.UpgradeGates {
		g := passingGates()
		g.Links = domain.LinksGate{Passed: passed, PagesWorse: 21, NodeIDs: []uuid.UUID{checkID(20)}}
		return g
	}
	counts := domain.UpgradeCounts{Sources: 11, MovedIntoSources: 12, WikiPages: 13, NewPages: 14, Changesets: 15,
		WantedBefore: domain.LintCount{Value: 16}, WantedNow: domain.LintCount{Value: 17}}
	check := func(passed bool) domain.UpgradeCheck {
		return domain.UpgradeCheck{ComputedAt: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC), StalePages: 4, Passed: passed,
			Gates: gates(passed),
			Rules: []domain.LintRuleResult{
				{Rule: domain.RuleWanted, Count: domain.LintCount{Value: 6}, Items: []domain.LintItem{}},
				{Rule: domain.RuleOrphans, Count: domain.LintCount{Value: 10000, Capped: true}, Items: slices.Clone(items), NextCursor: &next,
					StartCursor: &next},
			},
			Counts: counts, Paths: map[uuid.UUID]string{checkID(20): path},
		}
	}
	kept := make([]domain.LintItem, 5)
	for i := range kept {
		kept[i] = domain.LintItem{Rule: domain.RuleOrphans, ItemKey: fmt.Sprintf("o:%d", i), PageID: &pages[i]}
	}
	for _, passed := range []bool{false, true} {
		want := domain.UpgradeReport{ComputedAt: time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC), StalePages: 4, Passed: passed,
			Gates: gates(passed), Counts: counts,
			Rules: []domain.ReportRule{
				{Rule: domain.RuleWanted, Count: domain.LintCount{Value: 6}, Items: []domain.LintItem{}},
				{Rule: domain.RuleOrphans, Count: domain.LintCount{Value: 10000, Capped: true}, Items: kept},
			}}
		if r := domain.ReportOf(check(passed)); !reflect.DeepEqual(r, want) {
			t.Fatalf("passed %v: report = %+v, want %+v", passed, r, want)
		}
	}
	c := check(false)
	r := domain.ReportOf(c)
	c.Gates.Links.NodeIDs[0] = checkID(99)
	*c.Rules[1].Items[0].PageID = checkID(99)
	if r.Gates.Links.NodeIDs[0] != checkID(20) || *r.Rules[1].Items[0].PageID != checkID(40) {
		t.Fatal("the report shares the check's slices")
	}
}
