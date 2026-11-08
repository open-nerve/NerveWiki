package httpadapter_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/wiki/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/wiki/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// upgradeFake answers an upgrade's start and read with view or err, and
// records the locale the start was asked for.
type upgradeFake struct {
	mu     sync.Mutex
	view   app.UpgradeView
	err    error
	locale []string
}

func (f *upgradeFake) StartUpgrade(_ context.Context, id uuid.UUID, locale string) (app.UpgradeView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locale = append(f.locale, locale)
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.view, f.err
}

func (f *upgradeFake) GetUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.view, f.err
}

func upgradeServer(t *testing.T, f *upgradeFake) lintHTTP {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: lintAuth{}})
	httpadapter.Register(router, api, nil, f)
	return lintHTTP{Handler: router, contract: apitest.Load(t)}
}

func upgradeCall(t *testing.T, h lintHTTP, method string, nb uuid.UUID, body string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/api/v0/notebooks/"+nb.String()+"/upgrade", reader)
	req.Header.Set("Authorization", "Bearer session")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	response := rec.Result()
	h.contract.CheckResponse(t, req, response)
	out, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, out
}

func upgradeView() app.UpgradeView {
	agents, by, job, done := lintID(31), lintID(5), lintID(40), false
	at := lintTime().Add(time.Hour)
	return app.UpgradeView{Upgrade: domain.Upgrade{
		ID: lintID(30), NotebookID: lintID(10), State: domain.UpgradeRollingBack, Ending: domain.UpgradeRejected,
		SourceRootID: lintID(20), SeededRoot: true, SeededAgentsID: &agents, Locale: domain.LocaleZhCN,
		BaselineNodes: 7, BaselinePages: 5, BaselineWanted: 10000, BaselineWantedCapped: true,
		SubmittedBy: &by, SubmittedAt: &at, RollbackJobID: &job, RollbackComplete: &done,
		RollbackLeft: &domain.RollbackLeft{Nodes: 1, Pages: 2, Created: 3}, CreatedBy: lintID(1), CreatedAt: lintTime(),
	}, StartedByName: "Ada", SubmittedByName: "Bo"}
}

// The upgrade's two operations answer the use cases' views in the
// contract's shape: the pages the start wrote, raw then AGENTS; each
// step's field null until it; the template's language passed as sent.
func TestTheUpgradeIsAnswered(t *testing.T) {
	f := &upgradeFake{view: upgradeView()}
	h := upgradeServer(t, f)
	for _, call := range []struct{ method, body string }{{http.MethodGet, ""}, {http.MethodPost, `{"template_locale":"zh-CN"}`}} {
		response, body := upgradeCall(t, h, call.method, lintID(10), call.body)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", call.method, response.StatusCode, body)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		want := `{"baseline":{"nodes":7,"pages":5,"wanted":10000},"ending":"rejected","id":"00000000-0000-0000-0000-00000000001e",` +
			`"notebook_id":"00000000-0000-0000-0000-00000000000a","rollback_complete":false,"rollback_job_id":"00000000-0000-0000-0000-000000000028",` +
			`"rollback_left":{"created":3,"nodes":1,"pages":2},"seeded_page_ids":["00000000-0000-0000-0000-000000000014","00000000-0000-0000-0000-00000000001f"],` +
			`"source_root_id":"00000000-0000-0000-0000-000000000014","started_at":"2026-01-02T03:04:05Z",` +
			`"started_by":{"display_name":"Ada","user_id":"00000000-0000-0000-0000-000000000001"},"state":"rolling_back",` +
			`"submitted_at":"2026-01-02T04:04:05Z","submitted_by":{"display_name":"Bo","user_id":"00000000-0000-0000-0000-000000000005"}}`
		if normalized, _ := json.Marshal(got); string(normalized) != want {
			t.Errorf("%s answered %s\nwant %s", call.method, normalized, want)
		}
	}
	f.view = app.UpgradeView{Upgrade: domain.Upgrade{ID: lintID(30), NotebookID: lintID(10), State: domain.UpgradeUpgrading,
		SourceRootID: lintID(20), Locale: domain.LocaleEn, CreatedBy: lintID(1), CreatedAt: lintTime()}, StartedByName: "Ada"}
	response, body := upgradeCall(t, h, http.MethodPost, lintID(10), `{}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("start: %d %s", response.StatusCode, body)
	}
	for _, field := range []string{`"seeded_page_ids":[]`, `"submitted_by":null`, `"submitted_at":null`, `"rollback_job_id":null`,
		`"ending":null`, `"rollback_complete":null`, `"rollback_left":null`} {
		if !strings.Contains(string(body), field) {
			t.Errorf("an upgrade just started lacks %s: %s", field, body)
		}
	}
	if f.locale[0] != "zh-CN" || f.locale[1] != "" {
		t.Errorf("the start was asked for %q", f.locale)
	}
	// The binding does not check the enum: the use case answers 422 for
	// a language there is no template of (TestStartingAnUpgrade).
	upgradeCall(t, h, http.MethodPost, lintID(10), `{"template_locale":"fr"}`)
	if len(f.locale) != 3 || f.locale[2] != "fr" {
		t.Errorf("the start was asked for %q", f.locale)
	}
}

// Each code the upgrade's operations declare is answered (apitest's
// unanswered check), the edit sessions a refused start names in locks.
func TestTheUpgradesCodesAreAnswered(t *testing.T) {
	locked := domain.UpgradePagesLocked([]shared.LockHolder{{PageID: lintID(21), UserID: lintID(5), DisplayName: "Bo"}}, 101)
	for _, tt := range []struct {
		method string
		err    error
		status int
		code   string
	}{
		{http.MethodPost, shared.Invalid(shared.FieldError{Field: "template_locale", Code: shared.FieldInvalidFormat, Message: "must be zh-CN or en"}), 422, "validation_failed"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden"},
		{http.MethodPost, domain.ErrAlreadyLLMWiki, 409, "notebook.already_llm_wiki"},
		{http.MethodPost, domain.ErrUpgrading, 409, "notebook.upgrading"},
		{http.MethodPost, domain.ErrWriting, 409, "transfer.busy"},
		{http.MethodPost, locked, 409, "wiki.upgrade_pages_locked"},
		{http.MethodPost, domain.ErrUpgradeTitleTaken, 409, "wiki.upgrade_title_taken"},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy"},
		{http.MethodPost, domain.ErrUpgradeStaleIndex, 409, "wiki.upgrade_stale_index"},
		{http.MethodGet, domain.ErrNotebookNotFound, 404, "notebook.not_found"},
		{http.MethodGet, shared.Forbidden(), 403, "forbidden"},
		{http.MethodGet, domain.ErrUpgradeNotFound, 404, "wiki.upgrade_not_found"},
	} {
		t.Run(tt.method+"/"+tt.code, func(t *testing.T) {
			h := upgradeServer(t, &upgradeFake{err: tt.err})
			body := ""
			if tt.method == http.MethodPost {
				body = `{}`
			}
			response, got := upgradeCall(t, h, tt.method, lintID(10), body)
			var problem struct {
				Code   string `json:"code"`
				Detail string `json:"detail"`
				Locks  []struct {
					PageID      string `json:"page_id"`
					UserID      string `json:"user_id"`
					DisplayName string `json:"display_name"`
				} `json:"locks"`
			}
			if err := json.Unmarshal(got, &problem); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tt.status || problem.Code != tt.code {
				t.Fatalf("%d %s, want %d %s", response.StatusCode, got, tt.status, tt.code)
			}
			if tt.code == "server_busy" && response.Header.Get("Retry-After") != "1" {
				t.Errorf("server_busy without Retry-After: 1: %v", response.Header)
			}
			if tt.code == "wiki.upgrade_pages_locked" && (len(problem.Locks) != 1 || problem.Locks[0].DisplayName != "Bo" ||
				!strings.HasPrefix(problem.Detail, "101 pages")) {
				t.Errorf("the locks named: %s", got)
			}
		})
	}
}
