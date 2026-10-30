package httpadapter_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func lintRichFixture(t *testing.T, mode domain.Mode) *lintFixture {
	t.Helper()
	f := newLintFixture(t, func(f *lintFixture) { f.mode = mode })
	target := strings.Repeat("界", 85) + "x"
	hashes := []string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), strings.Repeat("4", 64), strings.Repeat("5", 64)}
	f.items[domain.RuleWanted] = []domain.LintItem{
		{Rule: domain.RuleWanted, ItemKey: "w:" + hashes[0], Target: target, TargetCut: true, Pages: 7},
		{Rule: domain.RuleWanted, ItemKey: "w:" + hashes[1], Target: target, TargetCut: true, Pages: 3},
	}
	f.positions[domain.RuleWanted] = []domain.LintPosition{{Pages: 7, Prefix: []byte(target), Hash: hashes[0]}, {Pages: 3, Prefix: []byte(target), Hash: hashes[1]}}
	for i, rule := range []domain.LintRule{domain.RuleAmbiguous, domain.RuleInvalidFrontmatter, domain.RuleMissingType, domain.RuleMissingSummary} {
		id := lintID(byte(21 + i))
		item := domain.LintItem{Rule: rule, PageID: &id, ItemKey: "p:" + id.String()}
		position := domain.LintPosition{ID: &id}
		if rule == domain.RuleAmbiguous {
			item.ItemKey = "a:" + id.String() + ":" + hashes[2]
			item.Target = "ambiguous"
			position.Hash = hashes[2]
		}
		f.items[rule] = []domain.LintItem{item}
		f.positions[rule] = []domain.LintPosition{position}
	}
	page, asset := lintID(25), lintID(26)
	created := time.Date(1980, time.March, 4, 5, 6, 7, 123000000, time.UTC)
	f.items[domain.RuleUndigestedSources] = []domain.LintItem{
		{Rule: domain.RuleUndigestedSources, NodeID: &page, ItemKey: "n:" + page.String(), Kind: domain.NodePage, CreatedAt: created, Bytes: 0},
		{Rule: domain.RuleUndigestedSources, NodeID: &asset, ItemKey: "n:" + asset.String(), Kind: domain.NodeAsset, CreatedAt: created.Add(time.Second), Bytes: 1099511627793},
	}
	f.positions[domain.RuleUndigestedSources] = []domain.LintPosition{{ID: &page, CreatedAt: created}, {ID: &asset, CreatedAt: created.Add(time.Second)}}
	id := lintID(27)
	f.items[domain.RuleInvalidSources] = []domain.LintItem{
		{Rule: domain.RuleInvalidSources, PageID: &id, ItemKey: "s:" + id.String() + ":" + hashes[3], Target: "broken", NotLink: false},
		{Rule: domain.RuleInvalidSources, PageID: &id, ItemKey: "s:" + id.String() + ":" + hashes[4], Target: "null", NotLink: true},
	}
	f.positions[domain.RuleInvalidSources] = []domain.LintPosition{{ID: &id, Hash: hashes[3]}, {ID: &id, Hash: hashes[4]}}
	for rule, items := range f.items {
		f.counts[rule] = domain.LintCount{Value: int64(len(items))}
		for i := range items {
			cursor, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, rule, f.positions[rule][i])
			if err != nil {
				t.Fatal(err)
			}
			items[i].Cursor = cursor
		}
	}
	return f
}

// The expected public fields are literal protocol values, not produced by
// the handler, generated union helpers, or the rule implementation.
func lintExpectedItems() map[string][]map[string]any {
	target := strings.Repeat("界", 85) + "x"
	return map[string][]map[string]any{
		"wanted": {
			{"rule": "wanted", "item_key": "w:" + strings.Repeat("1", 64), "pages": float64(7), "target": target, "target_cut": true},
			{"rule": "wanted", "item_key": "w:" + strings.Repeat("2", 64), "pages": float64(3), "target": target, "target_cut": true},
		},
		"orphans":             {{"rule": "orphans", "item_key": "p:00000000-0000-0000-0000-000000000000", "page_id": "00000000-0000-0000-0000-000000000000"}},
		"ambiguous":           {{"rule": "ambiguous", "item_key": "a:00000000-0000-0000-0000-000000000015:" + strings.Repeat("3", 64), "page_id": "00000000-0000-0000-0000-000000000015", "target": "ambiguous", "target_cut": false}},
		"invalid_frontmatter": {{"rule": "invalid_frontmatter", "item_key": "p:00000000-0000-0000-0000-000000000016", "page_id": "00000000-0000-0000-0000-000000000016"}},
		"missing_type":        {{"rule": "missing_type", "item_key": "p:00000000-0000-0000-0000-000000000017", "page_id": "00000000-0000-0000-0000-000000000017"}},
		"missing_summary":     {{"rule": "missing_summary", "item_key": "p:00000000-0000-0000-0000-000000000018", "page_id": "00000000-0000-0000-0000-000000000018"}},
		"undigested_sources": {
			{"rule": "undigested_sources", "item_key": "n:00000000-0000-0000-0000-000000000019", "node_id": "00000000-0000-0000-0000-000000000019", "kind": "page", "bytes": float64(0), "created_at": "1980-03-04T05:06:07.123Z"},
			{"rule": "undigested_sources", "item_key": "n:00000000-0000-0000-0000-00000000001a", "node_id": "00000000-0000-0000-0000-00000000001a", "kind": "asset", "bytes": float64(1099511627793), "created_at": "1980-03-04T05:06:08.123Z"},
		},
		"invalid_sources": {
			{"rule": "invalid_sources", "item_key": "s:00000000-0000-0000-0000-00000000001b:" + strings.Repeat("4", 64), "page_id": "00000000-0000-0000-0000-00000000001b", "target": "broken", "target_cut": false, "not_link": false},
			{"rule": "invalid_sources", "item_key": "s:00000000-0000-0000-0000-00000000001b:" + strings.Repeat("5", 64), "page_id": "00000000-0000-0000-0000-00000000001b", "target": "null", "target_cut": false, "not_link": true},
		},
	}
}

func TestLintProjectsEveryBranch(t *testing.T) {
	for _, mode := range []domain.Mode{domain.ModeStandard, domain.ModeLLMWiki} {
		t.Run(string(mode), func(t *testing.T) {
			f := lintRichFixture(t, mode)
			before, err := json.Marshal(f.items)
			if err != nil {
				t.Fatal(err)
			}
			response, body := lintCall(t, f.handler, lintPath)
			if response.StatusCode != 200 {
				t.Fatalf("rich summary: %d %s", response.StatusCode, body)
			}
			var summary struct {
				Mode  string `json:"mode"`
				Rules []struct {
					Rule   string           `json:"rule"`
					Count  int              `json:"count"`
					Capped bool             `json:"count_capped"`
					Items  []map[string]any `json:"items"`
					Next   *string          `json:"next_cursor"`
				} `json:"rules"`
			}
			if err := json.Unmarshal(body, &summary); err != nil {
				t.Fatal(err)
			}
			wantRules := []string{"wanted", "orphans", "ambiguous", "invalid_frontmatter", "missing_type", "missing_summary", "undigested_sources", "invalid_sources"}
			if mode == domain.ModeStandard {
				wantRules = wantRules[:4]
			}
			if summary.Mode != string(mode) || len(summary.Rules) != len(wantRules) {
				t.Fatalf("mode/rule count: %s", body)
			}
			wants := lintExpectedItems()
			for i, name := range wantRules {
				row := summary.Rules[i]
				if row.Rule != name || row.Count != len(wants[name]) || row.Capped || row.Next != nil {
					t.Fatalf("summary branch %s: %s", name, body)
				}
				for j := range wants[name] {
					wants[name][j]["cursor"] = f.items[domain.LintRule(name)][j].Cursor
				}
				if !reflect.DeepEqual(row.Items, wants[name]) {
					t.Fatalf("summary public fields %s = %v; want %v", name, row.Items, wants[name])
				}
				response, listBody := lintCall(t, f.handler, lintPath+"/"+name)
				if response.StatusCode != 200 {
					t.Fatalf("rich list %s: %d %s", name, response.StatusCode, listBody)
				}
				var page struct {
					Data []map[string]any `json:"data"`
					Next *string          `json:"next_cursor"`
				}
				if err := json.Unmarshal(listBody, &page); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(page.Data, wants[name]) || page.Next != nil {
					t.Fatalf("list public fields %s: %s", name, listBody)
				}
				for _, item := range page.Data {
					if value, ok := item["target"].(string); ok && (!utf8.ValidString(value) || len(value) > 256) {
						t.Fatalf("target UTF-8/byte limit: %q", value)
					}
				}
			}
			after, err := json.Marshal(f.items)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("HTTP/runtime mutated its rule port's facts")
			}
		})
	}
}
