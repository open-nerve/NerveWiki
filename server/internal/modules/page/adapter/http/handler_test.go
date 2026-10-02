package httpadapter_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/page/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeAuth takes "session" for a sign-in session's access token and "pat"
// for a personal access token, both alice's.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	actor := shared.Actor{UserID: id(1)}
	switch token {
	case "session":
		actor.SessionID = id(2)
	case "pat":
		actor.APITokenID = id(3)
	default:
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, actor), token, nil
}

// id is the fixtures' id ending in n.
func id(n int) uuid.UUID {
	return uuid.MustParse("0199a2b4-0000-7000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

func at() time.Time { return time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC) }

// notes is the page the fakes answer: Notes under Parent in notebook 10.
func notes() domain.Node {
	parent := id(11)
	return domain.Node{ID: id(12), NotebookID: id(10), ParentID: &parent, Kind: domain.KindPage, Name: "Notes", NameKey: "notes",
		CreatedAt: at(), UpdatedAt: at()}
}

func notesView() app.PageView {
	return app.PageView{Node: notes(), Ancestors: []domain.Ancestor{{ID: id(11), Name: "Parent"}},
		Content: app.ContentMeta{Revision: 3, ByteSize: 42, UpdatedBy: id(1), UpdatedAt: at()}}
}

const (
	treeNodeJSON = `{"created_at":"2026-10-02T10:00:00Z","id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Notes",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","parent_id":"0199a2b4-0000-7000-8000-000000000011",` +
		`"updated_at":"2026-10-02T10:00:00Z"}`
	rootJSON = `{"created_at":"2026-10-02T10:00:00Z","id":"0199a2b4-0000-7000-8000-000000000011","kind":"page","name":"Parent",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","parent_id":null,"updated_at":"2026-10-02T10:00:00Z"}`
	pageJSON = `{"ancestors":[{"id":"0199a2b4-0000-7000-8000-000000000011","name":"Parent"}],"byte_size":42,` +
		`"content_updated_at":"2026-10-02T10:00:00Z","content_updated_by":"0199a2b4-0000-7000-8000-000000000001",` +
		`"created_at":"2026-10-02T10:00:00Z","id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Notes",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","parent_id":"0199a2b4-0000-7000-8000-000000000011",` +
		`"revision":3,"updated_at":"2026-10-02T10:00:00Z"}`
)

// fakes are the use cases: each records what it got and answers err, or
// the fixtures' page.
type fakes struct {
	err error
	got []any
}

type (
	fakeList   struct{ *fakes }
	fakeCreate struct{ *fakes }
	fakeGet    struct{ *fakes }
	fakeRename struct{ *fakes }
)

func (f fakeList) Execute(_ context.Context, notebookID uuid.UUID) ([]domain.Node, error) {
	f.got = []any{notebookID}
	parent := notes()
	parent.ID, parent.ParentID, parent.Name = id(11), nil, "Parent"
	return []domain.Node{parent, notes()}, f.err
}

func (f fakeCreate) Execute(_ context.Context, notebookID uuid.UUID, d app.PageDraft, client domain.Client) (app.PageView, error) {
	f.got = []any{notebookID, d, client}
	return notesView(), f.err
}

func (f fakeGet) Execute(_ context.Context, pageID uuid.UUID) (app.PageView, error) {
	f.got = []any{pageID}
	return notesView(), f.err
}

func (f fakeRename) Execute(_ context.Context, nodeID uuid.UUID, name string, client domain.Client) (domain.Node, error) {
	f.got = []any{nodeID, name, client}
	return notes(), f.err
}

// serve mounts the module on f behind the platform's middlewares.
func (f *fakes) serve(t *testing.T) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	httpadapter.Register(router, httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}}), httpadapter.UseCases{
		ListNodes: fakeList{f}, CreatePage: fakeCreate{f}, GetPage: fakeGet{f}, RenameNode: fakeRename{f},
	})
	return router
}

// call sends method path with body and token, and checks the answer
// against the contract.
func call(t *testing.T, h http.Handler, token, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	apitest.Load(t).CheckResponse(t, req, res)
	got, _ := io.ReadAll(res.Body)
	return res.StatusCode, strings.TrimSuffix(string(got), "\n")
}

const (
	nodesPath = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/nodes"
	pagesPath = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/pages"
	pagePath  = "/api/v0/pages/0199a2b4-0000-7000-8000-000000000012"
	nodePath  = "/api/v0/nodes/0199a2b4-0000-7000-8000-000000000012"
)

func TestTheOperationsAnswerTheUseCases(t *testing.T) {
	parent := id(11)
	for _, tt := range []struct {
		name, token, method, path, body string
		status                          int
		want                            string
		got                             []any
	}{
		{"the tree", "session", http.MethodGet, nodesPath, "", http.StatusOK, `{"data":[` + rootJSON + "," + treeNodeJSON + "]}",
			[]any{id(10)}},
		{"a page last under a parent", "session", http.MethodPost, pagesPath,
			`{"parent_id":"0199a2b4-0000-7000-8000-000000000011","title":" Notes "}`, http.StatusCreated, pageJSON,
			[]any{id(10), app.PageDraft{ParentID: &parent, Title: " Notes "}, domain.ClientWeb}},
		{"a page first at the root", "pat", http.MethodPost, pagesPath, `{"parent_id":null,"title":"Notes","after_id":null}`,
			http.StatusCreated, pageJSON, []any{id(10), app.PageDraft{Title: "Notes", Position: app.First()}, domain.ClientAPI}},
		{"a page after a sibling", "session", http.MethodPost, pagesPath,
			`{"parent_id":null,"title":"Notes","after_id":"0199a2b4-0000-7000-8000-000000000013"}`, http.StatusCreated, pageJSON,
			[]any{id(10), app.PageDraft{Title: "Notes", Position: app.After(id(13))}, domain.ClientWeb}},
		{"a page", "session", http.MethodGet, pagePath, "", http.StatusOK, pageJSON, []any{id(12)}},
		{"a rename by the web", "session", http.MethodPatch, nodePath, `{"name":"Notes"}`, http.StatusOK, treeNodeJSON,
			[]any{id(12), "Notes", domain.ClientWeb}},
		{"a rename by the API", "pat", http.MethodPatch, nodePath, `{"name":"Notes"}`, http.StatusOK, treeNodeJSON,
			[]any{id(12), "Notes", domain.ClientAPI}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{}
			status, body := call(t, f.serve(t), tt.token, tt.method, tt.path, tt.body)
			if status != tt.status || body != tt.want {
				t.Errorf("%s %s %s = %d %s, want %d %s", tt.method, tt.path, tt.body, status, body, tt.status, tt.want)
			}
			if !reflect.DeepEqual(f.got, tt.got) {
				t.Errorf("the use case got %v, want %v", f.got, tt.got)
			}
		})
	}
}

// A page's parent_id is required, null at the root: the boundary refuses
// a body without it before any use case.
func TestCreatePageRequiresTheParent(t *testing.T) {
	f := &fakes{}
	status, body := call(t, f.serve(t), "session", http.MethodPost, pagesPath, `{"title":"Notes"}`)
	if status != http.StatusBadRequest || !strings.Contains(body, `"code":"bad_request"`) || f.got != nil {
		t.Errorf("POST without parent_id = %d %s, use case got %v; want 400, the use case not called", status, body, f.got)
	}
}

// Every code the operations declare, as the use cases answer it.
func TestTheOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "title", Code: shared.FieldInvalidFormat, Message: "must not contain /"})
	create := `{"parent_id":null,"title":"a/b"}`
	for _, tt := range []struct {
		method, path, body string
		err                error
		status             int
		code               string
	}{
		{http.MethodGet, nodesPath, "", domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPost, pagesPath, create, domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodPost, pagesPath, create, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, pagesPath, create, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPost, pagesPath, create, domain.ErrTitleTaken, http.StatusConflict, "page.title_taken"},
		{http.MethodPost, pagesPath, create, domain.ErrTooDeep, http.StatusConflict, "page.too_deep"},
		{http.MethodGet, pagePath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, domain.ErrTitleTaken, http.StatusConflict, "page.title_taken"},
	} {
		status, body := call(t, (&fakes{err: tt.err}).serve(t), "session", tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s: %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, tt.code)
		}
	}
}

// A path's id is a uuid: anything else is the boundary's 400, before any
// use case.
func TestAnIDThatIsNoUUID(t *testing.T) {
	for _, path := range []string{"/api/v0/notebooks/eng/nodes", "/api/v0/pages/notes", "/api/v0/nodes/notes"} {
		f := &fakes{}
		method, body := http.MethodGet, ""
		if strings.HasPrefix(path, "/api/v0/nodes/") {
			method, body = http.MethodPatch, `{"name":"Notes"}`
		}
		status, answer := call(t, f.serve(t), "session", method, path, body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s %s = %d %s, use case got %v; want 400 bad_request, the use case not called", method, path, status, answer, f.got)
		}
	}
}
