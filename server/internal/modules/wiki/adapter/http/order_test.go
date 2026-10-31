package httpadapter_test

import (
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/linking"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
)

func TestLintJudgesInTurn(t *testing.T) {
	foreign, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(99)}, domain.RuleOrphans, domain.LintPosition{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	local, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(10)}, domain.RuleOrphans, domain.LintPosition{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	hiddenLocal, err := (lintCursors{}).Encode(app.LintScope{NotebookID: lintID(11)}, domain.RuleOrphans, domain.LintPosition{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	hidden := strings.Replace(lintPath, "000a", "000b", 1)
	wrongRule, err := linking.EncodeLintCursor(linking.CursorBinding{NotebookID: lintID(10)}, linking.Wanted, linking.Position{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := linking.EncodeLintCursor(linking.CursorBinding{NotebookID: lintID(10), Explicit: true, Mode: linking.ModeStandard}, linking.Orphans, linking.Position{Begin: true})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		status     int
		code       string
		calls      []string
	}{
		{"unknown-bad-hidden-before-limit", hidden + "/SECRET_RULE?limit=0&cursor=bad", 400, "bad_request", []string{"decode"}},
		{"unknown-valid-hidden-before-limit", hidden + "/SECRET_RULE?limit=0&cursor=" + url.QueryEscape(foreign), 404, "notebook.not_found", []string{"decode", "workspace", "authorize"}},
		{"unknown-valid-visible-before-limit", lintPath + "/SECRET_RULE?limit=0&cursor=" + url.QueryEscape(foreign), 404, "wiki.lint_rule_not_found", []string{"decode", "workspace", "authorize"}},
		{"known-foreign-hidden-before-limit", hidden + "/orphans?limit=0&cursor=" + url.QueryEscape(foreign), 400, "bad_request", []string{"decode", "bind"}},
		{"known-foreign-visible-before-limit", lintPath + "/orphans?limit=0&cursor=" + url.QueryEscape(foreign), 400, "bad_request", []string{"decode", "bind"}},
		{"known-other-rule-before-limit", lintPath + "/orphans?limit=0&cursor=" + url.QueryEscape(wrongRule), 400, "bad_request", []string{"decode", "bind"}},
		{"known-explicit-scope-before-limit", lintPath + "/orphans?limit=0&cursor=" + url.QueryEscape(explicit), 400, "bad_request", []string{"decode", "bind"}},
		{"known-valid-hidden-before-limit", hidden + "/orphans?limit=0&cursor=" + url.QueryEscape(hiddenLocal), 404, "notebook.not_found", []string{"decode", "bind", "workspace", "authorize"}},
		{"known-valid-visible-limit", lintPath + "/orphans?limit=0&cursor=" + url.QueryEscape(local), 422, "validation_failed", []string{"decode", "bind", "workspace", "authorize"}},
		{"exclusive-before-limit", lintPath + "/missing_type?limit=0", 409, "notebook.not_llm_wiki", []string{"workspace", "authorize", "state"}},
		{"after-all-fixes-budget", lintPath + "/orphans?limit=1&cursor=" + url.QueryEscape(local), 503, "server_busy", []string{"decode", "bind", "workspace", "authorize", "snapshot", "state", "nodes", "prepare"}},
		{"summary-hidden-before-budget", hidden, 404, "notebook.not_found", []string{"workspace", "authorize"}},
		{"summary-budget", lintPath, 503, "server_busy", []string{"workspace", "authorize", "snapshot", "state", "nodes", "prepare"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLintFixture(t, func(f *lintFixture) { f.block = true; f.budget = 40 * time.Millisecond })
			response, body := lintCall(t, f.handler, tc.path)
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(body, &problem); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status || problem.Code != tc.code {
				t.Fatalf("ordered HTTP result = %d %s; want %d %s", response.StatusCode, body, tc.status, tc.code)
			}
			if strings.Contains(string(body), "SECRET_RULE") || strings.Contains(string(body), foreign) {
				t.Fatalf("problem leaked request identity: %s", body)
			}
			if got := f.trace(); !reflect.DeepEqual(got, tc.calls) {
				t.Fatalf("ordered HTTP calls = %v; want %v", got, tc.calls)
			}
			if tc.status == 503 && response.Header.Get("Retry-After") != "1" {
				t.Fatalf("busy Retry-After = %q", response.Header.Get("Retry-After"))
			}
		})
	}
}

func TestLintBindingAndAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name, path, token string
		status            int
		code              string
	}{
		{"summary-no-auth", lintPath, "", 401, "unauthorized"},
		{"list-no-auth", lintPath + "/orphans", "", 401, "unauthorized"},
		{"summary-bad-id", "/api/v0/notebooks/not-a-uuid/lint", "session", 400, "bad_request"},
		{"list-bad-id", "/api/v0/notebooks/not-a-uuid/lint/orphans", "session", 400, "bad_request"},
		{"noninteger-limit", lintPath + "/orphans?limit=one", "session", 400, "bad_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLintFixture(t)
			response, body := lintRequest(t, f.handler, tc.path, tc.token)
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(body, &problem); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status || problem.Code != tc.code {
				t.Fatalf("binding/auth = %d %s", response.StatusCode, body)
			}
			if len(f.trace()) != 0 {
				t.Fatalf("binding/auth reached runtime ports: %v", f.trace())
			}
			if tc.status == 401 && response.Header.Get("WWW-Authenticate") == "" {
				t.Fatal("missing authentication challenge")
			}
		})
	}
}
