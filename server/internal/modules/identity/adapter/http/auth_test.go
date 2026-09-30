package httpadapter_test

import (
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

func TestLoginAnswers200WithTheTokens(t *testing.T) {
	login := &fakeLogin{tokens: sampleTokens()}
	req := postJSON("/api/v0/auth/login", `{"email":" Alice@Corp.com","password":"Tr0ub4dor&3"}`)
	apitest.Load(t).CheckRequest(t, req)

	res, body := do(t, newServer(t, httpadapter.UseCases{Login: login}), req)

	if res.StatusCode != http.StatusOK || body != sampleTokensJSON {
		t.Errorf("POST /auth/login = %d %s, want 200 %s", res.StatusCode, body, sampleTokensJSON)
	}
	// The use case normalizes the address.
	want := app.LoginInput{Email: " Alice@Corp.com", Password: "Tr0ub4dor&3", UserAgent: "agent/1", IP: netip.MustParseAddr("203.0.113.7")}
	if login.got != want {
		t.Errorf("use case got %+v, want %+v", login.got, want)
	}
}

// The handler exit: an invalid credential is a 401 with the plain Bearer
// challenge that every other 401 carries.
func TestLoginProblems(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       string
		challenge  string
		retryAfter string
	}{
		{"unknown address or wrong password", domain.ErrInvalidCredentials, 401, "identity.invalid_credentials", "Bearer", ""},
		{"deactivated", domain.ErrAccountDeactivated, 403, "identity.account_deactivated", "", ""},
		{"hashing saturated", shared.ServerBusy(time.Second), 503, "server_busy", "", "1"},
		{"a fault", errors.New("database is down"), 500, "internal_error", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := httpadapter.UseCases{Login: &fakeLogin{err: tt.err}}
			res, body := do(t, newServer(t, uc), postJSON("/api/v0/auth/login", `{"email":"a@b.co","password":"x"}`))

			if res.StatusCode != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) ||
				res.Header.Get("WWW-Authenticate") != tt.challenge || res.Header.Get("Retry-After") != tt.retryAfter {
				t.Errorf("response = %d %s, WWW-Authenticate %q, Retry-After %q; want %d %s, %q, %q", res.StatusCode, body,
					res.Header.Get("WWW-Authenticate"), res.Header.Get("Retry-After"), tt.status, tt.code, tt.challenge, tt.retryAfter)
			}
		})
	}
}

// A login without a password is the platform's 400; the use case never runs.
func TestLoginWithoutAPassword(t *testing.T) {
	login := &fakeLogin{}

	res, body := do(t, newServer(t, httpadapter.UseCases{Login: login}), postJSON("/api/v0/auth/login", `{"email":"a@b.co"}`))

	want := `"errors":[{"field":"password","code":"required","message":"is required"}]`
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, want) || login.calls != 0 {
		t.Errorf("response = %d %s after %d logins, want 400 with %s and none", res.StatusCode, body, login.calls, want)
	}
}

// Refresh runs within auth.refresh_deadline, shorter than the request
// timeout (M1/P2 design 3.5).
func TestRefreshTokens(t *testing.T) {
	refresh := &fakeRefresh{tokens: sampleTokens()}
	req := postJSON("/api/v0/auth/refresh", `{"refresh_token":"nwk_rt_old"}`)
	apitest.Load(t).CheckRequest(t, req)

	res, body := do(t, newServer(t, httpadapter.UseCases{Refresh: refresh}), req)

	if res.StatusCode != http.StatusOK || body != sampleTokensJSON {
		t.Errorf("POST /auth/refresh = %d %s, want 200 %s", res.StatusCode, body, sampleTokensJSON)
	}
	if refresh.token != "nwk_rt_old" || refresh.ip != netip.MustParseAddr("203.0.113.7") || refresh.left <= 0 || refresh.left > refreshDeadline {
		t.Errorf("use case got %q from %v with %v left; want the token, the client, at most %v", refresh.token, refresh.ip, refresh.left, refreshDeadline)
	}
}

func TestRefreshTokensInvalid(t *testing.T) {
	uc := httpadapter.UseCases{Refresh: &fakeRefresh{err: domain.ErrRefreshTokenInvalid}}
	res, body := do(t, newServer(t, uc), postJSON("/api/v0/auth/refresh", `{"refresh_token":"nwk_rt_old"}`))

	if res.StatusCode != http.StatusUnauthorized || !strings.Contains(body, `"code":"identity.refresh_token_invalid"`) || res.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("response = %d %s, WWW-Authenticate %q; want 401 identity.refresh_token_invalid with Bearer",
			res.StatusCode, body, res.Header.Get("WWW-Authenticate"))
	}
}

func TestLogout(t *testing.T) {
	logout := &fakeLogout{}
	req := postJSON("/api/v0/auth/logout", `{"refresh_token":"nwk_rt_current"}`)
	apitest.Load(t).CheckRequest(t, req)

	res, body := do(t, newServer(t, httpadapter.UseCases{Logout: logout}), req)

	if res.StatusCode != http.StatusNoContent || body != "" {
		t.Errorf("POST /auth/logout = %d %q, want 204 and no body", res.StatusCode, body)
	}
	if logout.token != "nwk_rt_current" || logout.left <= 0 || logout.left > refreshDeadline {
		t.Errorf("use case got %q with %v left, want the token and at most %v", logout.token, logout.left, refreshDeadline)
	}
}

func TestLogoutFault(t *testing.T) {
	uc := httpadapter.UseCases{Logout: &fakeLogout{err: errors.New("database is down")}}
	res, body := do(t, newServer(t, uc), postJSON("/api/v0/auth/logout", `{"refresh_token":"nwk_rt_current"}`))

	if res.StatusCode != http.StatusInternalServerError || !strings.Contains(body, `"code":"internal_error"`) {
		t.Errorf("response = %d %s, want 500 internal_error", res.StatusCode, body)
	}
}
