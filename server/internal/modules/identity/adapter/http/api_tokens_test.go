package httpadapter_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

const tokenIDText = "0199a2b4-0000-7000-8000-00000000000a"

type fakeListTokens struct {
	tokens []domain.APIToken
	err    error
}

func (f *fakeListTokens) Execute(context.Context) ([]domain.APIToken, error) { return f.tokens, f.err }

type fakeCreateToken struct {
	got   app.CreateAPITokenInput
	calls int
	err   error
}

func (f *fakeCreateToken) Execute(_ context.Context, in app.CreateAPITokenInput) (app.CreatedAPIToken, error) {
	f.got = in
	f.calls++
	if f.err != nil {
		return app.CreatedAPIToken{}, f.err
	}
	return app.CreatedAPIToken{
		APIToken: domain.APIToken{ID: uuid.MustParse(tokenIDText), Name: in.Spec.Name, ExpiresAt: in.Spec.ExpiresAt, CreatedAt: created()},
		Token:    "nwk_pat_x",
	}, nil
}

type fakeRevokeToken struct {
	id  uuid.UUID
	err error
}

func (f *fakeRevokeToken) Execute(_ context.Context, id uuid.UUID) error {
	f.id = id
	return f.err
}

// withToken sends req with the bearer token fakeAuth accepts.
func withToken(req *http.Request) *http.Request {
	req.Header.Set("Authorization", "Bearer valid")
	return req
}

// The list shows every field, null for no expiry and no use, and never the
// token; an empty list is an empty array.
func TestListAPITokens(t *testing.T) {
	used := created().Add(time.Hour)
	tokens := []domain.APIToken{
		{ID: uuid.MustParse(tokenIDText), Name: "CI", ExpiresAt: &used, LastUsedAt: &used, CreatedAt: created()},
		{ID: uuid.MustParse(userIDText), Name: "agent", CreatedAt: created()},
	}
	h := newServer(t, httpadapter.UseCases{ListAPITokens: &fakeListTokens{tokens: tokens}})

	res, body := do(t, h, withToken(httptest.NewRequest(http.MethodGet, "/api/v0/me/api-tokens", nil)))

	want := `{"data":[{"created_at":"2026-09-25T10:00:00.123456Z","expires_at":"2026-09-25T11:00:00.123456Z","id":"` + tokenIDText +
		`","last_used_at":"2026-09-25T11:00:00.123456Z","name":"CI"},{"created_at":"2026-09-25T10:00:00.123456Z","expires_at":null,"id":"` +
		userIDText + `","last_used_at":null,"name":"agent"}]}`
	if res.StatusCode != http.StatusOK || strings.TrimSpace(body) != want {
		t.Errorf("list = %d %s, want 200 %s", res.StatusCode, body, want)
	}
	empty := newServer(t, httpadapter.UseCases{ListAPITokens: &fakeListTokens{}})
	if res, body := do(t, empty, withToken(httptest.NewRequest(http.MethodGet, "/api/v0/me/api-tokens", nil))); strings.TrimSpace(body) != `{"data":[]}` {
		t.Errorf("empty list = %d %s, want {\"data\":[]}", res.StatusCode, body)
	}
}

func TestCreateAPIToken(t *testing.T) {
	uc := &fakeCreateToken{}
	h := newServer(t, httpadapter.UseCases{CreateAPIToken: uc})

	res, body := do(t, h, withToken(postJSON("/api/v0/me/api-tokens",
		`{"name":"CI","expires_at":"2026-10-25T12:00:00+02:00","current_password":"Tr0ub4dor&3"}`)))

	expires := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if uc.got.Spec.Name != "CI" || uc.got.Spec.ExpiresAt == nil || !uc.got.Spec.ExpiresAt.Equal(expires) || uc.got.CurrentPassword != "Tr0ub4dor&3" {
		t.Errorf("the use case got %+v, want CI expiring at %v with the password", uc.got, expires)
	}
	if res.StatusCode != http.StatusCreated || !strings.Contains(body, `"token":"nwk_pat_x"`) || !strings.Contains(body, `"last_used_at":null`) {
		t.Errorf("create = %d %s, want 201 with the token, never used", res.StatusCode, body)
	}
	never := &fakeCreateToken{}
	if res, body := do(t, newServer(t, httpadapter.UseCases{CreateAPIToken: never}),
		withToken(postJSON("/api/v0/me/api-tokens", `{"name":"agent","current_password":"x"}`))); never.got.Spec.ExpiresAt != nil ||
		!strings.Contains(body, `"expires_at":null`) {
		t.Errorf("create without an expiry = %d %s, spec %+v; want one that never expires", res.StatusCode, body, never.got.Spec)
	}
}

func TestCreateAPITokenProblems(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "name", Code: shared.FieldRequired, Message: "is required"})
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid", invalid, 422, "validation_failed"},
		{"wrong password", domain.ErrCurrentPasswordIncorrect, 422, "identity.current_password_incorrect"},
		{"hasher busy", shared.ServerBusy(time.Second), 503, "server_busy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newServer(t, httpadapter.UseCases{CreateAPIToken: &fakeCreateToken{err: tt.err}})

			res, body := do(t, h, withToken(postJSON("/api/v0/me/api-tokens", `{"name":"","current_password":"x"}`)))

			if res.StatusCode != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
				t.Errorf("create = %d %s, want %d %s", res.StatusCode, body, tt.status, tt.code)
			}
		})
	}
}

// password_user counts attempts per account, before the use case runs: a
// wrong password costs a unit like a right one.
func TestCreateAPITokenIsLimitedPerAccount(t *testing.T) {
	uc := &fakeCreateToken{err: domain.ErrCurrentPasswordIncorrect}
	var logs bytes.Buffer
	h := limitedServer(t, httpadapter.UseCases{CreateAPIToken: uc}, &logs)
	create := func() (*http.Response, string) {
		return do(t, h, withToken(postJSON("/api/v0/me/api-tokens", `{"name":"CI","current_password":"x"}`)))
	}

	create()
	create()
	res, body := create()

	if res.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, `"code":"rate_limited"`) || uc.calls != 2 {
		t.Errorf("third attempt = %d %s after %d calls; want 429 rate_limited, the use case called twice", res.StatusCode, body, uc.calls)
	}
	if entries := rateLimitLogs(t, &logs); len(entries) != 1 || entries[0]["bucket"] != "password_user" {
		t.Errorf("rate limited logs = %v, want password_user", entries)
	}
}

func TestRevokeAPIToken(t *testing.T) {
	uc := &fakeRevokeToken{}
	h := newServer(t, httpadapter.UseCases{RevokeAPIToken: uc})

	res, body := do(t, h, withToken(httptest.NewRequest(http.MethodDelete, "/api/v0/api-tokens/"+tokenIDText, nil)))

	if res.StatusCode != http.StatusNoContent || body != "" || uc.id != uuid.MustParse(tokenIDText) {
		t.Errorf("revoke = %d %q, id %v; want 204 of %s", res.StatusCode, body, uc.id, tokenIDText)
	}
}

func TestRevokeAPITokenNotFound(t *testing.T) {
	h := newServer(t, httpadapter.UseCases{RevokeAPIToken: &fakeRevokeToken{err: domain.ErrAPITokenNotFound}})

	res, body := do(t, h, withToken(httptest.NewRequest(http.MethodDelete, "/api/v0/api-tokens/"+tokenIDText, nil)))

	if res.StatusCode != http.StatusNotFound || !strings.Contains(body, `"code":"identity.api_token_not_found"`) {
		t.Errorf("revoke = %d %s, want 404 identity.api_token_not_found", res.StatusCode, body)
	}
}
