package httpadapter_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/transfer/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/transfer/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
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
		{"the store without room for the body", func(h *harness) { h.archives.free = minFree + 10 }, importsPath(), "bob", nil,
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

// The route's limit bounds the whole body, not the file only: one that
// declares more is 413 at once, the connection closed, nothing stored;
// one sent without its length that goes on after the form past the limit
// is 400, its archive deleted. Nothing is enqueued.
func TestStartImportBoundsTheWholeBody(t *testing.T) {
	for _, tt := range []struct {
		name     string
		declared bool
		status   int
		code     string
		deleted  int
	}{
		{"its length declared", true, http.StatusRequestEntityTooLarge, "payload_too_large", 0},
		{"chunked", false, http.StatusBadRequest, "bad_request", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			contentType, body := form(t, archive("v.zip", "PK"))
			body = append(body, bytes.Repeat([]byte("z"), importMaxBytes+64<<10)...)
			var r io.Reader = bytes.NewReader(body)
			if !tt.declared {
				r = io.MultiReader(r)
			}
			res, answer := h.post(t, importsPath(), "bob", contentType, r)
			if res.StatusCode != tt.status || code(answer) != tt.code || tt.declared && !res.Close {
				t.Errorf("startImport = %d %s, closing %v; want %d %s", res.StatusCode, answer, res.Close, tt.status, tt.code)
			}
			if len(h.archives.stored()) != 0 || len(h.archives.deleted) != tt.deleted || len(h.queue.enqueued()) != 0 {
				t.Errorf("stored %v, deleted %v, enqueued %v", h.archives.stored(), h.archives.deleted, h.queue.enqueued())
			}
		})
	}
}

// The notebook an upload holds is released as its request ends: an import
// refused after its file was read leaves the next free to start.
func TestStartImportReleasesItsNotebook(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("v.zip", "PK"), part{name: "parent_id", value: pageID().String()})
	if res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body)); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("startImport with a part after the file = %d %s, want 400", res.StatusCode, answer)
	}
	contentType, body = form(t, archive("v.zip", "PK"))
	if res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body)); res.StatusCode != http.StatusAccepted {
		t.Errorf("the next startImport = %d %s, want 202", res.StatusCode, answer)
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

// The import's body is the largest archive and the Envelope, at the
// lowest rate: one that arrives slower is cut off as a body not received.
func TestStartImportsPolicy(t *testing.T) {
	limits := httpadapter.Limits{ImportMaxBytes: 5 << 20, MinRate: 32 << 10}
	if got, want := httpadapter.ImportPolicy(limits), (httpserver.StreamPolicy{MaxBytes: 5<<20 + 64<<10, MinRate: 32 << 10}); got != want {
		t.Errorf("the import's policy = %+v, want %+v", got, want)
	}
}

// A header line cut by the 4 KiB before the file is 400, the connection
// closed, nothing stored; the line the client sent is neither answered
// nor logged.
func TestStartImportRefusesAPrefaceCutInALine(t *testing.T) {
	h := newHarness(t)
	body := "--b\r\nContent-Disposition: form-data; name=\"parent_id\"\r\nX-" + strings.Repeat("Q", 5<<10) + ": v\r\n\r\n" +
		pageID().String() + "\r\n--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"v.zip\"\r\n\r\nPK\r\n--b--\r\n"
	res, answer := h.post(t, importsPath(), "bob", "multipart/form-data; boundary=b", strings.NewReader(body))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "4 KiB before the file") ||
		strings.Contains(string(answer), "QQQQ") || !res.Close {
		t.Errorf("startImport = %d %s, closing %v; want 400 for the 4 KiB before the file, closing", res.StatusCode, answer, res.Close)
	}
	if len(h.archives.uploads) != 0 || strings.Contains(h.logs.String(), "QQQQ") {
		t.Errorf("uploads %v, logs %q; want none, no line of the client's", h.archives.uploads, h.logs)
	}
}

// upload sends an import of what r brings as bob, in the background, of
// length bytes, unknown when negative: the channel tells when the client
// is done, whatever it got.
func (h *harness) upload(t *testing.T, contentType string, r io.Reader, length int64) chan struct{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.base+importsPath(), r)
	if err != nil {
		t.Fatal(err)
	}
	if length >= 0 {
		req.ContentLength = length
	}
	req.Header.Set("Authorization", "Bearer bob")
	req.Header.Set("Content-Type", contentType)
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		if res, err := h.client.Do(req); err == nil {
			_ = res.Body.Close()
		}
	}()
	return sent
}

// A client gone in the middle of its archive leaves none, its upload
// dropped, nothing enqueued, and is logged by its cause as no error; its
// notebook is free for the next import.
func TestAnImportCutByItsClientKeepsNothing(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("v.zip", strings.Repeat("z", 600)))
	r, w := io.Pipe()
	sent := h.upload(t, contentType, r, -1)
	_, _ = w.Write(body[:len(body)-300])
	<-h.archives.creating
	time.Sleep(50 * time.Millisecond)
	_ = w.CloseWithError(errors.New("the client left"))
	<-sent
	waitFor(t, func() bool { return strings.Contains(h.logs.String(), "import's upload not received") })
	if len(h.archives.stored()) != 0 || len(h.archives.dropped()) != 1 || len(h.queue.enqueued()) != 0 {
		t.Errorf("stored %v, dropped %v, enqueued %v; want the upload dropped, nothing else", h.archives.stored(), h.archives.dropped(),
			h.queue.enqueued())
	}
	if logs := h.logs.String(); strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want no error", logs)
	}
	contentType, body = form(t, archive("v.zip", "PK"))
	if res, answer := h.post(t, importsPath(), "bob", contentType, bytes.NewReader(body)); res.StatusCode != http.StatusAccepted {
		t.Errorf("the next startImport = %d %s, want 202", res.StatusCode, answer)
	}
}

// A shutdown in the middle of an archive cuts the import off: its upload
// dropped, nothing enqueued, the cut logged as no error.
func TestShutdownCutsAnImportOff(t *testing.T) {
	h := newHarness(t)
	stop, done := h.serve(t)
	contentType, body := form(t, archive("v.zip", strings.Repeat("z", 600)))
	r, w := io.Pipe()
	sent := h.upload(t, contentType, r, -1)
	_, _ = w.Write(body[:len(body)-300])
	<-h.archives.creating
	stop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the server did not stop")
	}
	_ = w.Close()
	<-sent
	if len(h.archives.stored()) != 0 || len(h.archives.dropped()) != 1 || len(h.queue.enqueued()) != 0 {
		t.Errorf("stored %v, dropped %v, enqueued %v; want the upload dropped, nothing else", h.archives.stored(), h.archives.dropped(),
			h.queue.enqueued())
	}
	if logs := h.logs.String(); !strings.Contains(logs, "import's upload cut off by the shutdown") || strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want the cut, and no error", logs)
	}
}

// A refusal does not wait for the file the client holds back until it
// hears: it is answered at once, the connection closed, not after the
// body's read deadline (5 s).
func TestStartImportRefusesWithoutTheFile(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("v.zip", "THE ARCHIVE"))
	head := body[:bytes.Index(body, []byte("THE ARCHIVE"))]
	r, w := io.Pipe()
	go func() { _, _ = w.Write(head) }() // the rest never comes
	t.Cleanup(func() { _ = w.Close() })
	start := time.Now()
	res, answer := h.post(t, importsPath(), "session", contentType, r)
	if res.StatusCode != http.StatusForbidden || time.Since(start) > 3*time.Second || !res.Close {
		t.Errorf("startImport = %d %s after %v, closing %v; want 403 at once, closing", res.StatusCode, answer, time.Since(start), res.Close)
	}
	if len(h.archives.uploads) != 0 {
		t.Errorf("uploads %v, want none", h.archives.uploads)
	}
}

// An upload under way counts in the store what it has yet to store, its
// declared length less the bytes it wrote: an export started meanwhile
// finds the room the bytes written left.
func TestAnUploadCountsItsBytesAsItStoresThem(t *testing.T) {
	h := newHarness(t)
	contentType, body := form(t, archive("v.zip", strings.Repeat("z", 3000)))
	r, w := io.Pipe()
	sent := h.upload(t, contentType, r, int64(len(body)))
	_, _ = w.Write(body[:len(body)-300])
	<-h.archives.creating
	waitFor(t, func() bool { return h.archives.uploaded() >= 2000 })
	// Without the bytes written, the room is one byte short.
	h.archives.setFree(minFree + int64(len(body)) - 1)
	if res, answer := h.send(t, http.MethodPost, exportsPath(), "bob", `{}`); res.StatusCode != http.StatusAccepted {
		t.Errorf("an export beside the upload = %d %s, want 202", res.StatusCode, answer)
	}
	_, _ = w.Write(body[len(body)-300:])
	_ = w.Close()
	<-sent
	if len(h.archives.stored()) != 1 {
		t.Errorf("stored %v, want the upload's archive", h.archives.stored())
	}
}
