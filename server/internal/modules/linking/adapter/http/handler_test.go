package httpadapter_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"uuid"

	httpadapter "github.com/open-nerve/NerveWiki/server/internal/modules/linking/adapter/http"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/linking/domain"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/apitest"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver/httpservertest"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// fakeAuth takes "session" for a sign-in session's access token, alice's.
type fakeAuth struct{}

func (fakeAuth) Authenticate(ctx context.Context, token string) (context.Context, string, error) {
	if token != "session" {
		return nil, "", shared.Unauthenticated()
	}
	return shared.WithActor(ctx, shared.Actor{UserID: id(1), SessionID: id(2)}), token, nil
}

// id is the fixtures' id ending in n.
func id(n int) uuid.UUID {
	return uuid.MustParse("0199a2b4-0000-7000-8000-0000000000" + string(rune('0'+n/10)) + string(rune('0'+n%10)))
}

// fakes are the use cases: each records what it got and answers err, or
// the fixtures' answer; next is the backlinks' next cursor.
type fakes struct {
	err  error
	next string
	got  []any
}

type (
	fakeBacklinks  struct{ *fakes }
	fakeProperties struct{ *fakes }
	fakeTags       struct{ *fakes }
	fakeTag        struct{ *fakes }
	fakeTargets    struct{ *fakes }
)

func (f fakeBacklinks) Execute(_ context.Context, pageID uuid.UUID, limit *int, cursor *string) (app.Backlinks, error) {
	f.got = []any{pageID, limit, cursor}
	return app.Backlinks{
		Pages: []app.Backlinking{
			{PageID: id(13), Links: 2, Contexts: []string{"see [[Notes]]", "…[[Notes]]…"}},
			{PageID: id(14), Links: 1, Contexts: []string{}},
		},
		NextCursor: f.next,
	}, f.err
}

func (f fakeProperties) Execute(_ context.Context, pageID uuid.UUID) (app.Properties, error) {
	f.got = []any{pageID}
	return app.Properties{
		Valid: true,
		Properties: []app.Property{
			{Key: "up", Value: json.RawMessage(`"[[Parent]]"`)},
			{Key: "big", Value: json.RawMessage(`1000000000000000000000`)},
			{Key: "sources", Value: json.RawMessage(`["[[A]]", "[[B]]"]`)},
		},
		Links: []app.PropertyLink{{Key: "up", NodeID: id(11)}, {Key: "sources.0"}},
	}, f.err
}

func (f fakeTags) Execute(_ context.Context, notebookID uuid.UUID) ([]app.Tag, error) {
	f.got = []any{notebookID}
	return []app.Tag{{Tag: "a/b", Pages: 1}, {Tag: "Project", Pages: 3}}, f.err
}

func (f fakeTag) Execute(_ context.Context, notebookID uuid.UUID, name string) ([]uuid.UUID, error) {
	f.got = []any{notebookID, name}
	return []uuid.UUID{id(12), id(13)}, f.err
}

func (f fakeTargets) Execute(_ context.Context, notebookID uuid.UUID) ([]app.LinkTarget, error) {
	f.got = []any{notebookID}
	return []app.LinkTarget{
		{ID: id(11), Name: "Parent", Link: "Parent", Aliases: []string{}},
		{ID: id(12), Name: "Notes", Link: "Parent/Notes", Aliases: []string{"N"}},
	}, f.err
}

// serve mounts the module on f behind the platform's middlewares.
func (f *fakes) serve(t *testing.T) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}})
	httpadapter.Register(router, api, httpadapter.UseCases{
		ListBacklinks: fakeBacklinks{f}, GetPageProperties: fakeProperties{f}, ListTags: fakeTags{f}, GetTag: fakeTag{f},
		ListLinkTargets: fakeTargets{f},
	})
	return router
}

// call sends GET path with the session's token, and checks the answer
// against the contract.
func call(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer session")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	apitest.Load(t).CheckResponse(t, req, res)
	got, _ := io.ReadAll(res.Body)
	return res.StatusCode, strings.TrimSuffix(string(got), "\n")
}

const (
	pagePath       = "/api/v0/pages/0199a2b4-0000-7000-8000-000000000012"
	backlinksPath  = pagePath + "/backlinks"
	propertiesPath = pagePath + "/properties"
	notebookPath   = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010"
	tagsPath       = notebookPath + "/tags"
	targetsPath    = notebookPath + "/link-targets"
)

func TestTheOperationsAnswerTheUseCases(t *testing.T) {
	ten, cursor := 10, "abc"
	for _, tt := range []struct {
		name, path, next string
		want             string
		got              []any
	}{
		{"the first and last backlinks", backlinksPath, "",
			`{"data":[{"contexts":["see [[Notes]]","…[[Notes]]…"],"count":2,"id":"0199a2b4-0000-7000-8000-000000000013"},` +
				`{"contexts":[],"count":1,"id":"0199a2b4-0000-7000-8000-000000000014"}],"next_cursor":null}`,
			[]any{id(12), (*int)(nil), (*string)(nil)}},
		{"backlinks after a cursor, more to come", backlinksPath + "?limit=10&cursor=abc", "def",
			`{"data":[{"contexts":["see [[Notes]]","…[[Notes]]…"],"count":2,"id":"0199a2b4-0000-7000-8000-000000000013"},` +
				`{"contexts":[],"count":1,"id":"0199a2b4-0000-7000-8000-000000000014"}],"next_cursor":"def"}`,
			[]any{id(12), &ten, &cursor}},
		// A value is written as the index has it: a large number keeps its digits.
		{"a page's properties", propertiesPath, "",
			`{"links":[{"key":"up","node_id":"0199a2b4-0000-7000-8000-000000000011"},{"key":"sources.0","node_id":null}],` +
				`"properties":[{"key":"up","value":"[[Parent]]"},{"key":"big","value":1000000000000000000000},` +
				`{"key":"sources","value":["[[A]]","[[B]]"]}],"valid":true}`,
			[]any{id(12)}},
		{"a notebook's tags", tagsPath, "", `{"data":[{"count":1,"tag":"a/b"},{"count":3,"tag":"Project"}]}`, []any{id(10)}},
		{"a nested tag's pages", tagsPath + "/a%2Fb", "",
			`{"data":[{"id":"0199a2b4-0000-7000-8000-000000000012"},{"id":"0199a2b4-0000-7000-8000-000000000013"}]}`,
			[]any{id(10), "a/b"}},
		{"a tag of no page's", tagsPath + "/%23a%20b", "",
			`{"data":[{"id":"0199a2b4-0000-7000-8000-000000000012"},{"id":"0199a2b4-0000-7000-8000-000000000013"}]}`,
			[]any{id(10), "#a b"}},
		{"a notebook's link targets", targetsPath, "",
			`{"data":[{"aliases":[],"id":"0199a2b4-0000-7000-8000-000000000011","kind":"page","link":"Parent","name":"Parent"},` +
				`{"aliases":["N"],"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","link":"Parent/Notes","name":"Notes"}]}`,
			[]any{id(10)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{next: tt.next}
			status, body := call(t, f.serve(t), tt.path)
			if status != http.StatusOK || body != tt.want {
				t.Errorf("got %d %s\nwant %s", status, body, tt.want)
			}
			if !reflect.DeepEqual(f.got, tt.got) {
				t.Errorf("the use case got %#v, want %#v", f.got, tt.got)
			}
		})
	}
}

// The use cases' errors are answered as problems with their codes.
func TestTheUseCasesErrorsAreProblems(t *testing.T) {
	_, invalidLimit := shared.PageSize(new(101))
	for _, tt := range []struct {
		name, path string
		err        error
		status     int
		code       string
	}{
		{"a cursor the list cannot read", backlinksPath + "?cursor=x", shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{"backlinks of no page", backlinksPath, domain.ErrPageNotFound, http.StatusNotFound, "page.not_found"},
		{"a limit out of range", backlinksPath + "?limit=101", invalidLimit, http.StatusUnprocessableEntity, "validation_failed"},
		{"properties of no page", propertiesPath, domain.ErrPageNotFound, http.StatusNotFound, "page.not_found"},
		{"tags of no notebook", tagsPath, domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{"a tag of no notebook", tagsPath + "/a", domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{"link targets of no notebook", targetsPath, domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status, body := call(t, (&fakes{err: tt.err}).serve(t), tt.path)
			var p struct{ Code string }
			if err := json.Unmarshal([]byte(body), &p); err != nil || status != tt.status || p.Code != tt.code {
				t.Errorf("got %d %s, want %d %s", status, body, tt.status, tt.code)
			}
		})
	}
}

// A limit that is no integer, or an id that is no uuid, is a bad request
// before any use case.
func TestParametersThatDoNotBindAreBadRequests(t *testing.T) {
	for _, path := range []string{backlinksPath + "?limit=x", "/api/v0/pages/x/properties", "/api/v0/notebooks/x/tags/a"} {
		f := &fakes{}
		if status, body := call(t, f.serve(t), path); status != http.StatusBadRequest || f.got != nil {
			t.Errorf("%s: %d %s, the use case got %v", path, status, body, f.got)
		}
	}
}
