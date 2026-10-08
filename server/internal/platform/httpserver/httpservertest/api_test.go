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

// The platform buckets a test passes are those the API takes: a public
// route takes the anonymous one, by IP.
func TestNewAPITakesTheBucketsPassed(t *testing.T) {
	const route = "GET /api/v0/open"
	anonymous := &keys{}
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: noAuth{}, PublicOperations: []string{route}, Anonymous: anonymous})
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	router.Handle(route, api.Stream(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), httpserver.StreamPolicy{MinRate: 1}))
	r := httptest.NewRequest(http.MethodGet, "/api/v0/open", nil)
	r.RemoteAddr = "203.0.113.7:5555"
	router.ServeHTTP(httptest.NewRecorder(), r)
	if !slices.Equal(anonymous.taken, []string{"203.0.113.7"}) {
		t.Errorf("the anonymous bucket took %q, want the client's IP", anonymous.taken)
	}
}
