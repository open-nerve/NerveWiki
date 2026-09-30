package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/bodyshape"
)

// Authenticator checks a bearer token (M1/P1 design 3.5). The identity
// module implements it; the platform does not know what an account is.
type Authenticator interface {
	// Authenticate returns a context carrying the caller. An invalid token
	// is an error with ProblemStatus() 401; any other error is an internal
	// fault.
	Authenticate(ctx context.Context, token string) (context.Context, error)
}

// APIConfig is what the per-route middlewares need.
type APIConfig struct {
	Logger        *slog.Logger
	Authenticator Authenticator
	// PublicOperations are the route patterns that need no token, e.g.
	// "POST /api/v0/auth/register": the union of every module's list.
	PublicOperations []string
	MaxBodyBytes     int64          // server.max_body_bytes
	RequestTimeout   time.Duration  // server.request_timeout
	TrustedProxies   []netip.Prefix // server.trusted_proxies
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
	clients        *clientIPs
}

// NewAPI returns the API value for cfg. It needs a logger, an
// authenticator, and a body limit and a request timeout above zero.
func NewAPI(cfg APIConfig) (*API, error) {
	var errs []error
	if cfg.Logger == nil {
		errs = append(errs, errors.New("no Logger"))
	}
	if cfg.Authenticator == nil {
		errs = append(errs, errors.New("no Authenticator"))
	}
	if cfg.MaxBodyBytes <= 0 {
		errs = append(errs, errors.New("MaxBodyBytes must be positive"))
	}
	if cfg.RequestTimeout <= 0 {
		errs = append(errs, errors.New("RequestTimeout must be positive"))
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
		clients:        &clientIPs{logger: cfg.Logger, trusted: cfg.TrustedProxies},
	}, nil
}

// Middlewares returns the per-route middlewares for a module's generated
// StdHTTPServerOptions.Middlewares; bodies is the module's generated
// bodyshape table. They run in this order (M1/P1 design 3.5):
//
//	request meta → request deadline → body limit → authentication →
//	body structure
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
		bodyshape.Middleware(bodies, a.Errors.BodyError),
	}
	slices.Reverse(inOrder)
	return inOrder
}

// RequestMeta describes the client of a request.
type RequestMeta struct {
	// ClientIP is the client's address: the connection's peer, or what a
	// trusted proxy forwarded; without zone, and an IPv4-mapped IPv6
	// address is its IPv4 address. Sessions record it. The zero Addr when
	// the peer address does not parse.
	ClientIP  netip.Addr
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
		meta := RequestMeta{ClientIP: a.clients.of(r), UserAgent: r.UserAgent()}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), metaKey{}, meta)))
	})
}

// deadline bounds the handler: server.write_timeout only fails the writes
// and never cancels the request's context, so without it a handler's
// database calls could outlive the response.
func (a *API) deadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), a.requestTimeout)
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

// authenticate denies by default (M1/P1 design 3.5): every operation needs a
// valid bearer token, except the public ones, which never look at it.
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
		ctx, err := a.authenticator.Authenticate(r.Context(), token)
		if err != nil {
			var pe ProblemError
			if errors.As(err, &pe) && pe.ProblemStatus() == http.StatusUnauthorized {
				a.unauthorized(w, r, err, true)
				return
			}
			a.Errors.Write(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
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
