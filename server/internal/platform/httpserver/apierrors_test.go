package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// problemErr is a ProblemError as a module's error type would be.
type problemErr struct {
	status int
	code   string
	detail string
	fields []error
	retry  time.Duration
}

func (e problemErr) Error() string             { return e.detail }
func (e problemErr) ProblemStatus() int        { return e.status }
func (e problemErr) ProblemCode() string       { return e.code }
func (e problemErr) ProblemFields() []error    { return e.fields }
func (e problemErr) RetryAfter() time.Duration { return e.retry }

type fieldErr struct{ field, code, message string }

func (f fieldErr) Error() string        { return f.message }
func (f fieldErr) ProblemField() string { return f.field }
func (f fieldErr) ProblemCode() string  { return f.code }

// minimalErr has only the required methods of ProblemError.
type minimalErr struct{}

func (minimalErr) Error() string       { return "the thing is gone" }
func (minimalErr) ProblemStatus() int  { return http.StatusNotFound }
func (minimalErr) ProblemCode() string { return "things.not_found" }

// Lookalikes of the binding errors that oapi-codegen declares in each
// module's gen package (identity's server.gen.go): the platform sees only
// their shape.
type (
	InvalidParamFormatError struct {
		ParamName string
		Err       error
	}
	RequiredParamError  struct{ ParamName string }
	RequiredHeaderError struct {
		ParamName string
		Err       error
	}
	TooManyValuesForParamError struct {
		ParamName string
		Count     int
	}
	numberedParamError struct{ ParamName int }
	codeError          string
)

func (e *InvalidParamFormatError) Error() string {
	return "Invalid format for parameter " + e.ParamName + ": " + e.Err.Error()
}
func (e *RequiredParamError) Error() string {
	return "Query argument " + e.ParamName + " is required, but not found"
}
func (e *RequiredHeaderError) Error() string {
	return "Header parameter " + e.ParamName + " is required, but not found"
}
func (e *TooManyValuesForParamError) Error() string {
	return fmt.Sprintf("Expected one value for %s, got %d", e.ParamName, e.Count)
}
func (e *numberedParamError) Error() string { return fmt.Sprint("parameter number ", e.ParamName) }
func (e codeError) Error() string           { return string(e) }

// The parameter comes from the binding error; its message names Go
// functions, so the detail is generic and the message goes to the debug log.
func TestAPIErrorsBadRequestNamesTheParameter(t *testing.T) {
	const detail = `"detail":"The request parameters do not match the API description."`
	invalid := &InvalidParamFormatError{ParamName: "limit", Err: errors.New(`error binding string parameter: strconv.ParseInt: parsing "abc": invalid syntax`)}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"invalid format", invalid, `,"errors":[{"field":"limit","code":"invalid_format","message":"has the wrong type or format"}]`},
		{"required", &RequiredParamError{ParamName: "view"}, `,"errors":[{"field":"view","code":"required","message":"is required"}]`},
		{"required header", &RequiredHeaderError{ParamName: "X-Thing", Err: errors.New("missing")},
			`,"errors":[{"field":"X-Thing","code":"required","message":"is required"}]`},
		{"too many values", &TooManyValuesForParamError{ParamName: "X-Thing", Count: 2},
			`,"errors":[{"field":"X-Thing","code":"invalid_format","message":"has the wrong type or format"}]`},
		{"not a binding error", errors.New("Invalid format for parameter limit"), ""},
		{"ParamName not a string", &numberedParamError{ParamName: 7}, ""},
		{"not a struct", codeError("limit"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := captureLogs(t)
			errs := NewAPIErrors(logger)
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { errs.BadRequest(w, r, tt.err) })
			req := httptest.NewRequest(http.MethodGet, "/api/v0/things?limit=abc", nil)
			req.Header.Set(HeaderRequestID, "req-9")

			rec := serve(middleware(h, logger), req)

			sent := rec.Result().Header // as the status went out, not as set after it
			want := `{"status":400,"code":"bad_request","title":"Bad Request",` + detail + tt.want + "}\n"
			if rec.Code != http.StatusBadRequest || sent.Get("Content-Type") != ContentTypeProblem || rec.Body.String() != want {
				t.Errorf("response = %d %s %s, want 400 problem+json %s", rec.Code, sent.Get("Content-Type"), rec.Body, want)
			}
			entry := findLog(logs(), "request parameters not bound")
			if entry == nil || entry["level"] != "DEBUG" || entry["error"] != tt.err.Error() || entry["request_id"] != "req-9" {
				t.Errorf("log = %v, want the binding error with request_id at debug level", entry)
			}
		})
	}
}

// The decoder's messages name Go types: the answer is generic and the
// message goes to the debug log (M0-P3 handoff 2).
func TestAPIErrorsBodyErrorHidesTheDecoderMessage(t *testing.T) {
	logger, logs := captureLogs(t)
	errs := NewAPIErrors(logger)
	decodeErr := errors.New("can't decode JSON body: json: cannot unmarshal number into Go struct field RegisterRequest.email of type string")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { errs.BodyError(w, r, decodeErr) })

	rec := serve(h, httptest.NewRequest(http.MethodPost, "/api/v0/things", nil))

	want := `{"status":400,"code":"bad_request","title":"Bad Request","detail":"The request body could not be decoded."}` + "\n"
	if rec.Code != http.StatusBadRequest || rec.Body.String() != want {
		t.Errorf("response = %d %s, want 400 %s", rec.Code, rec.Body, want)
	}
	if entry := findLog(logs(), "request body not decoded"); entry == nil || entry["level"] != "DEBUG" || entry["error"] != decodeErr.Error() {
		t.Errorf("log = %v, want the decoder's message at debug level", entry)
	}
}

func TestAPIErrorsBodyErrorOfAnOversizedBodyIs413(t *testing.T) {
	errs := NewAPIErrors(slog.New(slog.DiscardHandler))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs.BodyError(w, r, fmt.Errorf("can't decode JSON body: %w", &http.MaxBytesError{Limit: 1024}))
	})

	rec := serve(h, httptest.NewRequest(http.MethodPost, "/api/v0/things", nil))

	want := `{"status":413,"code":"payload_too_large","title":"Request Entity Too Large","detail":"The request body exceeds 1024 bytes."}` + "\n"
	if rec.Code != http.StatusRequestEntityTooLarge || rec.Body.String() != want {
		t.Errorf("response = %d %s, want %s", rec.Code, rec.Body, want)
	}
}

func TestWriteMapsProblemErrors(t *testing.T) {
	conflict := problemErr{status: http.StatusConflict, code: "things.taken", detail: "The name is taken."}
	invalid := problemErr{
		status: http.StatusUnprocessableEntity, code: "validation_failed", detail: "The request has invalid values.",
		fields: []error{
			fieldErr{"name", "too_long", "at most 255 characters"},
			errors.New("not a field error: skipped"),
			fieldErr{"tags[1].name", "required", "is required"},
		},
	}
	tests := []struct {
		name       string
		err        error
		status     int
		body       string
		retryAfter string
	}{
		{"problem error", conflict, http.StatusConflict,
			`{"status":409,"code":"things.taken","title":"Conflict","detail":"The name is taken."}`, ""},
		{"wrapped", fmt.Errorf("create thing: %w", conflict), http.StatusConflict,
			`{"status":409,"code":"things.taken","title":"Conflict","detail":"The name is taken."}`, ""},
		{"joined", errors.Join(errors.New("context"), conflict), http.StatusConflict,
			`{"status":409,"code":"things.taken","title":"Conflict","detail":"The name is taken."}`, ""},
		{"only the required methods", minimalErr{}, http.StatusNotFound,
			`{"status":404,"code":"things.not_found","title":"Not Found","detail":"the thing is gone"}`, ""},
		{"field errors", invalid, http.StatusUnprocessableEntity,
			`{"status":422,"code":"validation_failed","title":"Unprocessable Entity","detail":"The request has invalid values.",` +
				`"errors":[{"field":"name","code":"too_long","message":"at most 255 characters"},{"field":"tags[1].name","code":"required","message":"is required"}]}`, ""},
		{"retry after rounds up", problemErr{status: http.StatusServiceUnavailable, code: "server_busy", detail: "busy", retry: 1500 * time.Millisecond},
			http.StatusServiceUnavailable, `{"status":503,"code":"server_busy","title":"Service Unavailable","detail":"busy"}`, "2"},
		{"payload too large", fmt.Errorf("read body: %w", &http.MaxBytesError{Limit: 1048576}), http.StatusRequestEntityTooLarge,
			`{"status":413,"code":"payload_too_large","title":"Request Entity Too Large","detail":"The request body exceeds 1048576 bytes."}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := NewAPIErrors(slog.New(slog.DiscardHandler))
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { errs.Write(w, r, tt.err) })

			rec := serve(h, httptest.NewRequest(http.MethodPost, "/api/v0/things", nil))

			sent := rec.Result().Header
			if rec.Code != tt.status || sent.Get("Content-Type") != ContentTypeProblem || rec.Body.String() != tt.body+"\n" {
				t.Errorf("response = %d %s %s, want %d problem+json %s", rec.Code, sent.Get("Content-Type"), rec.Body, tt.status, tt.body)
			}
			if got := sent.Get("Retry-After"); got != tt.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tt.retryAfter)
			}
		})
	}
}

// Nothing was written yet, so the platform chain's recorder reports the
// response as not started and Write answers the 500 problem.
func TestWriteLogsAndHidesAnInternalError(t *testing.T) {
	logger, logs := captureLogs(t)
	errs := NewAPIErrors(logger)
	h := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, errors.New("dial tcp 10.0.0.5:5432: connection refused"))
	}), logger)
	req := httptest.NewRequest(http.MethodGet, "/api/v0/things", nil)
	req.Header.Set(HeaderRequestID, "req-7")

	rec := serve(h, req)

	if ct := rec.Result().Header.Get("Content-Type"); rec.Code != http.StatusInternalServerError || ct != ContentTypeProblem {
		t.Errorf("response = %d %s, want 500 problem+json", rec.Code, ct)
	}
	want := `{"status":500,"code":"internal_error","title":"Internal Server Error"}` + "\n"
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body, want)
	}
	entry := findLog(logs(), "API handler failed")
	if entry == nil || entry["level"] != "ERROR" || entry["error"] != "dial tcp 10.0.0.5:5432: connection refused" ||
		entry["request_id"] != "req-7" || entry["method"] != "GET" || entry["path"] != "/api/v0/things" {
		t.Errorf("log = %v, want the error with request_id, method and path at level ERROR", entry)
	}
}

// A client that went away is not a server fault: no 500, no ERROR line
// (M0-P3 handoff 2).
func TestWriteOfACancelledRequestIsNot500(t *testing.T) {
	logger, logs := captureLogs(t)
	errs := NewAPIErrors(logger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v0/things", nil).WithContext(ctx)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, fmt.Errorf("query things: %w", context.Canceled))
	})

	rec := serve(h, req)

	if rec.Code != statusClientClosedRequest {
		t.Errorf("status = %d, want %d", rec.Code, statusClientClosedRequest)
	}
	for _, e := range logs() {
		if e["level"] == "ERROR" {
			t.Errorf("log %v at level ERROR, want none", e)
		}
	}
	if entry := findLog(logs(), "client went away"); entry == nil || entry["level"] != "DEBUG" {
		t.Errorf("log = %v, want the cancellation at debug level", entry)
	}
}

// context.Canceled while the request itself is still live is a server fault.
func TestWriteOfACanceledErrorOnALiveRequestIs500(t *testing.T) {
	errs := NewAPIErrors(slog.New(slog.DiscardHandler))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { errs.Write(w, r, context.Canceled) })

	rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/v0/things", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

// Generated strict code also reports a write that failed after the response
// started (WriteHeader(200), then the buffered body fails to go out). A
// problem appended there would corrupt the 200, so the response is aborted.
func TestWriteAbortsAStartedResponse(t *testing.T) {
	logger, logs := captureLogs(t)
	errs := NewAPIErrors(logger)
	srv := httptest.NewServer(middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"product":`)
		_ = http.NewResponseController(w).Flush()
		errs.Write(w, r, errors.New("write tcp: broken pipe"))
	}), logger))
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v0/things", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(HeaderRequestID, "req-8")

	resp, err := client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	srv.Close() // waits for the handler, so its log lines are complete

	if resp.StatusCode != http.StatusOK || readErr == nil {
		t.Errorf("response = %d, read error %v; want the started 200 cut off", resp.StatusCode, readErr)
	}
	if string(body) != `{"product":` {
		t.Errorf("body = %q, want only what the handler wrote: no problem appended", body)
	}
	entries := logs()
	for _, e := range entries {
		if e["level"] == "ERROR" {
			t.Errorf("log %v at level ERROR, want none: the client may simply be gone", e)
		}
	}
	entry := findLog(entries, "response failed after it started")
	if entry == nil || entry["level"] != "WARN" || entry["error"] != "write tcp: broken pipe" ||
		entry["request_id"] != "req-8" || entry["method"] != "GET" || !strings.HasSuffix(entry["path"].(string), "/api/v0/things") {
		t.Errorf("log = %v, want the error with request_id, method and path at level WARN", entry)
	}
}

// wrappingWriter stands for a module middleware that wraps the writer.
type wrappingWriter struct{ http.ResponseWriter }

func (w wrappingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func TestResponseStartedSeesThroughWrappers(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	w := wrappingWriter{rec}
	if responseStarted(w) {
		t.Error("responseStarted() = true before anything was written")
	}
	rec.WriteHeader(http.StatusOK)
	if !responseStarted(w) {
		t.Error("responseStarted() = false after WriteHeader, through a wrapper")
	}
	if responseStarted(httptest.NewRecorder()) {
		t.Error("responseStarted() = true for a writer outside the platform chain")
	}
}
