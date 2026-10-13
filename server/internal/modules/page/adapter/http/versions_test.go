package httpadapter_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/page/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/page/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

type (
	fakeVersions    struct{ *fakes }
	fakeVersion     struct{ *fakes }
	fakeVersionView struct{ *fakes }
	fakeRestore     struct{ *fakes }
)

// version is the version the fakes answer: an edit session's, 3 on 1,
// being edited, by alice from the API.
func version() domain.Version {
	base := 1
	return domain.Version{ID: id(30), Base: &base, Revision: 3, ByteSize: 4, CreatedAt: at(), UpdatedAt: at().Add(time.Minute),
		Editing: true, Changeset: domain.VersionChangeset{ID: id(31), Kind: domain.ChangesetEdit, Client: domain.ClientAPI, By: id(1),
			ByName: "Alice", CreatedAt: at()}}
}

const (
	versionJSON = `{"base_revision":1,"byte_size":4,"changeset":` + changesetJSON + `,"created_at":"2026-10-02T10:00:00Z",` +
		`"editing":true,"id":"0199a2b4-0000-7000-8000-000000000030","revision":3,"updated_at":"2026-10-02T10:01:00Z"}`
	// versionContentJSON is the version with its content.
	versionContentJSON = `{"base_revision":1,"byte_size":4,"changeset":` + changesetJSON + `,"content":"a\r\nb",` +
		`"created_at":"2026-10-02T10:00:00Z","editing":true,"id":"0199a2b4-0000-7000-8000-000000000030","revision":3,` +
		`"updated_at":"2026-10-02T10:01:00Z"}`
	changesetJSON = `{"client":"api","created_at":"2026-10-02T10:00:00Z","created_by":{"display_name":"Alice",` +
		`"user_id":"0199a2b4-0000-7000-8000-000000000001"},"id":"0199a2b4-0000-7000-8000-000000000031","kind":"edit"}`
)

// Execute answers the version and, but for a cursor, one that created its
// page, of a restore, and the next cursor "next".
func (f fakeVersions) Execute(_ context.Context, pageID uuid.UUID, limit *int, cursor *string) (app.VersionList, error) {
	f.got = []any{pageID, limit, cursor}
	if cursor != nil && *cursor == "end" {
		return app.VersionList{}, f.err
	}
	if cursor != nil {
		return app.VersionList{Versions: []domain.Version{version()}}, f.err
	}
	first := version()
	first.ID, first.Base, first.Revision, first.Editing = id(32), nil, 1, false
	first.Changeset.Kind, first.Changeset.Client = domain.ChangesetVersionRestore, "mcp:claude-code"
	return app.VersionList{Versions: []domain.Version{version(), first}, NextCursor: "next"}, f.err
}

func (f fakeVersion) Execute(_ context.Context, pageID uuid.UUID, revision int) (app.VersionContent, error) {
	f.got = []any{pageID, revision}
	return app.VersionContent{Version: version(), Content: "a\r\nb"}, f.err
}

func (f fakeVersionView) Execute(_ context.Context, pageID uuid.UUID, revision int) (app.ReadingView, error) {
	f.got = []any{pageID, revision}
	return app.ReadingView{HTML: "<p>a</p>\n", Revision: 3, Expires: time.Date(2026, 10, 9, 14, 0, 0, 0, time.UTC)}, f.err
}

// Execute answers a restore that wrote, but for base 9, which the page
// holds already.
func (f fakeRestore) Execute(_ context.Context, pageID uuid.UUID, revision, base int, client domain.Client) (app.VersionRestored, error) {
	f.got = []any{pageID, revision, base, client}
	if base == 9 {
		return app.VersionRestored{Revision: 9}, f.err
	}
	return app.VersionRestored{Revision: 4, ChangesetID: id(33)}, f.err
}

const (
	versionsPath = pagePath + "/versions"
	versionPath  = versionsPath + "/2"
	restorePath  = versionPath + "/restore"
)

func TestTheVersionOperationsAnswerTheUseCases(t *testing.T) {
	limit, cursor, end := 2, "abc", "end"
	for _, tt := range []struct {
		name, token, method, path, body string
		want                            string
		got                             []any
	}{
		{"a page's versions", "session", http.MethodGet, versionsPath, "", `{"data":[` + versionJSON + `,{"base_revision":null,` +
			`"byte_size":4,"changeset":{"client":"mcp:claude-code","created_at":"2026-10-02T10:00:00Z","created_by":{"display_name":` +
			`"Alice","user_id":"0199a2b4-0000-7000-8000-000000000001"},"id":"0199a2b4-0000-7000-8000-000000000031","kind":` +
			`"version_restore"},"created_at":"2026-10-02T10:00:00Z","editing":false,"id":"0199a2b4-0000-7000-8000-000000000032",` +
			`"revision":1,"updated_at":"2026-10-02T10:01:00Z"}],"next_cursor":"next"}`, []any{id(12), (*int)(nil), (*string)(nil)}},
		{"a page of them", "session", http.MethodGet, versionsPath + "?limit=2&cursor=abc", "", `{"data":[` + versionJSON +
			`],"next_cursor":null}`, []any{id(12), &limit, &cursor}},
		{"none left", "session", http.MethodGet, versionsPath + "?cursor=end", "", `{"data":[],"next_cursor":null}`,
			[]any{id(12), (*int)(nil), &end}},
		{"a version", "session", http.MethodGet, versionPath, "", versionContentJSON,
			[]any{id(12), 2}},
		{"a version's view", "session", http.MethodGet, versionPath + "/view", "",
			`{"assets_expire_at":"2026-10-09T14:00:00Z","html":"\u003cp\u003ea\u003c/p\u003e\n","revision":3}`, []any{id(12), 2}},
		{"a restore by the web", "session", http.MethodPost, restorePath, `{"base_revision":3}`,
			`{"changed":true,"changeset_id":"0199a2b4-0000-7000-8000-000000000033","revision":4}`, []any{id(12), 2, 3, domain.ClientWeb}},
		{"a restore that wrote nothing, by the API", "pat", http.MethodPost, restorePath, `{"base_revision":9}`,
			`{"changed":false,"changeset_id":null,"revision":9}`, []any{id(12), 2, 9, domain.ClientAPI}},
		{"a version at revision 0", "session", http.MethodGet, versionsPath + "/0", "", versionContentJSON, []any{id(12), 0}},
		{"a version at a negative revision", "session", http.MethodGet, versionsPath + "/-3", "", versionContentJSON,
			[]any{id(12), -3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{}
			status, body := call(t, f.serve(t), tt.token, tt.method, tt.path, tt.body)
			if status != http.StatusOK || body != tt.want {
				t.Errorf("%s %s %s = %d %s, want 200 %s", tt.method, tt.path, tt.body, status, body, tt.want)
			}
			if !reflect.DeepEqual(f.got, tt.got) {
				t.Errorf("the use case got %v, want %v", f.got, tt.got)
			}
		})
	}
}

// Every code the version operations declare, as the use cases answer it.
func TestTheVersionOperationsAnswerEachProblem(t *testing.T) {
	busy := shared.ServerBusy(time.Second)
	restore := `{"base_revision":3}`
	for _, tt := range []struct {
		method, path, body string
		err                error
		status             int
		code               string
	}{
		{http.MethodGet, versionsPath, "", shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{http.MethodGet, versionsPath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, versionsPath, "", shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: "must be between 1 and 100"}), http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodGet, versionPath, "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, versionPath, "", domain.ErrVersionNotFound, http.StatusNotFound, "page.version_not_found"},
		{http.MethodGet, versionPath + "/view", "", domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, versionPath + "/view", "", domain.ErrVersionNotFound, http.StatusNotFound, "page.version_not_found"},
		{http.MethodGet, versionPath + "/view", "", busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodPost, restorePath, restore, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPost, restorePath, restore, domain.ErrVersionNotFound, http.StatusNotFound, "page.version_not_found"},
		{http.MethodPost, restorePath, restore, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, restorePath, restore, busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodPost, restorePath, restore, domain.ErrRevisionMismatch, http.StatusConflict, "page.revision_mismatch"},
		{http.MethodPost, restorePath, restore, domain.Locked(id(12), id(2), "Bob"), http.StatusConflict, "page.locked"},
	} {
		status, body := call(t, (&fakes{err: tt.err}).serve(t), "session", tt.method, tt.path, tt.body)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s: %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, tt.code)
		}
	}
}

// A path's page id is a uuid and its revision an integer, a restore's
// body has its base_revision and no other member: anything else is the
// boundary's 400, before any use case.
func TestAVersionsRequestTheBoundaryRefuses(t *testing.T) {
	for _, tt := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v0/pages/notes/versions", ""},
		{http.MethodGet, versionsPath + "/x", ""},
		{http.MethodGet, versionsPath + "/1.5", ""},
		{http.MethodGet, versionsPath + "/99999999999999999999", ""},
		{http.MethodGet, versionsPath + "/x/view", ""},
		{http.MethodGet, versionsPath + "?limit=x", ""},
		{http.MethodPost, versionsPath + "/x/restore", `{"base_revision":1}`},
		{http.MethodPost, restorePath, `{}`},
		{http.MethodPost, restorePath, `{"base_revision":"1"}`},
		{http.MethodPost, restorePath, `{"base_revision":1,"content":"x"}`},
		{http.MethodPost, restorePath, `{"base_revision":null}`},
		{http.MethodPost, restorePath, `{"base_revision":1.5}`},
		{http.MethodPost, restorePath, ""},
	} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", tt.method, tt.path, tt.body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s %s %s = %d %s, use case got %v; want 400 bad_request, the use case not called", tt.method, tt.path, tt.body,
				status, answer, f.got)
		}
	}
}
