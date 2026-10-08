package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
)

const uploadRoute = "POST /api/v0/uploads"

func TestNewAPIChecksTheWriteTimeout(t *testing.T) {
	for _, tt := range []struct {
		write, read time.Duration
		ok          bool
	}{
		{0, 3 * time.Second, true},
		{10 * time.Second, 3 * time.Second, true},
		{5 * time.Second, 3 * time.Second, false}, // RequestTimeout is 2s: not above them together
		{4 * time.Second, 3 * time.Second, false},
		{10 * time.Second, 0, false},
	} {
		cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
		cfg.WriteTimeout, cfg.BodyReadTimeout = tt.write, tt.read
		if _, err := NewAPI(cfg); (err == nil) != tt.ok {
			t.Errorf("NewAPI(write %v, read %v) error = %v, want ok %v", tt.write, tt.read, err, tt.ok)
		}
	}
}

func TestStreamPanicsOnAWiringFault(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	withoutWrite := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	withoutWrite.WriteTimeout = 0
	for name, build := range map[string]func(){
		"no rate":                   func() { newTestAPI(t).Stream(h, StreamPolicy{}) },
		"a negative body limit":     func() { newTestAPI(t).Stream(h, StreamPolicy{MinRate: 1, MaxBytes: -1}) },
		"a bucket without its name": func() { newTestAPI(t).Stream(h, StreamPolicy{MinRate: 1, Bucket: newFakeLimiter(1)}) },
		"no write timeout":          func() { buildAPI(t, withoutWrite).Stream(h, StreamPolicy{MinRate: 1}) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Stream with %s did not panic", name)
				}
			}()
			build()
		}()
	}
}

// recordingBody records whether the handler side read it.
type recordingBody struct {
	io.Reader
	read bool
}

func (b *recordingBody) Read(p []byte) (int, error) {
	b.read = true
	return b.Reader.Read(p)
}

func (*recordingBody) Close() error { return nil }

// A stream route answers 401 and 429 before its handler and before reading
// the body: no token, a failing one, an empty platform bucket, an empty
// route bucket. A route bucket takes the place of the platform's: by
// credential, or by IP on a public route.
func TestAStreamRouteAuthenticatesAndLimitsBeforeItsHandler(t *testing.T) {
	for _, tt := range []struct {
		name, route, token string
		platformEmpty      bool
		routeBucket        bool
		routeEmpty         bool
		status             int
		routeKey           string // the key the route bucket took a unit of
	}{
		{"no token", uploadRoute, "", false, false, false, http.StatusUnauthorized, ""},
		{"a failing token", uploadRoute, "bad", false, false, false, http.StatusUnauthorized, ""},
		{"an empty platform bucket", uploadRoute, "tok", true, false, false, http.StatusTooManyRequests, ""},
		{"an empty route bucket", uploadRoute, "tok", false, true, true, http.StatusTooManyRequests, ""},
		{"the route bucket by credential", uploadRoute, "tok", true, true, false, http.StatusNoContent, "session:tok"},
		{"the route bucket by IP on a public route", openRoute, "", true, true, false, http.StatusNoContent, "203.0.113.7"},
		{"the platform bucket", uploadRoute, "tok", false, false, false, http.StatusNoContent, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
			platform := newFakeLimiter(100)
			if tt.platformEmpty {
				platform = newFakeLimiter(0)
			}
			cfg.Authenticated, cfg.Anonymous = platform, platform
			api := buildAPI(t, cfg)
			policy := StreamPolicy{MinRate: 1 << 20}
			routeBucket := newFakeLimiter(100)
			if tt.routeEmpty {
				routeBucket = newFakeLimiter(0)
			}
			if tt.routeBucket {
				policy.Bucket, policy.BucketName = routeBucket, "uploads"
			}
			reached := false
			router := NewRouter(slog.New(slog.DiscardHandler))
			h := api.Stream(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusNoContent)
			}), policy)
			router.Handle(uploadRoute, h)
			router.Handle(openRoute, h)

			body := &recordingBody{Reader: strings.NewReader("bytes")}
			r := httptest.NewRequest(http.MethodPost, strings.TrimPrefix(tt.route, "POST "), body)
			r.RemoteAddr = "203.0.113.7:5555"
			if tt.token != "" {
				r.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
			router.ServeHTTP(w, r)

			if w.Code != tt.status || reached != (tt.status == http.StatusNoContent) {
				t.Fatalf("status %d, handler reached %v; want %d", w.Code, reached, tt.status)
			}
			if !reached && body.read {
				t.Error("the body was read before the request was turned away")
			}
			if tt.routeKey != "" {
				if routeBucket.taken[0] != tt.routeKey || len(platform.taken) != 0 {
					t.Errorf("route bucket took %q, platform's %q; want the route's to take %q alone",
						routeBucket.taken, platform.taken, tt.routeKey)
				}
			}
			if tt.status == http.StatusNoContent && !tt.routeBucket && len(platform.taken) != 1 {
				t.Errorf("the platform bucket took %q, want one unit", platform.taken)
			}
		})
	}
}

// deadlineWriter records the deadlines set on it, as a connection would
// take them.
type deadlineWriter struct {
	*httptest.ResponseRecorder
	mu            sync.Mutex
	reads, writes []time.Time
}

func (d *deadlineWriter) SetReadDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reads = append(d.reads, t)
	return nil
}

func (d *deadlineWriter) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writes = append(d.writes, t)
	return nil
}

// chunkedBody hands its content out size bytes at a time, and the last
// ones together with io.EOF.
type chunkedBody struct {
	content []byte
	size    int
}

func (b *chunkedBody) Read(p []byte) (int, error) {
	n := copy(p[:min(len(p), b.size)], b.content)
	b.content = b.content[n:]
	if len(b.content) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (*chunkedBody) Close() error { return nil }

// The read deadline starts at read_timeout and moves on by the time each
// 64 KiB take at the rate, a rate at which they take far less than the
// read timeout, the write deadline write_timeout − read_timeout
// after it; the read that ends the body moves neither. The handler has no
// request deadline; Bounded gives a step request_timeout; Sending moves the
// write deadline for its bytes.
func TestAStreamMovesItsDeadlinesWithItsBytes(t *testing.T) {
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	cfg.BodyReadTimeout, cfg.WriteTimeout, cfg.RequestTimeout = 3*time.Second, 10*time.Second, 2*time.Second
	api := buildAPI(t, cfg)
	const rate = 1 << 16 // a 64 KiB chunk a second
	var (
		ctxDeadline     bool
		bounded         time.Duration
		readErr         error
		sendingAt       time.Time
		writesAtSending int
	)
	w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	h := api.Stream(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, ctxDeadline = r.Context().Deadline()
		ctx, cancel := Bounded(r.Context())
		defer cancel()
		deadline, _ := ctx.Deadline()
		bounded = time.Until(deadline)
		buf := make([]byte, 16<<10) // reads of 16 KiB: the chunks pass at 64 KiB exactly
		for readErr == nil {
			_, readErr = r.Body.Read(buf)
		}
		if errors.Is(readErr, io.EOF) {
			readErr = nil
		}
		w.mu.Lock()
		writesAtSending = len(w.writes)
		w.mu.Unlock()
		sendingAt = time.Now()
		if err := Sending(r, 3*rate); err != nil {
			t.Error(err)
		}
		rw.WriteHeader(http.StatusOK)
	}), StreamPolicy{MinRate: rate, MaxBytes: 1 << 20})

	// 256 KiB in 16 KiB reads, the last with io.EOF: three chunks pass
	// before it, and the fourth, which it ends, moves nothing.
	r := post("/api/v0/uploads", "")
	r.Body = &chunkedBody{content: bytes.Repeat([]byte("x"), 256<<10), size: 16 << 10}
	start := time.Now()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK || readErr != nil {
		t.Fatalf("status %d, read error %v", w.Code, readErr)
	}
	if ctxDeadline || bounded <= time.Second || bounded > 2*time.Second {
		t.Errorf("handler's context has a deadline %v; Bounded leaves %v; want none and the request timeout", ctxDeadline, bounded)
	}
	if len(w.reads) != 4 {
		t.Fatalf("read deadlines %v, want the start and three chunks", w.reads)
	}
	if first := w.reads[0].Sub(start); first < 3*time.Second-time.Second || first > 3*time.Second+time.Second {
		t.Errorf("the first read deadline is %v after the start, want read_timeout", first)
	}
	for i := 1; i < len(w.reads); i++ {
		if step := w.reads[i].Sub(w.reads[i-1]); step != time.Second {
			t.Errorf("read deadline %d moved %v, want the second a 64 KiB chunk takes", i, step)
		}
	}
	for i := 0; i < writesAtSending; i++ {
		if gap := w.writes[i].Sub(w.reads[i]); gap != 7*time.Second {
			t.Errorf("write deadline %d is %v after its read deadline, want write_timeout - read_timeout", i, gap)
		}
	}
	if len(w.writes) != writesAtSending+1 {
		t.Fatalf("write deadlines %v, want one more from Sending", w.writes)
	}
	if sent := w.writes[len(w.writes)-1].Sub(sendingAt); sent < 6*time.Second-time.Second || sent > 6*time.Second+time.Second {
		t.Errorf("Sending moved the write deadline %v ahead, want read_timeout and three seconds", sent)
	}
}

func TestBoundedAndSendingNeedAStream(t *testing.T) {
	if err := Sending(post("/api/v0/uploads", ""), 1); !errors.Is(err, errNotStream) {
		t.Errorf("Sending outside a stream = %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Error("Bounded outside a stream did not panic")
		}
	}()
	Bounded(context.Background())
}

// httptest's recorder cannot set deadlines: Stream answers 500 and logs the
// fault instead of running the handler without them.
func TestAStreamRefusesAWriterWithoutDeadlines(t *testing.T) {
	logger, logs := captureLogs(t)
	api := buildAPI(t, testAPIConfig(&fakeAuth{}, logger))
	ran := false
	h := api.Stream(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true }), StreamPolicy{MinRate: 1})

	rec := serve(h, post("/api/v0/uploads", "body"))

	if ran || rec.Code != http.StatusInternalServerError {
		t.Errorf("handler ran = %t, status %d; want not run and 500", ran, rec.Code)
	}
	if entry := findLog(logs(), "cannot set the deadlines of a stream"); entry == nil || entry["level"] != "ERROR" {
		t.Errorf("log = %v", entry)
	}
}

func TestATooLargeStreamBodyFailsItsRead(t *testing.T) {
	api := newTestAPI(t)
	var readErr error
	h := api.Stream(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}), StreamPolicy{MinRate: 1, MaxBytes: 10})
	h.ServeHTTP(&deadlineWriter{ResponseRecorder: httptest.NewRecorder()}, post("/api/v0/uploads", "eleven char"))
	if _, ok := errors.AsType[*http.MaxBytesError](readErr); !ok {
		t.Errorf("read = %v, want *http.MaxBytesError", readErr)
	}
}

// A step is 64 KiB, or the bytes half the read timeout takes at the rate
// when fewer.
func TestTheStepOfAStream(t *testing.T) {
	for _, tt := range []struct {
		rate        int64
		readTimeout time.Duration
		want        int64
	}{
		{64 << 10, 30 * time.Second, 64 << 10},
		{4 << 10, 32 * time.Second, 64 << 10},
		{4 << 10, 30 * time.Second, 60 << 10},
		{1 << 10, 30 * time.Second, 15 << 10},
		{256 << 10, 300 * time.Millisecond, 39321},
		{1, time.Millisecond, 1},
		{math.MaxInt64, time.Millisecond, 64 << 10},
	} {
		if got := streamStep(tt.rate, tt.readTimeout); got != tt.want {
			t.Errorf("streamStep(%d, %v) = %d, want %d", tt.rate, tt.readTimeout, got, tt.want)
		}
	}
}

// When shutdown begins, a stream moving its bytes, a body not read to its
// end or an answer announced with Sending, is cut off: both deadlines pass
// and its context is cancelled. One between the two is left to finish, but
// can no longer announce an answer; nor is one whose handler has returned
// cut off. A request without a body never has a read deadline.
func TestAShutdownCutsAStreamOnlyWhileItMovesBytes(t *testing.T) {
	for _, tt := range []struct {
		name             string
		body, read, send bool
		cut              bool
	}{
		{"before its body", true, false, false, true},
		{"after its body", true, true, false, false},
		{"without a body", false, false, false, false},
		{"sending its answer", false, false, true, true},
		{"sending after its body", true, true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stopping, shutdown := context.WithCancel(context.Background())
			defer shutdown()
			var (
				ctxErr, sendErr error
				reads           int
				cutAt           []time.Time // deadlines set once shutdown began
			)
			w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
			h := newTestAPI(t).Stream(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				if tt.read {
					_, _ = io.ReadAll(r.Body)
				}
				if tt.send {
					if err := Sending(r, 10); err != nil {
						t.Error(err)
					}
				}
				w.mu.Lock()
				reads = len(w.reads)
				writes := len(w.writes)
				w.mu.Unlock()
				shutdown()
				s := r.Context().Value(streamKey{}).(*stream)
				for stopped, give := false, time.Now().Add(2*time.Second); !stopped; {
					if time.Now().After(give) {
						t.Error("the stream did not learn of the shutdown in 2s")
						return
					}
					time.Sleep(time.Millisecond)
					s.mu.Lock()
					stopped = s.stopped
					s.mu.Unlock()
				}
				select {
				case <-r.Context().Done():
				case <-time.After(200 * time.Millisecond):
				}
				ctxErr, sendErr = r.Context().Err(), Sending(r, 10)
				w.mu.Lock()
				cutAt = append(slices.Clone(w.reads[reads:]), w.writes[writes:]...)
				w.mu.Unlock()
			}), StreamPolicy{MinRate: 1 << 20})
			r := post("/api/v0/uploads", "some bytes")
			if !tt.body {
				r = httptest.NewRequest(http.MethodGet, "/api/v0/downloads", nil)
				r.Header.Set("Authorization", "Bearer tok")
			}
			h.ServeHTTP(w, r.WithContext(withStopping(r.Context(), stopping)))

			if !tt.body && reads != 0 {
				t.Errorf("a request without a body had %d read deadlines, want none", reads)
			}
			if cut := errors.Is(ctxErr, context.Canceled); cut != tt.cut {
				t.Errorf("the context ended with %v, want it cancelled %v", ctxErr, tt.cut)
			}
			if tt.cut && (len(cutAt) != 2 || cutAt[0].After(time.Now()) || cutAt[1].After(time.Now())) {
				t.Errorf("deadlines set once shutdown began %v, want both past", cutAt)
			}
			if !tt.cut && len(cutAt) != 0 {
				t.Errorf("deadlines set once shutdown began %v, want none", cutAt)
			}
			if !errors.Is(sendErr, errStreamStopped) {
				t.Errorf("Sending once shutdown began = %v, want errStreamStopped", sendErr)
			}
		})
	}
	t.Run("its handler returned", func(t *testing.T) {
		w := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
		s := &stream{rc: http.NewResponseController(w), sent: true}
		s.finish()
		if s.stop() || len(w.reads)+len(w.writes) != 0 {
			t.Errorf("stop cut a stream whose handler returned: deadlines %v, %v", w.reads, w.writes)
		}
	})
}

func TestTheTimeAtARateSaturates(t *testing.T) {
	for _, tt := range []struct {
		n, rate int64
		want    time.Duration
	}{
		{0, 1, 0},
		{-5, 1, 0},
		{1 << 16, 1 << 16, time.Second},
		{1, 4, 250 * time.Millisecond},
		{3 << 16, 1 << 16, 3 * time.Second},
		{math.MaxInt64, 1, maxWait},
		{math.MaxInt64, 1 << 16, maxWait},
	} {
		if got := atRate(tt.n, tt.rate); got != tt.want {
			t.Errorf("atRate(%d, %d) = %v, want %v", tt.n, tt.rate, got, tt.want)
		}
	}
}

// streamServer serves api's stream route h on a server whose read timeout
// is 300ms and write timeout 1s, as server.read_timeout and write_timeout,
// on connections with small socket buffers, so that a slow reader holds
// the writer back.
func streamServer(t *testing.T, h http.Handler) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	router := NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(uploadRoute, h)
	router.Handle("GET /api/v0/downloads", h)
	cfg := config.ServerConfig{
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       300 * time.Millisecond,
		WriteTimeout:      time.Second,
		ShutdownTimeout:   5 * time.Second,
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Addr = ln.Addr().String()
	srv := NewServer(cfg, router, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, smallBuffers{ln}) }()
	return "http://" + ln.Addr().String(), cancel, done
}

// smallBuffers gives each connection a 16 KiB send buffer.
type smallBuffers struct{ net.Listener }

func (l smallBuffers) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetWriteBuffer(16 << 10)
	}
	return c, err
}

// streamTestAPI has the server's timeouts of streamServer: read 300ms,
// request 500ms, write 1s.
func streamTestAPI(t *testing.T) *API {
	t.Helper()
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	cfg.BodyReadTimeout, cfg.RequestTimeout, cfg.WriteTimeout = 300*time.Millisecond, 500*time.Millisecond, time.Second
	return buildAPI(t, cfg)
}

// pacedBody sends its chunks one every pause, then stops for stall when
// stall is set: a client on a slow link, or one that stopped sending.
type pacedBody struct {
	chunks int
	size   int
	pause  time.Duration
	stall  chan struct{}
	sent   int
}

func (b *pacedBody) Read(p []byte) (int, error) {
	if b.sent == b.chunks {
		if b.stall != nil {
			<-b.stall
			return 0, errors.New("stopped")
		}
		return 0, io.EOF
	}
	time.Sleep(b.pause)
	b.sent++
	return copy(p, bytes.Repeat([]byte("x"), min(len(p), b.size))), nil
}

// upload streams body to the route with a token and answers its status and
// body, or the client's error.
func upload(url string, body io.Reader, length int64) (int, string, error) {
	req, err := http.NewRequest(http.MethodPost, url+"/api/v0/uploads", body)
	if err != nil {
		return 0, "", err
	}
	req.ContentLength = length
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data), err
}

// countingHandler reads the body, takes pause under Bounded as a write
// unit would, and answers how many bytes it read; it reports the read's
// error and when it ended.
func countingHandler(pause time.Duration, ended chan<- error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if ended != nil {
			ended <- err
		}
		if err != nil {
			return
		}
		ctx, cancel := Bounded(r.Context())
		defer cancel()
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return
		}
		_, _ = io.WriteString(w, strconv.FormatInt(n, 10))
	})
}

// A body that takes 750ms, past the 300ms read timeout, at twice the
// minimum rate is read whole, also at a rate at which 64 KiB take longer
// than the read timeout; the step after it, past the 1s write timeout,
// still answers.
func TestASlowStreamAboveItsRateIsReadAndAnswered(t *testing.T) {
	for _, tt := range []struct {
		rate  int64
		chunk int
	}{
		{256 << 10, 32 << 10},
		{64 << 10, 8 << 10},
	} {
		t.Run(fmt.Sprintf("at %d B/s", tt.rate), func(t *testing.T) {
			api := streamTestAPI(t)
			url, _, _ := streamServer(t, api.Stream(countingHandler(400*time.Millisecond, nil), StreamPolicy{MinRate: tt.rate, MaxBytes: 1 << 20}))

			start := time.Now()
			status, body, err := upload(url, &pacedBody{chunks: 12, size: tt.chunk, pause: 62 * time.Millisecond}, int64(12*tt.chunk))
			if err != nil || status != http.StatusOK || body != strconv.Itoa(12*tt.chunk) {
				t.Fatalf("upload = %d %q, %v; want 200 and every byte", status, body, err)
			}
			if elapsed := time.Since(start); elapsed < time.Second {
				t.Errorf("the upload took %v, want past the write timeout for the test to mean anything", elapsed)
			}
		})
	}
}

// A request without a body has no read deadline: its handler outlives the
// 300ms read timeout, its context not cancelled, and answers.
func TestAStreamWithoutABodyOutlivesTheReadTimeout(t *testing.T) {
	api := streamTestAPI(t)
	alive := make(chan error, 1)
	url, _, _ := streamServer(t, api.Stream(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(600 * time.Millisecond):
		case <-r.Context().Done():
		}
		alive <- r.Context().Err()
		_, _ = io.WriteString(w, "alive")
	}), StreamPolicy{MinRate: 1 << 20}))

	got, err := readSlowly(url)
	if err := <-alive; err != nil {
		t.Errorf("past the read timeout the handler's context ended with %v", err)
	}
	if err != nil || got != int64(len("alive")) {
		t.Errorf("read %d bytes, %v; want the answer", got, err)
	}
}

// A client that trickles 1 KiB every 100ms, far below the rate, is cut off
// at the read timeout: a trickle never moves the deadline.
func TestATrickleBelowTheRateIsCutOff(t *testing.T) {
	api := streamTestAPI(t)
	ended := make(chan error, 1)
	url, _, _ := streamServer(t, api.Stream(countingHandler(0, ended), StreamPolicy{MinRate: 256 << 10, MaxBytes: 1 << 20}))

	start := time.Now()
	go func() {
		_, _, _ = upload(url, &pacedBody{chunks: 100, size: 1 << 10, pause: 100 * time.Millisecond}, 100<<10)
	}()
	select {
	case err := <-ended:
		if ne, ok := errors.AsType[net.Error](err); !ok || !ne.Timeout() {
			t.Errorf("the read ended with %v, want a timeout", err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("cut off after %v, want at the 300ms read timeout", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a trickling body was still being read after 5s")
	}
}

// A body that stops after 128 KiB is cut off once the time its read bytes
// may take at the rate has passed after the read timeout.
func TestAStalledStreamIsCutOff(t *testing.T) {
	api := streamTestAPI(t)
	ended := make(chan error, 1)
	url, _, _ := streamServer(t, api.Stream(countingHandler(0, ended), StreamPolicy{MinRate: 256 << 10, MaxBytes: 1 << 20}))
	stall := make(chan struct{})
	t.Cleanup(func() { close(stall) })

	start := time.Now()
	go func() { _, _, _ = upload(url, &pacedBody{chunks: 4, size: 32 << 10, stall: stall}, 1<<20) }()
	select {
	case err := <-ended:
		if ne, ok := errors.AsType[net.Error](err); !ok || !ne.Timeout() {
			t.Errorf("the read ended with %v, want a timeout", err)
		}
		// 300ms and the time the bytes read when the deadline last moved
		// take at 256 KiB/s: it moves with every 38.4 KiB, half the read
		// timeout at the rate, so last with at least 89.6 KiB of the 128.
		if elapsed := time.Since(start); elapsed < 600*time.Millisecond || elapsed > 3*time.Second {
			t.Errorf("cut off after %v, want 650 to 800ms", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a stalled body was still being read after 5s")
	}
}

// readSlowly reads the response through a 16 KiB receive buffer at about
// 1 MiB/s, pacing by the bytes read, however few each read gets, and
// answers how much it read.
func readSlowly(url string) (int64, error) {
	const rate = 1 << 20
	dialer := &net.Dialer{}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := dialer.DialContext(ctx, network, addr)
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetReadBuffer(16 << 10)
		}
		return c, err
	}}
	req, err := http.NewRequest(http.MethodGet, url+"/api/v0/downloads", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := (&http.Client{Transport: transport, Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	start := time.Now()
	var total int64
	buf := make([]byte, 16<<10)
	for {
		n, err := resp.Body.Read(buf)
		total += int64(n)
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return total, err
		}
		time.Sleep(time.Until(start.Add(time.Duration(total) * time.Second / rate)))
	}
}

// A 4 MiB answer to a client reading about 1 MiB/s takes about 4s, past
// the 1s write timeout, and far more than the socket buffers hold:
// announced with Sending at 512 KiB/s, its deadline some 8.3s away, it
// goes out whole; not announced, the write deadline cuts it.
func TestAnAnswerAnnouncedWithSendingOutlastsTheWriteTimeout(t *testing.T) {
	const size = 4 << 20
	for _, announce := range []bool{true, false} {
		t.Run(fmt.Sprintf("announced %v", announce), func(t *testing.T) {
			api := streamTestAPI(t)
			wrote := make(chan error, 1)
			url, _, _ := streamServer(t, api.Stream(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if announce {
					if err := Sending(r, size); err != nil {
						wrote <- err
						return
					}
				}
				w.Header().Set("Content-Length", strconv.Itoa(size))
				_, err := w.Write(bytes.Repeat([]byte("x"), size))
				wrote <- err
			}), StreamPolicy{MinRate: 512 << 10}))

			got, err := readSlowly(url)
			werr := <-wrote
			if announce && (err != nil || got != size || werr != nil) {
				t.Errorf("read %d bytes (%v), the write %v; want all %d", got, err, werr, size)
			}
			if !announce && werr == nil {
				t.Errorf("the write succeeded (read %d bytes), want it cut off by the write timeout", got)
			}
		})
	}
}

// When the server starts shutting down, a stream still reading its body
// fails its read at once and its context is cancelled, so Serve returns
// long before shutdown_timeout: whether its client keeps sending, the next
// chunk finding the stream stopped, or has gone quiet with its deadline a
// minute away at a rate of 1 KiB/s, the deadline shutdown moves to now.
func TestShutdownEndsAStreamAtOnce(t *testing.T) {
	for _, tt := range []struct {
		name string
		rate int64
		body func(stall chan struct{}) *pacedBody
	}{
		{"a client sending", 256 << 10, func(stall chan struct{}) *pacedBody {
			return &pacedBody{chunks: 200, size: 64 << 10, pause: 50 * time.Millisecond, stall: stall}
		}},
		{"a quiet client", 1 << 10, func(stall chan struct{}) *pacedBody {
			return &pacedBody{chunks: 2, size: 64 << 10, stall: stall}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := streamTestAPI(t)
			reading := make(chan struct{})
			ended := make(chan error, 1)
			cancelled := make(chan error, 1)
			url, cancel, done := streamServer(t, api.Stream(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				buf := make([]byte, 1024)
				if _, err := io.ReadFull(r.Body, buf); err != nil {
					ended <- err
					return
				}
				close(reading)
				_, err := io.Copy(io.Discard, r.Body)
				ended <- err
				<-r.Context().Done()
				cancelled <- r.Context().Err()
			}), StreamPolicy{MinRate: tt.rate, MaxBytes: 64 << 20}))
			stall := make(chan struct{})
			t.Cleanup(func() { close(stall) })
			go func() { _, _, _ = upload(url, tt.body(stall), 64<<20) }()
			<-reading
			time.Sleep(100 * time.Millisecond) // the quiet client's bytes are in

			begin := time.Now()
			cancel()
			select {
			case err := <-ended:
				if err == nil {
					t.Error("the body read finished, want it failed by the shutdown")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("the body read still waits 3s after the shutdown began")
			}
			if err := <-cancelled; !errors.Is(err, context.Canceled) {
				t.Errorf("the handler's context ended with %v, want context.Canceled", err)
			}
			if err := wait(t, done); err != nil {
				t.Errorf("Serve() = %v", err)
			}
			if elapsed := time.Since(begin); elapsed > 2*time.Second {
				t.Errorf("shutdown took %v, want far less than the 5s shutdown_timeout", elapsed)
			}
		})
	}
}

// A stream whose body is in when the server starts shutting down finishes
// its step after it, such as writing what the body brought, and answers,
// as any request does; it can no longer announce a long answer.
func TestShutdownLetsAStreamFinishTheStepAfterItsBody(t *testing.T) {
	api := streamTestAPI(t)
	read := make(chan struct{})
	proceed := make(chan struct{})
	after := make(chan error, 2)
	url, cancel, done := streamServer(t, api.Stream(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			after <- err
			after <- err
			return
		}
		close(read)
		<-proceed
		ctx, cancelStep := Bounded(r.Context())
		defer cancelStep()
		select {
		case <-time.After(200 * time.Millisecond): // the step
		case <-ctx.Done():
		}
		after <- ctx.Err()
		after <- Sending(r, 1<<20)
		_, _ = io.WriteString(w, strconv.FormatInt(n, 10))
	}), StreamPolicy{MinRate: 256 << 10, MaxBytes: 1 << 20}))

	answered := make(chan string, 1)
	go func() {
		status, body, err := upload(url, &pacedBody{chunks: 2, size: 32 << 10}, 64<<10)
		answered <- fmt.Sprintf("%d %s %v", status, body, err)
	}()
	<-read
	cancel()
	time.Sleep(100 * time.Millisecond) // the shutdown reaches the stream
	close(proceed)

	if err := <-after; err != nil {
		t.Errorf("the step after the body ended with %v, want it finished", err)
	}
	if err := <-after; !errors.Is(err, errStreamStopped) {
		t.Errorf("Sending during shutdown = %v, want errStreamStopped", err)
	}
	if got, want := <-answered, fmt.Sprintf("200 %d <nil>", 64<<10); got != want {
		t.Errorf("the client got %q, want %q", got, want)
	}
	if err := wait(t, done); err != nil {
		t.Errorf("Serve() = %v", err)
	}
}
