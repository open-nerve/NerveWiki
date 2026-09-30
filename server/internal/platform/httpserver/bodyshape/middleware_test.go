package bodyshape

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// run sends body to the middleware for pattern and reports what reached the
// next handler, or the error given to onError.
func run(t *testing.T, pattern, body string, limit int64) (reached string, called bool, err error) {
	t.Helper()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		data, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		reached = string(data)
	})
	h := Middleware(things(), func(_ http.ResponseWriter, _ *http.Request, e error) { err = e })(next)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v0/things", strings.NewReader(body))
	req.Pattern = pattern
	if limit > 0 {
		req.Body = http.MaxBytesReader(rec, req.Body, limit)
	}
	h.ServeHTTP(rec, req)
	return reached, called, err
}

func TestMiddlewarePassesAValidBodyOnUnchanged(t *testing.T) {
	body := " \t\r\n" + `{ "name": "a",  "nested": {"a": "y"} }` + "\n"

	reached, called, err := run(t, pattern, body, 0)

	if !called || err != nil || reached != body {
		t.Errorf("next called %v with %q, error %v; want the body unchanged", called, reached, err)
	}
}

func TestMiddlewareAnswersEveryStructuralProblem(t *testing.T) {
	_, called, err := run(t, pattern, `{"extra":1,"nested":{"a":"y"}}`, 0)

	var shapeErr *Error
	if called || !errors.As(err, &shapeErr) {
		t.Fatalf("next called %v, error %v; want a *Error", called, err)
	}
	want := []FieldError{{"extra", "not_allowed"}, {"name", "required"}}
	if !slices.Equal(shapeErr.Fields, want) {
		t.Errorf("fields = %v, want %v", shapeErr.Fields, want)
	}
}

func TestMiddlewareAnswersABodyThatIsNotJSON(t *testing.T) {
	// A form feed is not JSON whitespace (RFC 8259 §2): the body is not JSON,
	// neither empty nor an object with space before it.
	for _, body := range []string{`{"name":`, `{"name":"a","nested":{"a":"y"}} trailing`, `nope`,
		"\f", "\f" + `{"name":"a","nested":{"a":"y"}}`} {
		_, called, err := run(t, pattern, body, 0)

		if called || !errors.Is(err, ErrNotJSON) {
			t.Errorf("body %q: next called %v, error %v; want ErrNotJSON", body, called, err)
		}
	}
}

// The generated decoder answers an empty body; routes without a root are
// not this middleware's business.
func TestMiddlewarePassesOnWhatItDoesNotCheck(t *testing.T) {
	tests := []struct{ name, pattern, body string }{
		{"empty body", pattern, ""},
		{"blank body", pattern, " \t\r\n"},
		{"route without a body schema", "GET /api/v0/things", "not even JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reached, called, err := run(t, tt.pattern, tt.body, 0)

			if !called || err != nil || reached != tt.body {
				t.Errorf("next called %v with %q, error %v; want the request passed on", called, reached, err)
			}
		})
	}
}

func TestMiddlewareReportsAnOversizedBody(t *testing.T) {
	_, called, err := run(t, pattern, `{"name":"`+strings.Repeat("a", 100)+`"}`, 16)

	var tooLarge *http.MaxBytesError
	if called || !errors.As(err, &tooLarge) {
		t.Errorf("next called %v, error %v; want the *http.MaxBytesError", called, err)
	}
}
