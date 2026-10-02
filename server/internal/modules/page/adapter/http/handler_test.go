package httpadapter_test

import (
	"context"
	"crypto/sha256"
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
	fakeRead   struct{ *fakes }
	fakeWrite  struct{ *fakes }
	fakeView   struct{ *fakes }
	fakeOpen   struct{ *fakes }
	fakeBeat   struct{ *fakes }
	fakeEnd    struct{ *fakes }
	fakeRename struct{ *fakes }
	fakeMove   struct{ *fakes }
	fakeDelete struct{ *fakes }
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

func (f fakeRead) Execute(_ context.Context, pageID uuid.UUID) (app.PageContent, error) {
	f.got = []any{pageID}
	sum := sha256.Sum256([]byte("a\r\nb"))
	return app.PageContent{Content: "a\r\nb", Revision: 3, Hash: sum[:]}, f.err
}

func (f fakeWrite) Execute(_ context.Context, pageID uuid.UUID, p app.ContentPut, client domain.Client) (app.PageView, error) {
	f.got = []any{pageID, p, client}
	return notesView(), f.err
}

func (f fakeView) Execute(_ context.Context, pageID uuid.UUID) (app.ReadingView, error) {
	f.got = []any{pageID}
	return app.ReadingView{HTML: "<h1 id=\"nw-notes\">Notes</h1>\n", Revision: 3}, f.err
}

func (f fakeRename) Execute(_ context.Context, nodeID uuid.UUID, name string, client domain.Client) (domain.Node, error) {
	f.got = []any{nodeID, name, client}
	return notes(), f.err
}

func (f fakeMove) Execute(_ context.Context, nodeID uuid.UUID, to app.Destination, client domain.Client) (domain.Node, error) {
	f.got = []any{nodeID, to, client}
	return notes(), f.err
}

func (f fakeDelete) Execute(_ context.Context, nodeID uuid.UUID, client domain.Client) error {
	f.got = []any{nodeID, client}
	return f.err
}

func session() app.EditSession {
	return app.EditSession{ID: id(20), NodeID: id(12), NotebookID: id(10), UserID: id(1), Client: domain.ClientWeb, CreatedAt: at(),
		ExpiresAt: at().Add(time.Minute)}
}

const sessionJSON = `{"expires_at":"2026-10-02T10:01:00Z","id":"0199a2b4-0000-7000-8000-000000000020",` +
	`"page_id":"0199a2b4-0000-7000-8000-000000000012"}`

func (f fakeOpen) Execute(_ context.Context, pageID uuid.UUID, client domain.Client) (app.EditSession, error) {
	f.got = []any{pageID, client}
	return session(), f.err
}

func (f fakeBeat) Execute(_ context.Context, sessionID uuid.UUID) (app.EditSession, error) {
	f.got = []any{sessionID}
	return session(), f.err
}

func (f fakeEnd) Execute(_ context.Context, sessionID uuid.UUID) error {
	f.got = []any{sessionID}
	return f.err
}

// serve mounts the module on f behind the platform's middlewares.
func (f *fakes) serve(t *testing.T) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}, BodyLimits: httpadapter.BodyLimits()})
	httpadapter.Register(router, api, httpadapter.UseCases{
		ListNodes: fakeList{f}, CreatePage: fakeCreate{f}, GetPage: fakeGet{f}, GetPageContent: fakeRead{f},
		PutPageContent: fakeWrite{f}, GetPageView: fakeView{f}, RenameNode: fakeRename{f}, MoveNode: fakeMove{f},
		DeleteNode: fakeDelete{f}, OpenSession: fakeOpen{f}, Heartbeat: fakeBeat{f}, EndSession: fakeEnd{f},
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
	nodesPath   = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/nodes"
	pagesPath   = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/pages"
	pagePath    = "/api/v0/pages/0199a2b4-0000-7000-8000-000000000012"
	viewPath    = pagePath + "/view"
	contentPath = pagePath + "/content"
	openPath    = pagePath + "/edit-sessions"
	sessionPath = "/api/v0/edit-sessions/0199a2b4-0000-7000-8000-000000000020"
	beatPath    = sessionPath + "/heartbeat"
	nodePath    = "/api/v0/nodes/0199a2b4-0000-7000-8000-000000000012"
	movePath    = nodePath + "/move"
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
		{"a page with a content", "session", http.MethodPost, pagesPath, `{"parent_id":null,"title":"Notes","content":"# a\r\n"}`,
			http.StatusCreated, pageJSON, []any{id(10), app.PageDraft{Title: "Notes", Content: "# a\r\n"}, domain.ClientWeb}},
		{"a page", "session", http.MethodGet, pagePath, "", http.StatusOK, pageJSON, []any{id(12)}},
		{"a page's content", "session", http.MethodGet, contentPath, "", http.StatusOK,
			`{"content":"a\r\nb","content_hash":"18745f36a05e29072709042d6062ce54f1b08ff36c27ba80c39f81fb010c8ce2","revision":3}`, []any{id(12)}},
		{"a content written by the web in a session", "session", http.MethodPut, contentPath,
			`{"content":"a\r\nb","base_revision":2,"edit_session_id":"0199a2b4-0000-7000-8000-000000000020"}`, http.StatusOK, pageJSON,
			[]any{id(12), app.ContentPut{Content: "a\r\nb", Base: 2, EditSession: id(20)}, domain.ClientWeb}},
		{"a content written by the API", "pat", http.MethodPut, contentPath, `{"content":"","base_revision":2}`, http.StatusOK, pageJSON,
			[]any{id(12), app.ContentPut{Base: 2}, domain.ClientAPI}},
		{"a page's reading view", "session", http.MethodGet, viewPath, "", http.StatusOK,
			`{"html":"\u003ch1 id=\"nw-notes\"\u003eNotes\u003c/h1\u003e\n","revision":3}`, []any{id(12)}},
		{"a rename by the web", "session", http.MethodPatch, nodePath, `{"name":"Notes"}`, http.StatusOK, treeNodeJSON,
			[]any{id(12), "Notes", domain.ClientWeb}},
		{"a rename by the API", "pat", http.MethodPatch, nodePath, `{"name":"Notes"}`, http.StatusOK, treeNodeJSON,
			[]any{id(12), "Notes", domain.ClientAPI}},
		{"a move last under a parent", "session", http.MethodPost, movePath, `{"parent_id":"0199a2b4-0000-7000-8000-000000000011"}`,
			http.StatusOK, treeNodeJSON, []any{id(12), app.Destination{ParentID: &parent}, domain.ClientWeb}},
		{"a move first at the root", "pat", http.MethodPost, movePath, `{"parent_id":null,"after_id":null}`, http.StatusOK,
			treeNodeJSON, []any{id(12), app.Destination{Position: app.First()}, domain.ClientAPI}},
		{"a move after a sibling", "session", http.MethodPost, movePath,
			`{"parent_id":null,"after_id":"0199a2b4-0000-7000-8000-000000000013"}`, http.StatusOK, treeNodeJSON,
			[]any{id(12), app.Destination{Position: app.After(id(13))}, domain.ClientWeb}},
		{"an edit session by the web", "session", http.MethodPost, openPath, "", http.StatusCreated, sessionJSON,
			[]any{id(12), domain.ClientWeb}},
		{"an edit session by the API", "pat", http.MethodPost, openPath, "", http.StatusCreated, sessionJSON,
			[]any{id(12), domain.ClientAPI}},
		{"a heartbeat", "session", http.MethodPost, beatPath, "", http.StatusOK, sessionJSON, []any{id(20)}},
		{"an end", "session", http.MethodDelete, sessionPath, "", http.StatusNoContent, "", []any{id(20)}},
		{"a deletion by the web", "session", http.MethodDelete, nodePath, "", http.StatusNoContent, "",
			[]any{id(12), domain.ClientWeb}},
		{"a deletion by the API", "pat", http.MethodDelete, nodePath, "", http.StatusNoContent, "", []any{id(12), domain.ClientAPI}},
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

// A page's parent_id is required, null at the root, where it is created
// and where it moves: the boundary refuses a body without it before any
// use case.
func TestTheParentIsRequired(t *testing.T) {
	for path, body := range map[string]string{pagesPath: `{"title":"Notes"}`, movePath: `{"after_id":null}`} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", http.MethodPost, path, body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("POST %s without parent_id = %d %s, use case got %v; want 400, the use case not called", path, status, answer, f.got)
		}
	}
}

// Every code the operations declare, as the use cases answer it.
func TestTheOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "title", Code: shared.FieldInvalidFormat, Message: "must not contain /"})
	create := `{"parent_id":null,"title":"a/b"}`
	move := `{"parent_id":"0199a2b4-0000-7000-8000-000000000012"}`
	parentInvalid := domain.NotAllowed("parent_id", "The parent is no page of this notebook.")
	write := `{"content":"x","base_revision":1}`
	contentInvalid := domain.CheckContent("content", "\x00")
	busy := shared.ServerBusy(time.Second)
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
		{http.MethodPost, pagesPath, create, busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodGet, pagePath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, viewPath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, viewPath, "", busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodGet, contentPath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPut, contentPath, write, contentInvalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPut, contentPath, write, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPut, contentPath, write, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPut, contentPath, write, domain.ErrEditSessionEnded, http.StatusConflict, "page.edit_session_ended"},
		{http.MethodPut, contentPath, write, domain.ErrRevisionMismatch, http.StatusConflict, "page.revision_mismatch"},
		{http.MethodPut, contentPath, write, busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, invalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPatch, nodePath, `{"name":"a/b"}`, domain.ErrTitleTaken, http.StatusConflict, "page.title_taken"},
		{http.MethodPost, movePath, move, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPost, movePath, move, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, movePath, move, parentInvalid, http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPost, movePath, move, domain.ErrCycle, http.StatusConflict, "page.cycle"},
		{http.MethodPost, movePath, move, domain.ErrTitleTaken, http.StatusConflict, "page.title_taken"},
		{http.MethodPost, movePath, move, domain.ErrTooDeep, http.StatusConflict, "page.too_deep"},
		{http.MethodDelete, nodePath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodDelete, nodePath, "", shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, openPath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPost, openPath, "", shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, beatPath, "", domain.ErrEditSessionNotFound, http.StatusNotFound, "page.edit_session_not_found"},
		{http.MethodPost, beatPath, "", shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodDelete, sessionPath, "", domain.ErrEditSessionNotFound, http.StatusNotFound, "page.edit_session_not_found"},
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
	for _, tt := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v0/notebooks/eng/nodes", ""},
		{http.MethodGet, "/api/v0/pages/notes", ""},
		{http.MethodGet, "/api/v0/pages/notes/view", ""},
		{http.MethodGet, "/api/v0/pages/notes/content", ""},
		{http.MethodPut, "/api/v0/pages/notes/content", `{"content":"x","base_revision":1}`},
		{http.MethodPut, contentPath, `{"content":"x","base_revision":1,"edit_session_id":"s"}`},
		{http.MethodPatch, "/api/v0/nodes/notes", `{"name":"Notes"}`},
		{http.MethodPost, "/api/v0/nodes/notes/move", `{"parent_id":null}`},
		{http.MethodDelete, "/api/v0/nodes/notes", ""},
		{http.MethodPost, "/api/v0/pages/notes/edit-sessions", ""},
		{http.MethodPost, "/api/v0/edit-sessions/s/heartbeat", ""},
		{http.MethodDelete, "/api/v0/edit-sessions/s", ""},
	} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", tt.method, tt.path, tt.body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s %s = %d %s, use case got %v; want 400 bad_request, the use case not called", tt.method, tt.path, status, answer, f.got)
		}
	}
}

// A content reaches the use case byte for byte, but the boundary refuses
// one JSON cannot carry as text: a lone surrogate's escape, which a
// decoder would turn into U+FFFD, and bytes that are not UTF-8.
func TestAContentTheBoundaryRefuses(t *testing.T) {
	for name, body := range map[string]string{
		"a lone high surrogate": `{"content":"a\ud800b","base_revision":1}`,
		"a lone low surrogate":  `{"content":"a\udc00","base_revision":1}`,
		"two high surrogates":   `{"content":"\ud800\ud800","base_revision":1}`,
		"bytes not UTF-8":       "{\"content\":\"a\xffb\",\"base_revision\":1}",
	} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", http.MethodPut, contentPath, body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s: %d %s, use case got %v; want 400 bad_request, the use case not called", name, status, answer, f.got)
		}
	}
	f := &fakes{}
	if status, _ := call(t, f.serve(t), "session", http.MethodPut, contentPath, `{"content":"\ud83d\ude00","base_revision":1}`); status != http.StatusOK ||
		f.got[1].(app.ContentPut).Content != "\U0001F600" {
		t.Errorf("a surrogate pair = %d, the use case got %v; want 200 and the one character", status, f.got)
	}
}

// The two routes of a content take a body past the platform's limit, up to
// the largest content in JSON's longest escapes; the others keep it.
func TestTheContentsRoutesTakeALargerBody(t *testing.T) {
	big := strings.Repeat("a", 2<<20)
	for _, tt := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"a content's write", http.MethodPut, contentPath, `{"base_revision":1,"content":"` + big + `"}`, http.StatusOK},
		{"a page with a content", http.MethodPost, pagesPath, `{"parent_id":null,"title":"Notes","content":"` + big + `"}`, http.StatusCreated},
		{"a rename", http.MethodPatch, nodePath, `{"name":"` + big + `"}`, http.StatusRequestEntityTooLarge},
	} {
		f := &fakes{}
		if status, _ := call(t, f.serve(t), "session", tt.method, tt.path, tt.body); status != tt.want {
			t.Errorf("%s of 2 MiB = %d, want %d", tt.name, status, tt.want)
		}
	}
	limits := httpadapter.BodyLimits()
	if len(limits) != 2 || limits["PUT /api/v0/pages/{page_id}/content"] != 6*domain.MaxContentBytes+64<<10 {
		t.Errorf("BodyLimits() = %v, want the two routes at six bytes a content's byte and 64 KiB", limits)
	}
}
