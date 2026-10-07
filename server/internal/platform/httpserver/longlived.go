package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

type stoppingKey struct{}

// withStopping puts stopping, which is cancelled when shutdown begins, in ctx.
func withStopping(ctx, stopping context.Context) context.Context {
	return context.WithValue(ctx, stoppingKey{}, stopping)
}

// LongLived wraps a handler that holds its response open, such as an event
// stream (v0.1 design 3.11), and does two things for it:
//
//   - It lifts the connection's write deadline, past which every write
//     fails.
//   - When the Server starts shutting down, it cancels h's context, so h
//     ends its response. Shutdown waits for every connection to go idle,
//     which a long-lived response never does on its own.
//
// The read deadline stays: it still bounds reading the request body, so a
// client cannot trickle one in forever. Once the body has been read, net/http
// lifts the read deadline itself while it watches the connection for the
// client going away. It starts watching only then: h must read the body to
// the end to learn, through its context, that the client has gone.
//
// Other requests are left alone: they finish normally during shutdown.
// Without a Server, as under httptest, only the write deadline is lifted. A
// writer that cannot lift it is a wiring fault: LongLived logs it and answers
// 500.
func LongLived(logger *slog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
			logger.ErrorContext(r.Context(), "cannot lift the write deadline of a long-lived response",
				slog.String("request_id", RequestID(r.Context())),
				slog.String("path", loggedPath(r)),
				slog.Any("error", err),
			)
			WriteProblem(w, Problem{
				Status: http.StatusInternalServerError,
				Code:   CodeInternal,
				Title:  http.StatusText(http.StatusInternalServerError),
			})
			return
		}
		ctx := r.Context()
		if stopping, ok := ctx.Value(stoppingKey{}).(context.Context); ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			defer context.AfterFunc(stopping, cancel)()
		}
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LongLived wraps the handler of a long-lived route, such as the event
// stream (M5 design 4.10), in the per-route middlewares that suit it:
//
//	request meta → within RequestTimeout: failure gate and authentication
//	→ rate limit → the package's LongLived
//
// Its opening, up to the handler, is bounded as any request is; the handler
// has no deadline, and the request reads no body. Lifting the write
// deadline comes last, so that a 401 or a 429 is answered first, also
// through a ResponseRecorder, on which lifting it fails. h's context carries
// what Reauthenticate needs.
func (a *API) LongLived(h http.Handler) http.Handler {
	return a.requestMeta(a.opening(a.authenticate(a.rateLimit(a.opened(a.reauthenticator(LongLived(a.logger, h)))))))
}

type openingKey struct{}

// opening bounds a long-lived request's authentication by RequestTimeout:
// a database that does not answer holds it no longer than any request.
// opened lifts the bound.
func (a *API) opening(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), a.requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, openingKey{}, r.Context())))
	})
}

// opened gives the handler what the authentication put in the context,
// without the opening's deadline: its context ends with the request's own.
func (a *API) opened(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request, ok := r.Context().Value(openingKey{}).(context.Context)
		if !ok {
			request = r.Context()
		}
		ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
		defer cancel()
		defer context.AfterFunc(request, cancel)()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type reauthKey struct{}

// reauthenticator puts in the request's context the authentication of its
// bearer token again, for Reauthenticate.
func (a *API) reauthenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := bearerToken(r.Header.Get("Authorization"))
		again := func(ctx context.Context) error {
			_, _, err := a.authenticator.Authenticate(ctx, token)
			return err
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), reauthKey{}, again)))
	})
}

var errNotLongLived = errors.New("httpserver: Reauthenticate outside API.LongLived")

// Reauthenticate authenticates the request's bearer token again, as
// API.LongLived did when the request came: nil while the credential is
// valid, and the Authenticator's error once it is not, a 401 for a
// credential revoked, signed out or expired since, or its account
// deactivated. It skips the failure gate, which the credential passed
// once, and takes no unit of the rate limit. A long-lived handler calls it
// at each heartbeat (M5 design 4.10). Outside API.LongLived it is an error.
func Reauthenticate(ctx context.Context) error {
	again, ok := ctx.Value(reauthKey{}).(func(context.Context) error)
	if !ok {
		return errNotLongLived
	}
	return again(ctx)
}
