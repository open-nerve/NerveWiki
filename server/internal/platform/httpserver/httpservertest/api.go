// Package httpservertest builds the platform's per-route middlewares for a
// module's HTTP tests. Architecture rule 8 lets only tests import it.
package httpservertest

import (
	"log/slog"
	"testing"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
)

// APIOptions are what a module's tests choose of the per-route middlewares:
// the module's authenticator, public operations, shorter deadlines and
// larger bodies. MaxBodyBytes is 1 MiB when zero.
type APIOptions struct {
	Authenticator    httpserver.Authenticator
	PublicOperations []string
	RequestTimeouts  map[string]time.Duration
	BodyLimits       map[string]int64
	MaxBodyBytes     int64
}

// NewAPI returns the per-route middlewares a module's HTTP tests mount the
// module behind (M1 handoff to M2, item 7): logs discarded, a request
// deadline of 5 s, and platform buckets that never run out. Rate limiting
// is tested apart, with small buckets.
func NewAPI(t testing.TB, o APIOptions) *httpserver.API {
	t.Helper()
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = 1 << 20
	}
	api, err := httpserver.NewAPI(httpserver.APIConfig{
		Logger:           slog.New(slog.DiscardHandler),
		Authenticator:    o.Authenticator,
		PublicOperations: o.PublicOperations,
		MaxBodyBytes:     o.MaxBodyBytes,
		RequestTimeout:   5 * time.Second,
		RequestTimeouts:  o.RequestTimeouts,
		BodyLimits:       o.BodyLimits,
		IPv6PrefixLen:    64,
		Anonymous:        unlimited{},
		Authenticated:    unlimited{},
		AuthFailure:      unlimited{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return api
}

// unlimited is a bucket that always has a unit.
type unlimited struct{}

func (unlimited) Allow(string) (time.Duration, bool) { return 0, true }

func (unlimited) Reserve(string) (func(), time.Duration, bool) { return func() {}, 0, true }
