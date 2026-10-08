package httpservertest_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
)

type noAuth struct{}

func (noAuth) Authenticate(ctx context.Context, _ string) (context.Context, string, error) {
	return ctx, "", nil
}

// deadlines is a recorder that takes deadlines, as a connection does.
type deadlines struct{ *httptest.ResponseRecorder }

func (deadlines) SetReadDeadline(time.Time) error  { return nil }
func (deadlines) SetWriteDeadline(time.Time) error { return nil }

// keys records the keys it took a unit of; it never runs out.
type keys struct {
	mu    sync.Mutex
	taken []string
}

func (k *keys) Allow(key string) (time.Duration, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.taken = append(k.taken, key)
	return 0, true
}

func (k *keys) Reserve(key string) (func(), time.Duration, bool) {
	_, _ = k.Allow(key)
	return func() {}, 0, true
}

// The anonymous bucket a test passes is the one the API takes: a public
// route takes it, by IP. (asset's HTTP tests watch the other two.)
func TestNewAPITakesTheAnonymousBucketPassed(t *testing.T) {
	const route = "GET /api/v0/open"
	anonymous := &keys{}
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: noAuth{}, PublicOperations: []string{route}, Anonymous: anonymous})
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(route, api.Stream(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		httpserver.StreamPolicy{MinRate: 1}))
	r := httptest.NewRequest(http.MethodGet, "/api/v0/open", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	w := deadlines{httptest.NewRecorder()}
	router.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || !slices.Equal(anonymous.taken, []string{"203.0.113.7"}) {
		t.Errorf("%d, the anonymous bucket took %q; want 204, the client's IP", w.Code, anonymous.taken)
	}
}
