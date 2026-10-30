package httpadapter_test

import (
	"encoding/binary"
	"encoding/json"
	"net/url"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintContinuesAfterTheExactItem(t *testing.T) {
	f := newLintFixture(t)
	f.items[domain.RuleOrphans] = nil
	f.positions[domain.RuleOrphans] = nil
	for i := 0; i < 10002; i++ {
		var id uuid.UUID
		binary.BigEndian.PutUint32(id[12:], uint32(i))
		position := domain.LintPosition{ID: &id}
		cursor, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, domain.RuleOrphans, position)
		if err != nil {
			t.Fatal(err)
		}
		f.items[domain.RuleOrphans] = append(f.items[domain.RuleOrphans], domain.LintItem{Rule: domain.RuleOrphans, PageID: &id, ItemKey: "p:" + id.String(), Cursor: cursor})
		f.positions[domain.RuleOrphans] = append(f.positions[domain.RuleOrphans], position)
	}
	f.counts[domain.RuleOrphans] = domain.LintCount{Value: 10000, Capped: true}
	response, body := lintCall(t, f.handler, lintPath)
	if response.StatusCode != 200 {
		t.Fatalf("capped summary: %d %s", response.StatusCode, body)
	}
	var summary struct {
		Rules []struct {
			Rule   string `json:"rule"`
			Count  int    `json:"count"`
			Capped bool   `json:"count_capped"`
			Items  []struct {
				Cursor string `json:"cursor"`
			} `json:"items"`
			Next *string `json:"next_cursor"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatal(err)
	}
	row := summary.Rules[1]
	if row.Rule != "orphans" || row.Count != 10000 || !row.Capped || len(row.Items) != 5 || row.Next == nil || *row.Next != row.Items[4].Cursor {
		t.Fatalf("count cap/preview continuation: %s", body)
	}
	read := func(cursor string, wantID string, more bool) string {
		t.Helper()
		response, body := lintCall(t, f.handler, lintPath+"/orphans?limit=1&cursor="+url.QueryEscape(cursor))
		if response.StatusCode != 200 {
			t.Fatalf("continuation: %d %s", response.StatusCode, body)
		}
		var page struct {
			Data []struct {
				ID     string `json:"page_id"`
				Key    string `json:"item_key"`
				Cursor string `json:"cursor"`
			} `json:"data"`
			Next *string `json:"next_cursor"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Data) != 1 || page.Data[0].ID != wantID || page.Data[0].Key != "p:"+wantID || page.Data[0].Cursor == cursor {
			t.Fatalf("exact continuation identity: %s", body)
		}
		if more {
			if page.Next == nil || *page.Next != page.Data[0].Cursor {
				t.Fatalf("continuation must follow the returned item: %s", body)
			}
		} else if page.Next != nil {
			t.Fatalf("last item must terminate: %s", body)
		}
		return page.Data[0].Cursor
	}
	mid := read(*row.Next, "00000000-0000-0000-0000-000000000005", true)
	read(mid, "00000000-0000-0000-0000-000000000006", true)
	begin, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, domain.RuleOrphans, domain.LintPosition{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	first := read(begin, "00000000-0000-0000-0000-000000000000", true)
	read(first, "00000000-0000-0000-0000-000000000001", true)
	// This complete fake port contains 10,002 identities. Position 9,999
	// continues into both real later items; the reported cap is not an end.
	afterCap := read(f.items[domain.RuleOrphans][9999].Cursor, "00000000-0000-0000-0000-000000002710", true)
	last := read(afterCap, "00000000-0000-0000-0000-000000002711", false)
	response, body = lintCall(t, f.handler, lintPath+"/orphans?limit=1&cursor="+url.QueryEscape(last))
	var end map[string]any
	if err := json.Unmarshal(body, &end); err != nil {
		t.Fatal(err)
	}
	data, ok := end["data"].([]any)
	if response.StatusCode != 200 || !ok || len(data) != 0 || end["next_cursor"] != nil {
		t.Fatalf("after terminal must be []/null: %s", body)
	}
}
