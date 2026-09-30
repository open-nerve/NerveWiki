package httpserver

import (
	"log/slog"
	"net/http"
	"time"
)

// Limiter is one rate-limit bucket, with a unit count per key (M1/P2 design
// 3.2). platform/ratelimit's buckets implement it; bootstrap sizes them from
// the configuration.
type Limiter interface {
	// Allow takes one unit of key's bucket, or tells how long until one is
	// back.
	Allow(key string) (retry time.Duration, ok bool)
	// Reserve takes one unit of key's bucket now; refund gives it back, at
	// most once.
	Reserve(key string) (refund func(), retry time.Duration, ok bool)
}

// rateLimit takes one unit of the caller's bucket (M1/P2 design 3.2): of its
// credential in authenticated when authentication gave it one, else of its
// client IP in anonymous. Requests a module turns away later still count.
func (a *API) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bucket, name, key := a.anonymous, "anonymous", RequestMetaFrom(r.Context()).IPKey
		if credential, ok := r.Context().Value(credentialKey{}).(string); ok {
			bucket, name, key = a.authenticated, "authenticated", credential
		}
		if retry, ok := bucket.Allow(key); !ok {
			a.tooManyRequests(w, r, name, retry)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tooManyRequests answers 429 rate_limited with Retry-After and logs which
// bucket turned the client away, at debug level: the access log has the 429
// at info already, and a line more per refused request would let a client
// that no bucket stops double the log (M1/P2 review M2).
func (a *API) tooManyRequests(w http.ResponseWriter, r *http.Request, bucket string, retry time.Duration) {
	a.logger.LogAttrs(r.Context(), slog.LevelDebug, "rate limited",
		slog.String("request_id", RequestID(r.Context())), slog.String("bucket", bucket),
		slog.String("ip", RequestMetaFrom(r.Context()).ClientIP.String()))
	a.Errors.Write(w, r, rateLimited(retry))
}

// rateLimited is the ProblemError of a request over its rate: 429
// rate_limited, retry after the wait.
type rateLimited time.Duration

func (rateLimited) Error() string               { return "Too many requests; retry later." }
func (rateLimited) ProblemStatus() int          { return http.StatusTooManyRequests }
func (rateLimited) ProblemCode() string         { return CodeRateLimited }
func (r rateLimited) RetryAfter() time.Duration { return time.Duration(r) }
