package httpadapter_test

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintReadsTheActualRuntime(t *testing.T) {
	f := newLintFixture(t)
	response, body := lintCall(t, f.handler, lintPath)
	if response.StatusCode != 200 {
		t.Fatalf("real runtime summary: status %d: %s", response.StatusCode, body)
	}
	var got struct {
		Mode       string `json:"mode"`
		ComputedAt string `json:"computed_at"`
		StalePages int    `json:"stale_pages"`
		Rules      []struct {
			Rule        string           `json:"rule"`
			Count       int              `json:"count"`
			CountCapped bool             `json:"count_capped"`
			Items       []map[string]any `json:"items"`
			NextCursor  *string          `json:"next_cursor"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "standard" || got.ComputedAt != "2026-01-02T03:04:05Z" || got.StalePages != 2 || len(got.Rules) != 4 {
		t.Fatalf("summary envelope: %s", body)
	}
	for i, rule := range []string{"wanted", "orphans", "ambiguous", "invalid_frontmatter"} {
		row := got.Rules[i]
		if row.Rule != rule || row.Items == nil || row.NextCursor != nil || row.CountCapped {
			t.Fatalf("rule %s: %s", rule, body)
		}
		if rule == "orphans" {
			if row.Count != 1 || len(row.Items) != 1 {
				t.Fatalf("orphan count/items: %s", body)
			}
			item := row.Items[0]
			if item["page_id"] != "00000000-0000-0000-0000-000000000000" || item["item_key"] != "p:00000000-0000-0000-0000-000000000000" || item["rule"] != "orphans" || item["cursor"] == "" || len(item) != 4 {
				t.Fatalf("orphan projection: %v", item)
			}
		} else if row.Count != 0 || len(row.Items) != 0 {
			t.Fatalf("empty %s: %s", rule, body)
		}
	}
	if strings.Contains(string(body), "private/title") || strings.Contains(string(body), "start_cursor") || strings.Contains(string(body), "timings") {
		t.Fatalf("internal data escaped: %s", body)
	}
	wantCalls := []string{"workspace", "authorize", "snapshot", "state", "nodes", "prepare", "paths", "workspace", "authorize"}
	if calls := f.trace(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("actual runtime calls = %v; want %v", calls, wantCalls)
	}
}

func TestLintListsWithTheBeginCursor(t *testing.T) {
	f := newLintFixture(t)
	begin, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, domain.RuleOrphans, domain.LintPosition{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	response, body := lintCall(t, f.handler, lintPath+"/orphans?limit=1&cursor="+url.QueryEscape(begin))
	if response.StatusCode != 200 {
		t.Fatalf("begin cursor list: status %d: %s", response.StatusCode, body)
	}
	var got struct {
		Mode       string           `json:"mode"`
		ComputedAt string           `json:"computed_at"`
		StalePages int              `json:"stale_pages"`
		Data       []map[string]any `json:"data"`
		NextCursor *string          `json:"next_cursor"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "standard" || got.ComputedAt != "2026-01-02T03:04:05Z" || got.StalePages != 2 || got.NextCursor != nil || len(got.Data) != 1 {
		t.Fatalf("list envelope: %s", body)
	}
	item := got.Data[0]
	if item["page_id"] != "00000000-0000-0000-0000-000000000000" || item["item_key"] != "p:00000000-0000-0000-0000-000000000000" || item["cursor"] == begin || item["cursor"] == "" || item["rule"] != "orphans" || len(item) != 4 {
		t.Fatalf("begin must return the real zero UUID item: %s", body)
	}
	f.mu.Lock()
	query := f.query
	f.mu.Unlock()
	if query.Rule == nil || *query.Rule != domain.RuleOrphans || query.Limit != 1 || query.After == nil || !query.After.Begin || query.After.ID != nil {
		t.Fatalf("HTTP input did not reach the actual runtime: %+v", query)
	}
}
