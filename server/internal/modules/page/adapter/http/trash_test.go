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
	fakeTrash       struct{ *fakes }
	fakeTrashItem   struct{ *fakes }
	fakeRestoreNode struct{ *fakes }
)

// trashItem is the item the fakes answer: Doc under P / Q, deleted by
// Alice with a page and an attachment under it, back under Q.
func trashItem() app.TrashItem {
	q := id(41)
	return app.TrashItem{
		Root: domain.TrashRoot{Node: domain.Node{ID: id(40), NotebookID: id(10), ParentID: &q, Kind: domain.KindPage, Name: "Doc",
			UpdatedBy: id(1)}, DeletedAt: at(), Pages: 2, Assets: 1, Height: 2, Revision: 6},
		Ancestors: []domain.Ancestor{{ID: id(42), Name: "P"}, {ID: q, Name: "Q"}}, DeletedByName: "Alice",
		RestorableUntil: at().Add(1439 * time.Hour), Placement: domain.Placement{ParentID: &q, Reason: domain.PlacedOriginal},
	}
}

// attachmentItem is an attachment's item: photo.png at the root, its
// parent in the trash.
func attachmentItem() app.TrashItem {
	return app.TrashItem{
		Root: domain.TrashRoot{Node: domain.Node{ID: id(43), NotebookID: id(10), Kind: domain.KindAsset, Name: "photo.png",
			UpdatedBy: id(2)}, DeletedAt: at(), Assets: 1},
		Ancestors: []domain.Ancestor{{ID: id(44), Name: "Gone"}}, DeletedByName: "Bob", RestorableUntil: at().Add(1439 * time.Hour),
		Placement: domain.Placement{Reason: domain.PlacedParentInTrash}, Asset: &app.AttachmentInfo{MIME: "image/png", Bytes: 70},
	}
}

const (
	trashItemJSON = `{"ancestors":[{"id":"0199a2b4-0000-7000-8000-000000000042","name":"P"},{"id":"0199a2b4-0000-7000-8000-000000000041",` +
		`"name":"Q"}],"asset":null,"assets":1,"deleted_at":"2026-10-02T10:00:00Z","deleted_by":{"display_name":"Alice","user_id":` +
		`"0199a2b4-0000-7000-8000-000000000001"},"id":"0199a2b4-0000-7000-8000-000000000040","kind":"page","name":"Doc",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","pages":2,"restorable_until":"2026-12-01T09:00:00Z","restore_to":` +
		`{"parent_id":"0199a2b4-0000-7000-8000-000000000041","reason":"original"},"revision":6}`
	attachmentItemJSON = `{"ancestors":[{"id":"0199a2b4-0000-7000-8000-000000000044","name":"Gone"}],"asset":{"byte_size":70,` +
		`"mime":"image/png"},"assets":1,"deleted_at":"2026-10-02T10:00:00Z","deleted_by":{"display_name":"Bob","user_id":` +
		`"0199a2b4-0000-7000-8000-000000000002"},"id":"0199a2b4-0000-7000-8000-000000000043","kind":"asset","name":"photo.png",` +
		`"notebook_id":"0199a2b4-0000-7000-8000-000000000010","pages":0,"restorable_until":"2026-12-01T09:00:00Z","restore_to":` +
		`{"parent_id":null,"reason":"parent_in_trash"},"revision":null}`
)

// Execute answers the two items and the next cursor "next", but for the
// cursor "end", none.
func (f fakeTrash) Execute(_ context.Context, notebookID uuid.UUID, limit *int, cursor *string) (app.TrashList, error) {
	f.got = []any{notebookID, limit, cursor}
	if cursor != nil && *cursor == "end" {
		return app.TrashList{}, f.err
	}
	return app.TrashList{Items: []app.TrashItem{trashItem(), attachmentItem()}, NextCursor: "next"}, f.err
}

// Execute answers the page's item, but for the node 43, the attachment's.
func (f fakeTrashItem) Execute(_ context.Context, nodeID uuid.UUID) (app.TrashItem, error) {
	f.got = []any{nodeID}
	if nodeID == id(43) {
		return attachmentItem(), f.err
	}
	return trashItem(), f.err
}

// Execute answers Doc back at the root, numbered, its parent in the trash.
func (f fakeRestoreNode) Execute(_ context.Context, nodeID uuid.UUID, client domain.Client) (app.NodeRestored, error) {
	f.got = []any{nodeID, client}
	return app.NodeRestored{Node: domain.Node{ID: nodeID, Kind: domain.KindPage, Name: "Doc 2"}, Renamed: true,
		Placement: domain.Placement{Reason: domain.PlacedParentInTrash}, ChangesetID: id(45)}, f.err
}

const (
	trashPath       = "/api/v0/notebooks/0199a2b4-0000-7000-8000-000000000010/trash"
	trashItemPath   = nodePath + "/trash-item"
	restoreNodePath = nodePath + "/restore"
)

func TestTheTrashOperationsAnswerTheUseCases(t *testing.T) {
	limit, cursor, end := 2, "abc", "end"
	for _, tt := range []struct {
		name, token, method, path string
		want                      string
		got                       []any
	}{
		{"a notebook's trash", "session", http.MethodGet, trashPath, `{"data":[` + trashItemJSON + `,` + attachmentItemJSON +
			`],"next_cursor":"next"}`, []any{id(10), (*int)(nil), (*string)(nil)}},
		{"a page of it", "session", http.MethodGet, trashPath + "?limit=2&cursor=abc", `{"data":[` + trashItemJSON + `,` +
			attachmentItemJSON + `],"next_cursor":"next"}`, []any{id(10), &limit, &cursor}},
		{"none left", "session", http.MethodGet, trashPath + "?cursor=end", `{"data":[],"next_cursor":null}`,
			[]any{id(10), (*int)(nil), &end}},
		{"a page's item", "session", http.MethodGet, trashItemPath, trashItemJSON, []any{id(12)}},
		{"an attachment's item", "session", http.MethodGet, "/api/v0/nodes/0199a2b4-0000-7000-8000-000000000043/trash-item",
			attachmentItemJSON, []any{id(43)}},
		{"a restore by the web", "session", http.MethodPost, restoreNodePath, `{"changeset_id":"0199a2b4-0000-7000-8000-000000000045",` +
			`"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Doc 2","placed":{"parent_id":null,"reason":` +
			`"parent_in_trash"},"renamed":true}`, []any{id(12), domain.ClientWeb}},
		{"a restore by the API", "pat", http.MethodPost, restoreNodePath, `{"changeset_id":"0199a2b4-0000-7000-8000-000000000045",` +
			`"id":"0199a2b4-0000-7000-8000-000000000012","kind":"page","name":"Doc 2","placed":{"parent_id":null,"reason":` +
			`"parent_in_trash"},"renamed":true}`, []any{id(12), domain.ClientAPI}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{}
			status, body := call(t, f.serve(t), tt.token, tt.method, tt.path, "")
			if status != http.StatusOK || body != tt.want {
				t.Errorf("%s %s = %d %s, want 200 %s", tt.method, tt.path, status, body, tt.want)
			}
			if !reflect.DeepEqual(f.got, tt.got) {
				t.Errorf("the use case got %v, want %v", f.got, tt.got)
			}
		})
	}
}

// Every code the trash operations declare, as the use cases answer it;
// root_id is page.not_trash_root's alone.
func TestTheTrashOperationsAnswerEachProblem(t *testing.T) {
	busy := shared.ServerBusy(time.Second)
	for _, tt := range []struct {
		method, path string
		err          error
		status       int
		code         string
	}{
		{http.MethodGet, trashPath, shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{http.MethodGet, trashPath, domain.ErrNotebookNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodGet, trashPath, shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange,
			Message: "must be between 1 and 100"}), http.StatusUnprocessableEntity, "validation_failed"},
		{http.MethodGet, trashItemPath, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodGet, trashItemPath, domain.ErrNotInTrash, http.StatusConflict, "page.not_in_trash"},
		{http.MethodGet, trashItemPath, domain.NotTrashRoot(id(40)), http.StatusConflict, "page.not_trash_root"},
		{http.MethodGet, trashItemPath, domain.ErrRestoreExpired, http.StatusConflict, "page.restore_expired"},
		{http.MethodPost, restoreNodePath, domain.ErrNotFound, http.StatusNotFound, "page.not_found"},
		{http.MethodPost, restoreNodePath, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, restoreNodePath, domain.ErrNotInTrash, http.StatusConflict, "page.not_in_trash"},
		{http.MethodPost, restoreNodePath, domain.NotTrashRoot(id(40)), http.StatusConflict, "page.not_trash_root"},
		{http.MethodPost, restoreNodePath, domain.ErrRestoreExpired, http.StatusConflict, "page.restore_expired"},
		{http.MethodPost, restoreNodePath, domain.ErrRestoreTooLarge, http.StatusUnprocessableEntity, "page.restore_too_large"},
		{http.MethodPost, restoreNodePath, busy, http.StatusServiceUnavailable, "server_busy"},
		{http.MethodPost, restoreNodePath, domain.ErrTrashChanged, http.StatusConflict, "page.trash_changed"},
	} {
		status, body := call(t, (&fakes{err: tt.err}).serve(t), "session", tt.method, tt.path, "")
		root := strings.Contains(body, `"root_id":"0199a2b4-0000-7000-8000-000000000040"`)
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) || root != (tt.code == "page.not_trash_root") ||
			strings.Count(body, "root_id") > 1 {
			t.Errorf("%s %s: %d %s, want %d %s, root_id on page.not_trash_root alone", tt.method, tt.path, status, body, tt.status,
				tt.code)
		}
	}
}

// A path's ids are uuids and a restore takes no body: anything else is the
// boundary's 400, before any use case.
func TestATrashRequestTheBoundaryRefuses(t *testing.T) {
	for _, tt := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v0/notebooks/notes/trash", ""},
		{http.MethodGet, trashPath + "?limit=x", ""},
		{http.MethodGet, "/api/v0/nodes/notes/trash-item", ""},
		{http.MethodPost, "/api/v0/nodes/notes/restore", ""},
	} {
		f := &fakes{}
		status, answer := call(t, f.serve(t), "session", tt.method, tt.path, tt.body)
		if status != http.StatusBadRequest || !strings.Contains(answer, `"code":"bad_request"`) || f.got != nil {
			t.Errorf("%s %s %s = %d %s, use case got %v; want 400 bad_request, the use case not called", tt.method, tt.path, tt.body,
				status, answer, f.got)
		}
	}
}
