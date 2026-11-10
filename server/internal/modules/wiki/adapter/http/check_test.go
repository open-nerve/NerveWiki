package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// checkRules are the eight rules' summaries of an LLM Wiki, an item each
// in the contract's shapes, cursors and continuations set.
func checkRules() []domain.LintRuleResult {
	hash := strings.Repeat("a", 64)
	page, node := lintID(21), lintID(22)
	next := "after"
	item := func(rule domain.LintRule) domain.LintItem {
		it := domain.LintItem{Rule: rule, Cursor: "c:" + string(rule)}
		switch rule {
		case domain.RuleWanted:
			it.ItemKey, it.Target, it.Pages = "w:"+hash, "Missing", 1
		case domain.RuleAmbiguous:
			it.ItemKey, it.PageID, it.Target = "a:"+page.String()+":"+hash, &page, "Two"
		case domain.RuleUndigestedSources:
			it.ItemKey, it.NodeID, it.Kind, it.CreatedAt = "n:"+node.String(), &node, domain.NodeAsset, lintTime()
		case domain.RuleInvalidSources:
			it.ItemKey, it.PageID, it.Target, it.NotLink = "s:"+page.String()+":"+hash, &page, "gone", true
		default:
			it.ItemKey, it.PageID = "p:"+page.String(), &page
		}
		return it
	}
	var rules []domain.LintRuleResult
	for _, rule := range domain.LintRules(domain.ModeLLMWiki) {
		rules = append(rules, domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: 7}, Items: []domain.LintItem{item(rule)},
			NextCursor: &next, StartCursor: &next})
	}
	return rules
}

// aCheck is a check that names nodes in the three gates that do.
func aCheck() *domain.UpgradeCheck {
	return &domain.UpgradeCheck{ComputedAt: lintTime(), StalePages: 2, Passed: false,
		Gates: domain.UpgradeGates{Agents: true, SourceRoot: true, Frontmatter: false, Sources: true, Index: false,
			ReservedTitles: domain.NodesGate{NodeIDs: []uuid.UUID{lintID(40), lintID(41)}},
			Links:          domain.LinksGate{PagesWorse: 7, NodeIDs: []uuid.UUID{lintID(42), lintID(43), lintID(44), lintID(45), lintID(46)}},
			Preserved: domain.PreservedGate{Passed: false, Nodes: 9, NodesKept: 8, Pages: 6, PagesKept: 5,
				NodeIDs: []uuid.UUID{lintID(47), lintID(48)}}},
		Rules: checkRules(),
		Counts: domain.UpgradeCounts{Sources: 1, MovedIntoSources: 2, WikiPages: 3, NewPages: 4, Changesets: 5,
			WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: domain.LintCount{Value: 6}},
		Paths: map[uuid.UUID]string{lintID(40): "index.md"},
	}
}

// The check answers the use case's check in the contract's shape (M11/P2
// design 3.13): eight gates, three naming their nodes, the eight rules'
// summaries, the counts; the paths, MCP's, not; the rule's page as lint's.
func TestTheCheckIsAnswered(t *testing.T) {
	f := &upgradeFake{checked: app.UpgradeChecked{Check: aCheck()}}
	h := upgradeServer(t, f)
	response, body := upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("check = %d %s", response.StatusCode, body)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	gates, counts := got["gates"].(map[string]any), got["counts"].(map[string]any)
	if got["passed"] != false || got["stale_pages"] != float64(2) || len(got["rules"].([]any)) != 8 || len(gates) != 8 ||
		len(gates["links"].(map[string]any)["node_ids"].([]any)) != 5 || gates["preserved"].(map[string]any)["nodes_kept"] != float64(8) ||
		gates["agents"].(map[string]any)["passed"] != true || counts["moved_into_sources"] != float64(2) ||
		counts["wanted_before"].(map[string]any)["count_capped"] != true || strings.Contains(string(body), "index.md") {
		t.Errorf("check = %s", body)
	}
	if len(f.queries) != 1 || f.queries[0] != (app.LintQuery{}) {
		t.Errorf("the check asked %+v", f.queries)
	}
	// A gate that names none answers [].
	empty := aCheck()
	empty.Gates.ReservedTitles.NodeIDs, empty.Gates.Links.NodeIDs, empty.Gates.Preserved.NodeIDs = nil, nil, nil
	h = upgradeServer(t, &upgradeFake{checked: app.UpgradeChecked{Check: empty}})
	if _, body := upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check", ""); strings.Count(string(body), `"node_ids":[]`) != 3 {
		t.Errorf("a check naming no node = %s", body)
	}

	rule, next := checkRules()[3], "more"
	rule.NextCursor = &next
	items := domain.LintResult{Kind: domain.LintItems, Mode: domain.ModeLLMWiki, ComputedAt: lintTime(), Rules: []domain.LintRuleResult{rule}}
	f = &upgradeFake{checked: app.UpgradeChecked{Items: &items}}
	h = upgradeServer(t, f)
	response, body = upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check/missing_type?limit=1&cursor=c", "")
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"next_cursor":"more"`) ||
		!strings.Contains(string(body), `"mode":"llm_wiki"`) {
		t.Errorf("the items = %d %s", response.StatusCode, body)
	}
	if q := f.queries; len(q) != 1 || q[0].Rule == nil || *q[0].Rule != "missing_type" || q[0].Limit == nil || *q[0].Limit != 1 ||
		q[0].Cursor == nil || *q[0].Cursor != "c" {
		t.Errorf("the items asked %+v", q)
	}
}

// Each code the check's two operations declare is answered (apitest's
// unanswered check), server_busy with Retry-After: 1.
func TestTheChecksCodesAreAnswered(t *testing.T) {
	for _, tt := range []struct {
		sub  string
		err  error
		code string
	}{
		{"/check", domain.ErrNotebookNotFound, "notebook.not_found"},
		{"/check", shared.Forbidden(), "forbidden"},
		{"/check", domain.ErrUpgradeState, "wiki.upgrade_state"},
		{"/check", shared.ServerBusy(time.Second), "server_busy"},
		{"/check/wanted", shared.InvalidCursor(), "bad_request"},
		{"/check/wanted", domain.ErrNotebookNotFound, "notebook.not_found"},
		{"/check/wanted", shared.Forbidden(), "forbidden"},
		{"/check/wanted", domain.ErrLintRuleNotFound, "wiki.lint_rule_not_found"},
		{"/check/wanted", domain.ErrUpgradeState, "wiki.upgrade_state"},
		{"/check/wanted", shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange, Message: "1 to 100"}),
			"validation_failed"},
		{"/check/wanted", shared.ServerBusy(time.Second), "server_busy"},
	} {
		t.Run(tt.sub+"/"+tt.code, func(t *testing.T) {
			h := upgradeServer(t, &upgradeFake{err: tt.err})
			response, got := upgradeCallAt(t, h, http.MethodGet, lintID(10), tt.sub, "")
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(got, &problem); err != nil {
				t.Fatal(err)
			}
			if problem.Code != tt.code || tt.code == "server_busy" && response.Header.Get("Retry-After") != "1" {
				t.Fatalf("%d %s, want %s", response.StatusCode, got, tt.code)
			}
		})
	}
}
