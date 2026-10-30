package mcpadapter

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

func lintRESTServer(t *testing.T, use LintUseCase) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: lintAuth{}})
	httpadapter.Register(router, api, use)
	return router
}

func lintRESTRecord(handler http.Handler, suffix string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v0/notebooks/"+lintID(10).String()+"/lint"+suffix, nil)
	req.Header.Set("Authorization", "Bearer fixture")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func lintAssertRESTItem(t *testing.T, got map[string]any, item domain.LintItem) {
	t.Helper()
	want := lintItemFields(item)
	delete(want, "path")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HTTP identity fields differ: got %v want %v", got, want)
	}
}

func TestLintSummaryAtZeroPreviewsStartsAtTheFirstItem(t *testing.T) {
	for _, account := range []bool{false, true} {
		t.Run(map[bool]string{false: "notebook", true: "account"}[account], func(t *testing.T) {
			data := lintDataSet(t, 10, 1)
			data.rows[5].Count.Value, data.rows[5].Items, data.rows[5].StartCursor = 0, nil, nil
			f := newLintRuntime(t, data)
			server, rest := lintServer(t, f.lint), lintRESTServer(t, f.lint)
			answer, size := lintCall(t, server, account, "lint", nil)
			var summary lintReadSummary
			if err := json.Unmarshal(answer.Structured, &summary); err != nil {
				t.Fatal(err)
			}
			if len(summary.Rules) != 8 {
				t.Fatal("zero-preview summary lost rules")
			}
			for i, row := range summary.Rules {
				if row.Rule != string(data.rows[i].Rule) || row.Count != data.rows[i].Count.Value || row.Capped || row.Items == nil || len(row.Items) != 0 {
					t.Fatalf("zero-preview count/identity/array changed: %s", answer.Structured)
				}
				if row.Count == 0 {
					if row.Next != nil {
						t.Fatal("empty rule must retain a null cursor at Fit(0)")
					}
					continue
				}
				if row.Next == nil {
					t.Fatalf("nonempty %s needs a Begin continuation at Fit(0)", row.Rule)
				}
				// Consume the actual MCP continuation through the real HTTP adapter
				// and both real MCP entries. The zero UUID is a real first orphan.
				rec := lintRESTRecord(rest, "/"+row.Rule+"?cursor="+url.QueryEscape(*row.Next))
				if rec.Code != http.StatusOK {
					t.Fatalf("HTTP Begin consumer: %d %s", rec.Code, rec.Body.String())
				}
				var page lintReadPage
				if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
					t.Fatal(err)
				}
				if len(page.Data) != 1 || page.Next != nil {
					t.Fatal("HTTP Begin skipped the first item or failed to terminate")
				}
				lintAssertRESTItem(t, page.Data[0], data.rows[i].Items[0])
				for _, entry := range []bool{false, true} {
					got, _ := lintCall(t, server, entry, "lint", map[string]any{"rule": row.Rule, "cursor": *row.Next})
					if err := json.Unmarshal(got.Structured, &page); err != nil {
						t.Fatal(err)
					}
					if len(page.Data) != 1 || page.Next != nil {
						t.Fatal("MCP Begin skipped the first item or failed to terminate")
					}
					lintAssertItem(t, page.Data[0], data.rows[i].Items[0])
				}
				decoded, err := (lintCodec{}).Decode(*row.Next)
				if err != nil {
					t.Fatal(err)
				}
				pos, err := decoded.PositionFor(app.LintScope{NotebookID: lintID(10)}, domain.LintRule(row.Rule))
				if err != nil || !reflect.DeepEqual(pos, domain.LintPosition{Begin: true}) {
					t.Fatalf("Fit(0) must encode only Begin for %s: %+v %v", row.Rule, pos, err)
				}
			}
			t.Logf("actual Fit(0) SDK=%d; seven nonempty Begin cursors consumed by HTTP and both MCP entries", size)
		})
	}
}

func lintWait(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func TestLintFollowersOwnTheirProjection(t *testing.T) {
	data := lintDataSet(t, 3, 5)
	f := newLintRuntime(t, data)
	gate := make(chan struct{})
	f.gate = gate
	var captures []domain.LintResult
	var expected []domain.LintResult
	var mu sync.Mutex
	// This observer delegates every answer to actual Execute/Runs. It retains
	// each caller's returned value, so Fit must not write even that private input.
	use := lintUseFunc(func(ctx context.Context, scope app.LintScope, query app.LintQuery) (domain.LintResult, error) {
		result, err := f.lint.Execute(ctx, scope, query)
		if err == nil {
			mu.Lock()
			captures = append(captures, result)
			expected = append(expected, domain.CloneLintResult(result))
			mu.Unlock()
		}
		return result, err
	})
	mcp, rest := lintServer(t, use), lintRESTServer(t, use)
	mcpDone, restDone := make(chan struct{}), make(chan struct{})
	var answer lintSDKResult
	var rec *httptest.ResponseRecorder
	go func() {
		defer close(mcpDone)
		answer, _ = lintCall(t, mcp, false, "lint", nil)
	}()
	lintWait(t, f.entered, "worker snapshot")
	go func() {
		defer close(restDone)
		rec = lintRESTRecord(rest, "")
	}()
	lintWait(t, f.joined, "real Runs follower")
	close(gate)
	lintWait(t, mcpDone, "MCP follower")
	lintWait(t, restDone, "HTTP follower")
	if rec == nil || rec.Code != http.StatusOK {
		t.Fatalf("HTTP follower did not succeed: %v", rec)
	}
	var trimmed, whole lintReadSummary
	if err := json.Unmarshal(answer.Structured, &trimmed); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &whole); err != nil {
		t.Fatal(err)
	}
	if len(trimmed.Rules) != 8 || len(whole.Rules) != 8 {
		t.Fatal("a follower lost a rule")
	}
	for i := range whole.Rules {
		if len(trimmed.Rules[i].Items) != 1 || len(whole.Rules[i].Items) != 5 {
			t.Fatal("each follower must retain its own trimmed or complete projection")
		}
		for j, item := range whole.Rules[i].Items {
			lintAssertRESTItem(t, item, data.rows[i].Items[j])
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(captures) != 2 || !reflect.DeepEqual(captures, expected) {
		t.Fatal("Fit wrote the result returned to a real Execute caller")
	}
	// Mutating either real caller's slice and pointer cannot affect the other.
	captures[0].Rules[1].Items[0].ItemKey = "caller-only"
	*captures[0].Rules[1].Items[0].Path = "caller-only"
	if !reflect.DeepEqual(captures[1], expected[1]) {
		t.Fatal("actual followers share mutable result storage")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prepares != 1 || f.snapshots != 1 || f.authorizes != 4 {
		t.Fatalf("same flight did not prepare once/authorize twice per caller: %d/%d/%d", f.prepares, f.snapshots, f.authorizes)
	}
	var started, joined int
	var flight uint64
	for _, event := range f.events {
		if event.Kind == app.RunCreated {
			started++
			flight = event.FlightID
		}
		if event.Kind == app.RunJoined {
			joined++
			if event.FlightID != flight {
				t.Fatal("follower joined a different flight")
			}
		}
	}
	if started != 1 || joined != 1 {
		t.Fatalf("expected one actual flight and follower, got %d/%d", started, joined)
	}
}
