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
	"uuid"

	"go.yaml.in/yaml/v3"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

// reportedView is an upgrade reviewing with a report kept: each of the
// eight rules with five items of its shape, three gates naming nodes.
func reportedView() app.UpgradeView {
	view := upgradeView()
	up := &view.Upgrade
	sub := lintID(41)
	up.State, up.Ending, up.RollbackJobID, up.RollbackComplete, up.RollbackLeft = domain.UpgradeReviewing, "", nil, nil, nil
	up.SubmissionID = &sub
	page, node := lintID(50), lintID(51)
	r := &domain.UpgradeReport{ComputedAt: lintTime(), Passed: true,
		Gates: domain.UpgradeGates{Agents: true, SourceRoot: true, Frontmatter: true, Sources: true, Index: true,
			ReservedTitles: domain.NodesGate{Passed: true, NodeIDs: []uuid.UUID{lintID(60)}},
			Links:          domain.LinksGate{Passed: true, NodeIDs: []uuid.UUID{lintID(61), lintID(62)}},
			Preserved:      domain.PreservedGate{Passed: true, Nodes: 7, NodesKept: 7, Pages: 5, PagesKept: 5, NodeIDs: []uuid.UUID{lintID(63)}}},
		Counts: domain.UpgradeCounts{Sources: 1, MovedIntoSources: 2, WikiPages: 3, NewPages: 4, Changesets: 5,
			WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: domain.LintCount{Value: 6}}}
	hash := strings.Repeat("a", 64)
	for _, rule := range domain.LintRules(domain.ModeLLMWiki) {
		rr := domain.ReportRule{Rule: rule, Count: domain.LintCount{Value: 5}}
		for range 5 {
			item := domain.LintItem{Rule: rule, PageID: &page}
			switch rule {
			case domain.RuleWanted:
				item.ItemKey, item.PageID, item.Target, item.Pages = "w:"+hash, nil, "Missing", 2
			case domain.RuleAmbiguous:
				item.ItemKey, item.Target, item.TargetCut = "a:"+page.String()+":"+hash, "Twin", true
			case domain.RuleInvalidSources:
				item.ItemKey, item.Target, item.NotLink = "s:"+page.String()+":"+hash, "literal", true
			case domain.RuleUndigestedSources:
				item.ItemKey, item.PageID, item.NodeID, item.Kind, item.Bytes, item.CreatedAt = "n:"+node.String(), nil, &node,
					domain.NodeAsset, 12, lintTime()
			default:
				item.ItemKey = "p:" + page.String()
			}
			rr.Items = append(rr.Items, item)
		}
		r.Rules = append(r.Rules, rr)
	}
	up.Report = r
	return view
}

// A report kept is answered with the upgrade, as the contract's
// UpgradeReport (apitest checks every member): eight rules of five items
// each, none with a cursor, the gates' nodes.
func TestTheReportIsAnswered(t *testing.T) {
	h := upgradeServer(t, &upgradeFake{view: reportedView()})
	for _, call := range []struct{ method, sub string }{{http.MethodPost, "/submit"}, {http.MethodGet, ""}} {
		response, body := upgradeCallAt(t, h, call.method, lintID(10), call.sub, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: %d %s", call.method, call.sub, response.StatusCode, body)
		}
		var got struct {
			State  string `json:"state"`
			Report struct {
				Gates map[string]struct {
					NodeIDs []string `json:"node_ids"`
				} `json:"gates"`
				Rules []struct {
					Rule  string           `json:"rule"`
					Items []map[string]any `json:"items"`
				} `json:"rules"`
				Counts map[string]any `json:"counts"`
			} `json:"report"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.State != "reviewing" || len(got.Report.Rules) != 8 || len(got.Report.Gates["links"].NodeIDs) != 2 ||
			len(got.Report.Gates["agents"].NodeIDs) != 0 || got.Report.Counts["new_pages"] != float64(4) {
			t.Fatalf("answered %s", body)
		}
		for _, rule := range got.Report.Rules {
			if len(rule.Items) != 5 || rule.Items[0]["rule"] != rule.Rule {
				t.Fatalf("the rule %s answered %v", rule.Rule, rule.Items)
			}
			if _, cursor := rule.Items[0]["cursor"]; cursor {
				t.Fatalf("an item of %s has a cursor", rule.Rule)
			}
		}
	}
}

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
