package httpadapter_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/identity/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/identity/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The ids and the instant the tests use.
const (
	userIDText    = "0199a2b4-0000-7000-8000-000000000001"
	sessionIDText = "0199a2b4-0000-7000-8000-000000000002"
)

func created() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 123456000, time.UTC) }

type fakeRegister struct {
	got    app.RegisterInput
	tokens app.Tokens
	err    error
}

func (f *fakeRegister) Execute(_ context.Context, in app.RegisterInput) (app.Tokens, error) {
	f.got = in
	return f.tokens, f.err
}

// fakeGetMe answers the actor's account with steps.
type fakeGetMe struct{ steps []string }

func (f fakeGetMe) Execute(ctx context.Context) (domain.User, error) {
	actor, err := shared.RequireActor(ctx)
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: actor.UserID, Email: "alice@corp.com", DisplayName: "alice", OnboardingSteps: f.steps}, nil
}

// fakeAuth accepts the token "valid" as a session of the account
// userIDText.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, error) {
	if token != "valid" {
		return nil, shared.Unauthenticated()
	}
	return shared.WithActor(ctx, shared.Actor{UserID: uuid.MustParse(userIDText), SessionID: uuid.MustParse(sessionIDText)}), nil
}

// newServer serves the module with uc; a nil use case gets an idle fake.
func newServer(t *testing.T, uc httpadapter.UseCases) http.Handler {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	router := httpserver.NewRouter(logger)
	api, err := httpserver.NewAPI(httpserver.APIConfig{
		Logger:           logger,
		Authenticator:    fakeAuth{},
		PublicOperations: httpadapter.PublicOperations(),
		MaxBodyBytes:     1024,
		RequestTimeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if uc.Register == nil {
		uc.Register = &fakeRegister{}
	}
	if uc.GetMe == nil {
		uc.GetMe = fakeGetMe{}
	}
	httpadapter.Register(router, api, uc)
	return router
}

func do(t *testing.T, h http.Handler, req *http.Request) (*http.Response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	apitest.Load(t).CheckResponse(t, req, res)
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

// registerRequest is a registration from 203.0.113.7 with agent/1.
func registerRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agent/1")
	req.RemoteAddr = "203.0.113.7:5555"
	return req
}

func TestRegisterAnswers201WithTheTokens(t *testing.T) {
	register := &fakeRegister{tokens: app.Tokens{
		AccessToken: "access", AccessExpiresIn: 15 * time.Minute, RefreshToken: "nwk_rt_x", RefreshExpiresAt: created().Add(720 * time.Hour),
	}}
	req := registerRequest(`{"email":"Alice@Corp.com","password":"Tr0ub4dor&3"}`)
	apitest.Load(t).CheckRequest(t, req)

	res, body := do(t, newServer(t, httpadapter.UseCases{Register: register}), req)

	want := `{"access_token":"access","access_token_expires_in":900,"refresh_token":"nwk_rt_x",` +
		`"refresh_token_expires_at":"2026-10-25T10:00:00.123456Z","token_type":"Bearer"}` + "\n"
	if res.StatusCode != http.StatusCreated || body != want {
		t.Errorf("POST /auth/register = %d %s, want 201 %s", res.StatusCode, body, want)
	}
	// The use case normalizes the address.
	wantIn := app.RegisterInput{Email: "Alice@Corp.com", Password: "Tr0ub4dor&3", UserAgent: "agent/1", IP: netip.MustParseAddr("203.0.113.7")}
	if register.got != wantIn {
		t.Errorf("use case got %+v, want %+v", register.got, wantIn)
	}
}

// The handler exit: every error the use case returns becomes its problem.
func TestRegisterProblems(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       string
		retryAfter string
	}{
		{"sign-up off", domain.ErrSignupDisabled, 403, "identity.signup_disabled", ""},
		{"invalid values", shared.Invalid(shared.FieldError{Field: "password", Code: shared.FieldCommonPassword, Message: "is too common"}), 422, "validation_failed", ""},
		{"address taken", domain.ErrEmailTaken, 409, "identity.email_taken", ""},
		{"hashing saturated", shared.ServerBusy(time.Second), 503, "server_busy", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := httpadapter.UseCases{Register: &fakeRegister{err: tt.err}}
			res, body := do(t, newServer(t, uc), registerRequest(`{"email":"a@b.co","password":"x"}`))

			if res.StatusCode != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) || res.Header.Get("Retry-After") != tt.retryAfter {
				t.Errorf("response = %d %s Retry-After %q, want %d %s", res.StatusCode, body, res.Header.Get("Retry-After"), tt.status, tt.code)
			}
		})
	}
}

// The body decoding exit: the structure check answers with fields, anything
// else with a generic detail, never a Go type name; the use case never runs.
func TestRegisterBodyProblems(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		want       string
	}{
		{"unknown and missing fields", `{"email":"a@b.co","extra":1}`, 400,
			`"errors":[{"field":"extra","code":"not_allowed","message":"is not a property of this request"},{"field":"password","code":"required","message":"is required"}]`},
		{"null for a string", `{"email":null,"password":"x"}`, 400, `"errors":[{"field":"email","code":"invalid_format"`},
		{"not JSON", `{"email":`, 400, `"detail":"The request body could not be decoded."`},
		{"empty", ``, 400, `"detail":"The request body could not be decoded."`},
		{"too large", `{"email":"` + strings.Repeat("a", 2000) + `"}`, 413, `"code":"payload_too_large"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			register := &fakeRegister{}
			res, body := do(t, newServer(t, httpadapter.UseCases{Register: register}), registerRequest(tt.body))

			if res.StatusCode != tt.status || !strings.Contains(body, tt.want) || strings.Contains(body, "Go struct") {
				t.Errorf("response = %d %s, want %d with %s", res.StatusCode, body, tt.status, tt.want)
			}
			if register.got != (app.RegisterInput{}) {
				t.Errorf("the use case ran with %+v", register.got)
			}
		})
	}
}

func TestGetMe(t *testing.T) {
	for _, tt := range []struct {
		steps []string
		want  string
	}{
		{nil, `[]`}, // no step yet: an array, never null
		{[]string{"profile", "workspace"}, `["profile","workspace"]`},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v0/me", nil)
		req.Header.Set("Authorization", "Bearer valid")
		apitest.Load(t).CheckRequest(t, req)

		res, body := do(t, newServer(t, httpadapter.UseCases{GetMe: fakeGetMe{steps: tt.steps}}), req)

		want := `{"display_name":"alice","email":"alice@corp.com","id":"` + userIDText + `","onboarding_steps":` + tt.want + `}` + "\n"
		if res.StatusCode != http.StatusOK || body != want {
			t.Errorf("GET /me = %d %s, want 200 %s", res.StatusCode, body, want)
		}
	}
}

func TestGetMeWithoutAValidToken(t *testing.T) {
	for _, header := range []string{"", "Bearer forged"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v0/me", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}

		res, body := do(t, newServer(t, httpadapter.UseCases{}), req)

		if res.StatusCode != http.StatusUnauthorized || !strings.Contains(body, `"code":"unauthorized"`) || res.Header.Get("WWW-Authenticate") == "" {
			t.Errorf("GET /me with %q = %d %s, want 401 unauthorized with a challenge", header, res.StatusCode, body)
		}
	}
}
