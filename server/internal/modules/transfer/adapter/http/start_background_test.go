package httpadapter_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

func restoresPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/trash-restores" }

func revertsPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/reverts" }

// backgroundAnswer is a background job's answer, as much as the tests
// read besides jobAnswer.
type backgroundAnswer struct {
	jobAnswer
	Kind              string  `json:"kind"`
	ChangesetID       *string `json:"changeset_id"`
	TargetChangesetID *string `json:"target_changeset_id"`
}

// A restore and a revert start queued, 202: the restore's root the item,
// named after it; the revert's target the changeset, named after its
// kind; each the changeset it will write decided, and enqueued.
func TestTheBackgroundStartsQueueTheJob(t *testing.T) {
	h := newHarness(t)
	item, target := pageID().String(), "0199a2b4-0000-7000-8000-000000000300"
	for _, tt := range []struct {
		name, path, body, kind, jobName string
	}{
		{"a restore", restoresPath(), `{"node_id": "` + item + `"}`, "trash_restore", "Box"},
		{"a revert", revertsPath(), `{"changeset_id": "` + target + `", "message": "Undo"}`, "revert", "import"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h.rows.clear()
			res, body := h.send(t, http.MethodPost, tt.path, "bobpat", tt.body)
			var j backgroundAnswer
			decode(t, body, &j)
			if res.StatusCode != http.StatusAccepted || j.Kind != tt.kind || j.State != "queued" || j.Name != tt.jobName || j.Client != "api" ||
				j.CreatedBy.DisplayName != "Bob" || j.ChangesetID == nil || j.Download != nil {
				t.Errorf("start = %d %s, want 202, a queued %s", res.StatusCode, body, tt.kind)
			}
			if tt.kind == "trash_restore" && (j.RootID == nil || *j.RootID != item || j.TargetChangesetID != nil) ||
				tt.kind == "revert" && (j.RootID != nil || j.TargetChangesetID == nil || *j.TargetChangesetID != target) {
				t.Errorf("start = %s, want what it starts on", body)
			}
			if ids := h.queue.enqueued(); len(ids) == 0 || ids[len(ids)-1].String() != j.ID {
				t.Errorf("enqueued %v, want %s", ids, j.ID)
			}
		})
	}
}

// Each refusal of a background start answers its code: the page module's
// checks', then a write into the notebook under way, then the queue full.
func TestTheBackgroundStartsRefusals(t *testing.T) {
	other := "/api/v0/notebooks/" + pageID().String()
	restore := func(id string) string { return `{"node_id": "` + id + `"}` }
	revert := func(id string) string { return `{"changeset_id": "` + id + `"}` }
	free := "0199a2b4-0000-7000-8000-000000000300"
	writing := func(h *harness) {
		h.rows.add(domain.Job{ID: pageID(), NotebookID: notebookID(), Kind: domain.KindImport, State: domain.StateRunning, CreatedBy: alice(),
			CreatedAt: now()})
	}
	full := func(h *harness) {
		for range maxQueued {
			h.job(alice(), domain.StateQueued, now(), "")
		}
	}
	for _, tt := range []struct {
		name    string
		prepare func(h *harness)
		path    string
		token   string
		body    string
		status  int
		code    string
	}{
		{"a notebook that is not", nil, other + "/trash-restores", "bob", restore(free), http.StatusNotFound, "notebook.not_found"},
		{"a reader", nil, restoresPath(), "session", restore(free), http.StatusForbidden, "forbidden"},
		{"no such node", nil, restoresPath(), "bob", restore(refusing("page.not_found")), http.StatusNotFound, "page.not_found"},
		{"a node alive", nil, restoresPath(), "bob", restore(refusing("page.not_in_trash")), http.StatusConflict, "page.not_in_trash"},
		{"no trash root", nil, restoresPath(), "bob", restore(refusing("page.not_trash_root")), http.StatusConflict, "page.not_trash_root"},
		{"an item expired", nil, restoresPath(), "bob", restore(refusing("page.restore_expired")), http.StatusConflict, "page.restore_expired"},
		{"a restore beside a write", writing, restoresPath(), "bob", restore(free), http.StatusConflict, "transfer.busy"},
		{"a restore with the queue full", full, restoresPath(), "bob", restore(free), http.StatusServiceUnavailable, "server_busy"},
		{"an empty message", nil, revertsPath(), "bob", `{"changeset_id": "` + free + `", "message": ""}`, http.StatusUnprocessableEntity,
			"validation_failed"},
		{"a revert of a notebook that is not", nil, other + "/reverts", "bob", revert(free), http.StatusNotFound, "notebook.not_found"},
		{"a revert by a reader", nil, revertsPath(), "session", revert(free), http.StatusForbidden, "forbidden"},
		{"no such changeset", nil, revertsPath(), "bob", revert(refusing("page.changeset_not_found")), http.StatusNotFound,
			"page.changeset_not_found"},
		{"a changeset trimmed", nil, revertsPath(), "bob", revert(refusing("page.changeset_trimmed")), http.StatusConflict,
			"page.changeset_trimmed"},
		{"a changeset too large", nil, revertsPath(), "bob", revert(refusing("page.revert_too_large")), http.StatusUnprocessableEntity,
			"page.revert_too_large"},
		{"a revert beside a write", writing, revertsPath(), "bob", revert(free), http.StatusConflict, "transfer.busy"},
		{"conflicts", nil, revertsPath(), "bob", revert(conflicting().String()), http.StatusConflict, "page.revert_conflict"},
		{"a protected page", nil, revertsPath(), "bob", revert(refusing("page.protected")), http.StatusConflict, "page.protected"},
		{"a source root", nil, revertsPath(), "bob", revert(refusing("page.source_admin_only")), http.StatusForbidden, "page.source_admin_only"},
		{"a page locked", nil, revertsPath(), "bob", revert(locking().String()), http.StatusConflict, "page.locked"},
		{"a revert with the queue full", full, revertsPath(), "bob", revert(free), http.StatusServiceUnavailable, "server_busy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.prepare != nil {
				tt.prepare(h)
			}
			res, body := h.send(t, http.MethodPost, tt.path, tt.token, tt.body)
			if res.StatusCode != tt.status || code(body) != tt.code {
				t.Errorf("start = %d %s, want %d %s", res.StatusCode, body, tt.status, tt.code)
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

// A background job reads with what it starts on, the changeset it writes,
// its counts, a restore's root's place, and a revert's conflicts and the
// lock that stopped it, its holder by name; the list leaves the conflicts
// out.
func TestABackgroundJobReadsWithItsReport(t *testing.T) {
	h := newHarness(t)
	changeset, target, holder := uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	finished := now().Add(-time.Minute)
	j := domain.Job{ID: uuid.NewV7(), NotebookID: notebookID(), Kind: domain.KindRevert, State: domain.StateFailed, Name: "import",
		CreatedBy: bob(), Client: domain.ClientWeb, CreatedAt: now().Add(-time.Hour), Finished: &finished, ChangesetID: &changeset,
		TargetChangesetID: &target, Report: &domain.Report{Failure: domain.FailureRevertConflict,
			Counts:    domain.Counts{Restored: 3, Deleted: 120, PutBack: 2, Written: 1, Skipped: 1},
			Conflicts: []domain.Conflict{{NodeID: pageID(), Name: "Spec", Reason: "title_taken", HolderID: &holder}}, ConflictCount: 7,
			Lock: &domain.Lock{PageID: pageID(), UserID: alice()}}}
	h.rows.add(j)
	parent := pageID()
	r := domain.Job{ID: uuid.NewV7(), NotebookID: notebookID(), Kind: domain.KindTrashRestore, State: domain.StateSucceeded, Name: "Box",
		CreatedBy: bob(), Client: domain.ClientWeb, CreatedAt: now().Add(-2 * time.Hour), Finished: &finished, ChangesetID: &changeset,
		RootID: &parent, Report: &domain.Report{Counts: domain.Counts{Pages: 2, Attachments: 1, Renamed: 1},
			Placed: &domain.Placement{ParentID: &parent, Reason: "original"}}}
	h.rows.add(r)
	var got struct {
		backgroundAnswer
		Report struct {
			Failure string           `json:"failure"`
			Counts  map[string]int64 `json:"counts"`
			Placed  *struct {
				ParentID *string `json:"parent_id"`
				Reason   string  `json:"reason"`
			} `json:"placed"`
			Lock *struct {
				PageID      string `json:"page_id"`
				DisplayName string `json:"display_name"`
			} `json:"lock"`
		} `json:"report"`
		Conflicts []struct {
			NodeID   string `json:"node_id"`
			Reason   string `json:"reason"`
			HolderID string `json:"holder_id"`
		} `json:"conflicts"`
		ConflictCount int `json:"conflict_count"`
	}
	res, body := h.send(t, http.MethodGet, "/api/v0/transfer-jobs/"+j.ID.String(), "bob", "")
	decode(t, body, &got)
	c := got.Report.Counts
	if res.StatusCode != http.StatusOK || got.Kind != "revert" || *got.ChangesetID != changeset.String() || *got.TargetChangesetID != target.String() ||
		got.Report.Failure != "revert_conflict" || c["restored"] != 3 || c["deleted"] != 120 || c["put_back"] != 2 || c["written"] != 1 ||
		c["skipped"] != 1 || got.Report.Placed != nil || got.Report.Lock == nil || got.Report.Lock.DisplayName != "Alice" ||
		len(got.Conflicts) != 1 || got.Conflicts[0].HolderID != holder.String() || got.ConflictCount != 7 {
		t.Errorf("getTransferJob = %d %s, want the revert's report", res.StatusCode, body)
	}
	res, body = h.send(t, http.MethodGet, "/api/v0/transfer-jobs/"+r.ID.String(), "bob", "")
	got.Conflicts, got.Report.Lock = nil, nil
	decode(t, body, &got)
	if res.StatusCode != http.StatusOK || got.Kind != "trash_restore" || got.Report.Placed == nil || *got.Report.Placed.ParentID != parent.String() ||
		got.Report.Counts["renamed"] != 1 || got.Report.Lock != nil || len(got.Conflicts) != 0 || got.ConflictCount != 0 {
		t.Errorf("getTransferJob = %d %s, want the restore's report", res.StatusCode, body)
	}
	res, body = h.send(t, http.MethodGet, jobsPath(), "bob", "")
	var list struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	decode(t, body, &list)
	var listed map[string]json.RawMessage
	for _, item := range list.Data {
		if string(item["id"]) == `"`+j.ID.String()+`"` {
			listed = item
		}
	}
	if res.StatusCode != http.StatusOK || listed == nil || listed["conflicts"] != nil || listed["conflict_count"] != nil ||
		!strings.Contains(string(listed["report"]), `"put_back":2`) || strings.Contains(string(listed["report"]), `"conflicts"`) ||
		strings.Contains(string(listed["report"]), `"conflict_count"`) {
		t.Errorf("listTransferJobs = %d %s; want the revert with its report, without its conflicts", res.StatusCode, body)
	}
}
