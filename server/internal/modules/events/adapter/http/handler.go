// Package httpadapter serves the event stream, GET /api/v0/events (M5
// design 4.10). Its one operation holds its response open, so it is
// written by hand, not generated: the contract marks it x-long-lived.
package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/modules/events/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/events/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// Route is the stream's route pattern.
const Route = "GET /api/v0/events"

// Opener opens the caller's stream: app.OpenStream.
type Opener interface {
	Execute(ctx context.Context) (*app.Stream, error)
}

// Config is what the handler needs besides the opener.
type Config struct {
	Errors    httpserver.APIErrors
	Logger    *slog.Logger
	Heartbeat time.Duration // events.heartbeat_interval
	// OpenTimeout bounds the opening, which reads what the caller sees:
	// server.request_timeout, as for any other request.
	OpenTimeout time.Duration
	// Now is the clock's time; After a timer, time.After in the server.
	Now   func() time.Time
	After func(time.Duration) <-chan time.Time
}

// Handler writes a stream: hello, then its events, a heartbeat every
// interval, and a reset frame when the stream is reset, the credential
// expires or fails at a heartbeat (M5 design 4.10).
type Handler struct {
	open Opener
	cfg  Config
}

// New returns the handler; API.LongLived wraps it.
func New(open Opener, cfg Config) *Handler {
	return &Handler{open: open, cfg: cfg}
}

// ServeHTTP opens the caller's stream, a problem when it cannot, and
// writes it until the client goes, the server stops, or it ends with a
// reset:
//
//   - each event the stream holds, as a frame;
//   - the stream reset: the events it still holds, then a reset frame
//     with its reason, so the frames keep the order of the events;
//   - the credential's expiry: reset expired;
//   - each heartbeat: the credential authenticated again, reset
//     unauthenticated when it fails with a 401, else a comment line. A
//     fault ends the stream without a frame: the client reconnects.
//
// A frame that cannot be written within a heartbeat ends it: the client is
// gone or has stopped reading.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	opening, cancel := context.WithTimeout(ctx, h.cfg.OpenTimeout)
	s, err := h.open.Execute(opening)
	cancel()
	if err != nil {
		h.cfg.Errors.Write(w, r.WithContext(opening), err)
		return
	}
	defer s.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	out := writer{w: w, rc: http.NewResponseController(w), wait: h.cfg.Heartbeat}
	if !out.write(domain.HelloFrame(h.cfg.Heartbeat)) {
		return
	}
	var expired <-chan time.Time
	if at, ok := httpserver.CredentialExpiry(ctx); ok {
		expired = h.cfg.After(at.Sub(h.cfg.Now()))
	}
	beat := h.cfg.After(h.cfg.Heartbeat)
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-s.Events():
			if !h.event(ctx, out, e) {
				return
			}
		case <-s.Done():
			if h.drain(ctx, s, out) {
				out.write(domain.ResetFrame(s.Reason()))
			}
			return
		case <-expired:
			out.write(domain.ResetFrame(domain.ResetExpired))
			return
		case <-beat:
			if err := h.reauthenticate(ctx); err != nil {
				switch {
				case ctx.Err() != nil:
					// The client went, or the server stops: no fault.
				case unauthenticated(err):
					out.write(domain.ResetFrame(domain.ResetUnauthenticated))
				default:
					h.cfg.Logger.ErrorContext(ctx, "event stream ended: the credential could not be checked",
						slog.String("request_id", httpserver.RequestID(ctx)), slog.Any("error", err))
				}
				return
			}
			if !out.write(domain.HeartbeatFrame()) {
				return
			}
			beat = h.cfg.After(h.cfg.Heartbeat)
		}
	}
}

// event writes e as a frame, and reports whether the stream goes on: an
// event that has no frame is logged and left out.
func (h *Handler) event(ctx context.Context, out writer, e domain.Event) bool {
	f, err := domain.EventFrame(e)
	if err != nil {
		h.cfg.Logger.WarnContext(ctx, "event not written", slog.Any("error", err))
		return true
	}
	return out.write(f)
}

// drain writes the events a reset stream holds, routed before its reset,
// and reports whether it could.
func (h *Handler) drain(ctx context.Context, s *app.Stream, out writer) bool {
	for {
		select {
		case e := <-s.Events():
			if !h.event(ctx, out, e) {
				return false
			}
		default:
			return true
		}
	}
}

// reauthenticate authenticates the stream's credential again, within a
// heartbeat.
func (h *Handler) reauthenticate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.Heartbeat)
	defer cancel()
	return httpserver.Reauthenticate(ctx)
}

// unauthenticated reports whether err is a credential's 401.
func unauthenticated(err error) bool {
	var pe httpserver.ProblemError
	return errors.As(err, &pe) && pe.ProblemStatus() == http.StatusUnauthorized
}

// writer writes frames and flushes each, each within wait: LongLived lifts
// the server's write deadline, and a client that stops reading would hold
// a write, and the stream, forever.
type writer struct {
	w    http.ResponseWriter
	rc   *http.ResponseController
	wait time.Duration
}

// write writes f and flushes it, and reports whether it could.
func (o writer) write(f []byte) bool {
	if o.rc.SetWriteDeadline(time.Now().Add(o.wait)) != nil {
		return false
	}
	if _, err := o.w.Write(f); err != nil {
		return false
	}
	return o.rc.Flush() == nil
}
