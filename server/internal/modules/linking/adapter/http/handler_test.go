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
// the fixtures' answer; next is the backlinks' next cursor, landing the
// landing's answer.
type fakes struct {
	err     error
	next    string
	landing domain.Landing
	got     []any
}

type (
	fakeBacklinks  struct{ *fakes }
	fakeProperties struct{ *fakes }
	fakeTags       struct{ *fakes }
	fakeTag        struct{ *fakes }
	fakeTargets    struct{ *fakes }
	fakeLanding    struct{ *fakes }
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
			{Key: "cover", Value: json.RawMessage(`"[[x.png]]"`)},
		},
		Links: []app.PropertyLink{
			{Key: "up", NodeID: id(11)}, {Key: "sources.0"}, {Key: "cover", NodeID: id(15), Asset: true, URL: "/x?a=1&b=2"},
			{Key: "gone", NodeID: id(16), Asset: true},
		},
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
		{ID: id(15), Asset: true, Name: "x.png", Link: "Parent/x.png", Aliases: []string{}},
	}, f.err
}

func (f fakeLanding) Execute(_ context.Context, pageID uuid.UUID, target *string) (domain.Landing, error) {
	f.got = []any{pageID, target}
	return f.landing, f.err
}

// serve mounts the module on f behind the platform's middlewares.
func (f *fakes) serve(t *testing.T) http.Handler {
	t.Helper()
	router := httpserver.NewRouter(slog.New(slog.DiscardHandler))
	api := httpservertest.NewAPI(t, httpservertest.APIOptions{Authenticator: fakeAuth{}})
	httpadapter.Register(router, api, httpadapter.UseCases{
		ListBacklinks: fakeBacklinks{f}, GetPageProperties: fakeProperties{f}, ListTags: fakeTags{f}, GetTag: fakeTag{f},
		ListLinkTargets: fakeTargets{f}, GetLinkLanding: fakeLanding{f},
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
	landingPath    = pagePath + "/link-landing"
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
		// A value is written as the index has it: a large number keeps its
		// digits. A link's kind is its node's, none for none; an
		// attachment's has its address, if it is given one (M7/P3 design
		// 5.6).
		{"a page's properties", propertiesPath, "",
			`{"links":[{"key":"up","kind":"page","node_id":"0199a2b4-0000-7000-8000-000000000011","url":null},` +
				`{"key":"sources.0","kind":null,"node_id":null,"url":null},` +
				`{"key":"cover","kind":"asset","node_id":"0199a2b4-0000-7000-8000-000000000015","url":"/x?a=1\u0026b=2"},` +
				`{"key":"gone","kind":"asset","node_id":"0199a2b4-0000-7000-8000-000000000016","url":null}],` +
				`"properties":[{"key":"up","value":"[[Parent]]"},{"key":"big","value":1000000000000000000000},` +
				`{"key":"sources","value":["[[A]]","[[B]]"]},{"key":"cover","value":"[[x.png]]"}],"valid":true}`,
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
				`{"aliases":["N"],"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","link":"Parent/Notes","name":"Notes"},` +
				`{"aliases":[],"id":"0199a2b4-0000-7000-8000-000000000015","kind":"asset","link":"Parent/x.png","name":"x.png"}]}`,
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

// A landing answers one of its three fields, the others null; the target
// is the use case's as the query has it, nil when absent.
func TestTheLandingAnswersOneOfItsFields(t *testing.T) {
	target := "../a b.md"
	for _, tt := range []struct {
		name, path string
		landing    domain.Landing
		want       string
		got        []any
	}{
		{"a target that leads to a page", landingPath + "?target=x", domain.Landing{Node: id(13)},
			`{"landing":null,"node_id":"0199a2b4-0000-7000-8000-000000000013","reason":null}`, []any{id(12), new("x")}},
		{"a landing under a page", landingPath + "?target=..%2Fa%20b.md", domain.Landing{Parent: id(11), Title: "a b"},
			`{"landing":{"parent_id":"0199a2b4-0000-7000-8000-000000000011","title":"a b"},"node_id":null,"reason":null}`,
			[]any{id(12), &target}},
		{"a landing at the root", landingPath + "?target=", domain.Landing{Title: "x"},
			`{"landing":{"parent_id":null,"title":"x"},"node_id":null,"reason":null}`, []any{id(12), new("")}},
		{"no landing", landingPath, domain.Landing{Reason: domain.TooDeep},
			`{"landing":null,"node_id":null,"reason":"too_deep"}`, []any{id(12), (*string)(nil)}},
		{"a target read as an attachment's", landingPath + "?target=x.png", domain.Landing{Reason: domain.TargetIsAsset},
			`{"landing":null,"node_id":null,"reason":"target_is_asset"}`, []any{id(12), new("x.png")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{landing: tt.landing}
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
		{"a landing in no page", landingPath, domain.ErrPageNotFound, http.StatusNotFound, "page.not_found"},
		{"a reader's landing", landingPath + "?target=x", shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{"a landing without a target", landingPath, shared.Invalid(shared.FieldError{Field: "target", Code: shared.FieldRequired, Message: "is required"}),
			http.StatusUnprocessableEntity, "validation_failed"},
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
	for _, path := range []string{backlinksPath + "?limit=x", "/api/v0/pages/x/properties", "/api/v0/notebooks/x/tags/a", "/api/v0/pages/x/link-landing?target=a"} {
		f := &fakes{}
		if status, body := call(t, f.serve(t), path); status != http.StatusBadRequest || f.got != nil {
			t.Errorf("%s: %d %s, the use case got %v", path, status, body, f.got)
		}
	}
}
