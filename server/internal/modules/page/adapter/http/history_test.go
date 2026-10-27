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
	fakeChangesets      struct{ *fakes }
	fakeChangeset       struct{ *fakes }
	fakeChanges         struct{ *fakes }
	fakeRevertChangeset struct{ *fakes }
)

// loggedChangeset is the changeset the fakes answer: Alice's revert from
// an MCP client, with its message, of Bob's edit, reverted since; a rename
// of Notes and a write of Gone, in the trash.
func loggedChangeset() app.LoggedChangeset {
	parent, why, base := id(11), "Why", 2
	return app.LoggedChangeset{
		Changeset: domain.Changeset{ID: id(50), NotebookID: id(10), Kind: domain.ChangesetRevert, Client: "mcp:a", Message: &why, By: id(1),
			ByName: "Alice", CreatedAt: at(), UpdatedAt: at().Add(time.Minute), RevertsID: id(51),
			Reverts: &domain.ChangesetBrief{ID: id(51), Kind: domain.ChangesetEdit, By: id(2), ByName: "Bob", CreatedAt: at()}},
		Facts: domain.ChangesetFacts{Nodes: 1, Contents: 1, RevertedBy: []uuid.UUID{id(52)}},
		Changes: []domain.ChangeRow{
			{NodeID: id(12), NodeKind: domain.KindPage, Name: "Notes", Alive: true,
				Before: &domain.ChangePlace{ParentID: &parent, ParentName: new("Parent"), Name: "Old", SortOrder: 1},
				After:  &domain.ChangePlace{ParentID: &parent, ParentName: new("Parent"), Name: "Notes", SortOrder: 1}},
			{NodeID: id(13), NodeKind: domain.KindPage, Name: "Gone", Content: &domain.ChangeVersion{Base: &base, Revision: 3}},
		},
	}
}

// plainChangeset is a creation with no message, no revert and no change.
func plainChangeset() app.LoggedChangeset {
	return app.LoggedChangeset{Changeset: domain.Changeset{ID: id(53), NotebookID: id(10), Kind: domain.ChangesetEdit,
		Client: domain.ClientWeb, By: id(2), ByName: "Bob", CreatedAt: at(), UpdatedAt: at()}}
}

const (
	renameJSON = `{"after":{"name":"Notes","parent_id":"0199a2b4-0000-7000-8000-000000000011","parent_name":"Parent"},` +
		`"before":{"name":"Old","parent_id":"0199a2b4-0000-7000-8000-000000000011","parent_name":"Parent"},"content":null,` +
		`"kind":"renamed","node":{"alive":true,"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Notes"}}`
	writeJSON = `{"after":null,"before":null,"content":{"base_revision":2,"revision":3},"kind":"content","node":{"alive":false,` +
		`"id":"0199a2b4-0000-7000-8000-000000000013","kind":"page","name":"Gone"}}`
	changesetFields = `"changes":[` + renameJSON + `,` + writeJSON + `],"client":"mcp:a","content_count":1,` +
		`"created_at":"2026-10-02T10:00:00Z","created_by":{"display_name":"Alice","user_id":"0199a2b4-0000-7000-8000-000000000001"},` +
		`"id":"0199a2b4-0000-7000-8000-000000000050","kind":"revert","message":"Why","node_count":1,` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","reverted_by":["0199a2b4-0000-7000-8000-000000000052"],` +
		`"reverts":{"created_at":"2026-10-02T10:00:00Z","created_by":{"display_name":"Bob","user_id":` +
		`"0199a2b4-0000-7000-8000-000000000002"},"id":"0199a2b4-0000-7000-8000-000000000051","kind":"edit"},"trimmed":false,` +
		`"updated_at":"2026-10-02T10:01:00Z"`
	loggedJSON = `{` + changesetFields + `}`
	plainJSON  = `{"changes":[],"client":"web","content_count":0,"created_at":"2026-10-02T10:00:00Z","created_by":` +
		`{"display_name":"Bob","user_id":"0199a2b4-0000-7000-8000-000000000002"},"id":"0199a2b4-0000-7000-8000-000000000053",` +
		`"kind":"edit","message":null,"node_count":0,"notebook_id":"0199a2b4-0000-7000-8000-000000000010","reverted_by":[],` +
		`"reverts":null,"trimmed":false,"updated_at":"2026-10-02T10:00:00Z"}`
)

// Execute answers the two changesets and the next cursor "next", but for
// the cursor "end", none.
func (f fakeChangesets) Execute(_ context.Context, notebookID uuid.UUID, q app.LogQuery) (app.ChangesetList, error) {
	f.got = []any{notebookID, q}
	if q.Cursor != nil && *q.Cursor == "end" {
		return app.ChangesetList{}, f.err
	}
	return app.ChangesetList{Changesets: []app.LoggedChangeset{loggedChangeset(), plainChangeset()}, NextCursor: "next"}, f.err
}

// Execute answers the changeset, renaming and not moving.
func (f fakeChangeset) Execute(_ context.Context, id uuid.UUID) (app.ChangesetDetail, error) {
	f.got = []any{id}
	return app.ChangesetDetail{LoggedChangeset: loggedChangeset(), Moves: domain.ChangesetMoves{Renames: true}}, f.err
}

// Execute answers the changeset's two changes, and no next cursor.
func (f fakeChanges) Execute(_ context.Context, id uuid.UUID, limit *int, cursor *string) (app.ChangeList, error) {
	f.got = []any{id, limit, cursor}
	return app.ChangeList{Changes: loggedChangeset().Changes}, f.err
}

// Execute answers a revert's changeset, but for the changeset 54, nothing
// changed.
func (f fakeRevertChangeset) Execute(_ context.Context, changesetID uuid.UUID, message *string, client domain.Client) (
	app.ChangesetReverted, error,
) {
	f.got = []any{changesetID, message, client}
	if changesetID == id(54) {
		return app.ChangesetReverted{}, f.err
	}
	return app.ChangesetReverted{ChangesetID: id(55), Changed: true}, f.err
}

const (
	changesetsPath = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/changesets"
	changesetPath  = "/api/v0/changesets/0199a2b4-0000-7000-8000-000000000050"
	changesPath    = changesetPath + "/changes"
	revertPath     = changesetPath + "/revert"
)

func TestTheHistoryOperationsAnswerTheUseCases(t *testing.T) {
	limit, cursor, end, mcp, why := 2, "abc", "end", "mcp", "Why"
	actor, page := id(1), id(12)
	for _, tt := range []struct {
		name, token, method, path, body string
		want                            string
		got                             []any
	}{
		{"a notebook's history", "session", http.MethodGet, changesetsPath, "", `{"data":[` + loggedJSON + `,` + plainJSON +
			`],"next_cursor":"next"}`, []any{id(10), app.LogQuery{}}},
		{"a page of it, filtered", "session", http.MethodGet, changesetsPath + "?actor=0199a2b4-0000-7000-8000-000000000001&client=mcp" +
			"&page_id=0199a2b4-0000-7000-8000-000000000012&limit=2&cursor=abc", "", `{"data":[` + loggedJSON + `,` + plainJSON +
			`],"next_cursor":"next"}`, []any{id(10), app.LogQuery{Actor: &actor, Client: &mcp, PageID: &page, Limit: &limit, Cursor: &cursor}}},
		{"none left", "session", http.MethodGet, changesetsPath + "?cursor=end", "", `{"data":[],"next_cursor":null}`,
			[]any{id(10), app.LogQuery{Cursor: &end}}},
		{"a changeset", "session", http.MethodGet, changesetPath, "", `{` + strings.Replace(strings.Replace(changesetFields,
			`"node_count"`, `"moves":false,"node_count"`, 1), `"reverted_by"`, `"renames":true,"reverted_by"`, 1) + `}`, []any{id(50)}},
		{"its changes", "session", http.MethodGet, changesPath + "?limit=2&cursor=abc", "", `{"data":[` + renameJSON + `,` + writeJSON +
			`],"next_cursor":null}`, []any{id(50), &limit, &cursor}},
		{"a revert by the web, with no body", "session", http.MethodPost, revertPath, "",
			`{"changed":true,"changeset_id":"0199a2b4-0000-7000-8000-000000000055"}`, []any{id(50), (*string)(nil), domain.ClientWeb}},
		{"a revert by the API, with a message", "pat", http.MethodPost, revertPath, `{"message":"Why"}`,
			`{"changed":true,"changeset_id":"0199a2b4-0000-7000-8000-000000000055"}`, []any{id(50), &why, domain.ClientAPI}},
		{"a revert with an empty object", "session", http.MethodPost, revertPath, `{}`,
			`{"changed":true,"changeset_id":"0199a2b4-0000-7000-8000-000000000055"}`, []any{id(50), (*string)(nil), domain.ClientWeb}},
		{"a revert that changed nothing", "session", http.MethodPost, "/api/v0/changesets/0199a2b4-0000-7000-8000-000000000054/revert",
			"", `{"changed":false,"changeset_id":null}`, []any{id(54), (*string)(nil), domain.ClientWeb}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{}
			status, body := call(t, f.serve(t), tt.token, tt.method, tt.path, tt.body)
			if status != http.StatusOK || body != tt.want {
				t.Errorf("%s %s = %d %s\nwant 200 %s", tt.method, tt.path, status, body, tt.want)
			}
			if !reflect.DeepEqual(f.got, tt.got) {
				t.Errorf("the use case got %v, want %v", f.got, tt.got)
			}
		})
	}
}

// Every code the history's operations declare, as the use cases answer
// it; conflicts and conflict_count are page.revert_conflict's alone.
func TestTheHistoryOperationsAnswerEachProblem(t *testing.T) {
	busy := shared.ServerBusy(time.Second)
	holder := id(13)
	conflict := domain.RevertConflicts([]domain.RevertConflict{{NodeID: id(12), Name: "Old", Reason: domain.ConflictTitleTaken,
		HolderID: &holder}}, 3)
	for _, tt := range []struct {
		method, path string
		err          error
		status       int
		code         string
	}{
		{http.MethodGet, changesetsPath, shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{http.MethodGet, changesetsPath, domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodGet, changesetsPath, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, changesetsPath, shared.Invalid(shared.FieldError{Field: "client", Code: shared.FieldNotAllowed,
			Message: "must be web, api, cli, mcp or mcp:<name>"}), http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodGet, changesetPath, domain.ErrChangesetNotFound, http.StatusNotFound, "page.changeset_not_found"},
		{http.MethodGet, changesPath, shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{http.MethodGet, changesPath, domain.ErrChangesetNotFound, http.StatusNotFound, "page.changeset_not_found"},
		{http.MethodGet, changesPath, shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: "must be between 1 and 100"}), http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPost, revertPath, shared.Invalid(shared.FieldError{Field: "message", Code: shared.FieldTooShort,
			Message: "must not be empty"}), http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodPost, revertPath, domain.ErrChangesetNotFound, http.StatusNotFound, "page.changeset_not_found"},
		{http.MethodPost, revertPath, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, revertPath, domain.ErrChangesetTrimmed, http.StatusConflict, "page.changeset_trimmed"},
		{http.MethodPost, revertPath, domain.ErrRevertTooLarge, http.StatusUnprocessableEntity, "page.revert_too_large"},
		{http.MethodPost, revertPath, conflict, http.StatusConflict, "page.revert_conflict"},
		{http.MethodPost, revertPath, domain.Refuse(domain.ErrProtected, id(12)), http.StatusConflict, "page.protected"},
		{http.MethodPost, revertPath, domain.Refuse(domain.ErrSourceAdminOnly, id(12)), http.StatusForbidden, "page.source_admin_only"},
		{http.MethodPost, revertPath, domain.Refuse(domain.ErrSourceTooLarge, id(12)), http.StatusUnprocessableEntity, "page.source_too_large"},
		{http.MethodPost, revertPath, domain.Refuse(shared.NewError(shared.KindConflict, "page.source_immutable", "Sources are immutable."), id(12)), http.StatusConflict, "page.source_immutable"},
		{http.MethodPost, revertPath, busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodPost, revertPath, domain.Locked(id(12), id(2), "Bob"), http.StatusConflict, "page.locked"},
	} {
		status, body := call(t, (&fakes{err: tt.err}).serve(t), "session", tt.method, tt.path, "")
		conflicts := strings.Contains(body, `"conflicts":[{"node_id":"0199a2b4-0000-7000-8000-000000000012","name":"Old",`+
			`"reason":"title_taken","holder_id":"0199a2b4-0000-7000-8000-000000000013"}],"conflict_count":3`)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) || conflicts != (tt.code == "page.revert_conflict") ||
			strings.Contains(body, "conflict_count") != conflicts {
			t.Errorf("%s %s: %d %s, want %d %s, conflicts on page.revert_conflict alone", tt.method, tt.path, status, body, tt.status,
				tt.code)
		}
	}
}

// A path's ids and the actor and page filters are uuids, a revert's body
// an object of a message: anything else is the boundary's 400, before any
// use case.
func TestAHistoryRequestTheBoundaryRefuses(t *testing.T) {
	for _, tt := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v0/notebooks/notes/changesets", ""},
		{http.MethodGet, changesetsPath + "?actor=alice", ""},
		{http.MethodGet, changesetsPath + "?page_id=notes", ""},
		{http.MethodGet, "/api/v0/changesets/notes", ""},
		{http.MethodGet, "/api/v0/changesets/notes/changes", ""},
		{http.MethodGet, changesPath + "?limit=x", ""},
		{http.MethodPost, "/api/v0/changesets/notes/revert", ""},
		{http.MethodPost, revertPath, `{"message":1}`},
		{http.MethodPost, revertPath, `{"message":"Why","force":true}`},
		{http.MethodPost, revertPath, `["Why"]`},
	} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", tt.method, tt.path, tt.body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s %s %s = %d %s, use case got %v; want 400 bad_request, the use case not called", tt.method, tt.path, tt.body,
				status, answer, f.got)
		}
	}
}
