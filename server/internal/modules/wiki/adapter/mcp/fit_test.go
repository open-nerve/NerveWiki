package mcpadapter

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

type lintReadPage struct {
	Mode       string           `json:"mode"`
	ComputedAt string           `json:"computed_at"`
	Stale      int64            `json:"stale_pages"`
	Data       []map[string]any `json:"data"`
	Next       *string          `json:"next_cursor"`
}
type lintReadSummary struct {
	Mode       string `json:"mode"`
	ComputedAt string `json:"computed_at"`
	Stale      int64  `json:"stale_pages"`
	Rules      []struct {
		Rule   string           `json:"rule"`
		Count  int64            `json:"count"`
		Capped bool             `json:"count_capped"`
		Items  []map[string]any `json:"items"`
		Next   *string          `json:"next_cursor"`
	} `json:"rules"`
}

func TestLintFitsTheSDK(t *testing.T) {
	for _, account := range []bool{false, true} {
		t.Run(map[bool]string{false: "notebook", true: "account"}[account], func(t *testing.T) {
			data := lintDataSet(t, 10, 1)
			f := newLintRuntime(t, data)
			server := lintServer(t, f.lint)
			for _, row := range data.rows {
				result, size := lintCall(t, server, account, "lint", map[string]any{"rule": string(row.Rule)})
				var got lintReadPage
				if err := json.Unmarshal(result.Structured, &got); err != nil {
					t.Fatal(err)
				}
				if got.Mode != "llm_wiki" || got.ComputedAt != "2026-01-02T03:04:05Z" || got.Stale != 7 || len(got.Data) != 1 || got.Next != nil {
					t.Fatalf("largest legal item must progress: %s", result.Structured)
				}
				lintAssertItem(t, got.Data[0], row.Items[0])
				t.Logf("largest legal %s SDK=%d cursor=%d", row.Rule, size, len(row.Items[0].Cursor))
			}
			pending, size := lintCall(t, server, account, "pending_sources", nil)
			var got lintReadPage
			if err := json.Unmarshal(pending.Structured, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != 1 || got.Data[0]["kind"] != "asset" || got.Data[0]["bytes"] != float64(9000) {
				t.Fatalf("pending source projection: %s", pending.Structured)
			}
			t.Logf("largest legal pending_sources SDK=%d", size)
			for _, mode := range []domain.Mode{domain.ModeStandard, domain.ModeLLMWiki} {
				small := lintDataSet(t, 1, 1)
				small.mode = mode
				runtime := newLintRuntime(t, small)
				answer, size := lintCall(t, lintServer(t, runtime.lint), account, "lint", nil)
				var got lintReadSummary
				if err := json.Unmarshal(answer.Structured, &got); err != nil {
					t.Fatal(err)
				}
				want := 8
				if mode == domain.ModeStandard {
					want = 4
				}
				if len(got.Rules) != want || got.Mode != string(mode) || got.Stale != 7 {
					t.Fatalf("summary branches: %s", answer.Structured)
				}
				for i, row := range got.Rules {
					if row.Rule != string(small.rows[i].Rule) || row.Count != 1 || row.Capped || len(row.Items) != 1 || row.Next != nil {
						t.Fatalf("summary branch %d: %s", i, answer.Structured)
					}
					lintAssertItem(t, row.Items[0], small.rows[i].Items[0])
				}
				t.Logf("%s summary SDK=%d", mode, size)
			}
		})
	}
}

// This expectation is the protocol field table, with scalar values supplied
// by the independent fixture. No production projection builds this oracle.
func lintAssertItem(t *testing.T, got map[string]any, item domain.LintItem) {
	t.Helper()
	want := lintItemFields(item)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("complete %s fields differ; got %v want %v", item.Rule, got, want)
	}
}

func lintItemFields(item domain.LintItem) map[string]any {
	want := map[string]any{"rule": string(item.Rule), "item_key": item.ItemKey, "cursor": item.Cursor}
	switch item.Rule {
	case domain.RuleWanted:
		want["target"], want["target_cut"], want["pages"] = item.Target, item.TargetCut, float64(item.Pages)
	case domain.RuleUndigestedSources:
		want["node_id"], want["path"], want["kind"], want["bytes"], want["created_at"] = item.NodeID.String(), *item.Path, string(item.Kind), float64(item.Bytes), item.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
	default:
		want["page_id"], want["path"] = item.PageID.String(), *item.Path
		if item.Rule == domain.RuleAmbiguous || item.Rule == domain.RuleInvalidSources {
			want["target"], want["target_cut"] = item.Target, item.TargetCut
		}
		if item.Rule == domain.RuleInvalidSources {
			want["not_link"] = item.NotLink
		}
	}
	return want
}

func TestLintSummaryShrinksTogether(t *testing.T) {
	data := lintDataSet(t, 3, 5)
	// Unequal previews include a short rule and a genuinely empty rule.
	data.rows[1].Items = data.rows[1].Items[:2]
	data.rows[1].Count.Value = 2
	data.rows[3].Items = data.rows[3].Items[:1]
	data.rows[3].Count.Value = 1
	data.rows[5].Items = nil
	data.rows[5].Count.Value = 0
	data.rows[5].StartCursor = nil
	before, err := json.Marshal(data.rows)
	if err != nil {
		t.Fatal(err)
	}
	f := newLintRuntime(t, data)
	answer, size := lintCall(t, lintServer(t, f.lint), false, "lint", nil)
	var got lintReadSummary
	if err := json.Unmarshal(answer.Structured, &got); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "llm_wiki" || got.ComputedAt != "2026-01-02T03:04:05Z" || got.Stale != 7 || len(got.Rules) != 8 {
		t.Fatalf("common-k envelope changed: %s", answer.Structured)
	}
	for i, row := range got.Rules {
		want := 1
		if i == 5 {
			want = 0
		}
		if row.Rule != string(data.rows[i].Rule) || row.Count != data.rows[i].Count.Value || row.Capped || len(row.Items) != want {
			t.Fatalf("common k=1 must serve every rule: %s", answer.Structured)
		}
		if want == 1 {
			lintAssertItem(t, row.Items[0], data.rows[i].Items[0])
		}
		if len(data.rows[i].Items) > want {
			if row.Next == nil || *row.Next != data.rows[i].Items[want-1].Cursor {
				t.Fatal("common-k cursor does not follow the actual item")
			}
		} else if row.Next != nil {
			t.Fatal("complete/empty rule cursor must be null")
		}
	}
	after, err := json.Marshal(data.rows)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Fit changed the runtime's rule facts")
	}
	f.mu.Lock()
	prepares := f.prepares
	f.mu.Unlock()
	if prepares != 1 {
		t.Fatalf("Fit repeated rule preparation %d times", prepares)
	}
	t.Logf("common k=1 SDK=%d", size)
}

func TestLintPageResumesMidway(t *testing.T) {
	for _, name := range []string{"lint", "pending_sources"} {
		t.Run(name, func(t *testing.T) {
			data := lintDataSet(t, 10, 5)
			f := newLintRuntime(t, data)
			server := lintServer(t, f.lint)
			ruleIndex := 2
			if name == "pending_sources" {
				ruleIndex = 6
			}
			var cursor *string
			seen := 0
			for call := 0; call < 4; call++ {
				args := map[string]any{"limit": 5}
				if name == "lint" {
					args["rule"] = "ambiguous"
				}
				if cursor != nil {
					args["cursor"] = *cursor
				}
				answer, size := lintCall(t, server, call%2 == 1, name, args)
				var got lintReadPage
				if err := json.Unmarshal(answer.Structured, &got); err != nil {
					t.Fatal(err)
				}
				want := min(2, 5-seen)
				if len(got.Data) != want || want == 0 {
					t.Fatalf("SDK page must fit two legal largest items: got %d after %d", len(got.Data), seen)
				}
				for i, item := range got.Data {
					lintAssertItem(t, item, data.rows[ruleIndex].Items[seen+i])
				}
				seen += len(got.Data)
				if seen < 5 {
					if got.Next == nil || *got.Next != data.rows[ruleIndex].Items[seen-1].Cursor {
						t.Fatal("mid-page continuation must name the last actual item")
					}
				} else if got.Next != nil {
					t.Fatal("final page continuation must be null")
				}
				t.Logf("%s page items=%d SDK=%d", name, len(got.Data), size)
				cursor = got.Next
				if cursor == nil {
					break
				}
			}
			if seen != 5 {
				t.Fatalf("pagination lost items: %d", seen)
			}
		})
	}
}

func TestLintEmptyShapesStayExplicit(t *testing.T) {
	data := lintDataSet(t, 1, 0)
	f := newLintRuntime(t, data)
	server := lintServer(t, f.lint)
	for _, name := range []string{"lint", "pending_sources"} {
		answer, _ := lintCall(t, server, false, name, nil)
		if name == "lint" {
			var got lintReadSummary
			if err := json.Unmarshal(answer.Structured, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Rules) != 8 {
				t.Fatal("empty summary lost rules")
			}
			for _, row := range got.Rules {
				if row.Count != 0 || row.Capped || row.Items == nil || len(row.Items) != 0 || row.Next != nil {
					t.Fatalf("empty rule is not []/null: %s", answer.Structured)
				}
			}
		} else {
			var got lintReadPage
			if err := json.Unmarshal(answer.Structured, &got); err != nil {
				t.Fatal(err)
			}
			if got.Data == nil || len(got.Data) != 0 || got.Next != nil {
				t.Fatalf("empty page is not []/null: %s", answer.Structured)
			}
		}
		if strings.Contains(string(answer.Structured), "start_cursor") || strings.Contains(string(answer.Structured), "timings") {
			t.Fatal("internal fields escaped")
		}
	}
}
