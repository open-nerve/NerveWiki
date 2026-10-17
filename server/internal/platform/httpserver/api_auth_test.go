package httpserver

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

// Authentication in the per-route middleware (M1/P1 design 3.5): what
// reaches the authenticator, what a failure answers and logs.

func TestAuthenticationDeniesByDefault(t *testing.T) {
	tests := []struct {
		name         string
		path, header string
		status       int
		challenge    string
		authCalls    int
	}{
		{"no token", "/api/v0/things", "", 401, "Bearer", 0},
		{"another scheme", "/api/v0/things", "Basic dXNlcjpwYXNz", 401, "Bearer", 0},
		{"empty bearer", "/api/v0/things", "Bearer ", 401, "Bearer", 0},
		{"a token with a space", "/api/v0/things", "Bearer tok en", 401, "Bearer", 0},
		{"invalid token", "/api/v0/things", "Bearer bad", 401, `Bearer error="invalid_token"`, 1},
		{"expired access token", "/api/v0/things", "Bearer expired", 401, `Bearer error="invalid_token"`, 1},
		{"valid token", "/api/v0/things", "Bearer tok", 204, "", 1},
		{"scheme in lower case", "/api/v0/things", "bearer tok", 204, "", 1},
		{"authenticator fault", "/api/v0/things", "Bearer boom", 500, "", 1},
		{"public without a token", "/api/v0/open", "", 204, "", 0},
		{"public ignores a bad token", "/api/v0/open", "Bearer bad", 204, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &fakeAuth{}
			router, got := mount(buildAPI(t, testAPIConfig(auth, slog.New(slog.DiscardHandler))))
			req := post(tt.path, `{"name":"a"}`)
			req.Header.Set("Authorization", tt.header)

			rec := serve(router, req)

			challenge := rec.Result().Header.Get("WWW-Authenticate")
			if rec.Code != tt.status || challenge != tt.challenge || auth.calls != tt.authCalls || got.called != (tt.status == 204) {
				t.Errorf("response = %d, WWW-Authenticate %q, authenticator called %d times, handler called %v; want %d, %q, %d",
					rec.Code, challenge, auth.calls, got.called, tt.status, tt.challenge, tt.authCalls)
			}
			if tt.status == 401 {
				if p := decodeProblem(t, rec); p.Code != CodeUnauthorized {
					t.Errorf("problem code = %q, want unauthorized", p.Code)
				}
			}
		})
	}
}

// The handler runs with the context the authenticator returned: the caller
// that authentication put in it.
func TestTheHandlerSeesTheAuthenticatedCaller(t *testing.T) {
	router, got := mount(newTestAPI(t))

	serve(router, post("/api/v0/things", `{"name":"a"}`))

	if caller, _ := got.ctx.Value(callerKey{}).(string); caller != "caller-tok" {
		t.Errorf("caller = %q, want caller-tok", caller)
	}
}

// Why a credential failed goes to the debug log, never into the response.
func TestAuthenticationFailureIsLoggedAtDebugLevel(t *testing.T) {
	logger, logs := captureLogs(t)
	router, _ := mount(buildAPI(t, testAPIConfig(&fakeAuth{}, logger)))
	req := post("/api/v0/things", `{"name":"a"}`)
	req.Header.Set("Authorization", "Bearer bad")

	rec := serve(router, req)

	entry := findLog(logs(), "authentication failed")
	if entry == nil || entry["level"] != "DEBUG" || entry["route"] != thingRoute || entry["error"] != "Authentication is required." {
		t.Errorf("log = %v, want the reason at debug level", entry)
	}
	if strings.Contains(rec.Body.String(), "Authentication is required.") {
		t.Errorf("body %s carries the authenticator's reason", rec.Body)
	}
}

// Authentication runs inside the request deadline, with the request meta
// in its context, and before the body is read: a request without a token
// never gets its body checked.
func TestAuthenticationRunsUnderTheDeadlineBeforeTheBodyCheck(t *testing.T) {
	auth := &fakeAuth{}
	router, _ := mount(buildAPI(t, testAPIConfig(auth, slog.New(slog.DiscardHandler))))

	serve(router, post("/api/v0/things", `{"name":"a"}`))
	if _, ok := auth.ctx.Deadline(); !ok || !RequestMetaFrom(auth.ctx).ClientIP.IsValid() {
		t.Errorf("the authenticator's context has deadline %v and meta %+v; want both", ok, RequestMetaFrom(auth.ctx))
	}

	req := post("/api/v0/things", `{"extra":1}`)
	req.Header.Del("Authorization")
	if rec := serve(router, req); rec.Code != http.StatusUnauthorized {
		t.Errorf("a broken body without a token: status %d, want 401", rec.Code)
	}
}

// A route of NoTokenDetails says its own detail in the 401 of a request
// without a token, the challenge still without an error (RFC 6750 3.1);
// a token that fails, and every other route, keep the platform's.
func TestARouteSaysWhyItNeedsAToken(t *testing.T) {
	cfg := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	cfg.NoTokenDetails = map[string]string{thingRoute: "This route takes a personal access token."}
	router, _ := mount(buildAPI(t, cfg))
	other := testAPIConfig(&fakeAuth{}, slog.New(slog.DiscardHandler))
	other.NoTokenDetails = map[string]string{"POST /api/v0/others": "This route takes a personal access token."}
	otherRouter, _ := mount(buildAPI(t, other))
	tests := []struct {
		name, header      string
		router            *Router
		challenge, detail string
	}{
		{"no token", "", router, "Bearer", "This route takes a personal access token."},
		{"another scheme", "Basic dXNlcjpwYXNz", router, "Bearer", "This route takes a personal access token."},
		{"a token that fails", "Bearer bad", router, `Bearer error="invalid_token"`, "The bearer token is invalid or has expired."},
		{"no token on another route", "", otherRouter, "Bearer", "This operation requires a bearer token."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := post("/api/v0/things", `{"name":"a"}`)
			req.Header.Set("Authorization", tt.header)
			rec := serve(tt.router, req)
			p := decodeProblem(t, rec)
			if challenge := rec.Result().Header.Get("WWW-Authenticate"); rec.Code != 401 || p.Code != CodeUnauthorized ||
				challenge != tt.challenge || p.Detail != tt.detail {
				t.Errorf("response = %d %q, WWW-Authenticate %q; want 401 %q, %q", rec.Code, p.Detail, challenge, tt.detail, tt.challenge)
			}
		})
	}
	other.NoTokenDetails = map[string]string{thingRoute: ""}
	if _, err := NewAPI(other); err == nil || !strings.Contains(err.Error(), `NoTokenDetails["`+thingRoute+`"] is empty`) {
		t.Errorf("NewAPI() of an empty detail = %v, want it refused", err)
	}
}
