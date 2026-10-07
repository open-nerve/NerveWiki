package httpserver

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
	"uuid"
)

// HeaderRequestID carries the request ID in requests and responses.
const HeaderRequestID = "X-Request-Id"

const maxRequestIDLen = 128

type requestIDKey struct{}

// apiPrefix is the API's path tree. No cache may store a response in it:
// every answer is the caller's own, and some carry tokens. The web UI's files
// are outside it and keep the caching webui sets.
const apiPrefix = "/api/"

// middleware wraps h in the platform chain. The order is fixed, outermost
// first: request ID -> recover -> access log -> security headers.
func middleware(h http.Handler, logger *slog.Logger) http.Handler {
	return withRequestID(withRecover(logger, withAccessLog(logger, withSecurityHeaders(h))))
}

// withSecurityHeaders sets the fixed headers before the handler writes.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setFixedHeaders(w.Header(), r)
		next.ServeHTTP(w, r)
	})
}

// setFixedHeaders sets the headers of every response: no MIME sniffing, no
// referrer beyond this site, and no framing by any page. The CSP goes on the
// HTML pages only, so webui adds it (P5). A response under apiPrefix also
// gets Cache-Control: no-store.
func setFixedHeaders(header http.Header, r *http.Request) {
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin")
	header.Set("X-Frame-Options", "DENY")
	if strings.HasPrefix(r.URL.Path, apiPrefix) {
		header.Set("Cache-Control", "no-store")
	}
}

// RequestID returns the ID the request ID middleware assigned to the request,
// for log lines outside this package; "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// withRequestID keeps the caller's X-Request-Id when it is a safe token and
// otherwise assigns a new UUIDv7. The ID is echoed in the response.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validRequestID(id) {
			id = uuid.NewV7().String()
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// validRequestID accepts 1 to 128 characters from [A-Za-z0-9._:-], which
// keeps caller-supplied IDs safe to log.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, c := range id {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '-', c == '_', c == '.', c == ':':
		default:
			return false
		}
	}
	return true
}

// withRecover turns a panic into a logged 500 problem. Headers the handler set
// are dropped, except the request ID: a Set-Cookie must not leak, and a stale
// Content-Length or Content-Encoding would corrupt the problem body. The
// fixed headers, which the chain set before the handler ran, are set again.
// If the response has already started, the connection is aborted instead so
// the client cannot mistake a truncated body for a complete one.
func withRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// A deliberate abort: let net/http handle it. net/http recognises
			// only this exact value, so errors.Is would claim wrapped ones it
			// then logs as panics.
			if v == http.ErrAbortHandler { //nolint:errorlint // must match net/http's identity check
				panic(v)
			}
			logger.ErrorContext(r.Context(), "panic serving request",
				slog.String("request_id", RequestID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", loggedPath(r)),
				slog.Any("panic", v),
				slog.String("stack", string(debug.Stack())),
			)
			if rec.status != 0 {
				panic(http.ErrAbortHandler)
			}
			header := rec.Header()
			for name := range header {
				if name != HeaderRequestID {
					delete(header, name)
				}
			}
			setFixedHeaders(header, r)
			WriteProblem(rec, Problem{
				Status: http.StatusInternalServerError,
				Code:   CodeInternal,
				Title:  http.StatusText(http.StatusInternalServerError),
			})
		}()
		next.ServeHTTP(rec, r)
	})
}

// loggedPath is r's path as the logs write it, as a path may hold a page's
// text (getTag's tag; v0.1 design 13.1 rule 10): the route the router found
// (a path it redirects to a clean one among them, fix check B3-M1), a
// slug's value and an id's that is a uuid in it, the uuid as uuids are
// written, and any other wildcard's, or one of no value, as the wildcard
// ("{tag}"); a path a route's subtree took, which no route of its own did
// (the API's 404), as the subtree and "..." (fix check B2-M1). The web
// app's paths, that "/" takes, are written as they are. It reads the route
// the router sets on r: once r has been served, or in its handler.
func loggedPath(r *http.Request) string {
	_, pattern, ok := strings.Cut(r.Pattern, " ")
	if !ok {
		pattern = r.Pattern
	}
	switch {
	case pattern == "" || pattern == "/":
		return r.URL.Path
	case strings.HasSuffix(pattern, "/") && r.URL.Path != pattern:
		return pattern + "..."
	case !strings.Contains(pattern, "{"):
		return pattern
	}
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		name, ok := strings.CutPrefix(segment, "{")
		if !ok {
			continue
		}
		name = strings.TrimSuffix(name, "}")
		v := r.PathValue(name)
		if id, err := uuid.Parse(v); err == nil && (name == "id" || strings.HasSuffix(name, "_id")) {
			segments[i] = id.String()
		} else if v != "" && name == "slug" {
			segments[i] = v
		}
	}
	return strings.Join(segments, "/")
}

// withAccessLog logs one line per request: method, path, status, duration
// and request ID. A request whose handler panicked is logged as a 500, the
// answer the recover middleware gives. The health probes log at debug level:
// an orchestrator probes every few seconds.
func withAccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		completed := false
		defer func() {
			status := rec.status
			switch {
			case !completed:
				status = http.StatusInternalServerError
			case status == 0:
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				level = slog.LevelDebug
			}
			logger.LogAttrs(r.Context(), level, "http request",
				slog.String("request_id", RequestID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", loggedPath(r)),
				slog.Int("status", status),
				slog.Duration("duration", time.Since(start)),
			)
		}()
		next.ServeHTTP(rec, r)
		completed = true
	})
}

// statusRecorder remembers the final status code written through it. A
// write or a flush commits the response, with 200 unless a status was set.
type statusRecorder struct {
	http.ResponseWriter
	status int // 0 until the response is committed
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 && code >= http.StatusOK {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.commit()
	return s.ResponseWriter.Write(b)
}

// FlushError is what http.ResponseController calls to flush: the flush sends
// the status and headers, so it commits the response as a write does.
func (s *statusRecorder) FlushError() error {
	s.commit()
	return http.NewResponseController(s.ResponseWriter).Flush()
}

// Flush serves handlers and libraries that assert http.Flusher, such as
// event streams.
func (s *statusRecorder) Flush() {
	_ = s.FlushError()
}

func (s *statusRecorder) commit() {
	if s.status == 0 {
		s.status = http.StatusOK
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
