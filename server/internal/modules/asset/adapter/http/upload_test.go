package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/asset/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// part is a part of an upload's form: a field, or a file when filename
// is set; header adds a header line.
type part struct {
	name, filename, value string
	header                textproto.MIMEHeader
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
		for k, v := range p.header {
			h[k] = v
		}
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

func file(name, content string) part { return part{name: "file", filename: name, value: content} }

// post sends an upload of body as token, none when it is empty, checks the
// answer against the contract, and answers it with its body.
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

func uploadPath() string { return "/api/v0/notebooks/" + notebookID().String() + "/assets" }

// problem is an answer's problem: its code and fields.
type problem struct {
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

// An upload of a file answers 201 with the attachment: its node, its file
// with the type the server told, and its addresses signed. The tree gets
// the node to create, decided on asset.upload from the credential's
// client; the row and the file are written.
func TestUploadCreatesTheAttachment(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, part{name: "parent_id", value: parentID().String()}, part{name: "name", value: "cover.png"},
		file("ignored.txt", "\x89PNG\r\n\x1a\nrest"))
	res, answer := h.post(t, uploadPath(), "pat", ct, bytes.NewReader(body))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d %s, want 201", res.StatusCode, answer)
	}
	var a struct {
		ID, NotebookID, Name, Mime, Sha256, ContentURL, DownloadURL string
		ParentID                                                    *string
		ByteSize                                                    int64
		Width                                                       *int
		ExpiresAt                                                   time.Time
	}
	if err := json.Unmarshal(answer, &struct {
		ID          *string    `json:"id"`
		NotebookID  *string    `json:"notebook_id"`
		Name        *string    `json:"name"`
		Mime        *string    `json:"mime"`
		Sha256      *string    `json:"sha256"`
		ContentURL  *string    `json:"content_url"`
		DownloadURL *string    `json:"download_url"`
		ParentID    **string   `json:"parent_id"`
		ByteSize    *int64     `json:"byte_size"`
		Width       **int      `json:"width"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}{&a.ID, &a.NotebookID, &a.Name, &a.Mime, &a.Sha256, &a.ContentURL, &a.DownloadURL, &a.ParentID, &a.ByteSize, &a.Width, &a.ExpiresAt}); err != nil {
		t.Fatal(err)
	}
	address := regexp.MustCompile(`^/api/v0/assets/` + a.ID + `/content\?b=[0-9a-f-]{36}&e=1791460800&s=[A-Za-z0-9_-]{22}$`)
	if a.Name != "cover.png" || a.Mime != "image/png" || a.ByteSize != 12 || len(a.Sha256) != 64 || a.ParentID == nil ||
		*a.ParentID != parentID().String() || a.Width != nil || !address.MatchString(a.ContentURL) ||
		!regexp.MustCompile(`&d=1$`).MatchString(a.DownloadURL) || !a.ExpiresAt.Equal(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("answer = %s", answer)
	}
	if len(h.tree.created) != 1 || h.tree.created[0].Client != "api" || h.tree.created[0].Action != "asset.upload" ||
		h.tree.created[0].Meta.MIME != "image/png" || *h.tree.created[0].ParentID != parentID() || h.tree.created[0].NotebookID != notebookID() {
		t.Errorf("created %+v, want cover.png under the parent from the API", h.tree.created)
	}
	if keys, _, open := h.files.state(); len(keys) != 1 || open != 0 || len(h.rows.rows) != 1 {
		t.Errorf("files %q, %d open, %d rows; want the file and its row", keys, open, len(h.rows.rows))
	}
}

// Without a name part, or with one empty or of blanks, the file part's
// file name is the name, its directory dropped; a session's upload is the
// web's.
func TestUploadNamesTheFileByItsFileName(t *testing.T) {
	for _, name := range []*part{nil, {name: "name"}, {name: "name", value: " \t "}} {
		h := newHarness(t)
		parts := []part{file(`photos/beach.jpg`, "x")}
		if name != nil {
			parts = append([]part{*name}, parts...)
		}
		ct, body := form(t, parts...)
		if res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body)); res.StatusCode != http.StatusCreated {
			t.Fatalf("upload with the name part %+v = %d %s, want 201", name, res.StatusCode, answer)
		}
		if n := h.tree.created[0]; n.Name != "beach.jpg" || n.ParentID != nil || n.Client != "web" {
			t.Errorf("with the name part %+v, created %+v; want beach.jpg at the root from the web", name, n)
		}
	}
}

// A part is stored as it was sent: a Content-Transfer-Encoding is not
// decoded.
func TestUploadStoresTheFileAsSent(t *testing.T) {
	h := newHarness(t)
	qp := part{name: "file", filename: "a.txt", value: "a=3Db",
		header: textproto.MIMEHeader{"Content-Transfer-Encoding": {"quoted-printable"}}}
	ct, body := form(t, qp)
	if res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body)); res.StatusCode != http.StatusCreated {
		t.Fatalf("upload = %d %s, want 201", res.StatusCode, answer)
	}
	keys, _, _ := h.files.state()
	if len(keys) != 1 {
		t.Fatalf("files %q, want one", keys)
	}
	f, err := h.files.Open(t.Context(), keys[0])
	if err != nil {
		t.Fatal(err)
	}
	if got, err := io.ReadAll(f); err != nil || string(got) != "a=3Db" {
		t.Errorf("the file holds %q, %v; want a=3Db", got, err)
	}
}

// A header line cut by the 4 KiB before the file is 400 as any preface
// past them, the parser's words unread: the line the client sent is not
// logged.
func TestUploadRefusesAPrefaceCutInALine(t *testing.T) {
	h := newHarness(t)
	key := "X-" + strings.Repeat("Q", 5<<10)
	ct, body := form(t, part{name: "name", value: "a.png", header: textproto.MIMEHeader{key: {"v"}}}, file("a.png", "x"))
	res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "4 KiB before the file") ||
		strings.Contains(string(answer), "QQQQ") {
		t.Errorf("upload = %d %s, want 400 for the 4 KiB before the file", res.StatusCode, answer)
	}
	if logs := h.logs.String(); strings.Contains(logs, "QQQQ") {
		t.Errorf("logs %q, want no line of the client's", logs)
	}
}

// A header line within the 4 KiB before the file that is not a header is
// a body not received, 400, logged by its cause: the line the client sent
// is not logged.
func TestUploadLogsAMalformedHeaderByItsCause(t *testing.T) {
	h := newHarness(t)
	body := "--b\r\nContent-Disposition: form-data; name=\"name\"\r\nQQQQ-no-colon\r\n\r\na.png\r\n" +
		"--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n\r\nx\r\n--b--\r\n"
	res, answer := h.post(t, uploadPath(), "session", "multipart/form-data; boundary=b", strings.NewReader(body))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "not read whole") {
		t.Errorf("upload = %d %s, want 400 for a body not received", res.StatusCode, answer)
	}
	waitFor(t, func() bool { return strings.Contains(h.logs.String(), "upload not received") })
	if logs := h.logs.String(); !strings.Contains(logs, "cause=malformed") || strings.Contains(logs, "QQQQ") {
		t.Errorf("logs %q, want the cause, malformed, and no line of the client's", logs)
	}
}

// How a body failed to arrive is named by its error: too slow, ended
// early, the connection failing, or malformed.
func TestReadCause(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want string
	}{
		{&net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}, "too slow"},
		{io.ErrUnexpectedEOF, "ended early"},
		{io.EOF, "ended early"},
		{&net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, "connection failed"},
		{errors.New("malformed MIME header line: x"), "malformed"},
	} {
		if got := httpadapter.ReadCause(tt.err); got != tt.want {
			t.Errorf("ReadCause(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

// A part after the file is 400 as soon as its header is read: its bytes,
// which the client may never end, are not waited for.
func TestUploadRefusesAPartAfterTheFileAtOnce(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", "x"), part{name: "name", value: "NEVER ENDS"})
	head := body[:bytes.Index(body, []byte("NEVER ENDS"))]
	r, w := io.Pipe()
	go func() { _, _ = w.Write(head) }() // the rest never comes
	t.Cleanup(func() { _ = w.Close() })
	start := time.Now()
	res, answer := h.post(t, uploadPath(), "session", ct, r)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "a part after the file") ||
		time.Since(start) > 3*time.Second {
		t.Errorf("upload = %d %s after %v, want 400 at once", res.StatusCode, answer, time.Since(start))
	}
}

// A step past the request's deadline, the checks or the unit, is answered
// 500 and logged as a request that ran out of time, a warning; the unit's
// leaves the file for the sweep, its outcome unknown.
func TestUploadPastItsDeadlineIsLoggedAsAWarning(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stall func(*tree)
		kept  bool
	}{
		{"the checks", func(tr *tree) { tr.stallCheck = true }, false},
		{"the unit", func(tr *tree) { tr.stallCreate = true }, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarnessWith(t, httpservertest.APIOptions{RequestTimeout: 100 * time.Millisecond})
			tt.stall(h.tree)
			ct, body := form(t, file("a.png", "x"))
			res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
			if res.StatusCode != http.StatusInternalServerError || !strings.Contains(string(answer), "internal_error") {
				t.Errorf("upload = %d %s, want 500 internal_error", res.StatusCode, answer)
			}
			if logs := h.logs.String(); !strings.Contains(logs, `level=WARN msg="API request deadline exceeded"`) ||
				strings.Contains(logs, "level=ERROR") {
				t.Errorf("logs %q, want the deadline as a warning, no error", logs)
			}
			if keys, _, _ := h.files.state(); (len(keys) == 1) != tt.kept {
				t.Errorf("files %q, want kept %v", keys, tt.kept)
			}
		})
	}
}

// A body that goes on after the form's end, past the route's limit, is
// 400: the file stored is deleted.
func TestUploadRefusesABodyAfterTheForm(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", "x"))
	body = append(body, strings.Repeat("z", maxBytes+httpadapter.Envelope)...)
	res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "after the form's end") {
		t.Errorf("upload = %d %s, want 400 for the body after the form", res.StatusCode, answer)
	}
	if keys, deleted, open := h.files.state(); len(keys) != 0 || len(deleted) != 1 || open != 0 || len(h.tree.created) != 0 {
		t.Errorf("files %q, deleted %q, %d open, created %+v; want the file deleted, nothing created", keys, deleted, open, h.tree.created)
	}
}

// Each malformed request, each refusal and each failure answers its code
// and leaves no file but a refusal's after the unit: a part after the
// file, a unit's refusal, deletes the file it stored; a failure of the
// unit that is no refusal keeps it.
func TestUploadAnswersEachFailure(t *testing.T) {
	big := strings.Repeat("x", maxBytes+1)
	longHeader := textproto.MIMEHeader{"X-Padding": {strings.Repeat("p", 4<<10)}}
	for _, tt := range []struct {
		name        string
		path        string
		contentType string
		parts       []part
		setup       func(h *harness)
		status      int
		code, field string
		stored      bool // a file was stored, then deleted
		kept        bool // a file was stored and kept
	}{
		{"a notebook id that is no id", "/api/v0/notebooks/nope/assets", "", []part{file("a.png", "x")}, nil, 400, "bad_request", "notebook_id", false, false},
		{"a body that is not multipart", "", "application/json", nil, nil, 400, "bad_request", "", false, false},
		{"multipart without a boundary", "", "multipart/form-data", nil, nil, 400, "bad_request", "", false, false},
		{"an unknown part", "", "", []part{{name: "title", value: "x"}, file("a.png", "x")}, nil, 400, "bad_request", "", false, false},
		{"a name twice", "", "", []part{{name: "name", value: "a"}, {name: "name", value: "b"}, file("a.png", "x")}, nil, 400, "bad_request", "", false, false},
		{"parent_id after the name", "", "", []part{{name: "name", value: "a"}, {name: "parent_id", value: parentID().String()}, file("a.png", "x")},
			nil, 400, "bad_request", "", false, false},
		{"no file", "", "", []part{{name: "name", value: "a"}}, nil, 400, "bad_request", "", false, false},
		{"a parent_id that is no id", "", "", []part{{name: "parent_id", value: "root"}, file("a.png", "x")}, nil, 400, "bad_request", "parent_id", false, false},
		{"a name of more than 1 KiB", "", "", []part{{name: "name", value: strings.Repeat("n", 1<<10+1)}, file("a.png", "x")}, nil, 400, "bad_request", "name", false, false},
		{"a name not UTF-8", "", "", []part{{name: "name", value: "a\xff.png"}, file("a.png", "x")}, nil, 400, "bad_request", "name", false, false},
		{"more than 4 KiB before the file", "", "", []part{{name: "name", value: "a.png", header: longHeader}, file("a.png", "x")}, nil, 400, "bad_request", "", false, false},
		{"a part after the file", "", "", []part{file("a.png", "x"), {name: "name", value: "a"}}, nil, 400, "bad_request", "", true, false},
		{"a file past the largest", "", "", []part{file("a.png", big)}, nil, 413, "payload_too_large", "", false, false},
		{"a notebook it cannot see", "", "", []part{file("a.png", "x")}, func(h *harness) {
			h.tree.checkErr = shared.NewError(shared.KindNotFound, "notebook.not_found", "No such notebook.")
		}, 404, "notebook.not_found", "", false, false},
		{"a reader", "", "", []part{file("a.png", "x")}, func(h *harness) { h.tree.checkErr = shared.Forbidden() }, 403, "forbidden", "", false, false},
		{"a name the rules refuse", "", "", []part{file("a.md", "x")}, func(h *harness) {
			h.tree.checkErr = shared.Invalid(shared.FieldError{Field: "name", Code: shared.FieldNotAllowed, Message: "ends with .md"})
		}, 422, "validation_failed", "name", false, false},
		{"a name taken", "", "", []part{file("a.png", "x")}, func(h *harness) {
			h.tree.checkErr = shared.NewError(shared.KindConflict, "page.title_taken", "Taken.")
		}, 409, "page.title_taken", "", false, false},
		{"a store keeping no room", "", "", []part{file("a.png", "x")}, func(h *harness) { h.files.free = 99 }, 507, "storage_full", "", false, false},
		{"a store running out of room", "", "", []part{file("a.png", strings.Repeat("x", 100))}, func(h *harness) { h.files.fullAfter = 50 },
			507, "storage_full", "", false, false},
		{"a name taken since the check", "", "", []part{file("a.png", "x")}, func(h *harness) {
			h.tree.createErr = shared.NewError(shared.KindConflict, "page.title_taken", "Taken.")
		}, 409, "page.title_taken", "", true, false},
		{"a unit that failed", "", "", []part{file("a.png", "x")}, func(h *harness) { h.tree.createErr = errors.New("commit: connection reset") },
			500, "internal_error", "", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			if tt.setup != nil {
				tt.setup(h)
			}
			ct, body := form(t, tt.parts...)
			if tt.contentType != "" {
				ct, body = tt.contentType, []byte("{}")
			}
			path := uploadPath()
			if tt.path != "" {
				path = tt.path
			}
			res, answer := h.post(t, path, "session", ct, bytes.NewReader(body))
			var p problem
			_ = json.Unmarshal(answer, &p)
			if res.StatusCode != tt.status || p.Code != tt.code || (tt.field != "") != (len(p.Errors) == 1) ||
				tt.field != "" && p.Errors[0].Field != tt.field {
				t.Errorf("upload = %d %s, want %d %s on %q", res.StatusCode, answer, tt.status, tt.code, tt.field)
			}
			keys, deleted, open := h.files.state()
			if open != 0 || (len(keys) == 1) != tt.kept || (len(deleted) == 1) != tt.stored {
				t.Errorf("files %q, deleted %q, %d open; want kept %v, deleted %v", keys, deleted, open, tt.kept, tt.stored)
			}
		})
	}
}

// A notebook_id that is no id is 400 before the token is read, as the
// generated operations' parameters (M1/P1 design 3.9).
func TestUploadBindsTheNotebookIDFirst(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", "x"))
	res, answer := h.post(t, "/api/v0/notebooks/nope/assets", "", ct, bytes.NewReader(body))
	var p problem
	_ = json.Unmarshal(answer, &p)
	if res.StatusCode != http.StatusBadRequest || len(p.Errors) != 1 || p.Errors[0].Field != "notebook_id" || !res.Close {
		t.Errorf("upload = %d %s, closing %v; want 400 on notebook_id, closing", res.StatusCode, answer, res.Close)
	}
}

// The checks before the file answer as soon as the file part begins, the
// connection closed after: the file is never read, nor stored, though the
// client has not sent it, and the answer does not wait for the body's
// read deadline (5 s).
func TestUploadDecidesBeforeTheFile(t *testing.T) {
	h := newHarness(t)
	h.tree.checkErr = shared.Forbidden()
	ct, body := form(t, part{name: "name", value: "a.png"}, file("a.png", "THE FILE"))
	head := body[:bytes.Index(body, []byte("THE FILE"))]
	r, w := io.Pipe()
	go func() { _, _ = w.Write(head) }() // the rest never comes
	t.Cleanup(func() { _ = w.Close() })
	start := time.Now()
	res, _ := h.post(t, uploadPath(), "session", ct, r)
	if res.StatusCode != http.StatusForbidden || time.Since(start) > 3*time.Second || !res.Close {
		t.Errorf("upload = %d after %v, closing %v; want 403 at once, closing", res.StatusCode, time.Since(start), res.Close)
	}
	if keys, _, open := h.files.state(); len(keys) != 0 || open != 0 || len(h.files.creating) != 0 {
		t.Errorf("files %q, %d open, %d created; want none", keys, open, len(h.files.creating))
	}
}

// send sends an upload of what r brings as a session, in the background:
// the channel tells when the client is done, whatever it got.
func (h *harness) send(t *testing.T, contentType string, r io.Reader) chan struct{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.base+uploadPath(), r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer session")
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

// A client gone in the middle of its file leaves no file, and is logged
// as no error.
func TestAnUploadCutByItsClientLeavesNoFile(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", strings.Repeat("x", 600)))
	r, w := io.Pipe()
	sent := h.send(t, ct, r)
	_, _ = w.Write(body[:len(body)-300])
	<-h.files.creating
	time.Sleep(50 * time.Millisecond)
	_ = w.CloseWithError(errors.New("the client left"))
	<-sent
	waitFor(t, func() bool { return strings.Contains(h.logs.String(), "upload not received") })
	if keys, _, open := h.files.state(); len(keys) != 0 || open != 0 {
		t.Errorf("files %q, %d open; want none", keys, open)
	}
	if strings.Contains(h.logs.String(), "level=ERROR") {
		t.Errorf("logs %q, want no error", h.logs)
	}
}

// A shutdown in the middle of a file cuts the upload off: no file is kept,
// no node created, and the cut is logged as no error.
func TestShutdownCutsAnUploadOff(t *testing.T) {
	h := newHarness(t)
	stop, done := h.serve(t)
	ct, body := form(t, file("a.png", strings.Repeat("x", 600)))
	r, w := io.Pipe()
	sent := h.send(t, ct, r)
	_, _ = w.Write(body[:len(body)-300])
	<-h.files.creating
	stop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the server did not stop")
	}
	_ = w.Close()
	<-sent
	if keys, _, open := h.files.state(); len(keys) != 0 || open != 0 || len(h.rows.rows) != 0 || len(h.tree.created) != 0 {
		t.Errorf("files %q, %d open, %d rows, created %+v; want none", keys, open, len(h.rows.rows), h.tree.created)
	}
	if logs := h.logs.String(); !strings.Contains(logs, "upload cut off by the shutdown") || strings.Contains(logs, "level=ERROR") {
		t.Errorf("logs %q, want the cut, and no error", logs)
	}
}

// waitFor waits up to 10 s for cond.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !cond(); {
		if time.Now().After(deadline) {
			t.Fatal("the condition never held")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
