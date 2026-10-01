package httpadapter_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/workspace/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// alice is the caller of every request.
func alice() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-000000000001") }

// fakeAuth accepts the token "valid" as alice.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	if token != "valid" {
		return nil, "", shared.Unauthenticated()
	}
	session := uuid.MustParse("0199a2b4-0000-7000-8000-000000000002")
	return shared.WithActor(ctx, shared.Actor{UserID: alice(), SessionID: session}), "session:" + session.String(), nil
}

// The fake use cases answer what their field says, and record the caller.
type fakeList struct{ list []app.Membership }

func (f fakeList) Execute(context.Context) ([]app.Membership, error) { return f.list, nil }

type fakeCreate struct {
	got []string
	err error
}

func (f *fakeCreate) Execute(_ context.Context, name, slug string) (app.Membership, error) {
	f.got = []string{name, slug}
	if f.err != nil {
		return app.Membership{}, f.err
	}
	return membership(slug, shared.WorkspaceAdmin), nil
}

type fakeGet struct{ err error }

func (f fakeGet) Execute(_ context.Context, slug string) (app.Membership, error) {
	if f.err != nil {
		return app.Membership{}, f.err
	}
	return membership(slug, shared.WorkspaceMember), nil
}

type fakeCheck struct{ reason string }

func (f fakeCheck) Execute(context.Context, string) (string, error) { return f.reason, nil }

func membership(slug string, role shared.WorkspaceRole) app.Membership {
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	return app.Membership{
		Workspace: domain.Workspace{ID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000a"), Slug: slug, Name: "Acme", CreatedAt: at, UpdatedAt: at},
		Role:      role,
	}
}

// serve mounts the module with uc behind the platform's middlewares; the
// use cases left nil answer nothing a test here calls.
func serve(t *testing.T, uc httpadapter.UseCases) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	httpadapter.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}}), uc)
	return router
}

// call sends method path with body as alice and checks the answer against
// the contract.
func call(t *testing.T, h http.Handler, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	apitest.Load(t).CheckResponse(t, req, res)
	got, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(got)
}

const acmeJSON = `{"created_at":"2026-10-01T10:00:00Z","id":"0199a2b4-0000-7000-8000-00000000000a","name":"Acme",` +
	`"role":"%s","slug":"acme","updated_at":"2026-10-01T10:00:00Z"}`

func TestListWorkspaces(t *testing.T) {
	status, body := call(t, serve(t, httpadapter.UseCases{ListWorkspaces: fakeList{}}), http.MethodGet, "/api/v0/workspaces", "")
	if status != http.StatusOK || body != `{"data":[]}`+"\n" {
		t.Errorf("no workspace: %d %s, want 200 and an empty list", status, body)
	}
	list := []app.Membership{membership("acme", shared.WorkspaceGuest)}
	status, body = call(t, serve(t, httpadapter.UseCases{ListWorkspaces: fakeList{list: list}}), http.MethodGet, "/api/v0/workspaces", "")
	if want := `{"data":[` + strings.Replace(acmeJSON, "%s", "guest", 1) + "]}\n"; status != http.StatusOK || body != want {
		t.Errorf("one workspace: %d %s, want 200 %s", status, body, want)
	}
}

func TestCreateWorkspace(t *testing.T) {
	create := &fakeCreate{}
	status, body := call(t, serve(t, httpadapter.UseCases{CreateWorkspace: create}), http.MethodPost, "/api/v0/workspaces",
		`{"name":" Acme ","slug":"acme"}`)
	if want := strings.Replace(acmeJSON, "%s", "admin", 1) + "\n"; status != http.StatusCreated || body != want {
		t.Errorf("POST = %d %s, want 201 %s", status, body, want)
	}
	if len(create.got) != 2 || create.got[0] != " Acme " || create.got[1] != "acme" {
		t.Errorf("the use case got %q, want the name and the slug as sent", create.got)
	}
}

func TestCreateWorkspaceAnswersEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "slug", Code: shared.FieldNotAllowed, Message: "is reserved"})
	deactivated := shared.NewError(shared.KindForbidden, "identity.account_deactivated", "This account is deactivated.")
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrCreationDisabled, http.StatusForbidden, "workspace.creation_disabled"},
		{invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{domain.ErrSlugTaken, http.StatusConflict, "workspace.slug_taken"},
		{deactivated, http.StatusForbidden, "identity.account_deactivated"},
	} {
		h := serve(t, httpadapter.UseCases{CreateWorkspace: &fakeCreate{err: tt.err}})
		status, body := call(t, h, http.MethodPost, "/api/v0/workspaces", `{"name":"Acme","slug":"acme"}`)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s: %d %s, want %d", tt.code, status, body, tt.status)
		}
	}
}

func TestGetWorkspace(t *testing.T) {
	status, body := call(t, serve(t, httpadapter.UseCases{GetWorkspace: fakeGet{}}), http.MethodGet, "/api/v0/workspaces/acme", "")
	if want := strings.Replace(acmeJSON, "%s", "member", 1) + "\n"; status != http.StatusOK || body != want {
		t.Errorf("GET = %d %s, want 200 %s", status, body, want)
	}
	status, body = call(t, serve(t, httpadapter.UseCases{GetWorkspace: fakeGet{err: domain.ErrNotFound}}),
		http.MethodGet, "/api/v0/workspaces/acme", "")
	if status != http.StatusNotFound || !strings.Contains(body, `"code":"workspace.not_found"`) {
		t.Errorf("GET of a workspace not seen = %d %s, want 404 workspace.not_found", status, body)
	}
}

func TestCheckWorkspaceSlug(t *testing.T) {
	for reason, want := range map[string]string{
		"":                  `{"available":true}`,
		domain.SlugTaken:    `{"available":false,"reason":"taken"}`,
		domain.SlugReserved: `{"available":false,"reason":"reserved"}`,
		domain.SlugInvalid:  `{"available":false,"reason":"invalid"}`,
	} {
		status, body := call(t, serve(t, httpadapter.UseCases{CheckSlug: fakeCheck{reason: reason}}),
			http.MethodGet, "/api/v0/workspace-slugs/acme", "")
		if status != http.StatusOK || body != want+"\n" {
			t.Errorf("reason %q: %d %s, want 200 %s", reason, status, body, want)
		}
	}
}
