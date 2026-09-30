package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
)

// TestPlatformProblemsMatchTheContract keeps Problem in step with the Problem
// schema of api/common.yaml: every problem the platform writes must validate.
func TestPlatformProblemsMatchTheContract(t *testing.T) {
	contract := apitest.Load(t)
	discard := slog.New(slog.DiscardHandler)
	errs := NewAPIErrors(discard)
	notReady := Check{Name: "database", Run: func(context.Context) error { return errors.New("down") }}
	shaped, _ := mount(newTestAPI(t))
	writing := func(err error) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { errs.Write(w, r, err) })
	}
	tests := []struct {
		name   string
		h      http.Handler
		method string
		target string
		body   string
		status int
	}{
		{"unknown API path", NewRouter(discard), http.MethodGet, "/api/v0/nope", "", http.StatusNotFound},
		{"not ready", NewRouter(discard, notReady), http.MethodGet, "/readyz", "", http.StatusServiceUnavailable},
		{"panic", middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), discard),
			http.MethodGet, "/api/v0/boom", "", http.StatusInternalServerError},
		{"parameter that does not bind", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			errs.BadRequest(w, r, &InvalidParamFormatError{ParamName: "limit", Err: errors.New("not a number")})
		}), http.MethodGet, "/api/v0/things?limit=x", "", http.StatusBadRequest},
		{"body not decoded", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			errs.BodyError(w, r, errors.New("EOF"))
		}), http.MethodPost, "/api/v0/things", "", http.StatusBadRequest},
		{"body of the wrong shape", shaped, http.MethodPost, "/api/v0/open", `{"extra":1}`, http.StatusBadRequest},
		{"body too large", shaped, http.MethodPost, "/api/v0/open", `{"name":"` + strings.Repeat("a", 100) + `"}`, http.StatusRequestEntityTooLarge},
		{"no bearer token", shaped, http.MethodPost, "/api/v0/things", `{"name":"a"}`, http.StatusUnauthorized},
		{"internal error", writing(errors.New("boom")), http.MethodGet, "/api/v0/things", "", http.StatusInternalServerError},
		{"domain error", writing(minimalErr{}), http.MethodGet, "/api/v0/things", "", http.StatusNotFound},
		{"field errors", writing(problemErr{
			status: http.StatusUnprocessableEntity, code: "validation_failed", detail: "The request has invalid values.",
			fields: []error{fieldErr{"name", "required", "is required"}, fieldErr{"title", "too_long", "is too long"}},
		}), http.MethodGet, "/api/v0/things", "", http.StatusUnprocessableEntity},
		{"retry later", writing(problemErr{
			status: http.StatusServiceUnavailable, code: "server_busy", detail: "The server is busy; retry shortly.", retry: 2 * time.Second,
		}), http.MethodGet, "/api/v0/things", "", http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(tt.h, httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body)))

			if ct := rec.Result().Header.Get("Content-Type"); rec.Code != tt.status || ct != ContentTypeProblem {
				t.Fatalf("response = %d %s %s, want %d problem+json", rec.Code, ct, rec.Body, tt.status)
			}
			contract.CheckSchema(t, "Problem", rec.Body.Bytes())
		})
	}
}
