package httpadapter_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// The upload's body as it comes (M7/P2 design 3.5): the 4 KiB before the
// file, what follows the file, the type of the body, and the refusals that
// close the connection.

// prefaced is a form whose bytes before the file's are exactly before: a
// name part padded by a header, then the file part holding content.
func prefaced(t *testing.T, before int, content string) (contentType string, body []byte) {
	t.Helper()
	build := func(pad int) (string, []byte) {
		return form(t, part{name: "name", value: "a.png", header: textproto.MIMEHeader{"X-Pad": {strings.Repeat("p", pad)}}},
			file("a.png", content))
	}
	_, body = build(100)
	contentType, body = build(100 + before - bytes.Index(body, []byte(content)))
	if at := bytes.Index(body, []byte(content)); at != before {
		t.Fatalf("the file starts at %d, want %d", at, before)
	}
	return contentType, body
}

// The bytes before the file may be 4 KiB, not one more, whether they come
// at once or in pieces; the file after them is read whole, up to the
// largest, the limit lifted once the file opens.
func TestUploadTakesFourKiBBeforeTheFile(t *testing.T) {
	content := strings.Repeat("F", maxBytes)
	h := newHarness(t)
	ct, body := prefaced(t, 4<<10, content)
	if res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body)); res.StatusCode != http.StatusCreated {
		t.Errorf("4096 bytes before the largest file = %d %s, want 201", res.StatusCode, answer)
	}
	if keys, _, _ := h.files.state(); len(keys) != 1 || len(h.files.files[keys[0]]) != maxBytes {
		t.Errorf("files %q, want the file whole", keys)
	}
	for _, pieces := range []bool{false, true} {
		h := newHarness(t)
		ct, body := prefaced(t, 4<<10+1, "x")
		var r io.Reader = bytes.NewReader(body)
		if pieces {
			pr, pw := io.Pipe()
			go func() {
				_, _ = pw.Write(body[:50])
				time.Sleep(100 * time.Millisecond)
				_, _ = pw.Write(body[50:])
				_ = pw.Close()
			}()
			r = pr
		}
		if res, answer := h.post(t, uploadPath(), "session", ct, r); res.StatusCode != http.StatusBadRequest ||
			!strings.Contains(string(answer), "4 KiB before the file") {
			t.Errorf("4097 bytes before the file, in pieces %v = %d %s, want 400 for the 4 KiB", pieces, res.StatusCode, answer)
		}
	}
}

// The 4 KiB running out in a part's value is the preface's 400, no field's.
func TestUploadRunsOutOfItsPrefaceInAValue(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, part{name: "name", value: strings.Repeat("n", 1000), header: textproto.MIMEHeader{"X-Pad": {strings.Repeat("p", 3500)}}},
		file("a.png", "x"))
	if at := bytes.Index(body, []byte("nnnn")); at > 4<<10-200 || at+1000 < 4<<10 {
		t.Fatalf("the value starts at %d, want it across 4096", at)
	}
	res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
	var p problem
	_ = json.Unmarshal(answer, &p)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "4 KiB before the file") || len(p.Errors) != 0 {
		t.Errorf("upload = %d %s, want 400 for the 4 KiB, no field", res.StatusCode, answer)
	}
}

// A body of another type than multipart/form-data is 400 though it has a
// boundary, and so is one whose boundary is empty.
func TestUploadTakesFormDataAlone(t *testing.T) {
	_, body := form(t, file("a.png", "x"))
	boundary := string(body[2:bytes.IndexByte(body, '\r')])
	for _, ct := range []string{"multipart/mixed; boundary=" + boundary, "application/json; boundary=" + boundary,
		`multipart/form-data; boundary=""`} {
		h := newHarness(t)
		res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
		if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), "not multipart/form-data with a boundary") {
			t.Errorf("%s = %d %s, want 400, not multipart/form-data", ct, res.StatusCode, answer)
		}
	}
}

// A body refused before the file answers at once and closes, though the
// client never ends it: a type that is not the form's, a part unknown; and
// so does a file past the largest, though the client goes on sending, its
// detail naming the largest.
func TestUploadRefusesItsBodyAtOnce(t *testing.T) {
	ct, body := form(t, part{name: "title", value: "x"}, file("a.png", "THE FILE"))
	bigCT, big := form(t, file("a.png", strings.Repeat("x", 4*maxBytes)))
	for _, tt := range []struct {
		name, contentType string
		head              []byte
		status            int
		detail            string
	}{
		{"another type", "application/json", []byte("{"), http.StatusBadRequest, "not multipart/form-data"},
		{"an unknown part", ct, body[:bytes.Index(body, []byte("THE FILE"))], http.StatusBadRequest, "parent_id, name and file"},
		{"a file past the largest", bigCT, big[:bytes.Index(big, []byte("xxxx"))+2*maxBytes], http.StatusRequestEntityTooLarge,
			"exceeds 1024 bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			r, w := io.Pipe()
			go func() { _, _ = w.Write(tt.head) }() // the rest never comes
			t.Cleanup(func() { _ = w.Close() })
			start := time.Now()
			res, answer := h.post(t, uploadPath(), "session", tt.contentType, r)
			if res.StatusCode != tt.status || !strings.Contains(string(answer), tt.detail) || time.Since(start) > 3*time.Second ||
				!res.Close {
				t.Errorf("upload = %d %s after %v, closing %v; want %d %q at once, closing", res.StatusCode, answer, time.Since(start),
					res.Close, tt.status, tt.detail)
			}
		})
	}
}

// A unit's failure is answered on the connection kept: the body was read
// to its end.
func TestUploadKeepsTheConnectionAfterTheBody(t *testing.T) {
	h := newHarness(t)
	h.tree.createErr = errors.New("commit: connection reset")
	ct, body := form(t, file("a.png", "x"))
	if res, _ := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body)); res.StatusCode != http.StatusInternalServerError || res.Close {
		t.Errorf("upload = %d, closing %v; want 500, the connection kept", res.StatusCode, res.Close)
	}
}

// The parts' details and fields: no file part; a parent_id too long, in
// braces or as a URN, invalid_format on parent_id; a name of 1 KiB is
// taken.
func TestUploadNamesWhatIsWrong(t *testing.T) {
	for _, tt := range []struct {
		name   string
		parts  []part
		status int
		detail string
		field  string
	}{
		{"no file", []part{{name: "name", value: "a"}}, http.StatusBadRequest, "no file part", ""},
		{"a parent_id too long", []part{{name: "parent_id", value: parentID().String() + "0"}, file("a.png", "x")},
			http.StatusBadRequest, "parent_id part", "parent_id"},
		{"a parent_id in braces", []part{{name: "parent_id", value: "{" + parentID().String() + "}"}, file("a.png", "x")},
			http.StatusBadRequest, "parent_id part", "parent_id"},
		{"a parent_id as a URN", []part{{name: "parent_id", value: "urn:uuid:" + parentID().String()}, file("a.png", "x")},
			http.StatusBadRequest, "parent_id part", "parent_id"},
		{"a name of 1 KiB", []part{{name: "name", value: strings.Repeat(" ", 1<<10-5) + "a.png"}, file("b.png", "x")},
			http.StatusCreated, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			ct, body := form(t, tt.parts...)
			res, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
			var p problem
			_ = json.Unmarshal(answer, &p)
			if res.StatusCode != tt.status || !strings.Contains(string(answer), tt.detail) ||
				tt.field != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tt.field || p.Errors[0].Code != "invalid_format") {
				t.Errorf("upload = %d %s, want %d %q on %q invalid_format", res.StatusCode, answer, tt.status, tt.detail, tt.field)
			}
			if logs := h.logs.String(); strings.Contains(logs, "upload not received") {
				t.Errorf("logs %q, want no body not received", logs)
			}
		})
	}
}

// What follows the file, malformed, is a body not read whole, whatever was
// read before the file: 400, the file deleted, nothing created. A part
// after the file whose header runs past the route's limit is a part after
// the file, 400.
func TestUploadReadsWhatFollowsTheFile(t *testing.T) {
	head := "--b\r\nContent-Disposition: form-data; name=\"name\"\r\nX-Pad: "
	rest := "\r\n\r\na.png\r\n--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n\r\n"
	for _, tt := range []struct {
		name, body, detail string
	}{
		{"malformed", head + "p" + rest + "FILE\r\n--b\r\nQQQQ-no-colon\r\n\r\nv\r\n--b--\r\n", "not read whole"},
		{"malformed, after 4 KiB", head + strings.Repeat("p", 4<<10-len(head)-len(rest)) + rest +
			"FILE\r\n--b\r\nQQQQ-no-colon\r\n\r\nv\r\n--b--\r\n", "not read whole"},
		{"a part whose header runs past the limit", head + "p" + rest + "FILE\r\n--b\r\nX-Long: " + strings.Repeat("q", 80<<10) +
			"\r\n\r\nv\r\n--b--\r\n", "a part after the file"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			res, answer := h.post(t, uploadPath(), "session", "multipart/form-data; boundary=b", strings.NewReader(tt.body))
			if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(answer), tt.detail) {
				t.Errorf("upload = %d %s, want 400 %q", res.StatusCode, answer, tt.detail)
			}
			if keys, deleted, _ := h.files.state(); len(keys) != 0 || len(deleted) != 1 || len(h.tree.created) != 0 {
				t.Errorf("files %q, deleted %q, created %+v; want the file deleted, nothing created", keys, deleted, h.tree.created)
			}
		})
	}
}

// A body that fails after the form's end creates nothing: the file is
// deleted, the failure logged as a body not received.
func TestUploadCutAfterTheFormCreatesNothing(t *testing.T) {
	h := newHarness(t)
	ct, body := form(t, file("a.png", "x"))
	r, w := io.Pipe()
	sent := h.send(t, ct, r)
	_, _ = w.Write(append(body, "zz"...))
	time.Sleep(100 * time.Millisecond)
	_ = w.CloseWithError(errors.New("the client left"))
	<-sent
	waitFor(t, func() bool { return strings.Contains(h.logs.String(), "upload not received") })
	if keys, _, _ := h.files.state(); len(keys) != 0 || len(h.tree.created) != 0 {
		t.Errorf("files %q, created %+v; want none", keys, h.tree.created)
	}
}

// A shutdown once the file is stored, before the form's end, cuts the
// upload off: the stored file is deleted, nothing created.
func TestShutdownAfterTheFileDeletesIt(t *testing.T) {
	h := newHarness(t)
	stop, done := h.serve(t)
	ct, body := form(t, file("a.png", "x"))
	cut := bytes.LastIndex(body, []byte("--")) + 2 // the closing boundary without its CRLF: the file ends, the form not
	r, w := io.Pipe()
	sent := h.send(t, ct, r)
	_, _ = w.Write(body[:cut])
	waitFor(t, func() bool { keys, _, _ := h.files.state(); return len(keys) == 1 })
	stop()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the server did not stop")
	}
	_ = w.Close()
	<-sent
	if keys, _, _ := h.files.state(); len(keys) != 0 || len(h.tree.created) != 0 {
		t.Errorf("files %q, created %+v; want none", keys, h.tree.created)
	}
	if logs := h.logs.String(); !strings.Contains(logs, "upload cut off by the shutdown") {
		t.Errorf("logs %q, want the cut", logs)
	}
}

// The attachment's every member: its notebook, uploader, time and SHA-256;
// an image's width and height, each its own.
func TestUploadAnswersEveryMember(t *testing.T) {
	h := newHarness(t)
	content := "\x89PNG\r\n\x1a\nrest"
	ct, body := form(t, file("a.png", content))
	_, answer := h.post(t, uploadPath(), "session", ct, bytes.NewReader(body))
	var a struct {
		NotebookID string    `json:"notebook_id"`
		CreatedBy  string    `json:"created_by"`
		CreatedAt  time.Time `json:"created_at"`
		Sha256     string    `json:"sha256"`
	}
	if err := json.Unmarshal(answer, &a); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	if a.NotebookID != notebookID().String() || a.CreatedBy != alice().String() || !a.CreatedAt.Equal(now()) ||
		a.Sha256 != hex.EncodeToString(sum[:]) {
		t.Errorf("answer = %s, want the notebook, alice, the time and the file's SHA-256", answer)
	}
	n, b := h.attach("w.png", "image/png", "abc")
	b.Width, b.Height = 3, 2
	h.rows.rows[n.ID] = b
	_, got := h.get(t, http.MethodGet, "/api/v0/assets/"+n.ID.String(), "session")
	var s struct{ Width, Height *int }
	if err := json.Unmarshal(got, &s); err != nil {
		t.Fatal(err)
	}
	if s.Width == nil || s.Height == nil || *s.Width != 3 || *s.Height != 2 {
		t.Errorf("getAsset = %s, want width 3, height 2", got)
	}
}
