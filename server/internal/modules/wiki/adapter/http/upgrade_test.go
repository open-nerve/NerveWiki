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

// upgradeFake answers an upgrade's start, read, abandonment, rollback's
// end and submission with view, its approval, rejection and send-back
// each with a view of its own (zero: view), or each with err, and records
// the locale the start was asked for.
type upgradeFake struct {
	mu                         sync.Mutex
	view                       app.UpgradeView
	approved, rejected, backed *app.UpgradeView
	checked                    app.UpgradeChecked
	err                        error
	locale                     []string
	queries                    []app.LintQuery
}

// own is view, or the fake's view for nil.
func (f *upgradeFake) own(view *app.UpgradeView) app.UpgradeView {
	if view == nil {
		return f.view
	}
	return *view
}

func (f *upgradeFake) CheckUpgrade(_ context.Context, id uuid.UUID, query app.LintQuery) (app.UpgradeChecked, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, query)
	if id != lintID(10) {
		return app.UpgradeChecked{}, domain.ErrNotebookNotFound
	}
	return f.checked, f.err
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

func (f *upgradeFake) AbandonUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.view, f.err
}

func (f *upgradeFake) SubmitUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.view, f.err
}

func (f *upgradeFake) EndUpgradeRollback(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.view, f.err
}

func (f *upgradeFake) ApproveUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.own(f.approved), f.err
}

func (f *upgradeFake) RejectUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.own(f.rejected), f.err
}

func (f *upgradeFake) SendBackUpgrade(_ context.Context, id uuid.UUID) (app.UpgradeView, error) {
	if id != lintID(10) {
		return app.UpgradeView{}, domain.ErrNotebookNotFound
	}
	return f.own(f.backed), f.err
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
	return upgradeCallAt(t, h, method, nb, "", body)
}

// upgradeCallAt calls the upgrade's path, then sub ("/abandon",
// "/end-rollback", "/submit", "/approve", "/reject" or "/send-back"), and
// checks the answer against the contract.
func upgradeCallAt(t *testing.T, h lintHTTP, method string, nb uuid.UUID, sub, body string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "/api/v0/notebooks/"+nb.String()+"/upgrade"+sub, reader)
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

// The upgrade's operations answer the use cases' views in the contract's
// shape: the pages the start wrote, raw then AGENTS; each step's field
// null until it; the template's language passed as sent. The abandonment,
// the rollback's end, the submission, the approval, the rejection and the
// send-back take no body. The approval, the rejection and the send-back
// each answer its own use case's view, whole, every value its own
// (M11/P3 design 3.11): approved, its submission kept; rolling back,
// rejected, with its job; upgrading, no submission.
func TestTheUpgradeIsAnswered(t *testing.T) {
	f := &upgradeFake{view: upgradeView()}
	h := upgradeServer(t, f)
	for _, call := range []struct{ method, sub, body string }{{http.MethodGet, "", ""}, {http.MethodPost, "", `{"template_locale":"zh-CN"}`},
		{http.MethodPost, "/abandon", ""}, {http.MethodPost, "/end-rollback", ""}, {http.MethodPost, "/submit", ""}} {
		response, body := upgradeCallAt(t, h, call.method, lintID(10), call.sub, call.body)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: %d %s", call.method, call.sub, response.StatusCode, body)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		want := `{"baseline":{"nodes":7,"pages":5,"wanted":10000},"ending":"rejected","id":"00000000-0000-0000-0000-00000000001e",` +
			`"notebook_id":"00000000-0000-0000-0000-00000000000a","report":null,"rollback_complete":false,"rollback_job_id":"00000000-0000-0000-0000-000000000028",` +
			`"rollback_left":{"created":3,"nodes":1,"pages":2},"seeded_page_ids":["00000000-0000-0000-0000-000000000014","00000000-0000-0000-0000-00000000001f"],` +
			`"source_root_id":"00000000-0000-0000-0000-000000000014","started_at":"2026-01-02T03:04:05Z",` +
			`"started_by":{"display_name":"Ada","user_id":"00000000-0000-0000-0000-000000000001"},"state":"rolling_back",` +
			`"submitted_at":"2026-01-02T04:04:05Z","submitted_by":{"display_name":"Bo","user_id":"00000000-0000-0000-0000-000000000005"}}`
		if normalized, _ := json.Marshal(got); string(normalized) != want {
			t.Errorf("%s %s answered %s\nwant %s", call.method, call.sub, normalized, want)
		}
	}
	agents, by := lintID(51), lintID(52)
	submitted := lintTime().Add(2 * time.Hour)
	f.approved = &app.UpgradeView{Upgrade: domain.Upgrade{
		ID: lintID(53), NotebookID: lintID(10), State: domain.UpgradeApproved, SourceRootID: lintID(54), SeededAgentsID: &agents,
		Locale: domain.LocaleEn, BaselineNodes: 17, BaselinePages: 13, BaselineWanted: 11, SubmittedBy: &by, SubmittedAt: &submitted,
		CreatedBy: lintID(55), CreatedAt: lintTime().Add(time.Minute),
	}, StartedByName: "Cy", SubmittedByName: "Di"}
	job, rejecter := lintID(62), lintID(61)
	rejected := lintTime().Add(4 * time.Hour)
	f.rejected = &app.UpgradeView{Upgrade: domain.Upgrade{
		ID: lintID(59), NotebookID: lintID(10), State: domain.UpgradeRollingBack, Ending: domain.UpgradeRejected, SourceRootID: lintID(60),
		Locale: domain.LocaleEn, BaselineNodes: 23, BaselinePages: 21, BaselineWanted: 2, SubmittedBy: &rejecter, SubmittedAt: &rejected,
		RollbackJobID: &job, CreatedBy: lintID(63), CreatedAt: lintTime().Add(5 * time.Minute),
	}, StartedByName: "Fa", SubmittedByName: "Gi"}
	f.backed = &app.UpgradeView{Upgrade: domain.Upgrade{
		ID: lintID(56), NotebookID: lintID(10), State: domain.UpgradeUpgrading, SourceRootID: lintID(57), SeededRoot: true,
		Locale: domain.LocaleZhCN, BaselineNodes: 19, BaselinePages: 18, BaselineWanted: 3, CreatedBy: lintID(58),
		CreatedAt: lintTime().Add(3 * time.Minute),
	}, StartedByName: "Ed"}
	for _, call := range []struct{ sub, want string }{
		{"/approve", `{"baseline":{"nodes":17,"pages":13,"wanted":11},"ending":null,"id":"00000000-0000-0000-0000-000000000035",` +
			`"notebook_id":"00000000-0000-0000-0000-00000000000a","report":null,"rollback_complete":null,"rollback_job_id":null,` +
			`"rollback_left":null,"seeded_page_ids":["00000000-0000-0000-0000-000000000033"],` +
			`"source_root_id":"00000000-0000-0000-0000-000000000036","started_at":"2026-01-02T03:05:05Z",` +
			`"started_by":{"display_name":"Cy","user_id":"00000000-0000-0000-0000-000000000037"},"state":"approved",` +
			`"submitted_at":"2026-01-02T05:04:05Z","submitted_by":{"display_name":"Di","user_id":"00000000-0000-0000-0000-000000000034"}}`},
		{"/reject", `{"baseline":{"nodes":23,"pages":21,"wanted":2},"ending":"rejected","id":"00000000-0000-0000-0000-00000000003b",` +
			`"notebook_id":"00000000-0000-0000-0000-00000000000a","report":null,"rollback_complete":null,` +
			`"rollback_job_id":"00000000-0000-0000-0000-00000000003e","rollback_left":null,"seeded_page_ids":[],` +
			`"source_root_id":"00000000-0000-0000-0000-00000000003c","started_at":"2026-01-02T03:09:05Z",` +
			`"started_by":{"display_name":"Fa","user_id":"00000000-0000-0000-0000-00000000003f"},"state":"rolling_back",` +
			`"submitted_at":"2026-01-02T07:04:05Z","submitted_by":{"display_name":"Gi","user_id":"00000000-0000-0000-0000-00000000003d"}}`},
		{"/send-back", `{"baseline":{"nodes":19,"pages":18,"wanted":3},"ending":null,"id":"00000000-0000-0000-0000-000000000038",` +
			`"notebook_id":"00000000-0000-0000-0000-00000000000a","report":null,"rollback_complete":null,"rollback_job_id":null,` +
			`"rollback_left":null,"seeded_page_ids":["00000000-0000-0000-0000-000000000039"],` +
			`"source_root_id":"00000000-0000-0000-0000-000000000039","started_at":"2026-01-02T03:07:05Z",` +
			`"started_by":{"display_name":"Ed","user_id":"00000000-0000-0000-0000-00000000003a"},"state":"upgrading",` +
			`"submitted_at":null,"submitted_by":null}`},
	} {
		response, body := upgradeCallAt(t, h, http.MethodPost, lintID(10), call.sub, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("POST %s: %d %s", call.sub, response.StatusCode, body)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if normalized, _ := json.Marshal(got); string(normalized) != call.want {
			t.Errorf("POST %s answered %s\nwant %s", call.sub, normalized, call.want)
		}
	}
	f.view = app.UpgradeView{Upgrade: domain.Upgrade{ID: lintID(30), NotebookID: lintID(10), State: domain.UpgradeUpgrading,
		SourceRootID: lintID(20), Locale: domain.LocaleEn, CreatedBy: lintID(1), CreatedAt: lintTime()}, StartedByName: "Ada"}
	response, body := upgradeCall(t, h, http.MethodPost, lintID(10), `{}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("start: %d %s", response.StatusCode, body)
	}
	for _, field := range []string{`"seeded_page_ids":[]`, `"submitted_by":null`, `"submitted_at":null`, `"rollback_job_id":null`,
		`"ending":null`, `"rollback_complete":null`, `"rollback_left":null`, `"report":null`} {
		if !strings.Contains(string(body), field) {
			t.Errorf("an upgrade just started lacks %s: %s", field, body)
		}
	}
	if f.locale[0] != "zh-CN" || f.locale[1] != "" {
		t.Errorf("the start was asked for %q", f.locale)
	}
	// The binding does not check the enum: the use case answers 422 for
	// a language there is no template of (wiki/app's TestUpgradeStart).
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
		sub    string
	}{
		{http.MethodPost, shared.Invalid(shared.FieldError{Field: "template_locale", Code: shared.FieldInvalidFormat, Message: "must be zh-CN or en"}), 422, "validation_failed", ""},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", ""},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", ""},
		{http.MethodPost, domain.ErrAlreadyLLMWiki, 409, "notebook.already_llm_wiki", ""},
		{http.MethodPost, domain.ErrUpgrading, 409, "notebook.upgrading", ""},
		{http.MethodPost, domain.ErrWriting, 409, "transfer.busy", ""},
		{http.MethodPost, locked, 409, "wiki.upgrade_pages_locked", ""},
		{http.MethodPost, domain.ErrUpgradeTitleTaken, 409, "wiki.upgrade_title_taken", ""},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy", ""},
		{http.MethodPost, domain.ErrUpgradeStaleIndex, 409, "wiki.upgrade_stale_index", ""},
		{http.MethodGet, domain.ErrNotebookNotFound, 404, "notebook.not_found", ""},
		{http.MethodGet, shared.Forbidden(), 403, "forbidden", ""},
		{http.MethodGet, domain.ErrUpgradeNotFound, 404, "wiki.upgrade_not_found", ""},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/abandon"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/abandon"},
		{http.MethodPost, domain.ErrUpgradeState, 409, "wiki.upgrade_state", "/abandon"},
		{http.MethodPost, domain.ErrRollbackWriting, 409, "transfer.busy", "/abandon"},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy", "/abandon"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/end-rollback"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/end-rollback"},
		{http.MethodPost, domain.ErrUpgradeState, 409, "wiki.upgrade_state", "/end-rollback"},
		{http.MethodPost, domain.ErrRollbackWriting, 409, "transfer.busy", "/end-rollback"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/submit"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/submit"},
		{http.MethodPost, domain.ErrUpgradeSubmitting, 409, "wiki.upgrade_state", "/submit"},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy", "/submit"},
		{http.MethodPost, domain.ErrUpgradeStaleIndex, 409, "wiki.upgrade_stale_index", "/submit"},
		{http.MethodPost, domain.UpgradeNotReady([]domain.UpgradeGateKey{domain.GateLinks, domain.GatePreserved}), 409,
			"wiki.upgrade_not_ready", "/submit"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/approve"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/approve"},
		{http.MethodPost, domain.ErrUpgradeState, 409, "wiki.upgrade_state", "/approve"},
		{http.MethodPost, domain.ApprovalNotReady([]domain.UpgradeGateKey{domain.GateAgents, domain.GateReservedTitles}), 409,
			"wiki.upgrade_not_ready", "/approve"},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy", "/approve"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/reject"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/reject"},
		{http.MethodPost, domain.ErrUpgradeState, 409, "wiki.upgrade_state", "/reject"},
		{http.MethodPost, shared.ServerBusy(time.Second), 503, "server_busy", "/reject"},
		{http.MethodPost, domain.ErrNotebookNotFound, 404, "notebook.not_found", "/send-back"},
		{http.MethodPost, shared.Forbidden(), 403, "forbidden", "/send-back"},
		{http.MethodPost, domain.ErrUpgradeSubmitting, 409, "wiki.upgrade_state", "/send-back"},
	} {
		t.Run(tt.method+tt.sub+"/"+tt.code, func(t *testing.T) {
			h := upgradeServer(t, &upgradeFake{err: tt.err})
			body := ""
			if tt.method == http.MethodPost && tt.sub == "" {
				body = `{}`
			}
			response, got := upgradeCallAt(t, h, tt.method, lintID(10), tt.sub, body)
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
			if tt.code == "wiki.upgrade_not_ready" && tt.sub == "/submit" && problem.Detail != "These gates did not pass: links, preserved. "+
				"The upgrade is upgrading again: check it, then submit it again." {
				t.Errorf("the gates named: %s", got)
			}
			if tt.code == "wiki.upgrade_not_ready" && tt.sub == "/approve" && problem.Detail != "These gates did not pass: agents, "+
				"reserved_titles. The upgrade is still in review: send it back, or reject it." {
				t.Errorf("the gates named: %s", got)
			}
			if tt.sub == "/send-back" && tt.code == "wiki.upgrade_state" &&
				problem.Detail != "The previous submission is still being checked: check again in a moment." {
				t.Errorf("a send-back while the submission is checked: %s", got)
			}
			if tt.code == "wiki.upgrade_pages_locked" && (len(problem.Locks) != 1 || problem.Locks[0].DisplayName != "Bo" ||
				!strings.HasPrefix(problem.Detail, "101 pages")) {
				t.Errorf("the locks named: %s", got)
			}
		})
	}
}
