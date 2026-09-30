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
