package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// StreamPolicy is how a stream route reads its body and sends its answer
// (M7 design 4.3; M7/P1 design 3.4).
type StreamPolicy struct {
	// MaxBytes bounds the request body; 0 is server.max_body_bytes.
	MaxBytes int64
	// MinRate is the slowest average rate, in bytes a second, at which the
	// body may arrive and an answer announced with Sending may leave.
	MinRate int64
	// Bucket, when set, is the route's rate-limit bucket in place of the
	// platform's anonymous and authenticated ones, by credential when the
	// request has one and by client IP when not; BucketName names it in
	// the logs.
	Bucket     Limiter
	BucketName string
}

// streamChunk is how many body bytes move the read deadline on once, at
// most: see streamStep.
const streamChunk = 64 << 10

// maxWait caps the time a stream's bytes may take at its rate, so that the
// deadlines stay far from overflowing whatever the byte count.
const maxWait = 100 * 365 * 24 * time.Hour

var errNotStream = errors.New("httpserver: outside API.Stream")

// ErrShuttingDown is Sending's error once the server is shutting down, a
// download not starting then, and a stream's body read's when shutdown cuts
// it off.
var ErrShuttingDown = errors.New("httpserver: the server is shutting down")

// Stream wraps the handler of a route that reads or writes its bytes as
// they come, such as a file upload or download, in the per-route
// middlewares that suit it:
//
//	request meta → within RequestTimeout: failure gate and authentication
//	→ the route's bucket → the body limit and its rate → h
//
// Its opening, up to the handler, is bounded as any request is. The handler
// has no request deadline: the connection's deadlines bound it instead,
// moving with the bytes. The read deadline starts at read_timeout from the
// handler's start and moves on with every 64 KiB of body (fewer at a low
// rate: streamStep) by the time they may take at MinRate, so a body whose
// average rate falls below it is cut off, and one that never comes is cut
// off at read_timeout; the write deadline follows it at write_timeout −
// read_timeout, the time to finish and answer once the body is in. A
// request without a body has no read deadline: net/http watches the
// connection from the start. Once Sending announces an answer, its write
// deadline starts at read_timeout from then and moves on with every step
// of bytes written by the time they may take at MinRate, as the read
// deadline does: an answer leaves at the rate, or is cut off, a client
// that stops reading at read_timeout past what the connection buffered.
// The handler's steps that are not the stream run under Bounded.
//
// When the server starts shutting down, a stream still moving its bytes, a
// body not yet read to its end or an answer announced with Sending, is cut
// off: the deadlines pass at once and h's context is cancelled. A step
// between the two, such as writing what the body brought, finishes as any
// request does, within shutdown_timeout; a Sending then fails.
//
// An answer before the handler, a 401 or a 429, to a request with a body
// closes the connection after it: the body is unread, and net/http would
// read up to 256 KiB more of it first, or wait for a client that holds it
// back until it hears. The handler decides for itself.
//
// A writer that cannot set deadlines is a wiring fault: Stream logs it and
// answers 500, after any 401 or 429.
//
// Stream panics on a policy without a rate, with a bucket but no name, or
// on an API without the read and write timeouts it needs: a wiring fault.
func (a *API) Stream(h http.Handler, p StreamPolicy) http.Handler {
	switch {
	case p.MinRate <= 0:
		panic(fmt.Sprintf("httpserver: StreamPolicy.MinRate %d must be positive", p.MinRate))
	case p.MaxBytes < 0:
		panic(fmt.Sprintf("httpserver: StreamPolicy.MaxBytes %d is negative", p.MaxBytes))
	case p.Bucket != nil && p.BucketName == "":
		panic("httpserver: StreamPolicy.Bucket has no name")
	case a.bodyReadTimeout <= 0 || a.writeTimeout <= a.bodyReadTimeout+a.requestTimeout:
		panic("httpserver: API.Stream needs BodyReadTimeout and a WriteTimeout above it and RequestTimeout")
	}
	return a.requestMeta(closingEarly(a.opening(a.authenticate(a.routeLimit(p, a.opened(a.streaming(p, h)))))))
}

// closingEarly sets Connection: close on the answers to a request with a
// body until the handler is reached, which streaming takes back.
func closingEarly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 {
			w.Header().Set("Connection", "close")
		}
		next.ServeHTTP(w, r)
	})
}

// streamStep is how many body bytes move the deadlines on once: 64 KiB, or
// the bytes half the read timeout takes at rate when fewer, so that a body
// arriving at the rate always has half the read timeout to spare.
func streamStep(rate int64, readTimeout time.Duration) int64 {
	if atRate(streamChunk, rate) <= readTimeout/2 {
		return streamChunk
	}
	return max(1, int64(float64(rate)*(readTimeout/2).Seconds()))
}

// routeLimit takes one unit of the route's bucket, or of the platform's
// when the route has none.
func (a *API) routeLimit(p StreamPolicy, next http.Handler) http.Handler {
	if p.Bucket == nil {
		return a.rateLimit(next)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := RequestMetaFrom(r.Context()).IPKey
		if credential, ok := r.Context().Value(credentialKey{}).(string); ok {
			key = credential
		}
		if retry, ok := p.Bucket.Allow(key); !ok {
			a.tooManyRequests(w, r, p.BucketName, retry)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type streamKey struct{}

// streaming sets the connection's deadlines, wraps the body and the
// answer, and runs h with the stream in its context.
func (a *API) streaming(p StreamPolicy, h http.Handler) http.Handler {
	limit := p.MaxBytes
	if limit == 0 {
		limit = a.maxBodyBytes
	}
	step := streamStep(p.MinRate, a.bodyReadTimeout)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := &stream{
			rc:             http.NewResponseController(w),
			start:          time.Now(),
			readTimeout:    a.bodyReadTimeout,
			writeSlack:     a.writeTimeout - a.bodyReadTimeout,
			requestTimeout: a.requestTimeout,
			minRate:        p.MinRate,
			step:           step,
			drained:        r.ContentLength == 0,
		}
		if err := s.arm(0); err != nil {
			a.logger.ErrorContext(r.Context(), "cannot set the deadlines of a stream",
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
		w.Header().Del("Connection")
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		if stopping, ok := ctx.Value(stoppingKey{}).(context.Context); ok {
			defer context.AfterFunc(stopping, func() {
				if s.stop() {
					cancel()
				}
			})()
		}
		defer s.finish()
		if !s.drained {
			r.Body = &rateBody{s: s, body: http.MaxBytesReader(w, r.Body, limit)}
		}
		h.ServeHTTP(&streamWriter{ResponseWriter: w, s: s}, r.WithContext(context.WithValue(ctx, streamKey{}, s)))
	})
}

// Bounded bounds a stream handler's step that is not the stream, such as a
// check before reading the body or the write after it, by
// server.request_timeout from now. Outside API.Stream it panics: a wiring
// fault.
func Bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	s, ok := ctx.Value(streamKey{}).(*stream)
	if !ok {
		panic(errNotStream)
	}
	return context.WithTimeout(ctx, s.requestTimeout)
}

// Sending announces the answer a stream handler is about to write, once:
// its write deadline is read_timeout from now, and moves on with its bytes
// at the route's MinRate (streamWriter). From then on the stream moves
// bytes until the handler returns: a shutdown cuts it off. It fails
// outside API.Stream, and with ErrShuttingDown once the server is shutting
// down.
func Sending(r *http.Request) error {
	s, ok := r.Context().Value(streamKey{}).(*stream)
	if !ok {
		return errNotStream
	}
	return s.sending()
}

// stream is one stream request's deadlines.
type stream struct {
	rc                                      *http.ResponseController
	start                                   time.Time
	readTimeout, writeSlack, requestTimeout time.Duration
	minRate, step                           int64

	mu       sync.Mutex
	read     int64     // body bytes read
	armed    int64     // read when the deadlines last moved
	drained  bool      // the body was read to its end, or there is none
	sent     bool      // Sending was called
	sentAt   time.Time // when
	stopped  bool      // the server is shutting down
	finished bool      // the handler returned
}

// arm moves the read deadline, while there is body left, to read_timeout
// after the start plus the time n body bytes take at the rate, and the
// write deadline after where it would be.
func (s *stream) arm(n int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrShuttingDown
	}
	s.armed = n
	read := s.start.Add(s.readTimeout + atRate(n, s.minRate))
	if !s.drained {
		if err := s.rc.SetReadDeadline(read); err != nil {
			return err
		}
	}
	return s.rc.SetWriteDeadline(read.Add(s.writeSlack))
}

func (s *stream) sending() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrShuttingDown
	}
	s.sent, s.sentAt = true, time.Now()
	return s.rc.SetWriteDeadline(s.sentAt.Add(s.readTimeout))
}

// isSending reports whether Sending announced the answer.
func (s *stream) isSending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent
}

// armSent moves the write deadline of the answer Sending announced to
// read_timeout after Sending plus the time n bytes of it take at the rate.
func (s *stream) armSent(n int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrShuttingDown
	}
	return s.rc.SetWriteDeadline(s.sentAt.Add(s.readTimeout + atRate(n, s.minRate)))
}

// stop marks the server shutting down and, while the stream moves bytes,
// makes both deadlines pass at once and keeps them there; it tells whether
// it did.
func (s *stream) stop() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.finished || (s.drained && !s.sent) {
		return false
	}
	now := time.Now()
	_ = s.rc.SetReadDeadline(now)
	_ = s.rc.SetWriteDeadline(now)
	return true
}

// finish marks the handler returned: a shutdown beginning now leaves the
// deadlines to net/http's last flush of the answer.
func (s *stream) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = true
}

// atRate is the time n bytes take at rate bytes a second, at most maxWait.
func atRate(n, rate int64) time.Duration {
	if n <= 0 {
		return 0
	}
	secs := n / rate
	if secs >= int64(maxWait/time.Second) {
		return maxWait
	}
	frac := time.Duration(float64(n%rate) / float64(rate) * float64(time.Second))
	return time.Duration(secs)*time.Second + frac
}

// rateBody is a stream's request body: every step of bytes read moves the
// deadlines on. A read that ends the body does not: once the body is read,
// net/http watches the connection with no read deadline, and one set then
// would end the request's context when it passed. For that reason a
// request without a body has none from the start.
type rateBody struct {
	s    *stream
	body io.ReadCloser
}

func (b *rateBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	switch {
	case errors.Is(err, io.EOF):
		b.s.mu.Lock()
		b.s.drained = true
		b.s.mu.Unlock()
	case err != nil:
		b.s.mu.Lock()
		stopped := b.s.stopped
		b.s.mu.Unlock()
		if stopped {
			return n, fmt.Errorf("%w: %w", ErrShuttingDown, err)
		}
	}
	if n > 0 && err == nil {
		b.s.mu.Lock()
		b.s.read += int64(n)
		due := b.s.read-b.s.armed >= b.s.step
		read := b.s.read
		b.s.mu.Unlock()
		if due {
			if aerr := b.s.arm(read); aerr != nil {
				return n, aerr
			}
		}
	}
	return n, err
}

func (b *rateBody) Close() error { return b.body.Close() }

// streamWriter is a stream's answer. Once Sending announced it, a write
// goes out a step of bytes at a time, each moving the write deadline on by
// the bytes before it (armSent): a step takes at most half the read
// timeout at the rate, so an answer leaving at the rate always has half
// of it to spare, and a client reading below the rate is cut off. Before,
// a write is the writer's.
type streamWriter struct {
	http.ResponseWriter
	s       *stream
	written int64 // bytes written since Sending
	armed   int64 // written when the deadline last moved
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if !w.s.isSending() {
		return w.ResponseWriter.Write(p)
	}
	n := 0
	for len(p) > 0 {
		if w.written > w.armed {
			if err := w.s.armSent(w.written); err != nil {
				return n, err
			}
			w.armed = w.written
		}
		k, err := w.ResponseWriter.Write(p[:min(int64(len(p)), w.s.step)])
		n += k
		w.written += int64(k)
		p = p[k:]
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *streamWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
