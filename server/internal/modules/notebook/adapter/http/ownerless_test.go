package httpadapter_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/app"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook/domain"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// aliceID is the former owner the fakes answer.
func aliceID() uuid.UUID { return uuid.MustParse("0199a2b4-0000-7000-8000-00000000000e") }

func ownerlessEngineering() app.OwnerlessNotebook {
	at := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	v := view(shared.NotebookAdmin)
	v.Notebook.Ownerless = &domain.Ownerless{Since: at, FormerOwner: aliceID()}
	return app.OwnerlessNotebook{
		Notebook: v.Notebook, MemberCount: 2, FormerOwner: app.Profile{DisplayName: "Alice", Email: "alice@corp.com"},
		SizeBytes: 1024, LastActivityAt: at.Add(time.Hour),
	}
}

const (
	aliceJSON      = `{"display_name":"Alice","email":"alice@corp.com","user_id":"0199a2b4-0000-7000-8000-00000000000e"}`
	bobProfileJSON = `{"display_name":"Bob","email":"bob@corp.com","user_id":"0199a2b4-0000-7000-8000-00000000000d"}`
	ownerlessJSON  = `{"former_owner":` + aliceJSON + `,"id":"0199a2b4-0000-7000-8000-00000000000b","last_activity_at":"2026-10-02T11:00:00Z",` +
		`"member_count":2,"name":"Engineering","ownerless_since":"2026-10-02T10:00:00Z","size_bytes":1024,"workspace_access":"viewer"}`
	eventJSON = `{"action":"taken_over","actor":` + bobProfileJSON + `,"created_at":"2026-10-02T12:00:00Z","former_owner":` + aliceJSON +
		`,"id":"0199a2b4-0000-7000-8000-00000000000f","notebook_id":"0199a2b4-0000-7000-8000-00000000000b","notebook_name":"Engineering"}`
)

type fakeListOwnerless struct{ fakeUseCase }

func (f *fakeListOwnerless) Execute(_ context.Context, slug string) ([]app.OwnerlessNotebook, error) {
	f.got = []any{slug}
	if f.err != nil {
		return nil, f.err
	}
	return []app.OwnerlessNotebook{ownerlessEngineering()}, nil
}

type fakeTakeOver struct{ fakeUseCase }

func (f *fakeTakeOver) Execute(_ context.Context, id uuid.UUID) (app.View, error) {
	return f.answer(shared.NotebookAdmin, id)
}

type fakeDeleteOwnerless struct{ fakeUseCase }

func (f *fakeDeleteOwnerless) Execute(_ context.Context, id uuid.UUID) error {
	f.got = []any{id}
	return f.err
}

// fakeListAudit answers one event, and next as the next page's cursor.
type fakeListAudit struct {
	fakeUseCase
	next string
}

func (f *fakeListAudit) Execute(_ context.Context, slug string, limit *int, cursor *string) (app.AuditPage, error) {
	f.got = []any{slug, limit, cursor}
	if f.err != nil {
		return app.AuditPage{}, f.err
	}
	e := domain.AuditEvent{
		ID: uuid.MustParse("0199a2b4-0000-7000-8000-00000000000f"), NotebookID: notebookID(), NotebookName: "Engineering",
		Action: domain.AuditTakenOver, FormerOwnerID: aliceID(), ActorID: bobID(), At: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC),
	}
	return app.AuditPage{Events: []app.ListedAuditEvent{{
		Event: e, FormerOwner: app.Profile{DisplayName: "Alice", Email: "alice@corp.com"}, Actor: app.Profile{DisplayName: "Bob", Email: "bob@corp.com"},
	}}, NextCursor: f.next}, nil
}

// ownerlessFakes are the ownerless operations' use cases, every one
// answering err.
type ownerlessFakes struct {
	listOwnerless   *fakeListOwnerless
	takeOver        *fakeTakeOver
	deleteOwnerless *fakeDeleteOwnerless
	listAudit       *fakeListAudit
}

func newOwnerlessFakes(err error) ownerlessFakes {
	u := fakeUseCase{err: err}
	return ownerlessFakes{&fakeListOwnerless{u}, &fakeTakeOver{u}, &fakeDeleteOwnerless{u}, &fakeListAudit{fakeUseCase: u}}
}

const (
	ownerlessListPath = "/api/v0/workspaces/acme/ownerless-notebooks"
	ownerlessPath     = "/api/v0/ownerless-notebooks/0199a2b4-0000-7000-8000-00000000000b"
	auditPath         = "/api/v0/workspaces/acme/notebook-audit-events"
)

func TestTheOwnerlessOperationsAnswerTheUseCases(t *testing.T) {
	f := newFakes(nil)
	h := f.serve(t)
	two, cursor := 2, "eyJ2IjoxfQ"
	for _, tt := range []struct {
		next, method, path string
		status             int
		want               string
		got                func() []any
		wantGot            []any
	}{
		{"", http.MethodGet, ownerlessListPath, http.StatusOK, `{"data":[` + ownerlessJSON + "]}", func() []any { return f.listOwnerless.got },
			[]any{"acme"}},
		{"", http.MethodPost, ownerlessPath + "/take-over", http.StatusOK, engineering("admin"), func() []any { return f.takeOver.got },
			[]any{notebookID()}},
		{"", http.MethodDelete, ownerlessPath, http.StatusNoContent, "", func() []any { return f.deleteOwnerless.got }, []any{notebookID()}},
		{"", http.MethodGet, auditPath, http.StatusOK, `{"data":[` + eventJSON + `],"next_cursor":null}`, func() []any { return f.listAudit.got },
			[]any{"acme", (*int)(nil), (*string)(nil)}},
		{"next", http.MethodGet, auditPath + "?limit=2&cursor=" + cursor, http.StatusOK, `{"data":[` + eventJSON + `],"next_cursor":"next"}`,
			func() []any { return f.listAudit.got }, []any{"acme", &two, &cursor}},
	} {
		f.listAudit.next = tt.next
		status, body := call(t, h, tt.method, tt.path, "")
		want := tt.want
		if want != "" {
			want += "\n"
		}
		if status != tt.status || body != want {
			t.Errorf("%s %s = %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, want)
		}
		if got := tt.got(); !sameAuditArgs(got, tt.wantGot) {
			t.Errorf("%s %s: the use case got %v, want %v", tt.method, tt.path, got, tt.wantGot)
		}
	}
}

// sameAuditArgs is sameArgs, with the limit behind its pointer.
func sameAuditArgs(a, b []any) bool {
	if len(a) == 3 && len(b) == 3 {
		x, _ := a[1].(*int)
		y, _ := b[1].(*int)
		if (x == nil) != (y == nil) || x != nil && *x != *y {
			return false
		}
		a, b = []any{a[0], a[2]}, []any{b[0], b[2]}
	}
	return sameArgs(a, b)
}

// Every code the ownerless operations declare, as the use cases answer it.
func TestTheOwnerlessOperationsAnswerEachProblem(t *testing.T) {
	invalid := shared.Invalid(shared.FieldError{Field: "limit", Code: shared.FieldOutOfRange, Message: "must be between 1 and 100"})
	for _, tt := range []struct {
		method, path string
		err          error
		status       int
		code         string
	}{
		{http.MethodGet, ownerlessListPath, domain.ErrWorkspaceNotFound, http.StatusNotFound, "workspace.not_found"},
		{http.MethodGet, ownerlessListPath, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodPost, ownerlessPath + "/take-over", domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodDelete, ownerlessPath, domain.ErrNotFound, http.StatusNotFound, "notebook.not_found"},
		{http.MethodGet, auditPath + "?cursor=x", shared.InvalidCursor(), http.StatusBadRequest, "bad_request"},
		{http.MethodGet, auditPath, domain.ErrWorkspaceNotFound, http.StatusNotFound, "workspace.not_found"},
		{http.MethodGet, auditPath, shared.Forbidden(), http.StatusForbidden, "forbidden"},
		{http.MethodGet, auditPath + "?limit=0", invalid, http.StatusUnprocessableEntity, "validation_failed"},
	} {
		status, body := call(t, newFakes(tt.err).serve(t), tt.method, tt.path, "")
		if status != tt.status || !strings.Contains(body, `"code":"`+tt.code+`"`) {
			t.Errorf("%s %s: %d %s, want %d %s", tt.method, tt.path, status, body, tt.status, tt.code)
		}
	}
}

// A limit that is no integer is the boundary's 400, before the use case.
func TestALimitThatIsNoInteger(t *testing.T) {
	f := newFakes(nil)
	if status, body := call(t, f.serve(t), http.MethodGet, auditPath+"?limit=ten", ""); status != http.StatusBadRequest ||
		!strings.Contains(body, `"code":"bad_request"`) || f.listAudit.got != nil {
		t.Errorf("GET = %d %s, use case got %v; want 400 bad_request, the use case not called", status, body, f.listAudit.got)
	}
}
