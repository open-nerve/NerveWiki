package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// problem is a problem answer, as much as the tests read.
type problem struct {
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

func code(body []byte) string {
	var p problem
	_ = json.Unmarshal(body, &p)
	return p.Code
}

// jobAnswer is a TransferJob or TransferJobDetail answer, as much as the
// tests read.
type jobAnswer struct {
	ID        string  `json:"id"`
	RootID    *string `json:"root_id"`
	Name      string  `json:"name"`
	State     string  `json:"state"`
	Client    string  `json:"client"`
	CreatedBy struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	} `json:"created_by"`
	Download *struct {
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"download"`
	Problems []struct {
		Path string  `json:"path"`
		Code string  `json:"code"`
		To   *string `json:"to"`
	} `json:"problems"`
}

func decode(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("the answer %s: %v", body, err)
	}
}

func exportsPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/exports" }

func jobsPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/transfer-jobs" }

// An export starts queued, 202, of the whole notebook or of a page, named
// after it, its client the credential's, and is enqueued.
func TestStartExportQueuesTheJob(t *testing.T) {
	h := newHarness(t)
	for _, tt := range []struct {
		name, token, body, root, client string
	}{
		{"the notebook", "session", `{}`, "", "web"},
		{"a page", "pat", `{"root_id": "` + pageID().String() + `"}`, "Spec", "api"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h.rows.clear()
			res, body := h.send(t, http.MethodPost, exportsPath(), tt.token, tt.body)
			var j jobAnswer
			decode(t, body, &j)
			wantName := "Eng"
			if tt.root != "" {
				wantName = tt.root
			}
			if res.StatusCode != http.StatusAccepted || j.State != "queued" || j.Name != wantName || j.Client != tt.client ||
				(tt.root == "") != (j.RootID == nil) || j.CreatedBy.DisplayName != "Alice" || j.Download != nil {
				t.Errorf("startExport = %d %s, want 202, queued, %s, by %s", res.StatusCode, body, wantName, tt.client)
			}
			if ids := h.queue.enqueued(); len(ids) == 0 || ids[len(ids)-1].String() != j.ID {
				t.Errorf("enqueued %v, want %s", ids, j.ID)
			}
		})
	}
}

// Each refusal of a start answers its code: a notebook the caller does not
// see, a root that is no page of it, an export of the caller's under way,
// the queue full (after 5 minutes), the store full.
func TestStartExportRefusals(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prepare func(h *harness)
		path    string
		token   string
		body    string
		status  int
		code    string
	}{
		{"a notebook not seen", nil, "/api/v0/notebooks/" + pageID().String() + "/exports", "session", `{}`,
			http.StatusNotFound, "notebook.not_found"},
		{"a root that is no page", nil, exportsPath(), "session", `{"root_id": "` + notebookID().String() + `"}`,
			http.StatusNotFound, "page.not_found"},
		{"an export under way", func(h *harness) { h.job(alice(), domain.StateRunning, now(), "") }, exportsPath(), "session", `{}`,
			http.StatusConflict, "transfer.busy"},
		{"the queue full", func(h *harness) {
			for range maxQueued {
				h.job(bob(), domain.StateQueued, now(), "")
			}
		}, exportsPath(), "session", `{}`, http.StatusServiceUnavailable, "server_busy"},
		{"the store full", func(h *harness) { h.archives.free = minFree - 1 }, exportsPath(), "session", `{}`,
			http.StatusInsufficientStorage, "storage_full"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.prepare != nil {
				tt.prepare(h)
			}
			res, body := h.send(t, http.MethodPost, tt.path, tt.token, tt.body)
			if res.StatusCode != tt.status || code(body) != tt.code {
				t.Errorf("startExport = %d %s, want %d %s", res.StatusCode, body, tt.status, tt.code)
			}
			if tt.code == "server_busy" && res.Header.Get("Retry-After") != strconv.Itoa(int(domain.QueueRetry/time.Second)) {
				t.Errorf("Retry-After = %q, want %v", res.Header.Get("Retry-After"), domain.QueueRetry)
			}
			if ids := h.queue.enqueued(); len(ids) != 0 {
				t.Errorf("enqueued %v, want none", ids)
			}
		})
	}
}

// A job reads with its problems and, an export that succeeded, its
// archive's address, signed as of now; another's job is its notebook's
// admin's alone, transfer.not_found to the rest, as a job that is not.
func TestGetTransferJob(t *testing.T) {
	h := newHarness(t)
	own := h.job(alice(), domain.StateSucceeded, now().Add(-time.Hour), "zip")
	others := h.job(bob(), domain.StateFailed, now().Add(-time.Hour), "")
	res, body := h.send(t, http.MethodGet, "/api/v0/transfer-jobs/"+own.ID.String(), "session", "")
	var j jobAnswer
	decode(t, body, &j)
	signed := h.signer.Sign(now(), own.ID)
	wantURL := "/api/v0/transfer-jobs/" + own.ID.String() + "/download?e=" + strconv.FormatInt(signed.Expires.Unix(), 10) + "&s=" + signed.Signature
	if res.StatusCode != http.StatusOK || j.State != "succeeded" || j.Download == nil || j.Download.URL != wantURL ||
		!j.Download.ExpiresAt.Equal(signed.Expires) || len(j.Problems) != 1 || j.Problems[0].Code != "renamed" || j.Problems[0].To == nil ||
		*j.Problems[0].To != "Eng/A 2.md" {
		t.Errorf("getTransferJob = %d %s, want 200, its address %s, its problem", res.StatusCode, body, wantURL)
	}
	for _, tt := range []struct {
		name, id, token string
		status          int
	}{
		{"another's to the admin", others.ID.String(), "bob", http.StatusOK},
		{"another's to a reader", others.ID.String(), "session", http.StatusNotFound},
		{"a job that is not", pageID().String(), "session", http.StatusNotFound},
	} {
		res, body := h.send(t, http.MethodGet, "/api/v0/transfer-jobs/"+tt.id, tt.token, "")
		if res.StatusCode != tt.status || tt.status == http.StatusNotFound && code(body) != "transfer.not_found" {
			t.Errorf("%s: getTransferJob = %d %s, want %d", tt.name, res.StatusCode, body, tt.status)
		}
		if tt.status == http.StatusOK {
			decode(t, body, &j)
			if j.Download != nil || len(j.Problems) != 0 {
				t.Errorf("%s: a failed job's address %+v, problems %+v; want none", tt.name, j.Download, j.Problems)
			}
		}
	}
}

// The list is the caller's jobs, newest first, a page at a time, or every
// one to the notebook's admin; a cursor it cannot read is bad_request, a
// notebook not seen notebook.not_found, a limit outside 1–100
// validation_failed.
func TestListTransferJobs(t *testing.T) {
	h := newHarness(t)
	a1 := h.job(alice(), domain.StateFailed, now().Add(-3*time.Hour), "")
	b1 := h.job(bob(), domain.StateQueued, now().Add(-2*time.Hour), "")
	a2 := h.job(alice(), domain.StateSucceeded, now().Add(-time.Hour), "zip")
	list := func(token, query string) ([]string, string) {
		t.Helper()
		res, body := h.send(t, http.MethodGet, jobsPath()+query, token, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("listTransferJobs%s = %d %s, want 200", query, res.StatusCode, body)
		}
		var page struct {
			Data []jobAnswer `json:"data"`
			Next *string     `json:"next_cursor"`
		}
		decode(t, body, &page)
		ids := make([]string, len(page.Data))
		for i, j := range page.Data {
			ids[i] = j.ID
		}
		next := ""
		if page.Next != nil {
			next = *page.Next
		}
		return ids, next
	}
	if ids, next := list("session", ""); strings.Join(ids, ",") != a2.ID.String()+","+a1.ID.String() || next != "" {
		t.Errorf("alice's list = %v, next %q; want her two, newest first", ids, next)
	}
	if ids, _ := list("bob", ""); strings.Join(ids, ",") != a2.ID.String()+","+b1.ID.String()+","+a1.ID.String() {
		t.Errorf("the admin's list = %v, want all three", ids)
	}
	first, next := list("bob", "?limit=2")
	rest, last := list("bob", "?limit=2&cursor="+next)
	if strings.Join(first, ",") != a2.ID.String()+","+b1.ID.String() || strings.Join(rest, ",") != a1.ID.String() || last != "" {
		t.Errorf("the admin's pages = %v then %v, last %q; want two, then one", first, rest, last)
	}
	for _, tt := range []struct {
		name, path, token string
		status            int
		code              string
	}{
		{"a cursor it cannot read", jobsPath() + "?cursor=nope", "session", http.StatusBadRequest, "bad_request"},
		{"a notebook not seen", "/api/v0/notebooks/" + pageID().String() + "/transfer-jobs", "session", http.StatusNotFound,
			"notebook.not_found"},
		{"a limit of 0", jobsPath() + "?limit=0", "session", http.StatusUnprocessableEntity, "validation_failed"},
		{"a limit of 101", jobsPath() + "?limit=101", "session", http.StatusUnprocessableEntity, "validation_failed"},
	} {
		if res, body := h.send(t, http.MethodGet, tt.path, tt.token, ""); res.StatusCode != tt.status || code(body) != tt.code {
			t.Errorf("%s: listTransferJobs = %d %s, want %d %s", tt.name, res.StatusCode, body, tt.status, tt.code)
		}
	}
}

// A cancel ends a queued job at once, asks a running one to stop, and is
// transfer.not_cancellable once the job ended; another's job is its
// notebook's admin's alone, transfer.not_found to the rest.
func TestCancelTransferJob(t *testing.T) {
	h := newHarness(t)
	queued := h.job(alice(), domain.StateQueued, now(), "")
	running := h.job(bob(), domain.StateRunning, now(), "")
	ended := h.job(alice(), domain.StateSucceeded, now(), "zip")
	for _, tt := range []struct {
		name, id, token string
		status          int
		code, state     string
	}{
		{"its own, queued", queued.ID.String(), "session", http.StatusOK, "", "cancelled"},
		{"another's, to a reader", running.ID.String(), "session", http.StatusNotFound, "transfer.not_found", ""},
		{"another's, running, to the admin", running.ID.String(), "bob", http.StatusOK, "", "running"},
		{"its own, ended", ended.ID.String(), "session", http.StatusConflict, "transfer.not_cancellable", ""},
		{"a job that is not", pageID().String(), "session", http.StatusNotFound, "transfer.not_found", ""},
	} {
		res, body := h.send(t, http.MethodPost, "/api/v0/transfer-jobs/"+tt.id+"/cancel", tt.token, "")
		var j jobAnswer
		_ = json.Unmarshal(body, &j)
		if res.StatusCode != tt.status || code(body) != tt.code || j.State != tt.state {
			t.Errorf("%s: cancelTransferJob = %d %s, want %d %s %s", tt.name, res.StatusCode, body, tt.status, tt.code, tt.state)
		}
	}
	if h.rows.get(running.ID).CancelRequested == nil {
		t.Error("the running job's cancel was not asked")
	}
}

// A path's id that is no id is 400, invalid_format on job_id, on every
// route that takes one, the download's sandboxed.
func TestRoutesBindTheJobID(t *testing.T) {
	h := newHarness(t)
	for _, tt := range []struct{ method, path, token string }{
		{http.MethodGet, "/api/v0/transfer-jobs/nope", "session"},
		{http.MethodPost, "/api/v0/transfer-jobs/nope/cancel", "session"},
		{http.MethodGet, "/api/v0/transfer-jobs/nope/download?e=1&s=" + strings.Repeat("A", 22), ""},
	} {
		res, body := h.send(t, tt.method, tt.path, tt.token, "")
		var p problem
		_ = json.Unmarshal(body, &p)
		if res.StatusCode != http.StatusBadRequest || len(p.Errors) != 1 || p.Errors[0].Field != "job_id" || p.Errors[0].Code != "invalid_format" {
			t.Errorf("%s %s = %d %s, want 400, invalid_format on job_id", tt.method, tt.path, res.StatusCode, body)
		}
	}
}
