package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// Authenticator checks a bearer token (M1/P1 design 3.5, M1/P2 design 3.3).
// The identity module implements it; the platform does not know what an
// account is.
type Authenticator interface {
	// Authenticate returns a context carrying the caller, and the caller's
	// rate-limit key (session:<id>). An invalid token is an error with
	// ProblemStatus() 401; for a token that is valid but for its expiry, the
	// error also has ExpiredCredential() true. Any other error is an
	// internal fault.
	Authenticate(ctx context.Context, token string) (context.Context, string, error)
}

// expiredCredential is optional on the 401 error of an Authenticator: a
// validly signed access token whose exp has passed is the client's normal
// cue to refresh, and does not count as a failure.
type expiredCredential interface {
	ExpiredCredential() bool
}

// APIConfig is what the per-route middlewares need.
type APIConfig struct {
	Logger        *slog.Logger
	Authenticator Authenticator
	// PublicOperations are the route patterns that need no token, e.g.
	// "POST /api/v0/auth/register": the union of every module's list.
	PublicOperations []string
	MaxBodyBytes     int64         // server.max_body_bytes
	RequestTimeout   time.Duration // server.request_timeout
	// RequestTimeouts are the route patterns whose deadline is shorter
	// than RequestTimeout, with that deadline: a module's protocol can need
	// an answer sooner (M1/P2 design 3.5). Their expiry is a request
	// deadline like any other.
	RequestTimeouts map[string]time.Duration
	TrustedProxies  []netip.Prefix // server.trusted_proxies
	IPv6PrefixLen   int            // ratelimit.ipv6_prefix_len
	// The platform's rate-limit buckets (M1/P2 design 3.2).
	Anonymous     Limiter // ratelimit.anonymous: public operations, by client IP
	Authenticated Limiter // ratelimit.authenticated: the rest, by credential
	AuthFailure   Limiter // ratelimit.auth_failure: the gate before authentication, by client IP
}

// API is what the platform hands to every module's HTTP adapter: the error
// mapping for the generated code, and the per-route middlewares.
type API struct {
	Errors         APIErrors
	logger         *slog.Logger
	authenticator  Authenticator
	public         map[string]bool
	maxBodyBytes   int64
	requestTimeout time.Duration
	timeouts       map[string]time.Duration
	clients        *clientIPs
	anonymous      Limiter
	authenticated  Limiter
	authFailure    Limiter
}

// NewAPI returns the API value for cfg. It needs a logger, an
// authenticator, the three buckets, a body limit and a request timeout
// above zero, route timeouts above zero and at most the request timeout,
// and an IPv6 prefix length from 1 to 128.
func NewAPI(cfg APIConfig) (*API, error) {
	var errs []error
	for _, dep := range []struct {
		name  string
		unset bool
	}{
		{"Logger", cfg.Logger == nil},
		{"Authenticator", cfg.Authenticator == nil},
		{"Anonymous", cfg.Anonymous == nil},
		{"Authenticated", cfg.Authenticated == nil},
		{"AuthFailure", cfg.AuthFailure == nil},
	} {
		if dep.unset {
			errs = append(errs, errors.New("no "+dep.name))
		}
	}
	if cfg.IPv6PrefixLen < 1 || cfg.IPv6PrefixLen > 128 {
		errs = append(errs, fmt.Errorf("IPv6PrefixLen %d is outside 1-128", cfg.IPv6PrefixLen))
	}
	if cfg.MaxBodyBytes <= 0 {
		errs = append(errs, errors.New("MaxBodyBytes must be positive"))
	}
	if cfg.RequestTimeout <= 0 {
		errs = append(errs, errors.New("RequestTimeout must be positive"))
	}
	for _, route := range slices.Sorted(maps.Keys(cfg.RequestTimeouts)) {
		if d := cfg.RequestTimeouts[route]; d <= 0 || d > cfg.RequestTimeout {
			errs = append(errs, fmt.Errorf("RequestTimeouts[%q] %v is outside (0, RequestTimeout %v]", route, d, cfg.RequestTimeout))
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("httpserver: APIConfig: %w", errors.Join(errs...))
	}
	public := make(map[string]bool, len(cfg.PublicOperations))
	for _, p := range cfg.PublicOperations {
		public[p] = true
	}
	return &API{
		Errors:         NewAPIErrors(cfg.Logger),
		logger:         cfg.Logger,
		authenticator:  cfg.Authenticator,
		public:         public,
		maxBodyBytes:   cfg.MaxBodyBytes,
		requestTimeout: cfg.RequestTimeout,
		timeouts:       maps.Clone(cfg.RequestTimeouts),
		clients:        &clientIPs{logger: cfg.Logger, trusted: cfg.TrustedProxies, v6Prefix: cfg.IPv6PrefixLen},
		anonymous:      cfg.Anonymous,
		authenticated:  cfg.Authenticated,
		authFailure:    cfg.AuthFailure,
	}, nil
}

// Middlewares returns the per-route middlewares for a module's generated
// StdHTTPServerOptions.Middlewares; bodies is the module's generated
// bodyshape table. They run in this order (M1/P2 design 3.3):
//
//	request meta → request deadline → body limit → failure gate and
//	authentication → rate limit → body structure
//
// The generated code wraps the last middleware of its list outermost, so
// the list is in reverse. Only API operations get them: a long-lived route
// is registered on the router directly, without a request deadline.
func (a *API) Middlewares(bodies *bodyshape.Table) []func(http.Handler) http.Handler {
	inOrder := []func(http.Handler) http.Handler{
		a.requestMeta,
		a.deadline,
		a.bodyLimit,
		a.authenticate,
		a.rateLimit,
		bodyshape.Middleware(bodies, a.Errors.BodyError),
	}
	slices.Reverse(inOrder)
	return inOrder
}

// RequestMeta describes the client of a request.
type RequestMeta struct {
	// ClientIP is the client's address: the connection's peer, or what a
	// trusted proxy forwarded; without zone, and an IPv4-mapped IPv6
	// address is its IPv4 address. Logs and sessions record it. The zero
	// Addr when the peer address does not parse.
	ClientIP netip.Addr
	// IPKey is what the per-IP rate-limit buckets count the client by: an
	// IPv4 address, or the prefix of an IPv6 one (ratelimit.ipv6_prefix_len).
	IPKey     string
	UserAgent string
}

type metaKey struct{}

// RequestMetaFrom returns the request's meta; the zero value outside the
// per-route middlewares.
func RequestMetaFrom(ctx context.Context) RequestMeta {
	m, _ := ctx.Value(metaKey{}).(RequestMeta)
	return m
}

func (a *API) requestMeta(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := a.clients.of(r)
		meta := RequestMeta{ClientIP: ip, IPKey: a.clients.key(ip), UserAgent: r.UserAgent()}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), metaKey{}, meta)))
	})
}

// deadline bounds the handler: server.write_timeout only fails the writes
// and never cancels the request's context, so without it a handler's
// database calls could outlive the response. A route of RequestTimeouts
// gets its own, shorter deadline.
func (a *API) deadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout, ok := a.timeouts[r.Pattern]
		if !ok {
			timeout = a.requestTimeout
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bodyLimit makes reading more than server.max_body_bytes fail with
// *http.MaxBytesError, which APIErrors answers with 413.
func (a *API) bodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, a.maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// credentialKey carries the rate-limit key of the request's credential from
// authenticate to rateLimit.
type credentialKey struct{}

// authenticate denies by default (M1/P1 design 3.5): every operation needs a
// valid bearer token, except the public ones, which never look at it.
//
// The failure gate comes first (M1/P2 design 3.3): a request with a token
// reserves a unit of its client IP's auth_failure bucket before the
// authenticator runs, and gets 429 without running it when the bucket is
// empty. A credential that fails keeps the unit; success, an expired access
// token and an internal fault give it back. Reserving first holds
// concurrent requests to the bucket too, so failures never exceed it.
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.public[r.Pattern] {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			a.unauthorized(w, r, errors.New("no bearer token"), false)
			return
		}
		refund, retry, ok := a.authFailure.Reserve(RequestMetaFrom(r.Context()).IPKey)
		if !ok {
			a.tooManyRequests(w, r, "auth_failure", retry)
			return
		}
		ctx, credential, err := a.authenticator.Authenticate(r.Context(), token)
		if err != nil {
			var pe ProblemError
			if errors.As(err, &pe) && pe.ProblemStatus() == http.StatusUnauthorized {
				var ec expiredCredential
				if errors.As(err, &ec) && ec.ExpiredCredential() {
					refund()
				}
				a.unauthorized(w, r, err, true)
				return
			}
			refund()
			a.Errors.Write(w, r, err)
			return
		}
		refund()
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, credentialKey{}, credential)))
	})
}

// unauthorized answers 401 with WWW-Authenticate (RFC 6750 3). Why the
// credential failed goes to the debug log only.
func (a *API) unauthorized(w http.ResponseWriter, r *http.Request, reason error, invalidToken bool) {
	a.logger.LogAttrs(r.Context(), slog.LevelDebug, "authentication failed",
		slog.String("request_id", RequestID(r.Context())), slog.String("route", r.Pattern), slog.Any("error", reason))
	challenge, detail := "Bearer", "This operation requires a bearer token."
	if invalidToken {
		challenge, detail = `Bearer error="invalid_token"`, "The bearer token is invalid or has expired."
	}
	w.Header().Set("WWW-Authenticate", challenge)
	WriteProblem(w, Problem{
		Status: http.StatusUnauthorized,
		Code:   CodeUnauthorized,
		Title:  http.StatusText(http.StatusUnauthorized),
		Detail: detail,
	})
}

// bearerToken returns the token of an "Authorization: Bearer <token>"
// header; the scheme is case-insensitive (RFC 9110 11.1).
func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}
