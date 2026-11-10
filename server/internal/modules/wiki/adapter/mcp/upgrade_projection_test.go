package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
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
// in the live tree; each of its counts its own value, the wanted pages
// as the upgrade started 5, now capped.
func bigCheck(pathBytes, gateBytes int) domain.UpgradeCheck {
	c := domain.UpgradeCheck{ComputedAt: lintTime(), Paths: map[uuid.UUID]string{},
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
		Links:     domain.LinksGate{PagesWorse: 9, NodeIDs: ids(5)},
		Preserved: domain.PreservedGate{Nodes: 7, NodesKept: 2, Pages: 6, PagesKept: 1, NodeIDs: ids(5)}}
	start := strings.Repeat("s", 2048)
	for _, rule := range domain.LintRules(domain.ModeLLMWiki) {
		r := domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: 9}, StartCursor: &start, NextCursor: &start}
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
	if len(links.Paths) >= 5 || len(links.Paths) != len(preserved.Paths) || links.PagesWorse != 9 || preserved.Nodes != 7 ||
		preserved.PagesKept != 1 {
		t.Fatalf("a check of long paths kept the gates %+v", c.Gates)
	}
	for _, rule := range c.Rules {
		if len(rule.Items) != 0 || rule.Next == nil || *rule.Next != strings.Repeat("s", 2048) || rule.Count != 9 {
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
		{"upgrade_verify", `"pages_worse":9`, upgradeFake{checked: app.UpgradeChecked{Check: &check}}},
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
