package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"go.yaml.in/yaml/v3"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// reportedView is an upgrade reviewing with a report kept, each number
// in it its own (review P2S2C-M6): each of the eight rules with items of
// its shape, the orphans five, the most a report keeps, the sources' an
// attachment and a page (review P2S2C-N9); each rule's count none of its
// items' number, capped or not apart from its value; each boolean of an
// item's rule both ways, an item's two booleans apart; three gates naming
// nodes. The gates are gateRows' third: the projection judges nothing,
// and the report passed with an agents gate that did not tells the two
// apart.
func reportedView() app.UpgradeView {
	view := upgradeView()
	up := &view.Upgrade
	sub := lintID(41)
	up.State, up.Ending, up.RollbackJobID, up.RollbackComplete, up.RollbackLeft = domain.UpgradeReviewing, "", nil, nil, nil
	up.SubmissionID = &sub
	r := &domain.UpgradeReport{ComputedAt: lintTime().Add(30 * time.Minute), StalePages: 21,
		Gates: domain.UpgradeGates{
			ReservedTitles: domain.NodesGate{NodeIDs: []uuid.UUID{lintID(60)}},
			Links:          domain.LinksGate{PagesWorse: 22, NodeIDs: []uuid.UUID{lintID(61), lintID(62)}},
			Preserved:      domain.PreservedGate{Nodes: 29, NodesKept: 28, Pages: 26, PagesKept: 25, NodeIDs: []uuid.UUID{lintID(63)}}},
		Counts: domain.UpgradeCounts{Sources: 1, MovedIntoSources: 2, WikiPages: 3, NewPages: 4, Changesets: 5,
			WantedBefore: domain.LintCount{Value: 8}, WantedNow: domain.LintCount{Value: 10000, Capped: true}}}
	withGates(&r.Passed, &r.Gates, gateRows()[2])
	hash, other := strings.Repeat("a", 64), strings.Repeat("b", 64)
	pageItem := func(rule domain.LintRule, n byte) domain.LintItem {
		page := lintID(n)
		return domain.LintItem{Rule: rule, ItemKey: "p:" + page.String(), PageID: &page}
	}
	ambiguous := func(n byte, target string, cut bool) domain.LintItem {
		page := lintID(n)
		return domain.LintItem{Rule: domain.RuleAmbiguous, ItemKey: "a:" + page.String() + ":" + hash, PageID: &page, Target: target,
			TargetCut: cut}
	}
	sources := func(h, target string, cut, notLink bool) domain.LintItem {
		page := lintID(59)
		return domain.LintItem{Rule: domain.RuleInvalidSources, ItemKey: "s:" + page.String() + ":" + h, PageID: &page, Target: target,
			TargetCut: cut, NotLink: notLink}
	}
	node, source := lintID(58), lintID(64)
	items := map[domain.LintRule][]domain.LintItem{
		domain.RuleWanted: {
			{Rule: domain.RuleWanted, ItemKey: "w:" + hash, Target: "Missing", TargetCut: true, Pages: 37},
			{Rule: domain.RuleWanted, ItemKey: "w:" + other, Target: "Gone", Pages: 39}},
		domain.RuleOrphans: {pageItem(domain.RuleOrphans, 50), pageItem(domain.RuleOrphans, 51), pageItem(domain.RuleOrphans, 52),
			pageItem(domain.RuleOrphans, 53), pageItem(domain.RuleOrphans, 54)},
		domain.RuleAmbiguous:          {ambiguous(50, "Twin", true), ambiguous(51, "Pair", false)},
		domain.RuleInvalidFrontmatter: {pageItem(domain.RuleInvalidFrontmatter, 55)},
		domain.RuleMissingType:        {pageItem(domain.RuleMissingType, 56)},
		domain.RuleMissingSummary:     {pageItem(domain.RuleMissingSummary, 57)},
		domain.RuleUndigestedSources: {
			{Rule: domain.RuleUndigestedSources, ItemKey: "n:" + node.String(), NodeID: &node, Kind: domain.NodeAsset, Bytes: 38,
				CreatedAt: lintTime().Add(-time.Hour)},
			{Rule: domain.RuleUndigestedSources, ItemKey: "n:" + source.String(), NodeID: &source, Kind: domain.NodePage, Bytes: 36,
				CreatedAt: lintTime().Add(-2 * time.Hour)}},
		domain.RuleInvalidSources: {sources(hash, "literal", false, true), sources(other, "Cut", true, false)},
	}
	for i, rule := range domain.LintRules(domain.ModeLLMWiki) {
		r.Rules = append(r.Rules, domain.ReportRule{Rule: rule, Count: domain.LintCount{Value: int64(41 + i), Capped: i%3 == 1},
			Items: items[rule]})
	}
	up.Report = r
	return view
}

// A report kept is answered with the upgrade, as the contract's
// UpgradeReport (apitest checks every member), whole: no item with a
// cursor (reviews P2S2C-M4, M5, M6). Its passed and gates as each of
// gateRows.
func TestTheReportIsAnswered(t *testing.T) {
	h := upgradeServer(t, &upgradeFake{view: reportedView()})
	for _, call := range []struct{ method, sub string }{{http.MethodPost, "/submit"}, {http.MethodGet, ""}} {
		response, body := upgradeCallAt(t, h, call.method, lintID(10), call.sub, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: %d %s", call.method, call.sub, response.StatusCode, body)
		}
		var got struct {
			State  string          `json:"state"`
			Report json.RawMessage `json:"report"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.State != "reviewing" {
			t.Fatalf("answered %s", body)
		}
		sameJSON(t, got.Report, wantReport)
	}
	// Each of gateRows, the report's passed with them: only a check that
	// passed keeps a report, so a report passes in use; answered here as
	// any, so that the projection's passed and gates are each their own.
	for i, row := range gateRows() {
		view := reportedView()
		withGates(&view.Upgrade.Report.Passed, &view.Upgrade.Report.Gates, row)
		_, body := upgradeCallAt(t, upgradeServer(t, &upgradeFake{view: view}), http.MethodGet, lintID(10), "", "")
		var got struct {
			Report json.RawMessage `json:"report"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if passed := gatesPassed(t, got.Report); passed != row {
			t.Errorf("row %d: the report's passed and gates %v, want %v", i, passed, row)
		}
	}
}

const wantReport = `{
	"computed_at": "2026-01-02T03:34:05Z", "stale_pages": 21, "passed": true,
	"gates": {
		"agents": {"passed": false}, "source_root": {"passed": true},
		"reserved_titles": {"passed": true, "node_ids": ["00000000-0000-0000-0000-00000000003c"]},
		"frontmatter": {"passed": true}, "sources": {"passed": false},
		"links": {"passed": false, "pages_worse": 22, "node_ids": ["00000000-0000-0000-0000-00000000003d", "00000000-0000-0000-0000-00000000003e"]},
		"preserved": {"passed": true, "nodes": 29, "nodes_kept": 28, "pages": 26, "pages_kept": 25, "node_ids": ["00000000-0000-0000-0000-00000000003f"]},
		"index": {"passed": false}},
	"rules": [
		{"rule": "wanted", "count": 41, "count_capped": false, "items": [
			{"rule": "wanted", "item_key": "w:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "target": "Missing", "target_cut": true, "pages": 37},
			{"rule": "wanted", "item_key": "w:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "target": "Gone", "target_cut": false, "pages": 39}]},
		{"rule": "orphans", "count": 42, "count_capped": true, "items": [
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000032", "page_id": "00000000-0000-0000-0000-000000000032"},
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000033", "page_id": "00000000-0000-0000-0000-000000000033"},
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000034", "page_id": "00000000-0000-0000-0000-000000000034"},
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000035", "page_id": "00000000-0000-0000-0000-000000000035"},
			{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000036", "page_id": "00000000-0000-0000-0000-000000000036"}]},
		{"rule": "ambiguous", "count": 43, "count_capped": false, "items": [
			{"rule": "ambiguous", "item_key": "a:00000000-0000-0000-0000-000000000032:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "page_id": "00000000-0000-0000-0000-000000000032", "target": "Twin", "target_cut": true},
			{"rule": "ambiguous", "item_key": "a:00000000-0000-0000-0000-000000000033:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "page_id": "00000000-0000-0000-0000-000000000033", "target": "Pair", "target_cut": false}]},
		{"rule": "invalid_frontmatter", "count": 44, "count_capped": false, "items": [
			{"rule": "invalid_frontmatter", "item_key": "p:00000000-0000-0000-0000-000000000037", "page_id": "00000000-0000-0000-0000-000000000037"}]},
		{"rule": "missing_type", "count": 45, "count_capped": true, "items": [
			{"rule": "missing_type", "item_key": "p:00000000-0000-0000-0000-000000000038", "page_id": "00000000-0000-0000-0000-000000000038"}]},
		{"rule": "missing_summary", "count": 46, "count_capped": false, "items": [
			{"rule": "missing_summary", "item_key": "p:00000000-0000-0000-0000-000000000039", "page_id": "00000000-0000-0000-0000-000000000039"}]},
		{"rule": "undigested_sources", "count": 47, "count_capped": false, "items": [
			{"rule": "undigested_sources", "item_key": "n:00000000-0000-0000-0000-00000000003a", "node_id": "00000000-0000-0000-0000-00000000003a", "kind": "asset", "bytes": 38,
				"created_at": "2026-01-02T02:04:05Z"},
			{"rule": "undigested_sources", "item_key": "n:00000000-0000-0000-0000-000000000040", "node_id": "00000000-0000-0000-0000-000000000040", "kind": "page", "bytes": 36,
				"created_at": "2026-01-02T01:04:05Z"}]},
		{"rule": "invalid_sources", "count": 48, "count_capped": true, "items": [
			{"rule": "invalid_sources", "item_key": "s:00000000-0000-0000-0000-00000000003b:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "page_id": "00000000-0000-0000-0000-00000000003b", "target": "literal", "target_cut": false,
				"not_link": true},
			{"rule": "invalid_sources", "item_key": "s:00000000-0000-0000-0000-00000000003b:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "page_id": "00000000-0000-0000-0000-00000000003b", "target": "Cut", "target_cut": true,
				"not_link": false}]}],
	"counts": {"sources": 1, "moved_into_sources": 2, "wiki_pages": 3, "new_pages": 4, "changesets": 5,
		"wanted_before": {"count": 8, "count_capped": false}, "wanted_now": {"count": 10000, "count_capped": true}}
}`

// Each UpgradeReport item of the contract is its rule's Lint item but its
// cursor (M11/P2 design 3.13): read from api/modules/wiki.yaml, pair by
// pair, so that a change of one side that misses the other fails here.
func TestTheReportItemsAreTheLintItemsButTheirCursor(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "..", "api", "modules", "wiki.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc.Components.Schemas
	pairs := 0
	for name, report := range schemas {
		item, ok := strings.CutPrefix(name, "UpgradeReport")
		if !ok || !strings.HasSuffix(item, "Item") || item == "Item" {
			continue
		}
		pairs++
		lint, ok := schemas["Lint"+item]
		if !ok {
			t.Errorf("%s has no Lint%s", name, item)
			continue
		}
		want := map[string]any{}
		for k, v := range lint {
			want[k] = v
		}
		props := map[string]any{}
		for k, v := range lint["properties"].(map[string]any) {
			if k != "cursor" {
				props[k] = v
			}
		}
		want["properties"] = props
		want["required"] = slices.DeleteFunc(slices.Clone(lint["required"].([]any)), func(v any) bool { return v == "cursor" })
		if len(want["required"].([]any)) != len(lint["required"].([]any))-1 || len(props) != len(lint["properties"].(map[string]any))-1 {
			t.Errorf("Lint%s has no cursor to leave out", item)
		}
		if !reflect.DeepEqual(report, want) {
			t.Errorf("%s is not Lint%s but its cursor:\n%v\nwant\n%v", name, item, report, want)
		}
	}
	union := func(name string) []string {
		var out []string
		for _, ref := range schemas[name]["oneOf"].([]any) {
			out = append(out, strings.TrimPrefix(ref.(map[string]any)["$ref"].(string), "#/components/schemas/"))
		}
		return out
	}
	lints, reports := union("LintItem"), union("UpgradeReportItem")
	for i := range lints {
		lints[i] = "UpgradeReport" + strings.TrimPrefix(lints[i], "Lint")
	}
	if pairs != 8 || !slices.Equal(lints, reports) {
		t.Fatalf("%d pairs; the report's items %v, want %v", pairs, reports, lints)
	}
	mapping := func(name string) map[string]any {
		return schemas[name]["discriminator"].(map[string]any)["mapping"].(map[string]any)
	}
	for rule, ref := range mapping("LintItem") {
		if want := strings.Replace(ref.(string), "/Lint", "/UpgradeReport", 1); mapping("UpgradeReportItem")[rule] != want {
			t.Errorf("the report's %s maps to %v, want %s", rule, mapping("UpgradeReportItem")[rule], want)
		}
	}
	if len(mapping("UpgradeReportItem")) != len(mapping("LintItem")) {
		t.Errorf("the report's mapping %v", mapping("UpgradeReportItem"))
	}
}
