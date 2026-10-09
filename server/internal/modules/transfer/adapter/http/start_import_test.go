package httpadapter_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
)

// part is a part of an import's form: a field, or a file when filename is
// set.
type part struct {
	name, filename, value string
}

// form writes parts as multipart/form-data, in their order.
func form(t *testing.T, parts ...part) (contentType string, body []byte) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		disposition := `form-data; name="` + p.name + `"`
		if p.filename != "" {
			disposition += `; filename="` + p.filename + `"`
		}
		h.Set("Content-Disposition", disposition)
		pw, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write([]byte(p.value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), b.Bytes()
}

func archive(name, content string) part { return part{name: "file", filename: name, value: content} }

func importsPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/imports" }

// post sends a form of body as token, checks the answer against the
// contract, and answers it with its body.
func (h *harness) post(t *testing.T, path, token, contentType string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.base+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", contentType)
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	answer, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body = io.NopCloser(bytes.NewReader(answer))
	h.contract.CheckResponse(t, req, res)
	return res, answer
}

// An import starts queued, 202, under the page given or at the root, named
// after its file mended, its client the credential's; its archive is
// stored at its id, and it is enqueued. Its log tells no file name.
func TestStartImportQueuesTheJob(t *testing.T) {
	for _, tt := range []struct {
		name, token, client string
		parts               []part
		root                bool
		file                string
	}{
		{"at the root", "bob", "web", []part{archive("vault.zip", "PK zip")}, false, "vault.zip"},
		{"under a page", "bobpat", "api", []part{{name: "parent_id", value: pageID().String()}, archive("a:b.zip", "PK zip")}, true, "a_b.zip"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			contentType, body := form(t, tt.parts...)
			res, answer := h.post(t, importsPath(), tt.token, contentType, bytes.NewReader(body))
			var j jobAnswer
			decode(t, answer, &j)
			if res.StatusCode != http.StatusAccepted || j.State != "queued" || j.Name != tt.file || j.Client != tt.client ||
				(j.RootID != nil) != tt.root || j.CreatedBy.DisplayName != "Bob" {
				t.Fatalf("startImport = %d %s", res.StatusCode, answer)
			}
			if ids := h.queue.enqueued(); len(ids) != 1 || ids[0].String() != j.ID {
				t.Errorf("enqueued %v, want %s", ids, j.ID)
			}
			if stored := h.archives.stored(); len(stored) != 1 || string(stored[uuid.MustParse(j.ID)]) != "PK zip" {
				t.Errorf("stored %v, want the archive at the job's id", stored)
			}
			if logs := h.logs.String(); !strings.Contains(logs, "import queued") || strings.Contains(logs, "vault") || strings.Contains(logs, "a:b") {
				t.Errorf("logs %q", logs)
			}
		})
	}
}

// Each refusal is answered before the file is read, the connection closed
// after: nothing is stored or enqueued.
func TestStartImportRefusesBeforeTheFile(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prepare func(h *harness)
		path    string
		token   string
		parts   []part
		status  int
		code    string
	}{
		{"a notebook that is not", nil, "/api/v0/notebooks/" + pageID().String() + "/imports", "bob", nil,
			http.StatusNotFound, "notebook.not_found"},
		{"a reader", nil, importsPath(), "session", nil, http.StatusForbidden, "forbidden"},
		{"a parent that is no page", nil, importsPath(), "bob", []part{{name: "parent_id", value: notebookID().String()}},
			http.StatusNotFound, "page.not_found"},
		{"an import under way", func(h *harness) {
			h.rows.add(domain.Job{ID: uuid.NewV7(), NotebookID: notebookID(), Kind: domain.KindImport, State: domain.StateRunning, CreatedBy: alice(),
				Client: domain.ClientWeb, CreatedAt: now()})
		}, importsPath(), "bob", nil, http.StatusConflict, "transfer.busy"},
		{"the queue full", func(h *harness) {
			for range maxQueued {
				h.job(alice(), domain.StateQueued, now(), "")
			}
		}, importsPath(), "bob", nil, http.StatusServiceUnavailable, "server_busy"},
		{"the store full", func(h *harness) { h.archives.free = minFree - 1 }, importsPath(), "bob", nil,
			http.StatusInsufficientStorage, "storage_full"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.prepare != nil {
				tt.prepare(h)
			}
			queued := len(h.queue.enqueued())
			contentType, body := form(t, append(tt.parts, archive("vault.zip", "PK zip"))...)
			res, answer := h.post(t, tt.path, tt.token, contentType, bytes.NewReader(body))
			if res.StatusCode != tt.status || code(answer) != tt.code || !res.Close {
				t.Errorf("startImport = %d %s, closing %v; want %d %s, closing", res.StatusCode, answer, res.Close, tt.status, tt.code)
			}
			if len(h.archives.uploads) != 0 || len(h.queue.enqueued()) != queued {
				t.Errorf("uploads %v, enqueued %v; want none", h.archives.uploads, h.queue.enqueued())
			}
		})
	}
}

// An archive larger than transfer.import_max_bytes is 413, nothing kept;
// one of the largest is taken.
func TestStartImportTakesItsLargestArchive(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("big.zip", strings.Repeat("z", importMaxBytes+1)))
	res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body))
	if res.StatusCode != http.StatusRequestEntityTooLarge || code(answer) != "payload_too_large" || len(h.archives.stored()) != 0 {
		t.Errorf("startImport = %d %s, stored %d; want 413, nothing", res.StatusCode, answer, len(h.archives.stored()))
	}
	contentType, body = form(t, archive("big.zip", strings.Repeat("z", importMaxBytes)))
	if res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body)); res.StatusCode != http.StatusAccepted {
		t.Errorf("startImport of the largest = %d %s, want 202", res.StatusCode, answer)
	}
}

// A form that is not the route's is 400, naming what is wrong; a part
// after the file deletes the archive stored.
func TestStartImportReadsItsForm(t *testing.T) {
	for _, tt := range []struct {
		name    string
		parts   []part
		field   string
		deleted bool
	}{
		{"an unknown part", []part{{name: "name", value: "x"}, archive("v.zip", "PK")}, "", false},
		{"a parent that is no id", []part{{name: "parent_id", value: "nope"}, archive("v.zip", "PK")}, "parent_id", false},
		{"no file", []part{{name: "parent_id", value: pageID().String()}}, "", false},
		{"a part after the file", []part{archive("v.zip", "PK"), {name: "parent_id", value: pageID().String()}}, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			contentType, body := form(t, tt.parts...)
			res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body))
			var p problem
			decode(t, answer, &p)
			if res.StatusCode != http.StatusBadRequest || p.Code != "bad_request" || tt.field != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tt.field) {
				t.Errorf("startImport = %d %s, want 400 on %q", res.StatusCode, answer, tt.field)
			}
			if len(h.archives.stored()) != 0 || len(h.queue.enqueued()) != 0 || (len(h.archives.deleted) == 1) != tt.deleted {
				t.Errorf("stored %v, deleted %v, enqueued %v", h.archives.stored(), h.archives.deleted, h.queue.enqueued())
			}
		})
	}
}

// The route binds the notebook's id first: one that is no id is 400
// before the credential is asked.
func TestStartImportBindsTheNotebookIDFirst(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("v.zip", "PK"))
	res, answer := h.post(t, "/api/v0/notebooks/nope/imports", "", contentType, bytes.NewReader(body))
	if res.StatusCode != http.StatusBadRequest || code(answer) != "bad_request" || !res.Close {
		t.Errorf("startImport = %d %s, want 400, closed", res.StatusCode, answer)
	}
}
