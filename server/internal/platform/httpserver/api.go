package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// APIConfig is what the per-route middlewares need.
type APIConfig struct {
	Logger         *slog.Logger
	MaxBodyBytes   int64         // server.max_body_bytes
	RequestTimeout time.Duration // server.request_timeout
}

// API is what the platform hands to every module's HTTP adapter: the error
// mapping for the generated code, and the per-route middlewares.
type API struct {
	Errors         APIErrors
	maxBodyBytes   int64
	requestTimeout time.Duration
}

// NewAPI returns the API value for cfg. It needs a logger, and a body limit
// and a request timeout above zero.
func NewAPI(cfg APIConfig) (*API, error) {
	var errs []error
	if cfg.Logger == nil {
		errs = append(errs, errors.New("no Logger"))
	}
	if cfg.MaxBodyBytes <= 0 {
		errs = append(errs, errors.New("MaxBodyBytes must be positive"))
	}
	if cfg.RequestTimeout <= 0 {
		errs = append(errs, errors.New("RequestTimeout must be positive"))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("httpserver: APIConfig: %w", errors.Join(errs...))
	}
	return &API{
		Errors:         NewAPIErrors(cfg.Logger),
		maxBodyBytes:   cfg.MaxBodyBytes,
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

// Middlewares returns the per-route middlewares for a module's generated
// StdHTTPServerOptions.Middlewares; bodies is the module's generated
// bodyshape table. They run in this order:
//
//	request deadline → body limit → body structure
//
// The generated code wraps the last middleware of its list outermost, so
// the list is in reverse. Only API operations get them: a long-lived route
// is registered on the router directly, without a request deadline.
func (a *API) Middlewares(bodies *bodyshape.Table) []func(http.Handler) http.Handler {
	inOrder := []func(http.Handler) http.Handler{
		a.deadline,
		a.bodyLimit,
		bodyshape.Middleware(bodies, a.Errors.BodyError),
	}
	slices.Reverse(inOrder)
	return inOrder
}

// deadline bounds the handler: server.write_timeout only fails the writes
// and never cancels the request's context, so without it a handler's
// database calls could outlive the response.
func (a *API) deadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), a.requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bodyLimit makes reading more than server.max_body_bytes fail with
// *http.MaxBytesError, which APIErrors answers with 413.
func (a *API) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, a.maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}
