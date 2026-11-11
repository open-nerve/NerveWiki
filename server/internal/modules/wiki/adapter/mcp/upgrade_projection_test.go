package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/mcpserver"
)

// checkRead is upgrade_verify's check, as these tests read it.
type checkRead struct {
	Passed bool `json:"passed"`
	Gates  map[string]struct {
		Passed     bool      `json:"passed"`
		PagesWorse int       `json:"pages_worse"`
		Nodes      int       `json:"nodes"`
		PagesKept  int       `json:"pages_kept"`
		Paths      []*string `json:"paths"`
	} `json:"gates"`
	Rules []struct {
		Rule  string           `json:"rule"`
		Count int64            `json:"count"`
		Items []map[string]any `json:"items"`
		Next  *string          `json:"next_cursor"`
	} `json:"rules"`
	Counts map[string]any `json:"counts"`
}

// bigCheck is a check whose eight rules have five items of paths of
// pathBytes each and start cursors of 2,048 bytes, and whose three gates
// name fourteen nodes of paths of gateBytes, the first of each gate's not
// in the live tree; each of its numbers its own value, the wanted pages
// as the upgrade started 5, now capped; its gates gateRows' fourth.
func bigCheck(pathBytes, gateBytes int) domain.UpgradeCheck {
	c := domain.UpgradeCheck{ComputedAt: lintTime(), StalePages: 8, Paths: map[uuid.UUID]string{},
		Counts: domain.UpgradeCounts{Sources: 1, MovedIntoSources: 6, WikiPages: 2, NewPages: 3, Changesets: 4,
			WantedBefore: domain.LintCount{Value: 5}, WantedNow: domain.LintCount{Value: 10000, Capped: true}}}
	n := byte(100)
	ids := func(k int) []uuid.UUID {
		out := make([]uuid.UUID, k)
		for i := range out {
			n++
			out[i] = lintID(n)
			if i > 0 {
				c.Paths[out[i]] = strings.Repeat("g", gateBytes)
			}
		}
		return out
	}
	c.Gates = domain.UpgradeGates{ReservedTitles: domain.NodesGate{NodeIDs: ids(4)},
		Links:     domain.LinksGate{PagesWorse: 19, NodeIDs: ids(5)},
		Preserved: domain.PreservedGate{Nodes: 17, NodesKept: 14, Pages: 16, PagesKept: 13, NodeIDs: ids(5)}}
	withGates(&c, gateRows()[3])
	start := strings.Repeat("s", 2048)
	for i, rule := range domain.LintRules(domain.ModeLLMWiki) {
		r := domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: int64(21 + i), Capped: i%3 == 2}, StartCursor: &start,
			NextCursor: &start}
		for i := range 5 {
			page, path := lintID(byte(10+i)), strings.Repeat("p", pathBytes)
			item := domain.LintItem{Rule: rule, ItemKey: "k", Cursor: strings.Repeat("c", 512), PageID: &page, NodeID: &page, Path: &path,
				Kind: domain.NodeAsset}
			r.Items = append(r.Items, item)
		}
		c.Rules = append(c.Rules, r)
	}
	return c
}

// gateRows are four patterns of a check's passed and its eight gates, in
// UpgradeGates' order. Each gate's column is its own and neither all
// passed nor all failed, so that any two gates exchanged, or a gate made
// constant, show in one row (review P2S2C-M6).
func gateRows() [][9]bool {
	return [][9]bool{
		// passed, agents, source root, reserved titles, frontmatter, sources, links, preserved, index
		{true, true, false, true, false, true, false, true, false},
		{false, true, true, false, false, false, true, true, false},
		{true, false, true, true, true, false, false, true, false},
		{true, false, false, false, true, true, true, false, true},
	}
}

// withGates sets c's passed and gates to row.
func withGates(c *domain.UpgradeCheck, row [9]bool) {
	g := &c.Gates
	c.Passed, g.Agents, g.SourceRoot, g.ReservedTitles.Passed, g.Frontmatter, g.Sources, g.Links.Passed, g.Preserved.Passed, g.Index =
		row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8]
}

// smallCheck is a check that fits whole, each of its numbers its own:
// its gates gateRows' third, each naming a node not in the live tree
// first; each rule's count its own, capped or not apart from its value,
// its start and continuation its own, the orphans' one item.
func smallCheck() domain.UpgradeCheck {
	page, path := lintID(30), "Notes/Orphan.md"
	c := domain.UpgradeCheck{ComputedAt: lintTime(), StalePages: 18,
		Gates: domain.UpgradeGates{ReservedTitles: domain.NodesGate{NodeIDs: []uuid.UUID{lintID(31), lintID(32)}},
			Links:     domain.LinksGate{PagesWorse: 29, NodeIDs: []uuid.UUID{lintID(33), lintID(34)}},
			Preserved: domain.PreservedGate{Nodes: 27, NodesKept: 24, Pages: 26, PagesKept: 23, NodeIDs: []uuid.UUID{lintID(35), lintID(36)}}},
		Counts: domain.UpgradeCounts{Sources: 11, MovedIntoSources: 16, WikiPages: 12, NewPages: 13, Changesets: 14,
			WantedBefore: domain.LintCount{Value: 15}, WantedNow: domain.LintCount{Value: 10000, Capped: true}},
		Paths: map[uuid.UUID]string{lintID(32): "index.md", lintID(34): "Plan.md", lintID(36): "raw/Kept.md"}}
	withGates(&c, gateRows()[2])
	for i, rule := range domain.LintRules(domain.ModeLLMWiki) {
		start, next := "start:"+string(rule), "next:"+string(rule)
		r := domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: int64(41 + i), Capped: i%3 == 0}, Items: []domain.LintItem{},
			StartCursor: &start, NextCursor: &next}
		if rule == domain.RuleOrphans {
			r.Items = []domain.LintItem{{Rule: rule, ItemKey: "p:" + page.String(), Cursor: "c:orphan", PageID: &page, Path: &path}}
		}
		c.Rules = append(c.Rules, r)
	}
	return c
}

// The check is answered whole when it fits (reviews P2S2C-M5, M6): each
// gate's paths, null for a node not in the live tree; the rules lint's
// summary's; the counts. Each gate is its own in one of gateRows.
func TestTheCheckIsAnsweredWhole(t *testing.T) {
	res := mustFit(t, smallCheck())
	raw, ok := res.Structured.(json.RawMessage)
	if !ok {
		t.Fatalf("structured %T", res.Structured)
	}
	var got, want any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantSmallCheck), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("answered\n%s\nwant\n%s", raw, wantSmallCheck)
	}
	for i, row := range gateRows() {
		c := smallCheck()
		withGates(&c, row)
		read := readCheck(t, mustFit(t, c))
		got := [9]bool{read.Passed}
		for j, gate := range []string{"agents", "source_root", "reserved_titles", "frontmatter", "sources", "links", "preserved", "index"} {
			got[j+1] = read.Gates[gate].Passed
		}
		if got != row {
			t.Errorf("row %d: passed and gates %v, want %v", i, got, row)
		}
	}
}

const wantSmallCheck = `{
	"computed_at": "2026-01-02T03:04:05Z", "stale_pages": 18, "passed": true,
	"gates": {
		"agents": {"passed": false}, "source_root": {"passed": true},
		"reserved_titles": {"passed": true, "paths": [null, "index.md"]},
		"frontmatter": {"passed": true}, "sources": {"passed": false},
		"links": {"passed": false, "pages_worse": 29, "paths": [null, "Plan.md"]},
		"preserved": {"passed": true, "nodes": 27, "nodes_kept": 24, "pages": 26, "pages_kept": 23, "paths": [null, "raw/Kept.md"]},
		"index": {"passed": false}},
	"rules": [
		{"rule": "wanted", "count": 41, "count_capped": true, "items": [], "next_cursor": "next:wanted"},
		{"rule": "orphans", "count": 42, "count_capped": false, "next_cursor": "next:orphans", "items": [
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-00000000001e", "cursor": "c:orphan", "page_id": "00000000-0000-0000-0000-00000000001e", "path": "Notes/Orphan.md"}]},
		{"rule": "ambiguous", "count": 43, "count_capped": false, "items": [], "next_cursor": "next:ambiguous"},
		{"rule": "invalid_frontmatter", "count": 44, "count_capped": true, "items": [], "next_cursor": "next:invalid_frontmatter"},
		{"rule": "missing_type", "count": 45, "count_capped": false, "items": [], "next_cursor": "next:missing_type"},
		{"rule": "missing_summary", "count": 46, "count_capped": false, "items": [], "next_cursor": "next:missing_summary"},
		{"rule": "undigested_sources", "count": 47, "count_capped": true, "items": [], "next_cursor": "next:undigested_sources"},
		{"rule": "invalid_sources", "count": 48, "count_capped": false, "items": [], "next_cursor": "next:invalid_sources"}],
	"counts": {"sources": 11, "moved_into_sources": 16, "wiki_pages": 12, "new_pages": 13, "changesets": 14,
		"wanted_before": {"count": 15, "count_capped": false}, "wanted_now": {"count": 10000, "count_capped": true}}
}`

func readCheck(t *testing.T, res mcpserver.Result) checkRead {
	t.Helper()
	raw, ok := res.Structured.(json.RawMessage)
	if !ok {
		t.Fatalf("structured %T", res.Structured)
	}
	if n := mcpserver.TextBytes(string(raw)); n > mcpserver.MaxAnswerBytes {
		t.Fatalf("the answer is %d bytes, over %d", n, mcpserver.MaxAnswerBytes)
	}
	var c checkRead
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// The check fits MCP's answer (M11/P2 design 3.14): small, whole, each
// gate's paths in its nodes' order, null for a node not in the live
// tree; its rules' items cut first, five to none, a rule of none
// continued from its start; when none of them fits, each gate's paths
// cut too, the gates' counts and passed whole.
func TestTheCheckFits(t *testing.T) {
	c := readCheck(t, mustFit(t, bigCheck(10, 10)))
	if len(c.Rules) != 8 || len(c.Rules[0].Items) != 5 || len(c.Gates["links"].Paths) != 5 || c.Gates["links"].Paths[0] != nil ||
		c.Gates["links"].Paths[1] == nil || *c.Gates["links"].Paths[1] != strings.Repeat("g", 10) {
		t.Fatalf("a small check = %+v", c)
	}
	if c.Rules[1].Items[0]["path"] != strings.Repeat("p", 10) {
		t.Fatalf("an orphan = %v, want its path", c.Rules[1].Items[0])
	}
	// The counts whole, so that none answers another's (review P2S2C-M5);
	// then the wanted pages the other way round: capped as the upgrade
	// started, now not.
	if got, want := fmt.Sprint(c.Counts), "map[changesets:4 moved_into_sources:6 new_pages:3 sources:1 "+
		"wanted_before:map[count:5 count_capped:false] wanted_now:map[count:10000 count_capped:true] wiki_pages:2]"; got != want {
		t.Errorf("counts %s, want %s", got, want)
	}
	before := bigCheck(10, 10)
	before.Counts.WantedBefore, before.Counts.WantedNow = domain.LintCount{Value: 10000, Capped: true}, domain.LintCount{Value: 7}
	if got, want := fmt.Sprint(readCheck(t, mustFit(t, before)).Counts), "map[changesets:4 moved_into_sources:6 new_pages:3 sources:1 "+
		"wanted_before:map[count:10000 count_capped:true] wanted_now:map[count:7 count_capped:false] wiki_pages:2]"; got != want {
		t.Errorf("counts, capped before, %s, want %s", got, want)
	}

	// Items of 2 KiB paths: some of each rule's cut, the gates whole.
	c = readCheck(t, mustFit(t, bigCheck(2048, 100)))
	k := len(c.Rules[0].Items)
	if k == 0 || k == 5 || c.Rules[0].Next == nil || len(c.Gates["reserved_titles"].Paths) != 4 ||
		len(c.Gates["preserved"].Paths) != 5 {
		t.Fatalf("a check of long items kept %d items, the gates %+v", k, c.Gates)
	}
	for _, rule := range c.Rules {
		if len(rule.Items) != k {
			t.Fatalf("the rule %s kept %d items, want %d as the others", rule.Rule, len(rule.Items), k)
		}
	}

	// Gates of 4 KiB paths: no rule's items fit; the paths are cut, the
	// gates' counts whole; each rule continues from its start.
	c = readCheck(t, mustFit(t, bigCheck(2048, 4096)))
	links, preserved := c.Gates["links"], c.Gates["preserved"]
	if len(links.Paths) >= 5 || len(links.Paths) != len(preserved.Paths) || links.PagesWorse != 19 || preserved.Nodes != 17 ||
		preserved.PagesKept != 13 {
		t.Fatalf("a check of long paths kept the gates %+v", c.Gates)
	}
	for i, rule := range c.Rules {
		if len(rule.Items) != 0 || rule.Next == nil || *rule.Next != strings.Repeat("s", 2048) || rule.Count != int64(21+i) {
			t.Fatalf("the rule %s = %+v, want no item, continued from its start", rule.Rule, rule)
		}
	}
}

func mustFit(t *testing.T, c domain.UpgradeCheck) mcpserver.Result {
	t.Helper()
	res, err := fitUpgradeCheck(c)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// upgradeFake answers an upgrade's check or submission.
type upgradeFake struct {
	checked app.UpgradeChecked
	view    app.UpgradeView
}

func (f upgradeFake) CheckUpgrade(context.Context, uuid.UUID, app.LintQuery) (app.UpgradeChecked, error) {
	return f.checked, nil
}

func (f upgradeFake) SubmitUpgrade(context.Context, uuid.UUID) (app.UpgradeView, error) {
	return f.view, nil
}

// upgrade_verify answers a check or a rule's page, as asked; upgrade_submit
// the upgrade's state and when it was submitted.
func TestTheUpgradeToolsAnswer(t *testing.T) {
	check, at := bigCheck(10, 10), lintTime().Add(time.Hour)
	page := domain.LintResult{Kind: domain.LintItems, Mode: domain.ModeLLMWiki, ComputedAt: lintTime(),
		Rules: []domain.LintRuleResult{{Rule: domain.RuleOrphans, Items: check.Rules[1].Items[:1]}}}
	for _, tc := range []struct {
		tool, want string
		fake       upgradeFake
	}{
		{"upgrade_verify", `"pages_worse":19`, upgradeFake{checked: app.UpgradeChecked{Check: &check}}},
		{"upgrade_verify", `"data":[{"rule":"orphans"`, upgradeFake{checked: app.UpgradeChecked{Items: &page}}},
		{"upgrade_submit", `{"state":"reviewing","submitted_at":"2026-01-02T04:04:05Z"}`, upgradeFake{view: app.UpgradeView{
			Upgrade: domain.Upgrade{State: domain.UpgradeReviewing, SubmittedAt: &at}}}},
	} {
		result, _ := lintCall(t, toolServer(t, nil, tc.fake), false, tc.tool, nil)
		if !strings.Contains(string(result.Structured), tc.want) {
			t.Errorf("%s answered %s, want %s", tc.tool, result.Structured, tc.want)
		}
	}
}
