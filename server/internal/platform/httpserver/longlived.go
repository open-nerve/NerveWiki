package httpserver

import (
	"context"
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
				slog.String("path", r.URL.Path),
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
