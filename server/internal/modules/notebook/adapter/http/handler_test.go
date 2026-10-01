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

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/notebook/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeAuth accepts the token "valid".
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	if token != "valid" {
		return nil, "", shared.Unauthenticated()
	}
	session := uuid.MustParse("0199a2b4-0000-7000-8000-000000000002")
	user := uuid.MustParse("0199a2b4-0000-7000-8000-000000000001")
	return shared.WithActor(ctx, shared.Actor{UserID: user, SessionID: session}), "session:" + session.String(), nil
}

// notebookID is the id of every notebook the fakes answer.
func notebookID() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-00000000000b") }

func view(role shared.NotebookRole) app.View {
	at := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	return app.View{
		Notebook: domain.Notebook{ID: notebookID(), WorkspaceID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000a"),
			Name: "Engineering", Access: shared.AccessViewer, CreatedAt: at, UpdatedAt: at},
		Role: role, MemberCount: 2,
	}
}

const engineeringJSON = `{"created_at":"2026-10-02T10:00:00Z","id":"0199a2b4-0000-7000-8000-00000000000b","member_count":2,` +
	`"name":"Engineering","role":"%s","updated_at":"2026-10-02T10:00:00Z",` +
	`"workspace_access":"viewer","workspace_id":"0199a2b4-0000-7000-8000-00000000000a"}`

func engineering(role string) string { return strings.Replace(engineeringJSON, "%s", role, 1) }

// The fake use cases record what they got and answer err, or a view.
type fakeUseCase struct {
	got []any
	err error
}

func (f *fakeUseCase) answer(role shared.NotebookRole, got ...any) (app.View, error) {
	f.got = got
	if f.err != nil {
		return app.View{}, f.err
	}
	return view(role), nil
}

type fakeList struct{ fakeUseCase }

func (f *fakeList) Execute(_ context.Context, slug string) ([]app.View, error) {
	f.got = []any{slug}
	if f.err != nil {
		return nil, f.err
	}
	return []app.View{view(shared.NotebookReader)}, nil
}

type fakeCreate struct{ fakeUseCase }

func (f *fakeCreate) Execute(_ context.Context, slug, name string, access *string) (app.View, error) {
	return f.answer(shared.NotebookAdmin, slug, name, access)
}

type fakeGet struct{ fakeUseCase }

func (f *fakeGet) Execute(_ context.Context, id uuid.UUID) (app.View, error) {
	return f.answer(shared.NotebookEditor, id)
}

type fakeUpdate struct{ fakeUseCase }

func (f *fakeUpdate) Execute(_ context.Context, id uuid.UUID, name, access *string) (app.View, error) {
	return f.answer(shared.NotebookAdmin, id, name, access)
}

type fakeDelete struct{ fakeUseCase }

func (f *fakeDelete) Execute(_ context.Context, id uuid.UUID) error {
	_, err := f.answer("", id)
	return err
}

// fakes are one of each use case, every one answering err.
type fakes struct {
	list   *fakeList
	create *fakeCreate
	get    *fakeGet
	update *fakeUpdate
	delete *fakeDelete
}

func newFakes(err error) fakes {
	u := fakeUseCase{err: err}
	return fakes{&fakeList{u}, &fakeCreate{u}, &fakeGet{u}, &fakeUpdate{u}, &fakeDelete{u}}
}

// serve mounts the module on f behind the platform's middlewares.
func (f fakes) serve(t *testing.T) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	httpadapter.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}}), httpadapter.UseCases{
		ListNotebooks: f.list, CreateNotebook: f.create, GetNotebook: f.get, UpdateNotebook: f.update, DeleteNotebook: f.delete,
	})
	return router
}

// call sends method path with body and checks the answer against the
// contract.
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

const notebookPath = "/api/v0/notebooks/0199a2b4-0000-7000-8000-00000000000b"

func TestTheOperationsAnswerTheUseCases(t *testing.T) {
	f := newFakes(nil)
	h := f.serve(t)
	viewer := "viewer"
	for _, tt := range []struct {
		method, path, body string
		status             int
		want               string
		got                func() []any
		wantGot            []any
	}{
		{http.MethodGet, "/api/v0/workspaces/acme/notebooks", "", http.StatusOK, `{"data":[` + engineering("reader") + "]}",
			func() []any { return f.list.got }, []any{"acme"}},
		{http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":" Notes ","workspace_access":"viewer"}`, http.StatusCreated,
			engineering("admin"), func() []any { return f.create.got }, []any{"acme", " Notes ", &viewer}},
		{http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"Notes"}`, http.StatusCreated,
			engineering("admin"), func() []any { return f.create.got }, []any{"acme", "Notes", (*string)(nil)}},
		{http.MethodGet, notebookPath, "", http.StatusOK, engineering("editor"),
			func() []any { return f.get.got }, []any{notebookID()}},
		{http.MethodPatch, notebookPath, `{"workspace_access":"viewer"}`, http.StatusOK, engineering("admin"),
			func() []any { return f.update.got }, []any{notebookID(), (*string)(nil), &viewer}},
		{http.MethodPatch, notebookPath, `{}`, http.StatusOK, engineering("admin"),
			func() []any { return f.update.got }, []any{notebookID(), (*string)(nil), (*string)(nil)}},
		{http.MethodDelete, notebookPath, "", http.StatusNoContent, "",
			func() []any { return f.delete.got }, []any{notebookID()}},
	} {
		status, body := call(t, h, tt.method, tt.path, tt.body)
		want := tt.want
		if want != "" {
			want += "\n"
		}
		if status != tt.status || body != want {
			t.Errorf("%s %s %s = %d %s, want %d %s", tt.method, tt.path, tt.body, status, body, tt.status, want)
		}
		if got := tt.got(); !sameArgs(got, tt.wantGot) {
			t.Errorf("%s %s %s: the use case got %v, want %v", tt.method, tt.path, tt.body, got, tt.wantGot)
		}
	}
}

// sameArgs compares the use cases' arguments, the strings behind pointers.
func sameArgs(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, xs := a[i].(*string)
		y, ys := b[i].(*string)
		switch {
		case xs != ys:
			return false
		case xs && ((x == nil) != (y == nil) || x != nil && *x != *y):
			return false
		case !xs && a[i] != b[i]:
			return false
		}
	}
	return true
}

// Every code the operations declare, as the use cases answer it.
func TestTheOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "name", Code: shared.FieldInvalidFormat, Message: "must not contain /"})
	for _, tt := range []struct {
		method, path, body string
		err                error
		status             int
		code               string
	}{
		{http.MethodGet, "/api/v0/workspaces/acme/notebooks", "", domain.ErrWorkspaceNotFound, http.StatusNotFound, "workspace.not_found"},
		{http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"a/b"}`, domain.ErrWorkspaceNotFound, http.StatusNotFound, "workspace.not_found"},
		{http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"a/b"}`, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, "/api/v0/workspaces/acme/notebooks", `{"name":"a/b"}`, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodGet, notebookPath, "", domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPatch, notebookPath, `{"name":"a/b"}`, domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPatch, notebookPath, `{"name":"a/b"}`, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPatch, notebookPath, `{"name":"a/b"}`, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodDelete, notebookPath, "", domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodDelete, notebookPath, "", shared.Forbidden(), http.StatusForbidden, "forbidden"},
	} {
		status, body := call(t, newFakes(tt.err).serve(t), tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s: %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, tt.code)
		}
	}
}

// A path's notebook id is a uuid: anything else is the boundary's 400,
// before any use case.
func TestANotebookIDThatIsNoUUID(t *testing.T) {
	f := newFakes(nil)
	status, body := call(t, f.serve(t), http.MethodGet, "/api/v0/notebooks/engineering", "")
	if status != http.StatusBadRequest || !strings.Contains(body, `"code":"bad_request"`) || f.get.got != nil {
		t.Errorf("GET = %d %s, use case got %v; want 400 bad_request, the use case not called", status, body, f.get.got)
	}
}
