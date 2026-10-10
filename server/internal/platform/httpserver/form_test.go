package httpserver_test

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// formRequest is a request of the form body, its boundary b.
func formRequest(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	return r
}

// textPart is a text part named name, file a file part, end the closing
// boundary.
func textPart(name, value string) string {
	return "--b\r\nContent-Disposition: form-data; name=\"" + name + "\"\r\n\r\n" + value + "\r\n"
}

func filePart(name, filename, content string) string {
	return "--b\r\nContent-Disposition: form-data; name=\"" + name + "\"; filename=\"" + filename + "\"\r\n\r\n" + content + "\r\n"
}

const formEnd = "--b--\r\n"

// detailOf is err's detail and part when it is a *FormError.
func detailOf(err error) (string, string) {
	var f *httpserver.FormError
	if !errors.As(err, &f) {
		return "", ""
	}
	return f.Detail, f.Part
}

// A form reads its text parts, each at most once, in order, any left
// out, then the file's part, whose bytes pass once it is open; then its
// end.
func TestAFormReadsItsPartsThenTheFile(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       map[string]string
	}{
		{"both", textPart("a", "1") + textPart("b", "two") + filePart("file", "x.zip", "THE FILE") + formEnd, map[string]string{"a": "1", "b": "two"}},
		{"one left out", textPart("b", "two") + filePart("file", "x.zip", "THE FILE") + formEnd, map[string]string{"b": "two"}},
		{"none", filePart("file", "x.zip", "THE FILE") + formEnd, map[string]string{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := formRequest(tt.body)
			form, err := httpserver.NewForm(r, []string{"a", "b"}, "file", 4<<10)
			if err != nil {
				t.Fatal(err)
			}
			values, file, err := form.Fields(map[string]int{"a": 1, "b": 3})
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != len(tt.want) || values["a"] != tt.want["a"] || values["b"] != tt.want["b"] || file.FileName() != "x.zip" {
				t.Errorf("Fields() = %v, %q; want %v, x.zip", values, file.FileName(), tt.want)
			}
			form.Open()
			content, err := io.ReadAll(file)
			if err != nil || string(content) != "THE FILE" {
				t.Errorf("the file = %q, %v", content, err)
			}
			if err := form.End(r); err != nil {
				t.Errorf("End() = %v, want nil", err)
			}
		})
	}
}

// What is wrong before the file is 400, named: another type, parts out of
// order, twice or unknown, no file part, a value too long or not UTF-8,
// more than the preface before the file.
func TestAFormRefusesWhatIsWrongBeforeTheFile(t *testing.T) {
	file := filePart("file", "x.zip", "F") + formEnd
	for _, tt := range []struct {
		name, contentType, body, detail, part string
	}{
		{"another type", "application/json", "{}", "not multipart/form-data", ""},
		{"no boundary", "multipart/form-data", file, "not multipart/form-data", ""},
		{"out of order", "", textPart("b", "x") + textPart("a", "x") + file, "The form's parts are a, b and file, each once and in this order.", ""},
		{"twice", "", textPart("a", "x") + textPart("a", "x") + file, "each once and in this order", ""},
		{"unknown", "", textPart("c", "x") + file, "each once and in this order", ""},
		{"after the file", "", filePart("file", "x.zip", "F") + textPart("a", "x") + formEnd, "", ""},
		{"no file", "", textPart("a", "x") + formEnd, "The form has no file part.", ""},
		{"a value too long", "", textPart("a", "xx") + file, "The form's a part is not valid.", "a"},
		{"a value not UTF-8", "", textPart("a", "\xff") + file, "The form's a part is not valid.", "a"},
		{"more than the preface before the file", "", textPart("b", strings.Repeat("x", 200)) + file, "more than 128 bytes before the file", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := formRequest(tt.body)
			if tt.contentType != "" {
				r.Header.Set("Content-Type", tt.contentType)
			}
			form, err := httpserver.NewForm(r, []string{"a", "b"}, "file", 128)
			if err == nil {
				_, _, err = form.Fields(map[string]int{"a": 1, "b": 1 << 10})
			}
			if tt.detail == "" {
				if err != nil {
					t.Errorf("Fields() = %v, want the file's part: what follows it is End's", err)
				}
				return
			}
			detail, part := detailOf(err)
			var p httpserver.ProblemError
			if !strings.Contains(detail, tt.detail) || part != tt.part || !errors.As(err, &p) || p.ProblemStatus() != http.StatusBadRequest {
				t.Errorf("Fields() = %v (part %q), want 400 %q on %q", err, part, tt.detail, tt.part)
			}
		})
	}
}

// What follows the file: a part after it is 400, and so is a body that
// goes on past the limit; a malformed rest is a body not read whole.
func TestAFormReadsWhatFollowsTheFile(t *testing.T) {
	for _, tt := range []struct {
		name, rest, detail string
		limit              int64
		read               bool
	}{
		{"a part after the file", textPart("a", "x") + formEnd, "The form has a part after the file.", 0, false},
		{"malformed", "--b\r\nQQQQ-no-colon\r\n\r\nv\r\n" + formEnd, "", 0, true},
		{"a body past the limit after the form", formEnd + strings.Repeat("z", 100), "The body goes on after the form's end.", 1, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := filePart("file", "x.zip", "F") + tt.rest
			r := formRequest(body)
			if tt.limit > 0 {
				r.Body = http.MaxBytesReader(httptest.NewRecorder(), r.Body, int64(len(body))-50)
			}
			form, err := httpserver.NewForm(r, nil, "file", 4<<10)
			if err != nil {
				t.Fatal(err)
			}
			_, file, err := form.Fields(nil)
			if err != nil {
				t.Fatal(err)
			}
			form.Open()
			if _, err := io.Copy(io.Discard, file); err != nil {
				t.Fatal(err)
			}
			err = form.End(r)
			var read *httpserver.BodyReadError
			if detail, _ := detailOf(err); tt.read && !errors.As(err, &read) || !tt.read && detail != tt.detail {
				t.Errorf("End() = %v, want %q (a body not read whole: %t)", err, tt.detail, tt.read)
			}
		})
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
		{&httpserver.BodyReadError{Err: io.EOF}, "ended early"},
		{&net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, "connection failed"},
		{errors.New("malformed MIME header line: x"), "malformed"},
	} {
		if got := httpserver.ReadCause(tt.err); got != tt.want {
			t.Errorf("ReadCause(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}
