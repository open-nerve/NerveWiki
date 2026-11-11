package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// checkRules are the eight rules' summaries of an LLM Wiki, an item each
// in the contract's shapes, cursors and continuations set; each rule's
// count its own, none its items' number, capped or not apart from its
// value.
func checkRules() []domain.LintRuleResult {
	hash := strings.Repeat("a", 64)
	page, node := lintID(21), lintID(22)
	next := "after"
	item := func(rule domain.LintRule) domain.LintItem {
		it := domain.LintItem{Rule: rule, Cursor: "c:" + string(rule)}
		switch rule {
		case domain.RuleWanted:
			it.ItemKey, it.Target, it.Pages = "w:"+hash, "Missing", 31
		case domain.RuleAmbiguous:
			it.ItemKey, it.PageID, it.Target = "a:"+page.String()+":"+hash, &page, "Two"
		case domain.RuleUndigestedSources:
			it.ItemKey, it.NodeID, it.Kind, it.Bytes, it.CreatedAt = "n:"+node.String(), &node, domain.NodeAsset, 32, lintTime()
		case domain.RuleInvalidSources:
			it.ItemKey, it.PageID, it.Target, it.NotLink = "s:"+page.String()+":"+hash, &page, "gone", true
		default:
			it.ItemKey, it.PageID = "p:"+page.String(), &page
		}
		return it
	}
	var rules []domain.LintRuleResult
	for i, rule := range domain.LintRules(domain.ModeLLMWiki) {
		rules = append(rules, domain.LintRuleResult{Rule: rule, Count: domain.LintCount{Value: int64(71 + i), Capped: i%3 == 0},
			Items: []domain.LintItem{item(rule)}, NextCursor: &next, StartCursor: &next})
	}
	return rules
}

// gateRows are four patterns of the check's passed and its eight gates,
// in UpgradeGates' order. Each gate's column is its own and neither all
// passed nor all failed, so that any two gates exchanged, or a gate made
// constant, show in one row (review P2S2C-M6); the second row is aCheck's.
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
func withGates(passed *bool, g *domain.UpgradeGates, row [9]bool) {
	*passed, g.Agents, g.SourceRoot, g.ReservedTitles.Passed, g.Frontmatter, g.Sources, g.Links.Passed, g.Preserved.Passed, g.Index =
		row[0], row[1], row[2], row[3], row[4], row[5], row[6], row[7], row[8]
}

// gatesPassed is the answer's passed and its gates' passed, in gateRows'
// order.
func gatesPassed(t *testing.T, body []byte) [9]bool {
	t.Helper()
	var got struct {
		Passed bool `json:"passed"`
		Gates  map[string]struct {
			Passed bool `json:"passed"`
		} `json:"gates"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	out := [9]bool{got.Passed}
	for i, gate := range []string{"agents", "source_root", "reserved_titles", "frontmatter", "sources", "links", "preserved", "index"} {
		out[i+1] = got.Gates[gate].Passed
	}
	return out
}

// sameJSON fails t unless got is want, as JSON values.
func sameJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("answered\n%s\nwant\n%s", got, want)
	}
}

// aCheck is a check that names nodes in the three gates that do, each
// number its own.
func aCheck() *domain.UpgradeCheck {
	c := &domain.UpgradeCheck{ComputedAt: lintTime(), StalePages: 11,
		Gates: domain.UpgradeGates{
			ReservedTitles: domain.NodesGate{NodeIDs: []uuid.UUID{lintID(40), lintID(41)}},
			Links:          domain.LinksGate{PagesWorse: 12, NodeIDs: []uuid.UUID{lintID(42), lintID(43), lintID(44), lintID(45), lintID(46)}},
			Preserved: domain.PreservedGate{Nodes: 19, NodesKept: 18, Pages: 16, PagesKept: 15,
				NodeIDs: []uuid.UUID{lintID(47), lintID(48)}}},
		Rules: checkRules(),
		Counts: domain.UpgradeCounts{Sources: 1, MovedIntoSources: 2, WikiPages: 3, NewPages: 4, Changesets: 5,
			WantedBefore: domain.LintCount{Value: 10000, Capped: true}, WantedNow: domain.LintCount{Value: 6}},
		Paths: map[uuid.UUID]string{lintID(40): "index.md"},
	}
	withGates(&c.Passed, &c.Gates, gateRows()[1])
	return c
}

// The check answers the use case's check in the contract's shape (M11/P2
// design 3.13), whole: eight gates, three naming their nodes, the eight
// rules' summaries, the counts; the paths, MCP's, not. Each gate is its
// own in one of gateRows; one of the wanted counts capped, the other not,
// the report's test the other way round (reviews P2S2C-M4, M5, M6).
func TestTheCheckIsAnswered(t *testing.T) {
	f := &upgradeFake{checked: app.UpgradeChecked{Check: aCheck()}}
	h := upgradeServer(t, f)
	response, body := upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("check = %d %s", response.StatusCode, body)
	}
	sameJSON(t, body, wantCheck)
	if len(f.queries) != 1 || f.queries[0] != (app.LintQuery{}) {
		t.Errorf("the check asked %+v", f.queries)
	}
	for i, row := range gateRows() {
		c := aCheck()
		withGates(&c.Passed, &c.Gates, row)
		h = upgradeServer(t, &upgradeFake{checked: app.UpgradeChecked{Check: c}})
		if _, body := upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check", ""); gatesPassed(t, body) != row {
			t.Errorf("row %d: passed and gates %v, want %v", i, gatesPassed(t, body), row)
		}
	}
	// A gate that names none answers [].
	empty := aCheck()
	empty.Gates.ReservedTitles.NodeIDs, empty.Gates.Links.NodeIDs, empty.Gates.Preserved.NodeIDs = nil, nil, nil
	h = upgradeServer(t, &upgradeFake{checked: app.UpgradeChecked{Check: empty}})
	if _, body := upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check", ""); strings.Count(string(body), `"node_ids":[]`) != 3 {
		t.Errorf("a check naming no node = %s", body)
	}

	// A rule's page as lint's, whole: two items, the page's stale pages
	// and continuation.
	rule, next, other := checkRules()[4], "more", lintID(23)
	second := rule.Items[0]
	second.ItemKey, second.PageID, second.Cursor = "p:"+other.String(), &other, "c:second"
	rule.Items, rule.NextCursor = append(rule.Items, second), &next
	items := domain.LintResult{Kind: domain.LintItems, Mode: domain.ModeLLMWiki, ComputedAt: lintTime(), StalePages: 13,
		Rules: []domain.LintRuleResult{rule}}
	f = &upgradeFake{checked: app.UpgradeChecked{Items: &items}}
	h = upgradeServer(t, f)
	response, body = upgradeCallAt(t, h, http.MethodGet, lintID(10), "/check/missing_type?limit=2&cursor=c", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the items = %d %s", response.StatusCode, body)
	}
	sameJSON(t, body, wantCheckItems)
	if q := f.queries; len(q) != 1 || q[0].Rule == nil || *q[0].Rule != "missing_type" || q[0].Limit == nil || *q[0].Limit != 2 ||
		q[0].Cursor == nil || *q[0].Cursor != "c" {
		t.Errorf("the items asked %+v", q)
	}
}

const wantCheck = `{
	"computed_at": "2026-01-02T03:04:05Z", "stale_pages": 11, "passed": false,
	"gates": {
		"agents": {"passed": true}, "source_root": {"passed": true},
		"reserved_titles": {"passed": false, "node_ids": ["00000000-0000-0000-0000-000000000028", "00000000-0000-0000-0000-000000000029"]},
		"frontmatter": {"passed": false}, "sources": {"passed": false},
		"links": {"passed": true, "pages_worse": 12, "node_ids": ["00000000-0000-0000-0000-00000000002a", "00000000-0000-0000-0000-00000000002b", "00000000-0000-0000-0000-00000000002c",
			"00000000-0000-0000-0000-00000000002d", "00000000-0000-0000-0000-00000000002e"]},
		"preserved": {"passed": true, "nodes": 19, "nodes_kept": 18, "pages": 16, "pages_kept": 15,
			"node_ids": ["00000000-0000-0000-0000-00000000002f", "00000000-0000-0000-0000-000000000030"]},
		"index": {"passed": false}},
	"rules": [
		{"rule": "wanted", "count": 71, "count_capped": true, "next_cursor": "after", "items": [{"rule": "wanted", "cursor": "c:wanted",
			"item_key": "w:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "target": "Missing", "target_cut": false, "pages": 31}]},
		{"rule": "orphans", "count": 72, "count_capped": false, "next_cursor": "after", "items": [{"rule": "orphans",
			"cursor": "c:orphans", "item_key": "p:00000000-0000-0000-0000-000000000015", "page_id": "00000000-0000-0000-0000-000000000015"}]},
		{"rule": "ambiguous", "count": 73, "count_capped": false, "next_cursor": "after", "items": [{"rule": "ambiguous",
			"cursor": "c:ambiguous", "item_key": "a:00000000-0000-0000-0000-000000000015:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "page_id": "00000000-0000-0000-0000-000000000015", "target": "Two", "target_cut": false}]},
		{"rule": "invalid_frontmatter", "count": 74, "count_capped": true, "next_cursor": "after", "items": [{"rule": "invalid_frontmatter",
			"cursor": "c:invalid_frontmatter", "item_key": "p:00000000-0000-0000-0000-000000000015", "page_id": "00000000-0000-0000-0000-000000000015"}]},
		{"rule": "missing_type", "count": 75, "count_capped": false, "next_cursor": "after", "items": [{"rule": "missing_type",
			"cursor": "c:missing_type", "item_key": "p:00000000-0000-0000-0000-000000000015", "page_id": "00000000-0000-0000-0000-000000000015"}]},
		{"rule": "missing_summary", "count": 76, "count_capped": false, "next_cursor": "after", "items": [{"rule": "missing_summary",
			"cursor": "c:missing_summary", "item_key": "p:00000000-0000-0000-0000-000000000015", "page_id": "00000000-0000-0000-0000-000000000015"}]},
		{"rule": "undigested_sources", "count": 77, "count_capped": true, "next_cursor": "after", "items": [{"rule": "undigested_sources",
			"cursor": "c:undigested_sources", "item_key": "n:00000000-0000-0000-0000-000000000016", "node_id": "00000000-0000-0000-0000-000000000016", "kind": "asset", "bytes": 32,
			"created_at": "2026-01-02T03:04:05Z"}]},
		{"rule": "invalid_sources", "count": 78, "count_capped": false, "next_cursor": "after", "items": [{"rule": "invalid_sources",
			"cursor": "c:invalid_sources", "item_key": "s:00000000-0000-0000-0000-000000000015:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "page_id": "00000000-0000-0000-0000-000000000015", "target": "gone", "target_cut": false,
			"not_link": true}]}],
	"counts": {"sources": 1, "moved_into_sources": 2, "wiki_pages": 3, "new_pages": 4, "changesets": 5,
		"wanted_before": {"count": 10000, "count_capped": true}, "wanted_now": {"count": 6, "count_capped": false}}
}`

const wantCheckItems = `{
	"mode": "llm_wiki", "computed_at": "2026-01-02T03:04:05Z", "stale_pages": 13, "next_cursor": "more",
	"data": [
		{"rule": "missing_type", "cursor": "c:missing_type", "item_key": "p:00000000-0000-0000-0000-000000000015", "page_id": "00000000-0000-0000-0000-000000000015"},
		{"rule": "missing_type", "cursor": "c:second", "item_key": "p:00000000-0000-0000-0000-000000000017", "page_id": "00000000-0000-0000-0000-000000000017"}]
}`

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
