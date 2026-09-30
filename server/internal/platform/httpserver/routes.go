package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

// readinessTimeout bounds one /readyz evaluation, so a hung dependency
// answers 503 instead of hanging the probe.
const readinessTimeout = 2 * time.Second

// Check is one readiness condition, such as "the database answers".
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Router is nervewiki's root router: an http.ServeMux that remembers every
// pattern registered on it, so a whole-program test can compare the API
// routes with the contract (P4). It satisfies the ServeMux
// interface of the generated code (HandleFunc and ServeHTTP).
type Router struct {
	mux      *http.ServeMux
	patterns []string
}

// NewRouter returns a router holding the platform routes:
//
//   - GET /healthz: liveness; never touches a dependency.
//   - GET /readyz: runs checks in order; the first failure answers 503.
//   - /api/: every API path no module handles answers 404 problem+json,
//     never the web UI.
//
// Modules and the web UI register their own routes on it.
func NewRouter(logger *slog.Logger, checks ...Check) *Router {
	r := &Router{mux: http.NewServeMux()}
	r.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeStatusOK(w)
	})
	r.HandleFunc("GET /readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), readinessTimeout)
		defer cancel()
		for _, c := range checks {
			if err := c.Run(ctx); err != nil {
				logger.WarnContext(ctx, "readiness check failed",
					slog.String("request_id", RequestID(ctx)),
					slog.String("check", c.Name),
					slog.Any("error", err),
				)
				WriteProblem(w, Problem{
					Status: http.StatusServiceUnavailable,
					Code:   CodeNotReady,
					Title:  http.StatusText(http.StatusServiceUnavailable),
					Detail: c.Name + " is not ready",
				})
				return
			}
		}
		writeStatusOK(w)
	})
	r.HandleFunc(apiPrefix, func(w http.ResponseWriter, req *http.Request) {
		WriteProblem(w, Problem{
			Status: http.StatusNotFound,
			Code:   CodeNotFound,
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "no API endpoint for " + req.Method + " " + req.URL.Path,
		})
	})
	return r
}

// HandleFunc registers handler for pattern and records the pattern.
func (r *Router) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
	r.mux.HandleFunc(pattern, handler)
}

// Handle registers h for pattern and records the pattern.
func (r *Router) Handle(pattern string, h http.Handler) {
	r.patterns = append(r.patterns, pattern)
	r.mux.Handle(pattern, h)
}

// ServeHTTP dispatches the request to the handler of the matching pattern.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// Patterns returns every pattern registered so far, in registration order.
func (r *Router) Patterns() []string {
	return slices.Clone(r.patterns)
}

func writeStatusOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
